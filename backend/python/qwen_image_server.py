"""Qwen-Image-2.1 sidecar.

Exposes /v1/images for Aishow's Go worker.
Load Qwen/Qwen-Image-2.1 via Diffusers QwenImage21Pipeline (t2i + instruction edit).
"""
from __future__ import annotations

import faulthandler
faulthandler.enable()

import gc
import json
import os
import queue
import re
import sys
import threading
import time
import traceback
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlparse
from urllib.request import urlopen

_here = Path(__file__).resolve().parent
if str(_here) not in sys.path:
    sys.path.insert(0, str(_here))

from aishow_paths import env_path, media_root

import torch  # noqa: E402
from PIL import Image  # noqa: E402

HOST = os.environ.get("QWEN_IMAGE_HOST", "127.0.0.1")
PORT = int(os.environ.get("QWEN_IMAGE_PORT", "30021"))
MODEL = os.environ.get("QWEN_IMAGE_MODEL", "Qwen/Qwen-Image-2.1")
OUT_DIR = env_path("QWEN_IMAGE_OUT_DIR", _here.parents[1] / "data" / "sidecar-out" / "qwen-image")
MEDIA_ROOT = media_root()
MAX_SHORT = int(os.environ.get("QWEN_IMAGE_MAX_SHORT_EDGE", "2048"))
DTYPE = os.environ.get("QWEN_IMAGE_DTYPE", "bfloat16")
OFFLOAD = os.environ.get("QWEN_IMAGE_OFFLOAD", "1").strip().lower() not in ("0", "false", "off", "no")

JOBS: dict[str, dict] = {}
LOCK = threading.Lock()
INFER_Q: queue.Queue[str] = queue.Queue()
PIPE = None
LOAD_T0 = 0.0
LOAD_ERROR = ""
LOAD_HINT = ""


def text_encoder_payload() -> dict:
    name = Path(MODEL).name if MODEL else "Qwen-Image-2.1"
    return {"text_encoder": name, "text_encoder_label": "Qwen-Image-2.1"}


class JobCancelled(Exception):
    pass


def request_cancel(job_id: str) -> bool:
    with LOCK:
        job = JOBS.get(job_id)
        if not job:
            return False
        job["cancel"] = True
        if job.get("status") in ("queued", "in_progress"):
            job["status"] = "cancelled"
            job["error"] = {"message": "已取消"}
        return True


