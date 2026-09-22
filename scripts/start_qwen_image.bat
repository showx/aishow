@echo off
setlocal
rem Qwen-Image-2.1 sidecar. Set QWEN_IMAGE_MODEL / QWEN_IMAGE_PYTHON in scripts\paths.bat.
rem Needs transformers>=5.17 and git diffusers. Do not reuse ComfyUI / LLaDA Python.

call "%~dp0_env.bat"
if not defined QWEN_IMAGE_MODEL (
  if defined MODELS_ROOT set "QWEN_IMAGE_MODEL=%MODELS_ROOT%\Qwen-Image-2.1"
)
if not defined QWEN_IMAGE_PYTHON (
  if defined MODELS_ROOT if exist "%MODELS_ROOT%\qwen-image-venv\Scripts\python.exe" (
    set "QWEN_IMAGE_PYTHON=%MODELS_ROOT%\qwen-image-venv\Scripts\python.exe"
  )
)
if not defined QWEN_IMAGE_PYTHON (
  echo 缺少 QWEN_IMAGE_PYTHON。
  echo 请先运行: powershell -ExecutionPolicy Bypass -File scripts\setup_qwen_image.ps1
  echo 不要复用 ComfyUI / LLaDA 的 Python（transformers 版本不够）。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not defined QWEN_IMAGE_MODEL (
  echo 缺少 QWEN_IMAGE_MODEL。请在 scripts\paths.bat 填写本地权重目录。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not defined QWEN_IMAGE_HOST set "QWEN_IMAGE_HOST=127.0.0.1"
if not defined QWEN_IMAGE_PORT set "QWEN_IMAGE_PORT=30021"
if not defined QWEN_IMAGE_OUT_DIR set "QWEN_IMAGE_OUT_DIR=%AISHOW_ROOT%\backend\data\sidecar-out\qwen-image"
if not defined QWEN_IMAGE_OFFLOAD set "QWEN_IMAGE_OFFLOAD=1"
set "PYTHONUNBUFFERED=1"
set "HF_HUB_OFFLINE=1"
set "TRANSFORMERS_OFFLINE=1"
set "PYTHONPATH=%AISHOW_ROOT%\backend\python;%PYTHONPATH%"

if not exist "%QWEN_IMAGE_MODEL%\model_index.json" (
  echo 找不到权重: %QWEN_IMAGE_MODEL%
  echo 请先运行: powershell -ExecutionPolicy Bypass -File scripts\download_qwen_image.ps1
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

cd /d "%AISHOW_ROOT%"
echo Python: %QWEN_IMAGE_PYTHON%
echo 权重: %QWEN_IMAGE_MODEL%
echo 地址: http://%QWEN_IMAGE_HOST%:%QWEN_IMAGE_PORT%
if defined AISHOW_SIDECAR_LOG (
  for %%I in ("%AISHOW_SIDECAR_LOG%") do if not exist "%%~dpI" mkdir "%%~dpI"
  echo 日志: %AISHOW_SIDECAR_LOG%
  echo ===== %date% %time% start =====>> "%AISHOW_SIDECAR_LOG%"
  "%QWEN_IMAGE_PYTHON%" "%AISHOW_ROOT%\backend\python\qwen_image_server.py" >> "%AISHOW_SIDECAR_LOG%" 2>&1
) else (
  "%QWEN_IMAGE_PYTHON%" "%AISHOW_ROOT%\backend\python\qwen_image_server.py"
)
echo.
echo 边车已退出。看到 ready / listening 之前请不要关窗口。
if defined AISHOW_SIDECAR_LOG echo 完整日志: %AISHOW_SIDECAR_LOG%
if /i not "%AISHOW_HEADLESS%"=="1" pause
