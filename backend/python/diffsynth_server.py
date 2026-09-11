"""DiffSynth MiniMax-H3 NF4 sidecar.

Exposes the same /v1/videos endpoints Aishow's Go worker already uses.
Load local NF4 FL2VA weights; Ref2VA is optional if that file exists.

Set H3_TURBO=1 to fold lightx2v / larry Turbo LoRA and sample in 4–6 steps
(the Pepe104 MiniMax-H3-Turbo-Lora Space, but quantized for 24GB).
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
from urllib.parse import urlparse
from urllib.request import urlopen

import torch
from PIL import Image

from diffsynth.pipelines.minimax_h3_audio_video import MiniMaxH3Pipeline, ModelConfig
from diffsynth.utils.data.audio_video import write_video_audio

from aishow_paths import env_path, media_root

ROOT = env_path("H3_ROOT", required=True)
NF4 = env_path("H3_NF4_DIR", ROOT / "models" / "MiniMax-H3-NF4")
PROCESSOR = env_path("H3_PROCESSOR", ROOT / "models" / "MiniMax-H3" / "FL2VA" / "processor")
TURBO = os.environ.get("H3_TURBO", "0").lower() in ("1", "true", "yes", "on")
HOST = os.environ.get("H3_HOST", "127.0.0.1")
PORT = int(os.environ.get("H3_PORT", "30012" if TURBO else "30010"))
OUT_DIR = env_path("H3_OUT_DIR", ROOT / "tmp" / "aishow-jobs")
MEDIA_ROOT = media_root()
LORA_DIR = env_path("H3_LORA_DIR", ROOT / "models" / "loras")
LORA_ALPHA = float(os.environ.get("H3_LORA_STRENGTH", "1.0"))

FL2VA = NF4 / "minimax-h3-fl2va-nf4.safetensors"
TE = NF4 / "minimax-h3-text-encoder-nf4.safetensors"
VVAE = NF4 / "video_vae_nf4.safetensors"
AVAE = NF4 / "audio_vae_nf4.safetensors"
LORA_CANDIDATES = [
    LORA_DIR / "minimax_h3_fl2v_turbo_4step_v1.2_768p_bf16.safetensors",
    LORA_DIR / "minimax_h3_fl2v_turbo_4step_v1.1_768p_bf16.safetensors",
    LORA_DIR / "minimax_h3_fl2v_turbo_4step_v1.0_768p_bf16.safetensors",
    LORA_DIR / "minimax_h3_fl2v_turbo_4step_v0.1.safetensors",
    LORA_DIR / "minimax_h3_turbo_4step_ema_ckpt850.safetensors",
]

JOBS: dict[str, dict] = {}
LOCK = threading.Lock()
INFER_Q: queue.Queue[str] = queue.Queue()
PIPE = None
LORA_FILE = ""
MAX_SHORT = int(os.environ.get("H3_MAX_SHORT_EDGE", "768" if TURBO else "1080"))
TEXT_ENCODER = TE.name
TEXT_ENCODER_LABEL = "Qwen3-VL 32B NF4"
MODEL_NAME = "MiniMax-H3-Turbo-LoRA" if TURBO else "MiniMax-H3-NF4"
LOG_TAG = "h3-turbo" if TURBO else "h3-nf4"


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
    return n


def canvas(aspect: str, short: int) -> tuple[int, int]:
    short = round32(min(short or 768, MAX_SHORT))
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
    h, w = table.get(aspect, (short, round32(int(short * 16 / 9))))
    return h, w


def parse_uri(uri: str) -> Path:
    if not uri:
        raise ValueError("empty uri")
    if uri.startswith("http://") or uri.startswith("https://"):
        dest = OUT_DIR / f"dl-{uuid.uuid4().hex}{Path(urlparse(uri).path).suffix or '.bin'}"
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


def vram_config() -> dict:
    # Official DiffSynth low-VRAM path: stay on disk until the CUDA prepare step.
    return {
        "offload_dtype": "disk",
        "offload_device": "disk",
        "onload_dtype": "disk",
        "onload_device": "disk",
        "preparing_dtype": torch.bfloat16,
        "preparing_device": "cuda",
        "computation_dtype": torch.bfloat16,
        "computation_device": "cuda",
    }


def resolve_lora() -> Path:
    forced = os.environ.get("H3_TURBO_LORA", "").strip()
    if forced:
        p = Path(forced)
        if not p.exists():
            raise FileNotFoundError(f"指定的 Turbo LoRA 不存在: {p}")
        return p
    for cand in LORA_CANDIDATES:
        if cand.exists() and cand.stat().st_size > 10 * 1024 * 1024:
            return cand
    raise FileNotFoundError(
        "缺少 Turbo LoRA。请先跑 scripts\\download_h3_turbo_lora.ps1，"
        "或把 lightx2v 的 minimax_h3_fl2v_turbo_4step_v1.2_768p_bf16.safetensors "
        f"放到 {LORA_DIR}"
    )


def load_pipe():
    global PIPE, LORA_FILE
    missing = [p for p in (FL2VA, TE, VVAE, AVAE, PROCESSOR) if not Path(p).exists()]
    if missing:
        raise FileNotFoundError("缺少 NF4 文件: " + ", ".join(str(p) for p in missing))
    lora_path = resolve_lora() if TURBO else None
    cfg = vram_config()
    vram_limit = torch.cuda.mem_get_info("cuda")[1] / (1024 ** 3) - 2
    print(f"[{LOG_TAG}] loading FL2VA, vram_limit={vram_limit:.1f} GB", flush=True)
    print(f"[{LOG_TAG}] dit={FL2VA}", flush=True)
    print(f"[{LOG_TAG}] te={TE} ({TEXT_ENCODER_LABEL})", flush=True)
    print(f"[{LOG_TAG}] processor={PROCESSOR}", flush=True)
    PIPE = MiniMaxH3Pipeline.from_pretrained(
        torch_dtype=torch.bfloat16,
        device="cuda",
        model_configs=[
            ModelConfig(path=str(FL2VA), **cfg),
            ModelConfig(path=str(TE), **cfg),
            ModelConfig(path=str(VVAE), **cfg),
            ModelConfig(path=str(AVAE), **cfg),
        ],
        processor_config=ModelConfig(path=str(PROCESSOR)),
        vram_limit=vram_limit,
    )
    if lora_path is not None:
        print(f"[{LOG_TAG}] lora={lora_path} alpha={LORA_ALPHA}", flush=True)
        PIPE.load_lora(PIPE.dit, str(lora_path), alpha=LORA_ALPHA)
        LORA_FILE = lora_path.name
    print(f"[{LOG_TAG}] ready", flush=True)


class Bar:
    def __init__(self, job: dict):
        self.job = job

    def __call__(self, iterable):
        try:
            total = len(iterable)
        except TypeError:
            total = None
        for i, item in enumerate(iterable, 1):
            if self.job.get("cancel"):
                raise JobCancelled("已取消")
            if total:
                self.job["progress"] = min(90, 22 + int(68 * i / total))
                if not self.job.get("cancel"):
                    self.job["status"] = "in_progress"
            yield item
            if self.job.get("cancel"):
                raise JobCancelled("已取消")


def run_job(job_id: str):
    job = JOBS[job_id]
    job["t0"] = time.time()
    try:
        if job.get("cancel"):
            raise JobCancelled("已取消")
        req = job["request"]
        target = req.get("target") or {}
        seconds = float(target.get("duration_seconds") or req.get("seconds") or 5)
        seconds = min(max(seconds, 2), 15)
        short = int(target.get("short_edge") or 480)
        height, width = canvas(target.get("aspect_ratio"), short)
        frames = align_frames(int(round(seconds * 24)))
        prompt = req.get("prompt") or ""
        steps = int(req.get("num_inference_steps") or (4 if TURBO else 50))
        if TURBO:
            steps = max(4, min(steps, 8))
        seed = int(req.get("seed") or 0)
        kwargs = {
            "prompt": prompt,
            "height": height,
            "width": width,
            "num_frames": frames,
            "num_inference_steps": steps,
            "seed": seed,
            "flow_shift": float(req.get("flow_shift") or (6 if TURBO else 12)),
            "audio_flow_shift": float(req.get("audio_flow_shift") or 3),
            "progress_bar_cmd": Bar(job),
        }
        keyframes = []
        indices = []
        for cond in req.get("conditions") or []:
            if cond.get("role") != "keyframe":
                continue
            img = Image.open(parse_uri(cond.get("uri", ""))).convert("RGB")
            keyframes.append(img)
            indices.append(int(cond.get("frame_index") or 0))
        if keyframes:
            kwargs["keyframes"] = keyframes
            kwargs["keyframe_indices"] = indices

        if job.get("cancel"):
            raise JobCancelled("已取消")
        job["status"] = "in_progress"
        job["progress"] = 20
        print(f"[{LOG_TAG}] infer {job_id} {height}x{width} frames={frames} steps={steps}", flush=True)
        video, audio = PIPE(**kwargs)
        OUT_DIR.mkdir(parents=True, exist_ok=True)
        dest = OUT_DIR / f"{job_id}.mp4"
        write_video_audio(video=video, audio=audio, output_path=str(dest), fps=24, audio_sample_rate=32000)
        job["path"] = str(dest)
        job["status"] = "completed"
        job["progress"] = 100
    except JobCancelled:
        job["status"] = "cancelled"
        job["error"] = {"message": "已取消"}
        print(f"[{LOG_TAG}] cancelled {job_id}", flush=True)
        try:
            torch.cuda.empty_cache()
        except Exception:
            pass
    except Exception as exc:
        job["status"] = "failed"
        job["error"] = {"message": str(exc)}
        job["trace"] = traceback.format_exc()
        print(job["trace"])


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print(f"[{LOG_TAG}]", fmt % args)

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
                "model": MODEL_NAME,
                "ready": PIPE is not None,
                "busy": current is not None,
                "progress": (current or {}).get("progress", 0),
                "elapsed_sec": int(time.time() - current["t0"]) if current and current.get("t0") else 0,
                "lora": LORA_FILE,
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
                "elapsed_sec": int(time.time() - job["t0"]) if job.get("t0") else 0,
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
        body = json.loads(raw or b"{}")
        task = str(body.get("task") or "t2va").lower()
        if task not in ("t2va", "fl2va", "i2va", "l2va"):
            self._json(400, {"error": "当前 NF4 只装了 FL2VA，参考生成需要 minimax-h3-ref2va-nf4.safetensors"})
            return
        job_id = uuid.uuid4().hex
        job = {"id": job_id, "status": "queued", "progress": 5, "request": body}
        with LOCK:
            JOBS[job_id] = job
        INFER_Q.put(job_id)
        self._json(200, {"id": job_id, "status": "queued", **text_encoder_payload()})


def main():
    os.environ.setdefault("DIFFSYNTH_DISK_MAP_BUFFER_SIZE", str(10**12))
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    load_pipe()
    httpd = ThreadingHTTPServer((HOST, PORT), Handler)
    threading.Thread(target=httpd.serve_forever, daemon=True, name="h3-http").start()
    print(f"[{LOG_TAG}] listening on http://{HOST}:{PORT}", flush=True)
    print(f"[{LOG_TAG}] inference runs on the main thread (Windows safetensors)", flush=True)
    while True:
        job_id = INFER_Q.get()
        run_job(job_id)


if __name__ == "__main__":
    main()
