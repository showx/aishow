@echo off
setlocal
rem HunyuanVideo-1.5 480p sidecar. Weights from download_hunyuan_video.ps1.

call "%~dp0_env.bat"
if not defined HUNYUAN_T2V_MODEL if defined MODELS_ROOT set "HUNYUAN_T2V_MODEL=%MODELS_ROOT%\HunyuanVideo-1.5-480p-t2v"
if not defined HUNYUAN_I2V_MODEL if defined MODELS_ROOT set "HUNYUAN_I2V_MODEL=%MODELS_ROOT%\HunyuanVideo-1.5-480p-i2v"
if not defined HUNYUAN_PYTHON (
  if defined MODELS_ROOT if exist "%MODELS_ROOT%\hunyuan-video-venv\Scripts\python.exe" (
    set "HUNYUAN_PYTHON=%MODELS_ROOT%\hunyuan-video-venv\Scripts\python.exe"
  )
)
if not defined HUNYUAN_PYTHON (
  echo 缺少 HUNYUAN_PYTHON。
  echo 请先运行: powershell -ExecutionPolicy Bypass -File scripts\setup_hunyuan_video.ps1
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%HUNYUAN_T2V_MODEL%\model_index.json" (
  echo 找不到文生权重: %HUNYUAN_T2V_MODEL%
  echo 请先运行: powershell -ExecutionPolicy Bypass -File scripts\download_hunyuan_video.ps1
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not defined HUNYUAN_HOST set "HUNYUAN_HOST=127.0.0.1"
if not defined HUNYUAN_PORT set "HUNYUAN_PORT=30022"
if not defined HUNYUAN_OUT_DIR set "HUNYUAN_OUT_DIR=%AISHOW_ROOT%\backend\data\sidecar-out\hunyuan-video"
if not defined HUNYUAN_OFFLOAD set "HUNYUAN_OFFLOAD=1"
set "PYTHONUNBUFFERED=1"
set "HF_HUB_OFFLINE=1"
set "TRANSFORMERS_OFFLINE=1"
set "PYTORCH_CUDA_ALLOC_CONF=expandable_segments:True"
set "PYTHONPATH=%AISHOW_ROOT%\backend\python;%PYTHONPATH%"

cd /d "%AISHOW_ROOT%"
echo Python: %HUNYUAN_PYTHON%
echo 文生: %HUNYUAN_T2V_MODEL%
echo 图生: %HUNYUAN_I2V_MODEL%
echo 地址: http://%HUNYUAN_HOST%:%HUNYUAN_PORT%
if defined AISHOW_SIDECAR_LOG (
  for %%I in ("%AISHOW_SIDECAR_LOG%") do if not exist "%%~dpI" mkdir "%%~dpI"
  echo 日志: %AISHOW_SIDECAR_LOG%
  echo ===== %date% %time% start =====>> "%AISHOW_SIDECAR_LOG%"
  "%HUNYUAN_PYTHON%" "%AISHOW_ROOT%\backend\python\hunyuan_video_server.py" >> "%AISHOW_SIDECAR_LOG%" 2>&1
) else (
  "%HUNYUAN_PYTHON%" "%AISHOW_ROOT%\backend\python\hunyuan_video_server.py"
)
echo.
echo 边车已退出。看到 ready / listening 之前请不要关窗口。
if defined AISHOW_SIDECAR_LOG echo 完整日志: %AISHOW_SIDECAR_LOG%
if /i not "%AISHOW_HEADLESS%"=="1" pause
