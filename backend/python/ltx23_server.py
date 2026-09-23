"""LTX-2.3 distilled sidecar.

Exposes /v1/videos for Aishow's Go worker.
Loads the monolith distilled checkpoint through Lightricks/LTX-2 DistilledPipeline.
24GB default is fp8-cast plus CPU offload.
"""
from __future__ import annotations

import faulthandler
faulthandler.enable()

import gc
import inspect
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

HOST = os.environ.get("LTX23_HOST", "127.0.0.1")
PORT = int(os.environ.get("LTX23_PORT", "30023"))
CHECKPOINT = os.environ.get("LTX23_CHECKPOINT", "")
UPSCALER = os.environ.get("LTX23_UPSCALER", "")
GEMMA = os.environ.get("LTX23_GEMMA", "")
OUT_DIR = env_path("LTX23_OUT_DIR", _here.parents[1] / "data" / "sidecar-out" / "ltx23")
MEDIA_ROOT = media_root()
FPS = float(os.environ.get("LTX23_FPS", "24"))
QUANT = os.environ.get("LTX23_QUANT", "fp8-cast")
OFFLOAD = os.environ.get("LTX23_OFFLOAD", "cpu")

JOBS: dict[str, dict] = {}
LOCK = threading.Lock()
INFER_Q: queue.Queue[str] = queue.Queue()
PIPE = None
LOAD_T0 = 0.0
LOAD_ERROR = ""
LOAD_HINT = ""


def text_encoder_payload() -> dict:
    return {"text_encoder": "Gemma-3-12B", "text_encoder_label": "LTX-2.3 Gemma 3"}


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


