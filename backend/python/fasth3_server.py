"""FastVideo FastH3 sidecar.

Exposes the same /v1/videos job API Aishow already uses.
Loads FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2 and samples the
trained 4-step DMD ladder [999, 749, 500, 250].
"""
from __future__ import annotations

import faulthandler
faulthandler.enable()

import json
import os
import queue
import re
import threading
import time
import traceback
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from aishow_paths import env_path, sidecar_out

os.environ.setdefault("FASTVIDEO_DMD_DENOISING_STEPS", "999,749,500,250")
os.environ.setdefault("FASTVIDEO_ATTENTION_BACKEND", "VIDEO_SPARSE_ATTN_H3")

HOST = os.environ.get("FASTH3_HOST", "127.0.0.1")
PORT = int(os.environ.get("FASTH3_PORT", "8000"))
MODEL = os.environ.get("FASTH3_MODEL", "FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2")
NUM_GPUS = int(os.environ.get("FASTH3_NUM_GPUS", "1"))
OUT_DIR = env_path("FASTH3_OUT_DIR", sidecar_out("fasth3"))
DMD_STEPS = [999, 749, 500, 250]

JOBS: dict[str, dict] = {}
LOCK = threading.Lock()
INFER_Q: queue.Queue[str] = queue.Queue()
GEN = None
TEXT_ENCODER = "minimax-h3-text-encoder"
TEXT_ENCODER_LABEL = "Qwen3-VL 32B"


def text_encoder_payload() -> dict:
    return {"text_encoder": TEXT_ENCODER, "text_encoder_label": TEXT_ENCODER_LABEL}


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


def result_path(result, dest: Path) -> Path:
    if dest.exists() and dest.stat().st_size > 0:
        return dest
    if isinstance(result, dict):
        for key in ("video_path", "output_path", "file_path"):
            p = result.get(key)
            if p and Path(p).exists():
                return Path(p)
    path = getattr(result, "video_path", None) or getattr(result, "output_path", None)
    if path and Path(path).exists():
        return Path(path)
    raise RuntimeError("FastH3 没有写出 MP4")


def ensure_model_index(model_dir: Path) -> None:
    dest = model_dir / "model_index.json"
    if dest.exists():
        return
    src = model_dir / "modular_model_index.json"
    if not src.exists():
        return
    raw = json.loads(src.read_text(encoding="utf-8"))
    index = {}
    for key, value in raw.items():
        if key.startswith("_"):
            index[key] = value
        elif isinstance(value, list) and len(value) >= 2:
            index[key] = [value[0], value[1]]
    dest.write_text(json.dumps(index, indent=2, ensure_ascii=False), encoding="utf-8")
    print(f"[fasth3] wrote {dest}", flush=True)


def register_fasth3_pipeline() -> None:
    try:
        from fastvideo.configs.pipelines.minimax_h3 import MiniMaxH3PipelineConfig
        from fastvideo.fastvideo_args import WorkloadType
        from fastvideo.registry import register_configs

        register_configs(
            sampling_param_cls=None,
            pipeline_config_cls=MiniMaxH3PipelineConfig,
            workload_types=(WorkloadType.T2V, WorkloadType.I2V),
            hf_model_paths=["FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2"],
            model_detectors=[
                lambda path: any(token in path.lower() for token in (
                    "minimax-fasth3",
                    "minimax_fasth3",
                    "fasth3-preview",
                )),
            ],
            model_family="minimax_h3",
            default_preset="minimax_h3_t2va",
        )
    except Exception as exc:
        print(f"[fasth3] registry patch skipped: {exc}", flush=True)


