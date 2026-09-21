@echo off
setlocal EnableDelayedExpansion
rem ComfyUI Timeline Director sidecar (port 30014).
rem SelfLift two-stage sampling via Songssx/ComfyUI-MiniMaxH3-TimelineDirector.

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

set "H3_DIRECTOR_HOST=127.0.0.1"
set "H3_DIRECTOR_PORT=30014"
set "H3_DIRECTOR_COMFY_URL=http://127.0.0.1:8188"
set "PYTHONUNBUFFERED=1"
set "PYTHONPATH=%AISHOW_ROOT%\backend\python;%PYTHONPATH%"
if exist "%FASTH3_GGUF_ROOT%\text_encoders\qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors" (
  set "H3_DIRECTOR_CLIP=qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors"
) else (
  set "H3_DIRECTOR_CLIP=qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors"
)
if defined AISHOW_TMP if not exist "%AISHOW_TMP%" mkdir "%AISHOW_TMP%"
if defined TRITON_CACHE_DIR if not exist "%TRITON_CACHE_DIR%" mkdir "%TRITON_CACHE_DIR%"

if not exist "%COMFY_PY%" (
  echo 找不到 ComfyUI Python: %COMFY_PY%
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

set "UNET_OK="
if exist "%FASTH3_GGUF_ROOT%\diffusion_models\minimax_h3_fused_refdelta_r1024_turbo8_mystic07_int8_convrot.safetensors" set "UNET_OK=1"
if exist "%FASTH3_GGUF_ROOT%\diffusion_models\minimax_h3_ref2va_pruned_int8_convrot.safetensors" set "UNET_OK=1"
if not defined UNET_OK (
  echo 找不到 Ref2VA INT8 / fused turbo UNET。请先跑 scripts\download_h3_ref2va.ps1
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

set "PLUGIN_DIR=%COMFY_ROOT%\ComfyUI\custom_nodes\ComfyUI-MiniMaxH3-TimelineDirector"
if not exist "%PLUGIN_DIR%\__init__.py" (
  echo 克隆 TimelineDirector 插件...
  git clone --depth 1 https://github.com/Songssx/ComfyUI-MiniMaxH3-TimelineDirector.git "%PLUGIN_DIR%"
  if errorlevel 1 (
    echo 克隆失败。请手动把插件放到:
    echo   %PLUGIN_DIR%
    if /i not "%AISHOW_HEADLESS%"=="1" pause
    exit /b 1
  )
  set "PLUGIN_JUST_CLONED=1"
)

set "COMPAT_SRC=%AISHOW_ROOT%\scripts\comfy_h3_compat"
set "COMPAT_DIR=%COMFY_ROOT%\ComfyUI\custom_nodes\aishow_h3_compat"
if exist "%COMPAT_SRC%\__init__.py" (
  if not exist "%COMPAT_DIR%\__init__.py" set "COMPAT_JUST_INSTALLED=1"
  if not exist "%COMPAT_DIR%" mkdir "%COMPAT_DIR%"
  copy /Y "%COMPAT_SRC%\__init__.py" "%COMPAT_DIR%\__init__.py" >nul
)
if exist "%PLUGIN_DIR%\minimax_h3_timeline_director.py" (
  "%COMFY_PY%" "%AISHOW_ROOT%\scripts\patch_h3_encode_ref_audio.py" "%PLUGIN_DIR%"
  if errorlevel 1 echo TimelineDirector 音频编码兼容补丁失败，参考音频可能仍会报 _encode_ref_audio。
)

set "UPSCALE_DIR=%COMFY_ROOT%\ComfyUI\models\latent_upscale_models"
set "HAS_UPSCALER="
if exist "%UPSCALE_DIR%\minimax_h3_latent_upscaler_3d_conv_v1_fp16.safetensors" set "HAS_UPSCALER=1"
if exist "%UPSCALE_DIR%\minimax_h3_latent_upscaler_3d_conv_v1_bf16.safetensors" set "HAS_UPSCALER=1"
if not defined HAS_UPSCALER (
  echo 未找到 H3 Latent Upscaler，二采会关闭、速度提升有限。
  echo 可选：powershell -File scripts\download_h3_latent_upscaler.ps1
)

curl.exe -s -o nul -m 3 http://127.0.0.1:8188/system_stats
if errorlevel 1 goto start_comfy
if defined PLUGIN_JUST_CLONED (
  echo 插件刚装上，但 ComfyUI 已经在跑。请关掉 8188 窗口后重新执行本脚本，否则找不到 Timeline Director 节点。
)
if defined COMPAT_JUST_INSTALLED (
  echo 已装 H3 _encode_ref_audio 兼容补丁，但 ComfyUI 已经在跑。请关掉 8188 窗口后重新执行本脚本。
)
goto comfy_ok

:start_comfy
echo 启动 ComfyUI  -^>  http://127.0.0.1:8188
start "H3 Director ComfyUI" /D "%COMFY_ROOT%" "%COMFY_PY%" -s ComfyUI\main.py --windows-standalone-build --listen 127.0.0.1 --port 8188 --fast fp16_accumulation
echo 等待 ComfyUI 就绪...
set /a _n=0
:wait_comfy
timeout /t 2 /nobreak >nul
curl.exe -s -o nul -m 3 http://127.0.0.1:8188/system_stats
if not errorlevel 1 goto comfy_ok
set /a _n+=1
if !_n! GEQ 90 (
  echo ComfyUI 90 秒还没起来，请看「H3 Director ComfyUI」窗口。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
goto wait_comfy

:comfy_ok
echo ComfyUI 已就绪  -^>  http://127.0.0.1:8188
echo Timeline Director 边车  -^>  http://127.0.0.1:30014
echo 工坊选「H3 Timeline Director」，推理模式 auto。文生 / 参考生成都走这套工作流。
echo.
cd /d "%AISHOW_ROOT%"
"%COMFY_PY%" "%AISHOW_ROOT%\backend\python\director_server.py"
echo.
echo Timeline Director 边车已退出。
if /i not "%AISHOW_HEADLESS%"=="1" pause
