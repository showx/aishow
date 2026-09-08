"""FastH3 GGUF sidecar for Aishow.

Same /v1/videos job API as fasth3_server.py, but submits a ComfyUI graph
(Q4 DiT + quantized Qwen3-VL + embedded VSA) instead of loading FastVideo.
"""
from __future__ import annotations

import json
import os
import queue
import re
import shutil
import threading
import time
import traceback
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

HOST = os.environ.get("FASTH3_HOST", "127.0.0.1")
PORT = int(os.environ.get("FASTH3_PORT", "8000"))
COMFY = os.environ.get("FASTH3_COMFY_URL", "http://127.0.0.1:8188").rstrip("/")
OUT_DIR = Path(os.environ.get("FASTH3_OUT_DIR", r"F:\models\aishow-fasth3-out"))
COMFY_OUTPUT = Path(os.environ.get(
    "FASTH3_COMFY_OUTPUT",
    r"E:\MiniMax-H3\ComfyUI_windows_portable\ComfyUI\output",
))

UNET = os.environ.get("FASTH3_GGUF_UNET", "FastH3-comfy-Q4_K_M.gguf")
VIDEO_VAE = os.environ.get("FASTH3_VIDEO_VAE", "minimax_h3_video_vae_fp16.safetensors")
AUDIO_VAE = os.environ.get("FASTH3_AUDIO_VAE", "minimax_h3_audio_vae_fp32.safetensors")
TOPK = float(os.environ.get("FASTH3_VSA_TOPK", "0.10"))
SHIFT_VIDEO = float(os.environ.get("FASTH3_SHIFT_VIDEO", "12"))
SHIFT_AUDIO = float(os.environ.get("FASTH3_SHIFT_AUDIO", "3"))
CLIP_DIRS = [
    Path(r"E:\MiniMax-H3\ComfyUI_windows_portable\ComfyUI\models\text_encoders"),
    Path(r"F:\models\fasth3-gguf\text_encoders"),
]
# Unsloth llama.cpp GGUF has no architecture tag; ComfyUI-GGUF rejects it.
# Prefer Comfy-Org quantized safetensors that CLIPLoader already sees.
CLIP_CANDIDATES = [
    ("qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors", "CLIPLoader"),
    ("qwen3vl_32b_minimax_h3_int8_convrot.safetensors", "CLIPLoader"),
]


def resolve_clip() -> tuple[str, str]:
    forced = os.environ.get("FASTH3_GGUF_CLIP", "").strip()
    if forced:
        loader = "CLIPLoaderGGUF" if forced.lower().endswith(".gguf") else "CLIPLoader"
        return forced, loader
    for name, loader in CLIP_CANDIDATES:
        if any((folder / name).exists() for folder in CLIP_DIRS):
            return name, loader
    raise RuntimeError(
        "找不到 ComfyUI 可用的 Qwen3-VL 文本编码器。"
        "请把 Comfy-Org 的 qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors "
        "放到 ComfyUI/models/text_encoders。"
    )


CLIP, CLIP_LOADER = resolve_clip()

JOBS: dict[str, dict] = {}
LOCK = threading.Lock()
INFER_Q: queue.Queue[str] = queue.Queue()


class JobCancelled(Exception):
    pass


def request_cancel(job_id: str) -> bool:
    with LOCK:
        job = JOBS.get(job_id)
        if not job:
            return False
        job["cancel"] = True
        prompt_id = job.get("prompt_id")
        if job.get("status") in ("queued", "in_progress"):
            job["status"] = "cancelled"
            job["error"] = {"message": "已取消"}
    if prompt_id:
        interrupt_comfy(prompt_id)
    return True


def round32(n: int) -> int:
    return max(32, int(n) // 32 * 32)


def align_frames(n: int) -> int:
    n = max(int(n), 1)
    while n % 17 != 5:
        n += 1
    return min(n, 345)


def parse_size(size: str | None, seconds: float) -> tuple[int, int, int]:
    width, height = 1344, 768
    if size and "x" in size.lower():
        a, b = size.lower().split("x", 1)
        try:
            width, height = round32(int(a)), round32(int(b))
        except ValueError:
            pass
    frames = align_frames(int(round(max(seconds, 1) * 24)))
    return width, height, frames


def http_json(method: str, url: str, payload=None, timeout: float = 30):
    data = None
    headers = {"Accept": "application/json"}
    if payload is not None:
        data = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = Request(url, data=data, headers=headers, method=method)
    try:
        with urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            if not raw:
                return {}
            return json.loads(raw.decode("utf-8"))
    except HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"ComfyUI {method} {url} HTTP {exc.code}: {body[:800]}") from exc
    except URLError as exc:
        raise RuntimeError(f"连不上 ComfyUI {url}: {exc.reason}") from exc


