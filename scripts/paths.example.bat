@echo off
rem 复制为本目录的 paths.bat 后按本机修改。不要提交 paths.bat。
rem 启动脚本和下载脚本都会加载这份文件。

rem ---- 必填：本机工具与权重根目录 ----
set "H3_ROOT=C:\path\to\MiniMax-H3"
set "COMFY_ROOT=%H3_ROOT%\ComfyUI_windows_portable"
set "DIFFSYNTH_ROOT=%H3_ROOT%\repo\DiffSynth-Studio-main"
set "MODELS_ROOT=C:\path\to\models"

rem ---- 常用派生（一般不用改）----
rem set "H3_NF4_DIR=%H3_ROOT%\models\MiniMax-H3-NF4"
rem set "H3_PROCESSOR=%H3_ROOT%\models\MiniMax-H3\FL2VA\processor"
rem set "H3_LORA_DIR=%H3_ROOT%\models\loras"
rem set "FASTH3_GGUF_ROOT=%MODELS_ROOT%\fasth3-gguf"
rem set "H3_PINKCHERRY_ROOT=%MODELS_ROOT%\pinkcherry-h3"
rem set "FASTH3_LOCAL_DIR=%MODELS_ROOT%\FastVideo-Minimax-FastH3-Preview-v0.2"

rem ---- LLaDA-Image（生图才需要）----
rem set "LLADA_REPO=C:\path\to\LLaDA-Image"
rem set "LLADA_MODEL=%MODELS_ROOT%\LLaDA-Image-Turbo"
rem set "LLADA_PYTHON=%COMFY_ROOT%\python_embeded\python.exe"

rem ---- HunyuanVideo-1.5 / LTX-2.3（视频才需要）----
rem set "HUNYUAN_T2V_MODEL=%MODELS_ROOT%\HunyuanVideo-1.5-480p-t2v"
rem set "HUNYUAN_I2V_MODEL=%MODELS_ROOT%\HunyuanVideo-1.5-480p-i2v"
rem set "HUNYUAN_PYTHON=%MODELS_ROOT%\hunyuan-video-venv\Scripts\python.exe"
rem set "LTX23_ROOT=%MODELS_ROOT%\LTX-2.3"
rem set "LTX23_GEMMA=%MODELS_ROOT%\gemma-3-12b-it-qat-q4_0-unquantized"
rem set "LTX23_REPO=%MODELS_ROOT%\LTX-2"
rem set "LTX23_PYTHON=%LTX23_REPO%\.venv\Scripts\python.exe"

rem ---- Qwen-Image-2.1（生图才需要；独立 venv，不要复用 ComfyUI / LLaDA Python）----
rem set "QWEN_IMAGE_MODEL=%MODELS_ROOT%\Qwen-Image-2.1"
rem set "QWEN_IMAGE_PYTHON=%MODELS_ROOT%\qwen-image-venv\Scripts\python.exe"
rem set "QWEN_IMAGE_OFFLOAD=1"

rem ---- 可选：缓存、代理、aria2 ----
rem set "AISHOW_TMP=%H3_ROOT%\tmp"
rem set "HF_HOME=%MODELS_ROOT%\huggingface"
rem set "MODELSCOPE_CACHE=%MODELS_ROOT%\modelscope"
rem set "ARIA2C=C:\path\to\aria2c.exe"
rem set "AISHOW_DOWNLOAD_PROXY=http://127.0.0.1:7897"