def load_gen():
    global GEN
    try:
        from fastvideo import VideoGenerator
    except ImportError as exc:
        raise RuntimeError(
            "未安装 FastVideo。请先: git clone https://github.com/hao-ai-lab/FastVideo.git "
            "&& UV_TORCH_BACKEND=cu126 uv pip install -e \".[fasth3]\""
        ) from exc
    local = os.environ.get("FASTH3_LOCAL_DIR", "").strip()
    model = local if local and Path(local).exists() else MODEL
    if local and Path(local).exists():
        ensure_model_index(Path(local))
    register_fasth3_pipeline()
    print(f"[fasth3] loading {model} gpus={NUM_GPUS}", flush=True)
    print(f"[fasth3] dmd_denoising_steps={DMD_STEPS}", flush=True)
    offload = os.environ.get("FASTH3_OFFLOAD", "1") not in ("0", "false", "False")
    kwargs = {
        "num_gpus": NUM_GPUS,
        "use_fsdp_inference": False,
        "dit_cpu_offload": offload,
        "text_encoder_cpu_offload": True,
        "image_encoder_cpu_offload": True,
        "vae_cpu_offload": True,
        "pin_cpu_memory": True,
    }
    if offload:
        kwargs["dit_layerwise_offload"] = True
    sparsity = os.environ.get("FASTH3_VSA_SPARSITY", "0.9")
    tile = os.environ.get("FASTH3_VSA_TILE", "64")
    try:
        kwargs["VSA_sparsity"] = float(sparsity)
        kwargs["VSA_tile_size"] = int(tile)
    except ValueError:
        pass
    from dataclasses import fields
    from fastvideo.fastvideo_args import FastVideoArgs
    allowed = {f.name for f in fields(FastVideoArgs)} | {"model_path"}
    kwargs = {k: v for k, v in kwargs.items() if k in allowed}
    print(f"[fasth3] from_pretrained kwargs={sorted(kwargs)}", flush=True)
    GEN = VideoGenerator.from_pretrained(model, **kwargs)
    print("[fasth3] ready", flush=True)


def run_job(job_id: str):
    job = JOBS[job_id]
    job["t0"] = time.time()
    try:
        if job.get("cancel"):
            raise JobCancelled("已取消")
        req = job["request"]
        seconds = float(req.get("seconds") or 5)
        width, height, frames = parse_size(req.get("size"), seconds)
        if req.get("num_frames"):
            frames = align_frames(int(req["num_frames"]))
        prompt = req.get("prompt") or ""
        seed = int(req.get("seed") or 1000)
        dest = OUT_DIR / f"{job_id}.mp4"
        OUT_DIR.mkdir(parents=True, exist_ok=True)
        job["status"] = "in_progress"
        job["progress"] = 20
        print(f"[fasth3] infer {job_id} {width}x{height} frames={frames}", flush=True)
        kwargs = {
            "prompt": prompt,
            "guidance_scale": float(req.get("guidance_scale") or 1.0),
            "height": height,
            "width": width,
            "num_frames": frames,
            "seed": seed,
            "output_path": str(OUT_DIR),
            "output_video_name": job_id,
            "save_video": True,
            "return_frames": False,
        }
        result = GEN.generate_video(**kwargs)
        path = result_path(result, dest)
        if path.resolve() != dest.resolve():
            dest.write_bytes(path.read_bytes())
            path = dest
        job["path"] = str(path)
        job["status"] = "completed"
        job["progress"] = 100
    except JobCancelled:
        job["status"] = "cancelled"
        job["error"] = {"message": "已取消"}
        print(f"[fasth3] cancelled {job_id}", flush=True)
    except Exception as exc:
        job["status"] = "failed"
        job["error"] = {"message": str(exc)}
        job["trace"] = traceback.format_exc()
        print(job["trace"], flush=True)


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("[fasth3]", fmt % args)

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
                "model": "fasth3",
                "ready": GEN is not None,
                "busy": current is not None,
                "progress": (current or {}).get("progress", 0),
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
        if path not in ("/v1/videos", "/v1/videos/generations"):
            self._json(404, {"error": "not found"})
            return
        body = json.loads(raw or b"{}")
        job_id = uuid.uuid4().hex
        job = {"id": job_id, "status": "queued", "progress": 5, "request": body}
        with LOCK:
            JOBS[job_id] = job
        INFER_Q.put(job_id)
        self._json(200, {"id": job_id, "object": "video", "status": "queued", **text_encoder_payload()})

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
    load_gen()
    httpd = ThreadingHTTPServer((HOST, PORT), Handler)
    threading.Thread(target=httpd.serve_forever, daemon=True, name="fasth3-http").start()
    print(f"[fasth3] listening on http://{HOST}:{PORT}", flush=True)
    while True:
        run_job(INFER_Q.get())


if __name__ == "__main__":
    main()
