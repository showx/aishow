"""Bridge MiniMax H3 helpers across ComfyUI layouts.

TimelineDirector (and other H3 plugins) call
``comfy_extras.nodes_minimax_h3._encode_ref_audio``. ComfyUI master moved that
helper to module level in 2026-08; portable builds through ~0.33 still keep it
as ``MiniMaxH3ReferenceToVideo._encode_ref_audio``. Alias whichever exists so
``MiniMaxH3FiniteSegmentSampler`` does not raise AttributeError.
"""

NODE_CLASS_MAPPINGS = {}
NODE_DISPLAY_NAME_MAPPINGS = {}

try:
    from comfy_extras import nodes_minimax_h3 as h3
except Exception as exc:
    print(f"[aishow-h3-compat] skip: {exc}", flush=True)
else:
    if not hasattr(h3, "_encode_ref_audio"):
        fn = getattr(getattr(h3, "MiniMaxH3ReferenceToVideo", None), "_encode_ref_audio", None)
        if fn is not None:
            h3._encode_ref_audio = fn
            print(
                "[aishow-h3-compat] aliased _encode_ref_audio from MiniMaxH3ReferenceToVideo",
                flush=True,
            )
        else:
            print(
                "[aishow-h3-compat] this ComfyUI has no _encode_ref_audio "
                "(module-level or MiniMaxH3ReferenceToVideo)",
                flush=True,
            )
