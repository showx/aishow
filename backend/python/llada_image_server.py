"""LLaDA-Image sidecar.

Exposes /v1/images for Aishow's Go worker.
Load inclusionAI/LLaDA-Image or LLaDA-Image-Turbo via Diffusers pipeline.
"""
from __future__ import annotations

import faulthandler
faulthandler.enable()

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

import torch
from PIL import Image

REPO = Path(os.environ.get("LLADA_REPO", r"F:\LLaDA-Image"))
PYDEPS = Path(os.environ.get("LLADA_PYDEPS", REPO / "pydeps"))
for p in (str(PYDEPS), str(REPO)):
    if p and p not in sys.path:
        sys.path.insert(0, p)


def _patch_transformers_rope() -> None:
    """LLaDA remote code expects transformers 4.x ROPE_INIT_FUNCTIONS['default']."""
    try:
        from transformers.modeling_rope_utils import ROPE_INIT_FUNCTIONS
    except Exception:
        return
    if "default" in ROPE_INIT_FUNCTIONS:
        return

    def _compute_default_rope_parameters(config, device=None, seq_len=None, **kwargs):
        base = getattr(config, "rope_theta", 10000.0)
        partial = getattr(config, "partial_rotary_factor", 1.0) or 1.0
        head_dim = getattr(config, "head_dim", None) or config.hidden_size // config.num_attention_heads
        dim = int(head_dim * partial)
        inv_freq = 1.0 / (
            base ** (torch.arange(0, dim, 2, dtype=torch.int64).to(device=device, dtype=torch.float) / dim)
        )
        return inv_freq, 1.0

    ROPE_INIT_FUNCTIONS["default"] = _compute_default_rope_parameters


_patch_transformers_rope()
os.environ.setdefault("LLADA_MOE_BACKEND", "eager")

from src import LLaDAImagePipeline  # noqa: E402

HOST = os.environ.get("LLADA_HOST", "127.0.0.1")
PORT = int(os.environ.get("LLADA_PORT", "30020"))
MODEL = os.environ.get("LLADA_MODEL", "inclusionAI/LLaDA-Image-Turbo")
OUT_DIR = Path(os.environ.get("LLADA_OUT_DIR", REPO / "tmp" / "aishow-jobs"))
MEDIA_ROOT = Path(os.environ.get("LLADA_MEDIA_ROOT", r"D:\code\aishow\backend\data\media"))
MAX_SHORT = int(os.environ.get("LLADA_MAX_SHORT_EDGE", "1536"))
DTYPE = os.environ.get("LLADA_DTYPE", "bfloat16")

JOBS: dict[str, dict] = {}
LOCK = threading.Lock()
INFER_Q: queue.Queue[str] = queue.Queue()
PIPE = None
LOAD_T0 = 0.0
LOAD_ERROR = ""
VARIANT = "turbo" if "turbo" in MODEL.lower() else "base"


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


def canvas(aspect: str, short: int, step: int) -> tuple[int, int]:
    short = align(min(short or 1024, MAX_SHORT), step)
    aspect = (aspect or "1:1").lower()
    table = {
        "1:1": (short, short),
        "16:9": (short, align(int(short * 16 / 9), step)),
        "9:16": (align(int(short * 16 / 9), step), short),
        "4:3": (short, align(int(short * 4 / 3), step)),
        "3:4": (align(int(short * 4 / 3), step), short),
        "21:9": (short, align(int(short * 21 / 9), step)),
    }
    if aspect in ("auto", "adaptive"):
        aspect = "1:1"
    h, w = table.get(aspect, (short, short))
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
    if name in ("fp8", "float8"):
        return torch.float8_e4m3fn
    return torch.bfloat16


def load_pipe():
    global PIPE, LOAD_ERROR
    dtype = torch_dtype()
    local = Path(MODEL)
    print(f"[llada-image] loading {MODEL} dtype={dtype} repo={REPO}", flush=True)
    if local.exists():
        size = sum(p.stat().st_size for p in local.rglob("*") if p.is_file())
        print(f"[llada-image] local weights {size / 1024**3:.1f} GB，首次读盘上 GPU 要几分钟", flush=True)
    kwargs = {"torch_dtype": dtype, "device": "cuda"}
    if local.exists():
        kwargs["local_files_only"] = True
    try:
        try:
            PIPE = LLaDAImagePipeline.from_pretrained(MODEL, **kwargs)
        except TypeError:
            PIPE = LLaDAImagePipeline.from_pretrained(MODEL, torch_dtype=dtype, local_files_only=local.exists())
            PIPE = PIPE.to("cuda")
        print("[llada-image] ready", flush=True)
    except Exception as exc:
        LOAD_ERROR = str(exc)
        print("[llada-image] load failed:", traceback.format_exc(), flush=True)
        raise