def align(n: int, step: int) -> int:
    return max(step, int(n) // step * step)


def canvas(aspect: str, short: int, step: int = 16) -> tuple[int, int]:
    short = align(min(short or 1024, MAX_SHORT), step)
    aspect = (aspect or "1:1").lower()
    table = {
        "1:1": (1, 1),
        "16:9": (16, 9),
        "9:16": (9, 16),
        "4:3": (4, 3),
        "3:4": (3, 4),
        "3:2": (3, 2),
        "2:3": (2, 3),
        "21:9": (21, 9),
    }
    if aspect in ("auto", "adaptive") or aspect not in table:
        aspect = "1:1"
    aw, ah = table[aspect]
    if aw >= ah:
        h = short
        w = align(int(short * aw / ah), step)
    else:
        w = short
        h = align(int(short * ah / aw), step)
    return h, w


def parse_uri(uri: str) -> Path:
    if not uri:
        raise ValueError("empty uri")
    if uri.startswith("http://") or uri.startswith("https://"):
        dest = OUT_DIR / f"dl-{uuid.uuid4().hex}{Path(urlparse(uri).path).suffix or '.png'}"
        dest.parent.mkdir(parents=True, exist_ok=True)
        with urlopen(uri) as resp, open(dest, "wb") as f:
            f.write(resp.read())
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


def torch_dtype():
    name = DTYPE.lower().replace("torch.", "")
    if name in ("fp16", "float16"):
        return torch.float16
    return torch.bfloat16


def _is_oom(exc: BaseException) -> bool:
    text = f"{type(exc).__name__} {exc}".lower()
    return any(token in text for token in (
        "out of memory", "oom", "cuda error", "cudaerror", "cudnn", "insufficient memory", "allocate",
    ))


def _release_cuda():
    gc.collect()
    try:
        torch.cuda.empty_cache()
        torch.cuda.ipc_collect()
    except Exception:
        pass


def load_pipe():
    global PIPE, LOAD_ERROR, LOAD_HINT
    try:
        from diffusers import QwenImage21Pipeline
    except ImportError:
        LOAD_ERROR = (
            "当前 Python 没有 QwenImage21Pipeline。"
            "请运行 scripts\\setup_qwen_image.ps1 装独立环境"
            "（transformers>=5.17 + git 版 diffusers），不要复用 ComfyUI / LLaDA 的 Python。"
        )
        print(f"[qwen-image] {LOAD_ERROR}", flush=True)
        return

    dtype = torch_dtype()
    local = Path(MODEL)
    print(f"[qwen-image] loading {MODEL} dtype={dtype} offload={OFFLOAD}", flush=True)
    if local.exists():
        size = sum(p.stat().st_size for p in local.rglob("*") if p.is_file())
        print(f"[qwen-image] local weights {size / 1024**3:.1f} GB，首次读盘要几分钟", flush=True)
    kwargs = {"torch_dtype": dtype}
    if local.exists() or os.environ.get("HF_HUB_OFFLINE") == "1":
        kwargs["local_files_only"] = True
    attempts = 4
    last_err = ""
    for i in range(attempts):
        LOAD_ERROR = ""
        LOAD_HINT = f"正在装载 Qwen-Image-2.1（第 {i + 1}/{attempts} 次）"
        try:
            pipe = QwenImage21Pipeline.from_pretrained(MODEL, **kwargs)
            if OFFLOAD:
                pipe.enable_model_cpu_offload()
                LOAD_HINT = "已开 CPU offload，适合 24GB"
            else:
                pipe = pipe.to("cuda")
            PIPE = pipe
            LOAD_HINT = ""
            print("[qwen-image] ready", flush=True)
            return
        except Exception as exc:
            last_err = str(exc)
            print("[qwen-image] load failed:", traceback.format_exc(), flush=True)
            PIPE = None
            _release_cuda()
            if _is_oom(exc) and i + 1 < attempts:
                wait = 20
                LOAD_HINT = f"显存不够，{wait} 秒后重试（{i + 2}/{attempts}）"
                print(f"[qwen-image] OOM, retry in {wait}s", flush=True)
                time.sleep(wait)
                continue
            break
    LOAD_HINT = ""
    LOAD_ERROR = last_err or "Qwen-Image-2.1 装载失败"
    print(f"[qwen-image] giving up: {LOAD_ERROR}", flush=True)


def _collect_images(req: dict) -> list:
    images = []
    for cond in req.get("conditions") or []:
        if str(cond.get("type") or "").lower() not in ("", "image"):
            continue
        uri = cond.get("uri")
        if not uri:
            continue
        images.append(Image.open(parse_uri(uri)).convert("RGB"))
        if len(images) >= 10:
            break
    return images


def run_job(job_id: str):
    job = JOBS[job_id]
    job["t0"] = time.time()
    try:
        if job.get("cancel"):
            raise JobCancelled("已取消")
        req = job["request"]
        target = req.get("target") or {}
        task = str(req.get("task") or "t2i").lower()
        short = int(target.get("short_edge") or 1024)
        height, width = canvas(target.get("aspect_ratio"), short, 16)
        prompt = req.get("prompt") or ""
        negative = req.get("negative_prompt") or ""
        steps = int(req.get("num_inference_steps") or 40)
        guidance = float(req.get("guidance_scale") or 1.0)
        seed = int(req.get("seed") or 0)
        images = _collect_images(req) if task == "i2i" else []
        if task == "i2i" and not images:
            raise ValueError("指令编辑需要至少一张参考图")

        if job.get("cancel"):
            raise JobCancelled("已取消")
        job["status"] = "in_progress"
        job["progress"] = 20
        print(f"[qwen-image] infer {job_id} {task} {width}x{height} steps={steps} cfg={guidance}", flush=True)
        gen = torch.Generator("cuda").manual_seed(seed)
        kwargs = {
            "prompt": prompt,
            "width": width,
            "height": height,
            "num_inference_steps": steps,
            "true_cfg_scale": guidance,
            "generator": gen,
        }
        if negative:
            kwargs["negative_prompt"] = negative
        if images:
            kwargs["image"] = images if len(images) > 1 else images[0]

        def on_step(_pipe, i, _t, cb):
            if job.get("cancel"):
                raise JobCancelled("已取消")
            job["progress"] = min(90, 22 + int(68 * (int(i) + 1) / max(steps, 1)))
            return cb

        try:
            out = PIPE(callback_on_step_end=on_step, **kwargs)
        except TypeError:
            if images and len(images) > 1:
                kwargs["image"] = images[0]
                try:
                    out = PIPE(callback_on_step_end=on_step, **kwargs)
                except TypeError:
                    out = PIPE(**kwargs)
            else:
                out = PIPE(**kwargs)
        result = out.images[0]
        OUT_DIR.mkdir(parents=True, exist_ok=True)
        dest = OUT_DIR / f"{job_id}.png"
        result.save(dest)
        job["path"] = str(dest)
        job["status"] = "completed"
        job["progress"] = 100
    except JobCancelled:
        job["status"] = "cancelled"
        job["error"] = {"message": "已取消"}
        print(f"[qwen-image] cancelled {job_id}", flush=True)
        _release_cuda()
    except Exception as exc:
        job["status"] = "failed"
        job["error"] = {"message": str(exc)}
        job["trace"] = traceback.format_exc()
        print(job["trace"], flush=True)
        _release_cuda()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("[qwen-image]", fmt % args)

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
            elapsed = int(time.time() - LOAD_T0) if LOAD_T0 else 0
            self._json(200, {
                "ok": True,
                "model": MODEL,
                "variant": "qwen-image-2.1",
                "ready": PIPE is not None,
                "loading": PIPE is None and not LOAD_ERROR,
                "busy": current is not None,
                "progress": (current or {}).get("progress", 0),
                "elapsed_sec": elapsed,
                "error": LOAD_ERROR or None,
                "hint": LOAD_HINT or None,
                **text_encoder_payload(),
            })
            return
        m = re.fullmatch(r"/v1/images/([^/]+)/content", path)
        if m:
            job = JOBS.get(m.group(1))
            if not job or not job.get("path") or not Path(job["path"]).exists():
                self._json(404, {"error": "no image"})
                return
            data = Path(job["path"]).read_bytes()
            self.send_response(200)
            self.send_header("Content-Type", "image/png")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)
            return
        m = re.fullmatch(r"/v1/images/([^/]+)", path)
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
                **text_encoder_payload(),
            })
            return
        self._json(404, {"error": "not found"})

    def do_POST(self):
        path = self.path.split("?", 1)[0]
        n = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(n) if n else b"{}"
        m = re.fullmatch(r"/v1/images/([^/]+)/cancel", path)
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
        if path != "/v1/images":
            self._json(404, {"error": "not found"})
            return
        if PIPE is None:
            self._json(503, {"error": LOAD_ERROR or "Qwen-Image-2.1 仍在加载权重，请稍后再投"})
            return
        body = json.loads(raw or b"{}")
        task = str(body.get("task") or "t2i").lower()
        if task not in ("t2i", "i2i"):
            self._json(400, {"error": "Qwen-Image-2.1 仅支持 t2i / i2i"})
            return
        job_id = uuid.uuid4().hex
        job = {"id": job_id, "status": "queued", "progress": 5, "request": body}
        with LOCK:
            JOBS[job_id] = job
        INFER_Q.put(job_id)
        self._json(200, {"id": job_id, "status": "queued", **text_encoder_payload()})


def main():
    global LOAD_T0
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    LOAD_T0 = time.time()
    httpd = ThreadingHTTPServer((HOST, PORT), Handler)
    threading.Thread(target=httpd.serve_forever, daemon=True, name="qwen-image-http").start()
    print(f"[qwen-image] listening on http://{HOST}:{PORT}（先探活，权重仍在加载）", flush=True)
    load_pipe()
    if PIPE is None:
        print("[qwen-image] weights not loaded; health stays up so the control plane can see the error", flush=True)
    else:
        print("[qwen-image] inference runs on the main thread", flush=True)
    while True:
        if PIPE is None:
            time.sleep(2)
            continue
        job_id = INFER_Q.get()
        run_job(job_id)


if __name__ == "__main__":
    main()