def align(n: int, step: int = 64) -> int:
    return max(step, int(n) // step * step)


def canvas(aspect: str, short: int) -> tuple[int, int]:
    short = 768 if int(short or 512) >= 640 else 512
    short = align(short, 64)
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
        width = align(int(short * aw / ah), 64)
    else:
        width = short
        height = align(int(short * ah / aw), 64)
    return height, width


def frames_for(seconds: float) -> int:
    n = int(round(max(2.0, seconds) * FPS))
    k = max(2, round((n - 1) / 8))
    return 8 * k + 1


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


def _first_image(req: dict) -> str | None:
    for cond in req.get("conditions") or []:
        kind = str(cond.get("type") or "image").lower()
        if kind not in ("", "image"):
            continue
        uri = cond.get("uri")
        if uri:
            return str(parse_uri(uri))
    return None


def _image_inputs(path: str | None):
    if not path:
        return []
    from ltx_pipelines.utils.args import ImageConditioningInput

    sig = inspect.signature(ImageConditioningInput)
    kwargs = {}
    for name, param in sig.parameters.items():
        if name in ("self",):
            continue
        if name in ("path", "image", "image_path"):
            kwargs[name] = path
        elif name in ("frame_idx", "frame", "index", "latent_idx"):
            kwargs[name] = 0
        elif name == "strength":
            kwargs[name] = 1.0
        elif param.default is inspect.Parameter.empty:
            raise TypeError(f"ImageConditioningInput 缺少已知字段 {name}")
    try:
        return [ImageConditioningInput(**kwargs)]
    except TypeError:
        return [ImageConditioningInput(path, 0, 1.0)]


def _quantization():
    mode = (QUANT or "").strip().lower()
    if mode in ("", "0", "none", "off", "bf16"):
        return None
    from ltx_core.quantization.fp8_cast import build_policy

    return build_policy(CHECKPOINT)


def _offload():
    from ltx_pipelines.utils.types import OffloadMode

    name = (OFFLOAD or "cpu").strip().lower()
    if name in ("0", "none", "off"):
        return getattr(OffloadMode, "NONE")
    for cand in ("CPU", "cpu"):
        if hasattr(OffloadMode, cand):
            return getattr(OffloadMode, cand)
    return OffloadMode.NONE


def _release_cuda():
    gc.collect()
    try:
        import torch

        torch.cuda.empty_cache()
        torch.cuda.ipc_collect()
    except Exception:
        pass


def load_pipe():
    global PIPE, LOAD_ERROR, LOAD_HINT
    missing = []
    if not CHECKPOINT or not Path(CHECKPOINT).exists():
        missing.append(f"蒸馏权重 {CHECKPOINT or '(未设置)'}")
    if not UPSCALER or not Path(UPSCALER).exists():
        missing.append(f"空间超分 {UPSCALER or '(未设置)'}")
    if not GEMMA or not Path(GEMMA).joinpath("config.json").exists():
        missing.append(f"Gemma {GEMMA or '(未设置)'}")
    if missing:
        LOAD_ERROR = "缺少 " + "；".join(missing)
        print(f"[ltx23] {LOAD_ERROR}", flush=True)
        return
    try:
        from ltx_pipelines.distilled import DistilledPipeline
        from ltx_pipelines.utils.model_paths import ModelPaths
    except ImportError:
        LOAD_ERROR = "当前 Python 没有 ltx_pipelines。请运行 scripts\\setup_ltx23.ps1，并用它的 .venv 启动。"
        print(f"[ltx23] {LOAD_ERROR}", flush=True)
        return

    print(f"[ltx23] loading {CHECKPOINT} quant={QUANT} offload={OFFLOAD}", flush=True)
    LOAD_HINT = "正在装载 LTX-2.3 distilled（fp8 + CPU offload 时第一次会比较久）"
    try:
        paths = ModelPaths.from_monolith(CHECKPOINT, GEMMA)
        PIPE = DistilledPipeline(
            model_paths=paths,
            spatial_upsampler_path=UPSCALER,
            loras=[],
            quantization=_quantization(),
            offload_mode=_offload(),
        )
        LOAD_HINT = ""
        LOAD_ERROR = ""
        print("[ltx23] ready", flush=True)
    except Exception as exc:
        PIPE = None
        LOAD_HINT = ""
        LOAD_ERROR = str(exc)
        print("[ltx23] load failed:", traceback.format_exc(), flush=True)
        _release_cuda()


def run_job(job_id: str):
    job = JOBS[job_id]
    try:
        if job.get("cancel"):
            raise JobCancelled("已取消")
        req = job["request"]
        task = str(req.get("task") or "t2va").lower()
        image = _first_image(req) if task == "i2va" else None
        if task == "i2va" and not image:
            raise ValueError("图生视频需要一张首帧")
        target = req.get("target") or {}
        height, width = canvas(target.get("aspect_ratio") or "16:9", int(target.get("short_edge") or 512))
        seconds = float(target.get("duration_seconds") or req.get("seconds") or 5)
        num_frames = frames_for(seconds)
        seed = int(req.get("seed") or 0)
        prompt = req.get("prompt") or ""
        if job.get("cancel"):
            raise JobCancelled("已取消")
        job["status"] = "in_progress"
        job["progress"] = 20
        print(f"[ltx23] infer {job_id} {width}x{height} frames={num_frames}", flush=True)
        result = PIPE(
            prompt=prompt,
            seed=seed,
            height=height,
            width=width,
            frame_rate=FPS,
            images=_image_inputs(image),
            num_frames=num_frames,
        )
        if job.get("cancel"):
            raise JobCancelled("已取消")
        job["progress"] = 88
        from ltx_core.model.video_vae import get_video_chunks_number
        from ltx_pipelines.utils.media_io import encode_video

        OUT_DIR.mkdir(parents=True, exist_ok=True)
        dest = OUT_DIR / f"{job_id}.mp4"
        encode_video(
            video=result.video,
            fps=FPS,
            audio=result.audio,
            output_path=str(dest),
            video_chunks_number=get_video_chunks_number(result.num_frames, result.tiling_config),
        )
        job["path"] = str(dest)
        job["status"] = "completed"
        job["progress"] = 100
    except JobCancelled:
        job["status"] = "cancelled"
        job["error"] = {"message": "已取消"}
        print(f"[ltx23] cancelled {job_id}", flush=True)
        _release_cuda()
    except Exception as exc:
        job["status"] = "failed"
        job["error"] = {"message": str(exc)}
        print(traceback.format_exc(), flush=True)
        _release_cuda()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("[ltx23]", fmt % args)

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
                "model": CHECKPOINT,
                "variant": "ltx-2.3-distilled",
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
            self._json(503, {"error": LOAD_ERROR or "LTX-2.3 仍在加载权重，请稍后再投"})
            return
        body = json.loads(raw or b"{}")
        task = str(body.get("task") or "t2va").lower()
        if task not in ("t2va", "i2va"):
            self._json(400, {"error": "LTX-2.3 仅支持文生和首帧图生"})
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
    threading.Thread(target=httpd.serve_forever, daemon=True, name="ltx23-http").start()
    print(f"[ltx23] listening on http://{HOST}:{PORT}（先探活，权重仍在加载）", flush=True)
    load_pipe()
    if PIPE is None:
        print("[ltx23] weights not loaded; health stays up so the control plane can see the error", flush=True)
    else:
        print("[ltx23] inference runs on the main thread", flush=True)
    while True:
        if PIPE is None:
            time.sleep(2)
            continue
        job_id = INFER_Q.get()
        run_job(job_id)


if __name__ == "__main__":
    main()
