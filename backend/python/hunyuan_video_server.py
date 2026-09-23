"""HunyuanVideo-1.5 sidecar.

Exposes /v1/videos for Aishow's Go worker.
480p text-to-video and image-to-video via Diffusers, one pipeline resident at a time.
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

HOST = os.environ.get("HUNYUAN_HOST", "127.0.0.1")
PORT = int(os.environ.get("HUNYUAN_PORT", "30022"))
T2V_MODEL = os.environ.get("HUNYUAN_T2V_MODEL", "")
I2V_MODEL = os.environ.get("HUNYUAN_I2V_MODEL", "")
OUT_DIR = env_path("HUNYUAN_OUT_DIR", _here.parents[1] / "data" / "sidecar-out" / "hunyuan-video")
MEDIA_ROOT = media_root()
DTYPE = os.environ.get("HUNYUAN_DTYPE", "bfloat16")
OFFLOAD = os.environ.get("HUNYUAN_OFFLOAD", "1").strip().lower() not in ("0", "false", "off", "no")
FPS = int(os.environ.get("HUNYUAN_FPS", "24"))

JOBS: dict[str, dict] = {}
LOCK = threading.Lock()
INFER_Q: queue.Queue[str] = queue.Queue()
PIPE = None
PIPE_KIND = ""
LOAD_T0 = 0.0
LOAD_ERROR = ""
LOAD_HINT = ""


def text_encoder_payload() -> dict:
    return {"text_encoder": "Qwen2.5-VL-7B", "text_encoder_label": "HunyuanVideo-1.5 Qwen2.5-VL"}


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


def canvas(aspect: str, step: int = 16) -> tuple[int, int]:
    short = 480
    aspect = (aspect or "16:9").lower()
    table = {
        "16:9": (16, 9),
        "9:16": (9, 16),
        "1:1": (1, 1),
        "4:3": (4, 3),
        "3:4": (3, 4),
        "21:9": (21, 9),
    }
    aw, ah = table.get(aspect, (16, 9))
    if aw >= ah:
        height = short
        width = align(int(short * aw / ah), step)
    else:
        width = short
        height = align(int(short * ah / aw), step)
    return height, width


def frames_for(seconds: float) -> int:
    n = int(round(max(2.0, seconds) * FPS))
    k = max(4, round((n - 1) / 4))
    return 4 * k + 1


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


def _first_image(req: dict) -> Image.Image | None:
    for cond in req.get("conditions") or []:
        kind = str(cond.get("type") or "image").lower()
        if kind not in ("", "image"):
            continue
        uri = cond.get("uri")
        if not uri:
            continue
        return Image.open(parse_uri(uri)).convert("RGB")
    return None


def load_pipe(kind: str):
    global PIPE, PIPE_KIND, LOAD_ERROR, LOAD_HINT
    if PIPE is not None and PIPE_KIND == kind:
        return PIPE
    model = T2V_MODEL if kind == "t2v" else I2V_MODEL
    if not model or not Path(model).joinpath("model_index.json").exists():
        raise FileNotFoundError(f"缺少 HunyuanVideo-1.5 {kind} 权重: {model or '(未设置)'}")
    try:
        from diffusers import HunyuanVideo15ImageToVideoPipeline, HunyuanVideo15Pipeline
    except ImportError as exc:
        raise RuntimeError(
            "当前 Python 没有 HunyuanVideo15Pipeline。请运行 scripts\\setup_hunyuan_video.ps1"
        ) from exc

    if PIPE is not None:
        print(f"[hunyuan] unload {PIPE_KIND} before loading {kind}", flush=True)
        PIPE = None
        PIPE_KIND = ""
        _release_cuda()

    dtype = torch_dtype()
    cls = HunyuanVideo15Pipeline if kind == "t2v" else HunyuanVideo15ImageToVideoPipeline
    print(f"[hunyuan] loading {kind} {model} dtype={dtype} offload={OFFLOAD}", flush=True)
    kwargs = {"torch_dtype": dtype, "local_files_only": True}
    last_err = ""
    for i in range(3):
        LOAD_ERROR = ""
        LOAD_HINT = f"正在装载 HunyuanVideo-1.5 {kind}（第 {i + 1}/3 次）"
        try:
            pipe = cls.from_pretrained(model, **kwargs)
            if OFFLOAD:
                pipe.enable_model_cpu_offload()
            else:
                pipe = pipe.to("cuda")
            if getattr(pipe, "vae", None) is not None:
                pipe.vae.enable_tiling()
            PIPE = pipe
            PIPE_KIND = kind
            LOAD_HINT = ""
            print(f"[hunyuan] {kind} ready", flush=True)
            return pipe
        except Exception as exc:
            last_err = str(exc)
            print("[hunyuan] load failed:", traceback.format_exc(), flush=True)
            PIPE = None
            PIPE_KIND = ""
            _release_cuda()
            if _is_oom(exc) and i + 1 < 3:
                LOAD_HINT = f"显存不够，20 秒后重试（{i + 2}/3）"
                time.sleep(20)
                continue
            break
    LOAD_HINT = ""
    LOAD_ERROR = last_err or "HunyuanVideo-1.5 装载失败"
    raise RuntimeError(LOAD_ERROR)


def run_job(job_id: str):
    job = JOBS[job_id]
    job["t0"] = time.time()
    try:
        if job.get("cancel"):
            raise JobCancelled("已取消")
        req = job["request"]
        task = str(req.get("task") or "t2va").lower()
        image = _first_image(req) if task == "i2va" else None
        if task == "i2va" and image is None:
            raise ValueError("图生视频需要一张首帧")
        kind = "i2v" if image is not None else "t2v"
        target = req.get("target") or {}
        height, width = canvas(target.get("aspect_ratio") or "16:9")
        seconds = float(target.get("duration_seconds") or req.get("seconds") or 5)
        num_frames = frames_for(seconds)
        steps = int(req.get("num_inference_steps") or 30)
        steps = min(50, max(10, steps))
        seed = int(req.get("seed") or 0)
        prompt = req.get("prompt") or ""
        if job.get("cancel"):
            raise JobCancelled("已取消")
        job["status"] = "in_progress"
        job["progress"] = 12
        pipe = load_pipe(kind)
        job["progress"] = 22
        print(
            f"[hunyuan] infer {job_id} {kind} {width}x{height} frames={num_frames} steps={steps}",
            flush=True,
        )
        try:
            gen = torch.Generator(device="cuda").manual_seed(seed)
        except Exception:
            gen = torch.Generator().manual_seed(seed)
        kwargs = {
            "prompt": prompt,
            "height": height,
            "width": width,
            "num_frames": num_frames,
            "num_inference_steps": steps,
            "generator": gen,
        }
        if image is not None:
            kwargs["image"] = image

        def on_step(_pipe, i, _t, cb):
            if job.get("cancel"):
                raise JobCancelled("已取消")
            job["progress"] = min(90, 24 + int(64 * (int(i) + 1) / max(steps, 1)))
            return cb

        try:
            out = pipe(callback_on_step_end=on_step, **kwargs)
        except TypeError:
            out = pipe(**kwargs)
        frames = out.frames[0]
        from diffusers.utils import export_to_video

        OUT_DIR.mkdir(parents=True, exist_ok=True)
        dest = OUT_DIR / f"{job_id}.mp4"
        export_to_video(frames, str(dest), fps=FPS)
        job["path"] = str(dest)
        job["status"] = "completed"
        job["progress"] = 100
    except JobCancelled:
        job["status"] = "cancelled"
        job["error"] = {"message": "已取消"}
        print(f"[hunyuan] cancelled {job_id}", flush=True)
        _release_cuda()
    except Exception as exc:
        job["status"] = "failed"
        job["error"] = {"message": str(exc)}
        job["trace"] = traceback.format_exc()
        print(job["trace"], flush=True)
        _release_cuda()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("[hunyuan]", fmt % args)

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
                "model": T2V_MODEL,
                "variant": "hunyuan-video-1.5-480p",
                "pipe": PIPE_KIND or None,
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
                **text_encoder_payload(),
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
        if path != "/v1/videos":
            self._json(404, {"error": "not found"})
            return
        if PIPE is None:
            self._json(503, {"error": LOAD_ERROR or "HunyuanVideo-1.5 仍在加载权重，请稍后再投"})
            return
        body = json.loads(raw or b"{}")
        task = str(body.get("task") or "t2va").lower()
        if task not in ("t2va", "i2va"):
            self._json(400, {"error": "HunyuanVideo-1.5 仅支持文生和首帧图生"})
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
    threading.Thread(target=httpd.serve_forever, daemon=True, name="hunyuan-http").start()
    print(f"[hunyuan] listening on http://{HOST}:{PORT}（先探活，权重仍在加载）", flush=True)
    try:
        load_pipe("t2v")
    except Exception as exc:
        print(f"[hunyuan] giving up: {exc}", flush=True)
    if PIPE is None:
        print("[hunyuan] weights not loaded; health stays up so the control plane can see the error", flush=True)
    else:
        print("[hunyuan] inference runs on the main thread", flush=True)
    while True:
        if PIPE is None:
            time.sleep(2)
            continue
        job_id = INFER_Q.get()
        run_job(job_id)


if __name__ == "__main__":
    main()