def comfy_ready() -> bool:
    try:
        http_json("GET", f"{COMFY}/system_stats", timeout=3)
        return True
    except Exception:
        return False


def interrupt_comfy(prompt_id: str | None = None) -> None:
    try:
        if prompt_id:
            http_json("POST", f"{COMFY}/queue", {"delete": [prompt_id]}, timeout=8)
    except Exception as exc:
        print(f"[fasth3-gguf] queue delete skipped: {exc}", flush=True)
    try:
        http_json("POST", f"{COMFY}/interrupt", {}, timeout=8)
    except Exception as exc:
        print(f"[fasth3-gguf] interrupt skipped: {exc}", flush=True)


def build_prompt(job_id: str, prompt: str, width: int, height: int, frames: int, seed: int, steps: int) -> dict:
    prefix = f"aishow/{job_id}"
    return {
        "1": {"class_type": "UnetLoaderGGUF", "inputs": {"unet_name": UNET}},
        "2": {"class_type": CLIP_LOADER, "inputs": {"clip_name": CLIP, "type": "minimax", "device": "default"}},
        "3": {"class_type": "VAELoader", "inputs": {"vae_name": VIDEO_VAE}},
        "4": {"class_type": "VAELoader", "inputs": {"vae_name": AUDIO_VAE}},
        "5": {
            "class_type": "MiniMaxH3SigmaShift",
            "inputs": {"model": ["1", 0], "shift_video": SHIFT_VIDEO, "shift_audio": SHIFT_AUDIO},
        },
        "6": {
            "class_type": "H3VSAEmbedded",
            "inputs": {
                "model": ["5", 0],
                "topk_ratio": TOPK,
                "min_tokens": 0,
                "keep_gates_on_gpu": False,
            },
        },
        "7": {
            "class_type": "MiniMaxH3ImageToVideo",
            "inputs": {
                "clip": ["2", 0],
                "vae": ["3", 0],
                "prompt": prompt,
                "width": width,
                "height": height,
                "length": frames,
            },
        },
        "8": {
            "class_type": "BasicGuider",
            "inputs": {"model": ["6", 0], "conditioning": ["7", 0]},
        },
        "9": {"class_type": "KSamplerSelect", "inputs": {"sampler_name": "euler"}},
        "10": {
            "class_type": "BasicScheduler",
            "inputs": {"model": ["6", 0], "scheduler": "simple", "steps": steps, "denoise": 1.0},
        },
        "11": {"class_type": "RandomNoise", "inputs": {"noise_seed": seed & 0xFFFFFFFFFFFFFFFF}},
        "12": {
            "class_type": "SamplerCustomAdvanced",
            "inputs": {
                "noise": ["11", 0],
                "guider": ["8", 0],
                "sampler": ["9", 0],
                "sigmas": ["10", 0],
                "latent_image": ["7", 1],
            },
        },
        "13": {"class_type": "VAEDecode", "inputs": {"samples": ["12", 0], "vae": ["3", 0]}},
        "14": {"class_type": "VAEDecodeAudio", "inputs": {"samples": ["12", 0], "vae": ["4", 0]}},
        "15": {
            "class_type": "CreateVideo",
            "inputs": {"images": ["13", 0], "audio": ["14", 0], "fps": 24.0, "bit_depth": 8},
        },
        "16": {
            "class_type": "SaveVideo",
            "inputs": {
                "video": ["15", 0],
                "filename_prefix": prefix,
                "format": "mp4",
                "codec": "auto",
            },
        },
    }


def submit_prompt(graph: dict, client_id: str) -> str:
    body = http_json("POST", f"{COMFY}/prompt", {"prompt": graph, "client_id": client_id}, timeout=60)
    if body.get("error") or body.get("node_errors"):
        raise RuntimeError(f"ComfyUI 拒绝工作流: {json.dumps(body, ensure_ascii=False)[:1200]}")
    prompt_id = body.get("prompt_id")
    if not prompt_id:
        raise RuntimeError(f"ComfyUI 未返回 prompt_id: {body}")
    return prompt_id