def run_job(job_id: str):
    job = JOBS[job_id]
    job["t0"] = time.time()
    try:
        if job.get("cancel"):
            raise JobCancelled("已取消")
        req = job["request"]
        target = req.get("target") or {}
        task = str(req.get("task") or "t2i").lower()
        quality = str(req.get("quality") or VARIANT).lower()
        if task == "i2i":
            mode = "editing"
            step = 32
        else:
            mode = "text"
            step = 16
        short = int(target.get("short_edge") or 1024)
        height, width = canvas(target.get("aspect_ratio"), short, step)
        prompt = req.get("prompt") or ""
        negative = req.get("negative_prompt") or ""
        if quality == "base":
            steps = int(req.get("num_inference_steps") or 50)
            guidance = float(req.get("guidance_scale") or 5.0)
        else:
            steps = int(req.get("num_inference_steps") or 4)
            guidance = float(req.get("guidance_scale") or 1.0)
        seed = int(req.get("seed") or 0)
        image = None
        if mode == "editing":
            ref = None
            for cond in req.get("conditions") or []:
                if str(cond.get("type") or "").lower() not in ("", "image"):
                    continue
                uri = cond.get("uri")
                if uri:
                    ref = uri
                    if str(cond.get("role") or "") == "reference":
                        break
            if not ref:
                raise ValueError("指令编辑需要一张参考图")
            image = Image.open(parse_uri(ref)).convert("RGB")

        if job.get("cancel"):
            raise JobCancelled("已取消")
        job["status"] = "in_progress"
        job["progress"] = 20
        print(f"[llada-image] infer {job_id} {mode} {width}x{height} steps={steps}", flush=True)
        gen = torch.Generator("cuda").manual_seed(seed)
        kwargs = {
            "prompt": prompt,
            "negative_prompt": negative,
            "generation_mode": mode,
            "height": height,
            "width": width,
            "num_inference_steps": steps,
            "guidance_scale": guidance,
            "generator": gen,
        }
        if image is not None:
            kwargs["image"] = image

        def on_step(_pipe, i, _t, cb):
            if job.get("cancel"):
                raise JobCancelled("已取消")
            job["progress"] = min(90, 22 + int(68 * (int(i) + 1) / max(steps, 1)))
            return cb

        try:
            out = PIPE(callback_on_step_end=on_step, **kwargs)
        except TypeError:
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
        print(f"[llada-image] cancelled {job_id}", flush=True)
        try:
            torch.cuda.empty_cache()
        except Exception:
            pass
    except Exception as exc:
        job["status"] = "failed"
        job["error"] = {"message": str(exc)}
        job["trace"] = traceback.format_exc()
        print(job["trace"], flush=True)
        try:
            torch.cuda.empty_cache()
        except Exception:
            pass


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("[llada-image]", fmt % args)

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
                "variant": VARIANT,
                "ready": PIPE is not None,
                "loading": PIPE is None and not LOAD_ERROR,
                "busy": current is not None,
                "progress": (current or {}).get("progress", 0),
                "elapsed_sec": elapsed,
                "error": LOAD_ERROR or None,
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
            self._json(503, {"error": LOAD_ERROR or "LLaDA-Image 仍在加载权重，请稍后再投"})
            return
        body = json.loads(raw or b"{}")
        task = str(body.get("task") or "t2i").lower()
        if task not in ("t2i", "i2i"):
            self._json(400, {"error": "LLaDA-Image 仅支持 t2i / i2i"})
            return
        job_id = uuid.uuid4().hex
        job = {"id": job_id, "status": "queued", "progress": 5, "request": body}
        with LOCK:
            JOBS[job_id] = job
        INFER_Q.put(job_id)
        self._json(200, {"id": job_id, "status": "queued"})


def main():
    global LOAD_T0
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    LOAD_T0 = time.time()
    httpd = ThreadingHTTPServer((HOST, PORT), Handler)
    threading.Thread(target=httpd.serve_forever, daemon=True, name="llada-http").start()
    print(f"[llada-image] listening on http://{HOST}:{PORT}（先探活，权重仍在加载）", flush=True)
    load_pipe()
    print("[llada-image] inference runs on the main thread", flush=True)
    while True:
        job_id = INFER_Q.get()
        run_job(job_id)


if __name__ == "__main__":
    main()
