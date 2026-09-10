@echo off
setlocal EnableDelayedExpansion
rem ComfyUI Ref2VA INT8 sidecar for Aishow (port 30011).
rem ComfyUI: E:\MiniMax-H3\ComfyUI_windows_portable  :8188
rem UNET: F:\models\fasth3-gguf\diffusion_models\minimax_h3_ref2va_pruned_int8_convrot.safetensors

set "COMFY_ROOT=E:\MiniMax-H3\ComfyUI_windows_portable"
set "COMFY_PY=%COMFY_ROOT%\python_embeded\python.exe"
set "H3_REF2VA_HOST=127.0.0.1"
set "H3_REF2VA_PORT=30011"
set "H3_REF2VA_COMFY_URL=http://127.0.0.1:8188"
set "H3_REF2VA_OUT_DIR=F:\models\aishow-fasth3-out"
set "H3_REF2VA_COMFY_ROOT=%COMFY_ROOT%\ComfyUI"
set "H3_REF2VA_COMFY_OUTPUT=%COMFY_ROOT%\ComfyUI\output"
set "H3_REF2VA_COMFY_INPUT=%COMFY_ROOT%\ComfyUI\input"
set "H3_MEDIA_ROOT=D:\code\aishow\backend\data\media"
if exist "F:\models\fasth3-gguf\text_encoders\qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors" (
  set "H3_REF2VA_CLIP=qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors"
) else (
  set "H3_REF2VA_CLIP=qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors"
)
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
if not exist "F:\models\fasth3-gguf\diffusion_models\minimax_h3_ref2va_pruned_int8_convrot.safetensors" (
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
cd /d D:\code\aishow
"%COMFY_PY%" D:\code\aishow\backend\python\ref2va_int8_server.py
echo.
echo Ref2VA INT8 边车已退出。
if /i not "%AISHOW_HEADLESS%"=="1" pause