def history_entry(prompt_id: str) -> dict | None:
    data = http_json("GET", f"{COMFY}/history/{prompt_id}", timeout=15)
    if not data:
        return None
    return data.get(prompt_id) or (next(iter(data.values())) if data else None)


def comfy_error_text(entry: dict, status: str) -> str:
    for item in (entry.get("status") or {}).get("messages") or []:
        if not (isinstance(item, list) and len(item) >= 2 and item[0] == "execution_error"):
            continue
        payload = item[1] if isinstance(item[1], dict) else {}
        node = payload.get("node_type") or payload.get("node_id") or "?"
        msg = (payload.get("exception_message") or "").strip() or status
        return f"ComfyUI {node} 失败: {msg.splitlines()[0]}"
    return f"ComfyUI 推理失败: {status}"


def collect_videos(entry: dict) -> list[Path]:
    found: list[Path] = []
    outputs = entry.get("outputs") or {}
    for node_out in outputs.values():
        if not isinstance(node_out, dict):
            continue
        for key in ("videos", "gifs", "images", "audio"):
            items = node_out.get(key) or []
            if isinstance(items, dict):
                items = [items]
            for item in items:
                if not isinstance(item, dict):
                    continue
                name = item.get("filename") or ""
                if not str(name).lower().endswith((".mp4", ".webm", ".mkv", ".mov")):
                    continue
                sub = item.get("subfolder") or ""
                kind = item.get("type") or "output"
                if kind != "output":
                    continue
                path = COMFY_OUTPUT.joinpath(*Path(sub).parts) / name if sub else COMFY_OUTPUT / name
                if path.exists():
                    found.append(path)
    return found


def fallback_video(job_id: str) -> Path | None:
    folder = COMFY_OUTPUT / "aishow"
    if not folder.exists():
        return None
    cands = sorted(folder.glob(f"{job_id}*"), key=lambda p: p.stat().st_mtime, reverse=True)
    for path in cands:
        if path.suffix.lower() in {".mp4", ".webm", ".mkv", ".mov"} and path.stat().st_size > 0:
            return path
    return None


def wait_comfy(job: dict, prompt_id: str, timeout: float = 3600) -> Path:
    t0 = time.time()
    last_status = ""
    while time.time() - t0 < timeout:
        if job.get("cancel"):
            raise JobCancelled("已取消")
        try:
            entry = history_entry(prompt_id)
        except Exception as exc:
            print(f"[fasth3-gguf] history poll: {exc}", flush=True)
            time.sleep(2)
            continue
        if entry:
            status = ((entry.get("status") or {}).get("status_str") or "").lower()
            if status and status != last_status:
                print(f"[fasth3-gguf] comfy {prompt_id} {status}", flush=True)
                last_status = status
            completed = bool((entry.get("status") or {}).get("completed"))
            if status == "error" or (completed and status and status != "success"):
                raise RuntimeError(comfy_error_text(entry, status))
            if completed or status == "success":
                videos = collect_videos(entry)
                if videos:
                    return videos[0]
                fallback = fallback_video(job["id"])
                if fallback:
                    return fallback
                raise RuntimeError("ComfyUI 已完成但没有写出 MP4")
        else:
            try:
                q = http_json("GET", f"{COMFY}/queue", timeout=8)
                running = any(prompt_id == item[1] for item in (q.get("queue_running") or []) if len(item) > 1)
                pending = any(prompt_id == item[1] for item in (q.get("queue_pending") or []) if len(item) > 1)
                job["progress"] = 55 if running else 25
            except Exception:
                job["progress"] = max(int(job.get("progress") or 20), 20)
        time.sleep(2)
    raise TimeoutError("等待 ComfyUI 超时")


