@echo off
setlocal
rem Full FastVideo bf16 FastH3. 24GB cards should use start_fasth3_gguf.bat.

call "%~dp0_env.bat"

if defined FASTH3_PYTHON set "PYTHON=%FASTH3_PYTHON%"
if not defined PYTHON if defined FASTVIDEO_ROOT if exist "%FASTVIDEO_ROOT%\.venv\Scripts\python.exe" (
  set "PYTHON=%FASTVIDEO_ROOT%\.venv\Scripts\python.exe"
)
if not defined PYTHON set "PYTHON=python"

if defined FASTVIDEO_ROOT (
  if exist "%FASTVIDEO_ROOT%\examples\serving\openai_fasth3.yaml" (
    echo Using official FastVideo serve from %FASTVIDEO_ROOT%
    cd /d "%FASTVIDEO_ROOT%"
    set "FASTVIDEO_DMD_DENOISING_STEPS=999,749,500,250"
    set "FASTVIDEO_ATTENTION_BACKEND=VIDEO_SPARSE_ATTN_H3"
    fastvideo serve --config "%~dp0openai_fasth3.yaml" --server.host 127.0.0.1
    goto :done
  )
)

set "FASTH3_HOST=127.0.0.1"
set "FASTH3_PORT=8000"
set "FASTH3_NUM_GPUS=1"
set "FASTH3_MODEL=FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2"
if not defined FASTH3_LOCAL_DIR if defined H3_ROOT if exist "%H3_ROOT%\models\FastVideo-Minimax-FastH3-Preview-v0.2" (
  set "FASTH3_LOCAL_DIR=%H3_ROOT%\models\FastVideo-Minimax-FastH3-Preview-v0.2"
)
set "FASTVIDEO_DMD_DENOISING_STEPS=999,749,500,250"
set "FASTVIDEO_ATTENTION_BACKEND=VIDEO_SPARSE_ATTN_H3"
set "FASTH3_VSA_SPARSITY=0.9"
set "FASTH3_VSA_TILE=64"
set "PYTHONUNBUFFERED=1"
cd /d "%AISHOW_ROOT%"
"%PYTHON%" "%AISHOW_ROOT%\backend\python\fasth3_server.py"

:done
echo.
echo FastH3 边车已退出。看到 ready / listening 之前请不要关窗口。
if /i not "%AISHOW_HEADLESS%"=="1" pause
