@echo off
setlocal EnableDelayedExpansion
rem FastH3 GGUF Q4 for Aishow: ComfyUI (8188) + /v1/videos sidecar (8000).
rem Weights: F:\models\fasth3-gguf
rem ComfyUI: E:\MiniMax-H3\ComfyUI_windows_portable

set "COMFY_ROOT=E:\MiniMax-H3\ComfyUI_windows_portable"
set "COMFY_PY=%COMFY_ROOT%\python_embeded\python.exe"
set "FASTH3_HOST=127.0.0.1"
set "FASTH3_PORT=8000"
set "FASTH3_COMFY_URL=http://127.0.0.1:8188"
set "FASTH3_OUT_DIR=F:\models\aishow-fasth3-out"
set "FASTH3_COMFY_OUTPUT=%COMFY_ROOT%\ComfyUI\output"
set "PYTHONUNBUFFERED=1"
set "TEMP=E:\MiniMax-H3\tmp"
set "TMP=E:\MiniMax-H3\tmp"
set "TRITON_CACHE_DIR=E:\MiniMax-H3\tmp\triton-cache"
set "TRITON_HOME=E:\MiniMax-H3\tmp\triton-home"
if not exist "%TEMP%" mkdir "%TEMP%"
if not exist "%TRITON_CACHE_DIR%" mkdir "%TRITON_CACHE_DIR%"

if not exist "%COMFY_PY%" (
  echo 找不到 ComfyUI Python: %COMFY_PY%
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "F:\models\fasth3-gguf\diffusion_models\FastH3-comfy-Q4_K_M.gguf" (
  echo 找不到 FastH3 Q4 GGUF。请先跑 scripts\download_fasth3_gguf.ps1
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

curl.exe -s -o nul -m 3 http://127.0.0.1:8188/system_stats
if not errorlevel 1 goto comfy_ok

echo 启动 ComfyUI  ->  http://127.0.0.1:8188
start "FastH3 ComfyUI" /D "%COMFY_ROOT%" "%COMFY_PY%" -s ComfyUI\main.py --windows-standalone-build --listen 127.0.0.1 --port 8188 --fast fp16_accumulation
echo 等待 ComfyUI 就绪...
set /a _n=0
:wait_comfy
timeout /t 2 /nobreak >nul
curl.exe -s -o nul -m 3 http://127.0.0.1:8188/system_stats
if not errorlevel 1 goto comfy_ok
set /a _n+=1
if !_n! GEQ 90 (
  echo ComfyUI 90 秒还没起来，请看「FastH3 ComfyUI」窗口。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
goto wait_comfy

:comfy_ok
echo ComfyUI 已就绪  ->  http://127.0.0.1:8188
echo FastH3 GGUF 边车  ->  http://127.0.0.1:8000
echo 工坊选「FastH3 本地」，推理模式 auto。
echo.
cd /d D:\code\aishow
"%COMFY_PY%" D:\code\aishow\backend\python\fasth3_gguf_server.py
echo.
echo FastH3 GGUF 边车已退出。
if /i not "%AISHOW_HEADLESS%"=="1" pause