def run_job(job_id: str):
    job = JOBS[job_id]
    try:
        if job.get("cancel"):
            raise JobCancelled("已取消")
        if not comfy_ready():
            raise RuntimeError(f"ComfyUI 未就绪，请先开 {COMFY}（start_fasth3_gguf.bat）")
        req = job["request"]
        seconds = float(req.get("seconds") or 5)
        width, height, frames = parse_size(req.get("size"), seconds)
        if req.get("num_frames"):
            frames = align_frames(int(req["num_frames"]))
        prompt = (req.get("prompt") or "").strip()
        if not prompt:
            raise ValueError("提示词为空")
        seed = int(req.get("seed") or 1000)
        steps = int(req.get("num_inference_steps") or 4)
        if steps in (0, 5):
            steps = 4
        steps = max(4, min(steps, 8))
        job["status"] = "in_progress"
        job["progress"] = 20
        print(f"[fasth3-gguf] infer {job_id} {width}x{height} frames={frames} steps={steps}", flush=True)
        graph = build_prompt(job_id, prompt, width, height, frames, seed, steps)
        prompt_id = submit_prompt(graph, job_id)
        job["prompt_id"] = prompt_id
        job["progress"] = 28
        print(f"[fasth3-gguf] comfy prompt {prompt_id}", flush=True)
        src = wait_comfy(job, prompt_id)
        dest = OUT_DIR / f"{job_id}.mp4"
        OUT_DIR.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(src, dest)
        job["path"] = str(dest)
        job["status"] = "completed"
        job["progress"] = 100
        print(f"[fasth3-gguf] wrote {dest} from {src}", flush=True)
    except JobCancelled:
        job["status"] = "cancelled"
        job["error"] = {"message": "已取消"}
        print(f"[fasth3-gguf] cancelled {job_id}", flush=True)
    except Exception as exc:
        job["status"] = "failed"
        job["error"] = {"message": str(exc)}
        job["trace"] = traceback.format_exc()
        print(job["trace"], flush=True)


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("[fasth3-gguf]", fmt % args)

    def _json(self, code, payload):
        raw = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        path = self.path.split("?", 1)[0]
        if path in ("/", "/health", "/v1/models"):
            current = next((j for j in JOBS.values() if j.get("status") == "in_progress"), None)
            self._json(200, {
                "ok": True,
                "status": "ok",
                "model": "fasth3-gguf",
                "ready": comfy_ready(),
                "busy": current is not None,
                "progress": (current or {}).get("progress", 0),
                "comfy": COMFY,
            })
            return
        m = re.fullmatch(r"/v1/videos/([^/]+)/content", path)
        if m:
            job = JOBS.get(m.group(1))
            if not job or not job.get("path") or not Path(job["path"]).exists():
                self._json(404, {"error": "no video"})
                return
            data = Path(job["path"]).read_bytes()
            self.send_response(200)
            self.send_header("Content-Type", "video/mp4")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return
        m = re.fullmatch(r"/v1/videos/([^/]+)", path)
        if m:
            job = JOBS.get(m.group(1))
            if not job:
                self._json(404, {"error": "not found"})
                return
            self._json(200, {
                "id": job["id"],
                "status": job["status"],
                "progress": job.get("progress", 0),
                "error": job.get("error"),
            })
            return
        self._json(404, {"error": "not found"})

    def do_POST(self):
        path = self.path.split("?", 1)[0]
        n = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(n) if n else b"{}"
        m = re.fullmatch(r"/v1/videos/([^/]+)/cancel", path)
        if m:
            if request_cancel(m.group(1)):
                self._json(200, {"ok": True, "id": m.group(1), "status": "cancelled"})
            else:
                self._json(404, {"error": "not found"})
            return
        if path not in ("/v1/videos", "/v1/videos/generations"):
            self._json(404, {"error": "not found"})
            return
        body = json.loads(raw or b"{}")
        job_id = uuid.uuid4().hex
        job = {"id": job_id, "status": "queued", "progress": 5, "request": body}
        with LOCK:
            JOBS[job_id] = job
        INFER_Q.put(job_id)
        self._json(200, {"id": job_id, "object": "video", "status": "queued"})

    def do_DELETE(self):
        path = self.path.split("?", 1)[0]
        m = re.fullmatch(r"/v1/videos/([^/]+)", path)
        if not m:
            self._json(404, {"error": "not found"})
            return
        if request_cancel(m.group(1)):
            self._json(200, {"id": m.group(1), "deleted": True, "status": "cancelled"})
        else:
            self._json(404, {"error": "not found"})


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    ready = comfy_ready()
    print(f"[fasth3-gguf] comfy={COMFY} ready={ready}", flush=True)
    print(f"[fasth3-gguf] unet={UNET} clip={CLIP_LOADER}:{CLIP}", flush=True)
    httpd = ThreadingHTTPServer((HOST, PORT), Handler)
    threading.Thread(target=httpd.serve_forever, daemon=True, name="fasth3-gguf-http").start()
    print(f"[fasth3-gguf] listening on http://{HOST}:{PORT}", flush=True)
    if not ready:
        print("[fasth3-gguf] 等待 ComfyUI 起来后再投任务", flush=True)
    while True:
        run_job(INFER_Q.get())


if __name__ == "__main__":
    main()
