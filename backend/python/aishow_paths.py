"""Portable path helpers for Aishow Python sidecars.

Prefer environment variables from scripts/paths.bat.
Fall back to repo-relative locations so clones work without machine paths.
"""
from __future__ import annotations

import os
from pathlib import Path

BACKEND = Path(__file__).resolve().parents[1]
REPO = BACKEND.parent


def env_str(name: str, default: str = "") -> str:
    return os.environ.get(name, "").strip() or default


def env_path(name: str, default: str | Path | None = None, required: bool = False) -> Path:
    raw = env_str(name)
    if raw:
        return Path(raw)
    if default is not None:
        return Path(default)
    if required:
        raise SystemExit(
            f"缺少环境变量 {name}。请复制 scripts/paths.example.bat 为 scripts/paths.bat 并填写本机路径。"
        )
    return Path()


def media_root() -> Path:
    for key in ("H3_MEDIA_ROOT", "LLADA_MEDIA_ROOT", "AISHOW_MEDIA_ROOT"):
        raw = env_str(key)
        if raw:
            return Path(raw)
    return BACKEND / "data" / "media"


def sidecar_out(name: str) -> Path:
    root = env_str("MODELS_ROOT")
    if root:
        return Path(root) / "out" / name
    return BACKEND / "data" / "sidecar-out" / name


def path_list(name: str, defaults: list[Path] | None = None) -> list[Path]:
    raw = env_str(name)
    if raw:
        return [Path(p.strip()) for p in raw.split(os.pathsep) if p.strip()]
    out: list[Path] = []
    for p in defaults or []:
        if p and str(p).strip() and p != Path():
            out.append(Path(p))
    return out
