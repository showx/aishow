@echo off
rem Copy to paths.bat and edit. Do not commit paths.bat.
rem
rem Set only MODELS_ROOT. Weights, ComfyUI, DiffSynth, the LLaDA repo,
rem virtualenvs, Hugging Face / ModelScope caches, and sidecar output
rem all live in this one folder. Copy the folder to move machines.
rem
rem   MiniMax-H3\                          H3 / ComfyUI / DiffSynth
rem   fasth3-gguf\  pinkcherry-h3\
rem   Qwen-Image-2.1\  qwen-image-venv\
rem   HunyuanVideo-1.5-480p-t2v\  HunyuanVideo-1.5-480p-i2v\  hunyuan-video-venv\
rem   LTX-2.3\  LTX-2\  gemma-3-12b-it-qat-q4_0-unquantized\
rem   LLaDA-Image\  LLaDA-Image-Turbo\
rem   huggingface\  modelscope\  out\

set "MODELS_ROOT=C:\path\to\models"

rem Derived by scripts\_env.bat. Uncomment only to override.
rem set "H3_ROOT=%MODELS_ROOT%\MiniMax-H3"
rem set "COMFY_ROOT=%H3_ROOT%\ComfyUI_windows_portable"
rem set "DIFFSYNTH_ROOT=%H3_ROOT%\repo\DiffSynth-Studio-main"
rem set "LLADA_REPO=%MODELS_ROOT%\LLaDA-Image"
rem set "LLADA_MODEL=%MODELS_ROOT%\LLaDA-Image-Turbo"
rem set "HF_HOME=%MODELS_ROOT%\huggingface"
rem set "MODELSCOPE_CACHE=%MODELS_ROOT%\modelscope"
rem set "QWEN_IMAGE_OFFLOAD=1"

rem Optional tools that do not belong in the models folder.
rem set "ARIA2C=C:\path\to\aria2c.exe"
rem set "AISHOW_DOWNLOAD_PROXY=http://127.0.0.1:7897"
rem set "HF_TOKEN="
