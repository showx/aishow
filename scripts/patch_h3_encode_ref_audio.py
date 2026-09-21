"""Make TimelineDirector accept both ComfyUI _encode_ref_audio layouts.

ComfyUI portable through ~0.33 keeps the helper on MiniMaxH3ReferenceToVideo.
Master (2026-08) moved it to module level. The plugin always calls
``h3_nodes._encode_ref_audio`` and crashes on the older layout.

Usage:
  python patch_h3_encode_ref_audio.py <plugin-dir>
"""
from __future__ import annotations

import sys
from pathlib import Path

MARKER = "def _aishow_encode_ref_audio("
HELPER = '''
def _aishow_encode_ref_audio(audio_vae, audio):
    """Resolve ComfyUI's ref-audio encoder across layouts.

    Module-level on ComfyUI master (2026-08); class staticmethod on <=0.33.
    """
    fn = getattr(h3_nodes, "_encode_ref_audio", None)
    if fn is None:
        fn = getattr(
            getattr(h3_nodes, "MiniMaxH3ReferenceToVideo", None),
            "_encode_ref_audio",
            None,
        )
    if fn is None:
        raise RuntimeError(
            "This ComfyUI has no _encode_ref_audio (module-level or "
            "MiniMaxH3ReferenceToVideo). Update ComfyUI and restart port 8188."
        )
    return fn(audio_vae, audio)

'''
IMPORT = "from comfy_extras import nodes_minimax_h3 as h3_nodes\n"
ANCHOR = "from server import PromptServer\n"
OLD_CALL = "h3_nodes._encode_ref_audio("
NEW_CALL = "_aishow_encode_ref_audio("


def patch(path: Path) -> str:
    text = path.read_text(encoding="utf-8")
    if IMPORT not in text:
        return "skip-layout"
    changed = False
    if MARKER not in text:
        if ANCHOR in text:
            text = text.replace(ANCHOR, ANCHOR + "\n" + HELPER.lstrip("\n"), 1)
        else:
            text = text.replace(IMPORT, IMPORT + HELPER, 1)
        changed = True
    if OLD_CALL in text:
        text = text.replace(OLD_CALL, NEW_CALL)
        changed = True
    if not changed:
        return "skip-already"
    path.write_text(text, encoding="utf-8")
    return "patched"


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: patch_h3_encode_ref_audio.py <plugin-dir>")
        return 2
    plugin = Path(sys.argv[1])
    path = plugin / "minimax_h3_timeline_director.py"
    if not path.is_file():
        print(f"[aishow] 找不到 {path}")
        return 1
    result = patch(path)
    print(f"[aishow] TimelineDirector _encode_ref_audio: {result}")
    return 0 if result != "skip-layout" else 1


if __name__ == "__main__":
    raise SystemExit(main())
