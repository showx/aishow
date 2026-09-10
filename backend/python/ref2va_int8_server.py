"""MiniMax H3 Ref2VA INT8 sidecar for Aishow.

Same /v1/videos job API as the NF4 / FastH3 sidecars. Submits the official
ComfyUI R2V graph with minimax_h3_ref2va_pruned_int8_convrot.safetensors.
Listen on 30011 so Aishow's existing Ref2VA URL can reach it.
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
from urllib.parse import urlparse
from urllib.request import Request, urlopen

HOST = os.environ.get("H3_REF2VA_HOST", "127.0.0.1")
PORT = int(os.environ.get("H3_REF2VA_PORT", "30011"))
COMFY = os.environ.get("H3_REF2VA_COMFY_URL", "http://127.0.0.1:8188").rstrip("/")
OUT_DIR = Path(os.environ.get("H3_REF2VA_OUT_DIR", r"F:\models\aishow-fasth3-out"))
COMFY_ROOT = Path(os.environ.get(
    "H3_REF2VA_COMFY_ROOT",
    r"E:\MiniMax-H3\ComfyUI_windows_portable\ComfyUI",
))
COMFY_OUTPUT = Path(os.environ.get("H3_REF2VA_COMFY_OUTPUT", str(COMFY_ROOT / "output")))
COMFY_INPUT = Path(os.environ.get("H3_REF2VA_COMFY_INPUT", str(COMFY_ROOT / "input")))
MEDIA_ROOT = Path(os.environ.get("H3_MEDIA_ROOT", r"D:\code\aishow\backend\data\media"))

UNET = os.environ.get("H3_REF2VA_UNET", "minimax_h3_ref2va_pruned_int8_convrot.safetensors")
VIDEO_VAE = os.environ.get("H3_VIDEO_VAE", "minimax_h3_video_vae_fp16.safetensors")
AUDIO_VAE = os.environ.get("H3_AUDIO_VAE", "minimax_h3_audio_vae_fp32.safetensors")
CLIP_DIRS = [
    Path(r"F:\models\fasth3-gguf\text_encoders"),
    Path(r"E:\MiniMax-H3\ComfyUI_windows_portable\ComfyUI\models\text_encoders"),
]
CLIP_CANDIDATES = [
    "qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors",
    "qwen3vl_32b_minimax_h3_ultra_uncensored_heretic_int8_convrot.safetensors",
    "qwen3vl_32b_h3_ultra_uncensored_heretic_int8_convrot.safetensors",
    "qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors",
]


def resolve_clip() -> str:
    forced = os.environ.get("H3_REF2VA_CLIP", "").strip()
    if forced:
        return forced
    for name in CLIP_CANDIDATES:
        if any((folder / name).exists() for folder in CLIP_DIRS):
            return name
    return CLIP_CANDIDATES[-1]


CLIP = resolve_clip()
SHIFT_VIDEO = float(os.environ.get("H3_SHIFT_VIDEO", "12"))
SHIFT_AUDIO = float(os.environ.get("H3_SHIFT_AUDIO", "3"))

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


def canvas(aspect: str | None, short: int) -> tuple[int, int]:
    short = round32(min(max(short or 480, 32), 1080))
    aspect = (aspect or "16:9").lower()
    table = {
        "16:9": (short, round32(int(short * 16 / 9))),
        "9:16": (round32(int(short * 16 / 9)), short),
        "1:1": (short, short),
        "4:3": (short, round32(int(short * 4 / 3))),
        "3:4": (round32(int(short * 4 / 3)), short),
        "21:9": (short, round32(int(short * 21 / 9))),
    }
    if aspect in ("auto", "adaptive"):
        aspect = "16:9"
    height, width = table.get(aspect, (short, round32(int(short * 16 / 9))))
    return width, height


def parse_uri(uri: str) -> Path:
    if not uri:
        raise ValueError("empty uri")
    if uri.startswith("http://") or uri.startswith("https://"):
        dest = OUT_DIR / f"dl-{uuid.uuid4().hex}{Path(urlparse(uri).path).suffix or '.bin'}"
        dest.parent.mkdir(parents=True, exist_ok=True)
        req = Request(uri, headers={"Accept": "*/*"})
        with urlopen(req, timeout=120) as resp, open(dest, "wb") as f:
            shutil.copyfileobj(resp, f)
        return dest
    raw = uri
    if raw.startswith("file://"):
        raw = raw[7:]
        if re.match(r"^/[A-Za-z]:", raw):
            raw = raw[1:]
        raw = raw.replace("/", os.sep)
        if raw.startswith(os.sep + "data" + os.sep + "minimax-h3") or raw.startswith("/data/minimax-h3"):
            tail = raw.split("minimax-h3", 1)[-1].lstrip("\\/")
            raw = str(MEDIA_ROOT / tail)
    p = Path(raw)
    if p.exists():
        return p
    raise FileNotFoundError(f"素材不存在: {uri} -> {p}")


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
        print(f"[ref2va-int8] queue delete skipped: {exc}", flush=True)
    try:
        http_json("POST", f"{COMFY}/interrupt", {}, timeout=8)
    except Exception as exc:
        print(f"[ref2va-int8] interrupt skipped: {exc}", flush=True)


def stage_input(src: Path, job_id: str, index: int) -> str:
    COMFY_INPUT.mkdir(parents=True, exist_ok=True)
    suffix = src.suffix.lower() or ".bin"
    name = f"aishow-{job_id}-{index:02d}{suffix}"
    dest = COMFY_INPUT / name
    shutil.copyfile(src, dest)
    return name


def cond_kind(cond: dict) -> str:
    typ = str(cond.get("type") or "").lower()
    if typ in ("image", "video", "audio"):
        return typ
    uri = str(cond.get("uri") or "").lower()
    if uri.endswith((".mp4", ".webm", ".mkv", ".mov", ".avi")):
        return "video"
    if uri.endswith((".wav", ".mp3", ".flac", ".ogg", ".m4a", ".aac")):
        return "audio"
    return "image"


def ensure_prompt_tags(prompt: str, n_img: int, n_vid: int, n_aud: int) -> str:
    text = (prompt or "").strip()
    if re.search(r"<(Picture|Video|Audio)\s+\d+>", text, re.I):
        return text
    bits = []
    for i in range(1, n_img + 1):
        bits.append(f"<Picture {i}>")
    audio_n = 1
    for i in range(1, n_vid + 1):
        bits.append(f"<Audio {audio_n}>")
        audio_n += 1
        bits.append(f"<Video {i}>")
    for _ in range(n_aud):
        bits.append(f"<Audio {audio_n}>")
        audio_n += 1
    if not bits:
        return text
    return "使用 " + "、".join(bits) + " 作为参考。" + (" " + text if text else "")


def build_prompt(
    job_id: str,
    prompt: str,
    width: int,
    height: int,
    frames: int,
    seed: int,
    steps: int,
    shift_video: float,
    shift_audio: float,
    images: list[str],
    videos: list[str],
    audios: list[str],
) -> dict:
    prefix = f"aishow/{job_id}"
    graph = {
        "1": {
            "class_type": "UNETLoader",
            "inputs": {"unet_name": UNET, "weight_dtype": "default"},
        },
        "2": {
            "class_type": "CLIPLoader",
            "inputs": {"clip_name": CLIP, "type": "minimax", "device": "default"},
        },
        "3": {"class_type": "VAELoader", "inputs": {"vae_name": VIDEO_VAE}},
        "4": {"class_type": "VAELoader", "inputs": {"vae_name": AUDIO_VAE}},
        "5": {
            "class_type": "MiniMaxH3SigmaShift",
            "inputs": {"model": ["1", 0], "shift_video": shift_video, "shift_audio": shift_audio},
        },
        "7": {
            "class_type": "MiniMaxH3ReferenceToVideo",
            "inputs": {
                "clip": ["2", 0],
                "vae": ["3", 0],
                "audio_vae": ["4", 0],
                "prompt": prompt,
                "width": width,
                "height": height,
                "length": frames,
                "ref_image_size": "match",
            },
        },
        "8": {
            "class_type": "BasicGuider",
            "inputs": {"model": ["5", 0], "conditioning": ["7", 0]},
        },
        "9": {"class_type": "KSamplerSelect", "inputs": {"sampler_name": "res_multistep"}},
        "10": {
            "class_type": "BasicScheduler",
            "inputs": {"model": ["5", 0], "scheduler": "simple", "steps": steps, "denoise": 1.0},
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
    nid = 20
    ref = graph["7"]["inputs"]
    for i, name in enumerate(images[:9]):
        node = str(nid)
        graph[node] = {"class_type": "LoadImage", "inputs": {"image": name}}
        ref[f"ref_images.ref_image_{i}"] = [node, 0]
        nid += 1
    for i, name in enumerate(videos[:3]):
        load_id = str(nid)
        split_id = str(nid + 1)
        graph[load_id] = {"class_type": "LoadVideo", "inputs": {"file": name}}
        graph[split_id] = {"class_type": "GetVideoComponents", "inputs": {"video": [load_id, 0]}}
        ref[f"ref_videos.ref_video_{i}"] = [split_id, 0]
        ref[f"ref_video_audios.ref_video_audio_{i}"] = [split_id, 1]
        nid += 2
    for i, name in enumerate(audios[:3]):
        node = str(nid)
        graph[node] = {"class_type": "LoadAudio", "inputs": {"audio": name}}
        ref[f"ref_audios.ref_audio_{i}"] = [node, 0]
        nid += 1
    return graph


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


def wait_comfy(job: dict, prompt_id: str, timeout: float = 14400) -> Path:
    t0 = time.time()
    last_status = ""
    while time.time() - t0 < timeout:
        if job.get("cancel"):
            raise JobCancelled("已取消")
        try:
            entry = history_entry(prompt_id)
        except Exception as exc:
            print(f"[ref2va-int8] history poll: {exc}", flush=True)
            time.sleep(2)
            continue
        if entry:
            status = ((entry.get("status") or {}).get("status_str") or "").lower()
            if status and status != last_status:
                print(f"[ref2va-int8] comfy {prompt_id} {status}", flush=True)
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
    job["t0"] = time.time()
    try:
        if job.get("cancel"):
            raise JobCancelled("已取消")
        if not comfy_ready():
            raise RuntimeError(f"ComfyUI 未就绪，请先开 {COMFY}（start_h3_ref2va_int8.bat）")
        req = job["request"]
        target = req.get("target") or {}
        seconds = float(target.get("duration_seconds") or req.get("seconds") or 5)
        seconds = min(max(seconds, 2), 15)
        short = int(target.get("short_edge") or 480)
        width, height = canvas(target.get("aspect_ratio"), short)
        frames = align_frames(int(round(seconds * 24)))
        seed = int(req.get("seed") or 1000)
        steps = int(req.get("num_inference_steps") or 20)
        if steps <= 0:
            steps = 20
        steps = max(8, min(steps, 50))
        shift_video = float(req.get("flow_shift") or SHIFT_VIDEO)
        shift_audio = float(req.get("audio_flow_shift") or SHIFT_AUDIO)

        images: list[str] = []
        videos: list[str] = []
        audios: list[str] = []
        for i, cond in enumerate(req.get("conditions") or []):
            if not isinstance(cond, dict):
                continue
            uri = str(cond.get("uri") or "").strip()
            if not uri:
                continue
            src = parse_uri(uri)
            name = stage_input(src, job_id, i)
            kind = cond_kind(cond)
            if kind == "video":
                videos.append(name)
            elif kind == "audio":
                audios.append(name)
            else:
                images.append(name)
        if not images and not videos and not audios:
            raise ValueError("参考生成至少需要一张参考图、一段参考视频或一段参考音频")

        prompt = ensure_prompt_tags(req.get("prompt") or "", len(images), len(videos), len(audios))
        job["status"] = "in_progress"
        job["progress"] = 20
        print(
            f"[ref2va-int8] infer {job_id} {width}x{height} frames={frames} steps={steps} "
            f"img={len(images)} vid={len(videos)} aud={len(audios)}",
            flush=True,
        )
        graph = build_prompt(
            job_id, prompt, width, height, frames, seed, steps,
            shift_video, shift_audio, images, videos, audios,
        )
        prompt_id = submit_prompt(graph, job_id)
        job["prompt_id"] = prompt_id
        job["progress"] = 28
        print(f"[ref2va-int8] comfy prompt {prompt_id}", flush=True)
        src = wait_comfy(job, prompt_id)
        dest = OUT_DIR / f"{job_id}.mp4"
        OUT_DIR.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(src, dest)
        job["path"] = str(dest)
        job["status"] = "completed"
        job["progress"] = 100
        print(f"[ref2va-int8] wrote {dest} from {src}", flush=True)
    except JobCancelled:
        job["status"] = "cancelled"
        job["error"] = {"message": "已取消"}
        print(f"[ref2va-int8] cancelled {job_id}", flush=True)
    except Exception as exc:
        job["status"] = "failed"
        job["error"] = {"message": str(exc)}
        job["trace"] = traceback.format_exc()
        print(job["trace"], flush=True)


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("[ref2va-int8]", fmt % args)

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
                "model": "minimax-h3-ref2va-int8",
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
                "elapsed_sec": int(time.time() - job["t0"]) if job.get("t0") else 0,
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
        if path == "/shutdown":
            self._json(200, {"ok": True, "shutting_down": True})
            threading.Thread(target=lambda: (time.sleep(0.35), os._exit(0)), daemon=True).start()
            return
        if path not in ("/v1/videos", "/v1/videos/generations"):
            self._json(404, {"error": "not found"})
            return
        body = json.loads(raw or b"{}")
        task = str(body.get("task") or "ref2va").lower()
        if task not in ("ref2va", ""):
            self._json(400, {"error": "这条 INT8 边车只承接参考生成 ref2va"})
            return
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
    COMFY_INPUT.mkdir(parents=True, exist_ok=True)
    ready = comfy_ready()
    print(f"[ref2va-int8] comfy={COMFY} ready={ready}", flush=True)
    print(f"[ref2va-int8] unet={UNET} clip={CLIP}", flush=True)
    httpd = ThreadingHTTPServer((HOST, PORT), Handler)
    threading.Thread(target=httpd.serve_forever, daemon=True, name="ref2va-int8-http").start()
    print(f"[ref2va-int8] listening on http://{HOST}:{PORT}", flush=True)
    if not ready:
        print("[ref2va-int8] 等待 ComfyUI 起来后再投任务", flush=True)
    while True:
        run_job(INFER_Q.get())


if __name__ == "__main__":
    main()
