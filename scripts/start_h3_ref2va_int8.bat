@echo off
setlocal EnableDelayedExpansion
rem ComfyUI Ref2VA INT8 sidecar (port 30011).

call "%~dp0_env.bat"
if not defined COMFY_ROOT (
  echo 缺少 COMFY_ROOT。请复制 scripts\paths.example.bat 为 scripts\paths.bat 并填写。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not defined FASTH3_GGUF_ROOT (
  echo 缺少 FASTH3_GGUF_ROOT / MODELS_ROOT。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

set "H3_REF2VA_HOST=127.0.0.1"
set "H3_REF2VA_PORT=30011"
set "H3_REF2VA_COMFY_URL=http://127.0.0.1:8188"
set "PYTHONUNBUFFERED=1"
if exist "%FASTH3_GGUF_ROOT%\text_encoders\qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors" (
  set "H3_REF2VA_CLIP=qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors"
) else (
  set "H3_REF2VA_CLIP=qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors"
)
if defined AISHOW_TMP if not exist "%AISHOW_TMP%" mkdir "%AISHOW_TMP%"
if defined TRITON_CACHE_DIR if not exist "%TRITON_CACHE_DIR%" mkdir "%TRITON_CACHE_DIR%"

if not exist "%COMFY_PY%" (
  echo 找不到 ComfyUI Python: %COMFY_PY%
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%FASTH3_GGUF_ROOT%\diffusion_models\minimax_h3_ref2va_pruned_int8_convrot.safetensors" (
  echo 找不到 Ref2VA INT8。请先跑 scripts\download_h3_ref2va.ps1
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

curl.exe -s -o nul -m 3 http://127.0.0.1:8188/system_stats
if not errorlevel 1 goto comfy_ok

echo 启动 ComfyUI  -^>  http://127.0.0.1:8188
start "H3 Ref2VA ComfyUI" /D "%COMFY_ROOT%" "%COMFY_PY%" -s ComfyUI\main.py --windows-standalone-build --listen 127.0.0.1 --port 8188 --fast fp16_accumulation
echo 等待 ComfyUI 就绪...
set /a _n=0
:wait_comfy
timeout /t 2 /nobreak >nul
curl.exe -s -o nul -m 3 http://127.0.0.1:8188/system_stats
if not errorlevel 1 goto comfy_ok
set /a _n+=1
if !_n! GEQ 90 (
  echo ComfyUI 90 秒还没起来，请看「H3 Ref2VA ComfyUI」窗口。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
goto wait_comfy

:comfy_ok
echo ComfyUI 已就绪  -^>  http://127.0.0.1:8188
echo Ref2VA INT8 边车  -^>  http://127.0.0.1:30011
echo 工坊选「H3-Base 本地」+「参考生成」，推理模式 auto。
echo.
cd /d "%AISHOW_ROOT%"
"%COMFY_PY%" "%AISHOW_ROOT%\backend\python\ref2va_int8_server.py"
echo.
echo Ref2VA INT8 边车已退出。
if /i not "%AISHOW_HEADLESS%"=="1" pause
