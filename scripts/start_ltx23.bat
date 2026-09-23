@echo off
setlocal
rem LTX-2.3 distilled sidecar. Needs the LTX-2 repo venv from setup_ltx23.ps1.

call "%~dp0_env.bat"
if not defined LTX23_ROOT if defined MODELS_ROOT set "LTX23_ROOT=%MODELS_ROOT%\LTX-2.3"
if not defined LTX23_GEMMA if defined MODELS_ROOT set "LTX23_GEMMA=%MODELS_ROOT%\gemma-3-12b-it-qat-q4_0-unquantized"
if not defined LTX23_REPO if defined MODELS_ROOT set "LTX23_REPO=%MODELS_ROOT%\LTX-2"
if not defined LTX23_CHECKPOINT set "LTX23_CHECKPOINT=%LTX23_ROOT%\ltx-2.3-22b-distilled-1.1.safetensors"
if not defined LTX23_UPSCALER set "LTX23_UPSCALER=%LTX23_ROOT%\ltx-2.3-spatial-upscaler-x2-1.1.safetensors"
if not defined LTX23_PYTHON if exist "%LTX23_REPO%\.venv\Scripts\python.exe" set "LTX23_PYTHON=%LTX23_REPO%\.venv\Scripts\python.exe"
if not defined LTX23_PYTHON (
  echo 缺少 LTX23_PYTHON。
  echo 请先运行: powershell -ExecutionPolicy Bypass -File scripts\setup_ltx23.ps1
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%LTX23_CHECKPOINT%" (
  echo 找不到蒸馏权重: %LTX23_CHECKPOINT%
  echo 请先运行: powershell -ExecutionPolicy Bypass -File scripts\download_ltx23.ps1
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%LTX23_UPSCALER%" (
  echo 找不到空间超分: %LTX23_UPSCALER%
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%LTX23_GEMMA%\config.json" (
  echo 找不到 Gemma 文本编码器: %LTX23_GEMMA%
  echo 门控仓库需要先在 Hugging Face 接受协议并设置 HF_TOKEN，再重跑下载脚本。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not defined LTX23_HOST set "LTX23_HOST=127.0.0.1"
if not defined LTX23_PORT set "LTX23_PORT=30023"
if not defined LTX23_OUT_DIR set "LTX23_OUT_DIR=%AISHOW_ROOT%\backend\data\sidecar-out\ltx23"
if not defined LTX23_QUANT set "LTX23_QUANT=fp8-cast"
if not defined LTX23_OFFLOAD set "LTX23_OFFLOAD=cpu"
set "PYTHONUNBUFFERED=1"
set "HF_HUB_OFFLINE=1"
set "TRANSFORMERS_OFFLINE=1"
set "PYTORCH_CUDA_ALLOC_CONF=expandable_segments:True"
set "PYTHONPATH=%AISHOW_ROOT%\backend\python;%LTX23_REPO%\packages\ltx-pipelines\src;%LTX23_REPO%\packages\ltx-core\src;%PYTHONPATH%"

cd /d "%AISHOW_ROOT%"
echo Python: %LTX23_PYTHON%
echo 权重: %LTX23_CHECKPOINT%
echo Gemma: %LTX23_GEMMA%
echo 地址: http://%LTX23_HOST%:%LTX23_PORT%
if defined AISHOW_SIDECAR_LOG (
  for %%I in ("%AISHOW_SIDECAR_LOG%") do if not exist "%%~dpI" mkdir "%%~dpI"
  echo 日志: %AISHOW_SIDECAR_LOG%
  echo ===== %date% %time% start =====>> "%AISHOW_SIDECAR_LOG%"
  "%LTX23_PYTHON%" "%AISHOW_ROOT%\backend\python\ltx23_server.py" >> "%AISHOW_SIDECAR_LOG%" 2>&1
) else (
  "%LTX23_PYTHON%" "%AISHOW_ROOT%\backend\python\ltx23_server.py"
)
echo.
echo 边车已退出。看到 ready / listening 之前请不要关窗口。
if defined AISHOW_SIDECAR_LOG echo 完整日志: %AISHOW_SIDECAR_LOG%
if /i not "%AISHOW_HEADLESS%"=="1" pause
