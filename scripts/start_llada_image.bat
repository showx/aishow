@echo off
setlocal
rem LLaDA-Image 边车。源码与依赖在 F:\LLaDA-Image，复用 ComfyUI 的 CUDA PyTorch。
rem 默认 Turbo。换 Base：set LLADA_MODEL=F:\models\LLaDA-Image

if not defined LLADA_PYTHON set "LLADA_PYTHON=E:\MiniMax-H3\ComfyUI_windows_portable\python_embeded\python.exe"
if not defined LLADA_REPO set "LLADA_REPO=F:\LLaDA-Image"
if not defined LLADA_PYDEPS set "LLADA_PYDEPS=%LLADA_REPO%\pydeps"
if not defined LLADA_MODEL set "LLADA_MODEL=F:\models\LLaDA-Image-Turbo"
if not defined LLADA_OUT_DIR set "LLADA_OUT_DIR=%LLADA_REPO%\tmp\aishow-jobs"
if not defined LLADA_MEDIA_ROOT set "LLADA_MEDIA_ROOT=D:\code\aishow\backend\data\media"
if not defined LLADA_HOST set "LLADA_HOST=127.0.0.1"
if not defined LLADA_PORT set "LLADA_PORT=30020"
set "PYTHONUNBUFFERED=1"
set "PYTHONPATH=%LLADA_PYDEPS%;%LLADA_REPO%;%PYTHONPATH%"
set "HF_HOME=F:\huggingface"
set "MODELSCOPE_CACHE=F:\modelscope"
set "TMP=F:\tmp"
set "TEMP=F:\tmp"

if not exist "%LLADA_REPO%\src" (
  echo 找不到 LLaDA-Image 源码: %LLADA_REPO%
  pause
  exit /b 1
)
if not exist "%LLADA_MODEL%\model_index.json" (
  echo 找不到权重: %LLADA_MODEL%
  echo 请先等权重下载完成，或把 LLADA_MODEL 指到本地 snapshot。
  pause
  exit /b 1
)

cd /d D:\code\aishow
"%LLADA_PYTHON%" D:\code\aishow\backend\python\llada_image_server.py
echo.
echo 边车已退出。看到 ready / listening 之前请不要关窗口。
pause
