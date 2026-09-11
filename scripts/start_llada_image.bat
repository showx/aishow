@echo off
setlocal
rem LLaDA-Image sidecar. Set LLADA_REPO / LLADA_MODEL in scripts\paths.bat.
rem Default Turbo. Switch to Base: set LLADA_MODEL=inclusionAI/LLaDA-Image

call "%~dp0_env.bat"
if not defined LLADA_REPO (
  echo 缺少 LLADA_REPO。请在 scripts\paths.bat 里填写 LLaDA-Image 源码目录。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not defined LLADA_PYTHON (
  echo 缺少 LLADA_PYTHON。可复用 ComfyUI 的 python_embeded，或本机 CUDA PyTorch。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not defined LLADA_MODEL set "LLADA_MODEL=inclusionAI/LLaDA-Image-Turbo"
if not defined LLADA_HOST set "LLADA_HOST=127.0.0.1"
if not defined LLADA_PORT set "LLADA_PORT=30020"
set "PYTHONUNBUFFERED=1"
set "HF_HUB_OFFLINE=1"
set "TRANSFORMERS_OFFLINE=1"
set "PYTHONPATH=%LLADA_PYDEPS%;%LLADA_REPO%;%PYTHONPATH%"

if not exist "%LLADA_REPO%\src" (
  echo 找不到 LLaDA-Image 源码: %LLADA_REPO%
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%LLADA_MODEL%\model_index.json" (
  echo 找不到权重: %LLADA_MODEL%
  echo 请先下载权重，或把 LLADA_MODEL 指到本地 snapshot。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

cd /d "%AISHOW_ROOT%"
"%LLADA_PYTHON%" "%AISHOW_ROOT%\backend\python\llada_image_server.py"
echo.
echo 边车已退出。看到 ready / listening 之前请不要关窗口。
if /i not "%AISHOW_HEADLESS%"=="1" pause
