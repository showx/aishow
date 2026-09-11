@echo off
setlocal EnableDelayedExpansion
rem Isolated PinkCherry FL2VA INT8 sidecar (API 30013, ComfyUI 8189).
rem Do not reuse FastH3/Ref2VA ComfyUI on 8188 or fasth3-gguf.

call "%~dp0_env.bat"
if not defined COMFY_ROOT (
  echo 缺少 COMFY_ROOT。请复制 scripts\paths.example.bat 为 scripts\paths.bat 并填写。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not defined H3_PINKCHERRY_ROOT (
  echo 缺少 H3_PINKCHERRY_ROOT / MODELS_ROOT。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

set "PC_ROOT=%H3_PINKCHERRY_ROOT%"
set "PC_UNET=PinkCherry_fl2va_MiniMax_H3_pruned_int8_convrot-beta-0.6.safetensors"
if not defined H3_PINKCHERRY_COMFY_HOME set "H3_PINKCHERRY_COMFY_HOME=%TEMP%\pinkcherry-comfy"
if not defined H3_PINKCHERRY_COMFY_OUTPUT set "H3_PINKCHERRY_COMFY_OUTPUT=%H3_PINKCHERRY_COMFY_HOME%\output"
if not defined H3_PINKCHERRY_COMFY_INPUT set "H3_PINKCHERRY_COMFY_INPUT=%H3_PINKCHERRY_COMFY_HOME%\input"
set "H3_PINKCHERRY_HOST=127.0.0.1"
set "H3_PINKCHERRY_PORT=30013"
set "H3_PINKCHERRY_COMFY_URL=http://127.0.0.1:8189"
set "H3_PINKCHERRY_UNET=%PC_UNET%"
set "PYTHONUNBUFFERED=1"

if defined AISHOW_TMP if not exist "%AISHOW_TMP%" mkdir "%AISHOW_TMP%"
if defined TRITON_CACHE_DIR if not exist "%TRITON_CACHE_DIR%" mkdir "%TRITON_CACHE_DIR%"
if not exist "%PC_ROOT%\diffusion_models" mkdir "%PC_ROOT%\diffusion_models"
if defined FASTH3_GGUF_ROOT if exist "%FASTH3_GGUF_ROOT%\diffusion_models\%PC_UNET%" if not exist "%PC_ROOT%\diffusion_models\%PC_UNET%" (
  echo 发现误放在 fasth3-gguf 的 PinkCherry 权重，正在移到独立目录...
  move /y "%FASTH3_GGUF_ROOT%\diffusion_models\%PC_UNET%" "%PC_ROOT%\diffusion_models\%PC_UNET%"
)
if defined FASTH3_GGUF_ROOT if exist "%FASTH3_GGUF_ROOT%\diffusion_models\%PC_UNET%" if exist "%PC_ROOT%\diffusion_models\%PC_UNET%" (
  echo 独立目录已有 PinkCherry 权重，删除 fasth3-gguf 里的副本。
  del /f /q "%FASTH3_GGUF_ROOT%\diffusion_models\%PC_UNET%"
)
if not exist "%PC_ROOT%\text_encoders" mkdir "%PC_ROOT%\text_encoders"
if not exist "%PC_ROOT%\vae" mkdir "%PC_ROOT%\vae"
if not exist "%H3_PINKCHERRY_COMFY_INPUT%" mkdir "%H3_PINKCHERRY_COMFY_INPUT%"
if not exist "%H3_PINKCHERRY_COMFY_OUTPUT%" mkdir "%H3_PINKCHERRY_COMFY_OUTPUT%"
if not exist "%H3_PINKCHERRY_OUT_DIR%" mkdir "%H3_PINKCHERRY_OUT_DIR%"
if not exist "%H3_PINKCHERRY_COMFY_HOME%\temp" mkdir "%H3_PINKCHERRY_COMFY_HOME%\temp"

if defined FASTH3_GGUF_ROOT (
  call :linkfile "%FASTH3_GGUF_ROOT%\text_encoders\qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors" "%PC_ROOT%\text_encoders\qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors"
  call :linkfile "%FASTH3_GGUF_ROOT%\text_encoders\qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors" "%PC_ROOT%\text_encoders\qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors"
  call :linkfile "%FASTH3_GGUF_ROOT%\vae\minimax_h3_video_vae_fp16.safetensors" "%PC_ROOT%\vae\minimax_h3_video_vae_fp16.safetensors"
  call :linkfile "%FASTH3_GGUF_ROOT%\vae\minimax_h3_audio_vae_fp32.safetensors" "%PC_ROOT%\vae\minimax_h3_audio_vae_fp32.safetensors"
)

if exist "%PC_ROOT%\text_encoders\qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors" (
  set "H3_PINKCHERRY_CLIP=qwen3vl_32b_heretic_minimax_h3_nvfp4.safetensors"
) else (
  set "H3_PINKCHERRY_CLIP=qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors"
)
set "H3_PINKCHERRY_VIDEO_VAE=minimax_h3_video_vae_fp16.safetensors"
set "H3_PINKCHERRY_AUDIO_VAE=minimax_h3_audio_vae_fp32.safetensors"

if not exist "%COMFY_PY%" (
  echo 找不到 ComfyUI Python: %COMFY_PY%
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%PC_ROOT%\diffusion_models\%PC_UNET%" (
  echo 找不到 PinkCherry INT8。请先跑 scripts\download_h3_pinkcherry_int8.ps1
  echo 权重必须在 %PC_ROOT%\diffusion_models\ ，不要放进 fasth3-gguf。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

set "PC_YAML=%AISHOW_SCRIPTS%pinkcherry_extra_model_paths.local.yaml"
powershell -NoProfile -Command ^
  "$root = $env:H3_PINKCHERRY_ROOT -replace '\\','/'; $p = '%PC_YAML%'; $t = @'
pinkcherry:
    base_path: PLACEHOLDER
    is_default: true
    diffusion_models: diffusion_models/
    unet: diffusion_models/
    text_encoders: text_encoders/
    clip: text_encoders/
    vae: vae/
'@ -replace 'PLACEHOLDER', $root; [IO.File]::WriteAllText($p, $t)"

curl.exe -s -o nul -m 3 http://127.0.0.1:8189/system_stats
if not errorlevel 1 goto comfy_ok

echo 启动 PinkCherry 专用 ComfyUI  -^>  http://127.0.0.1:8189
echo （不用 8188，那是 FastH3 / Ref2VA 的）
start "PinkCherry ComfyUI :8189" /D "%COMFY_ROOT%" "%COMFY_PY%" -s ComfyUI\main.py --windows-standalone-build --listen 127.0.0.1 --port 8189 --fast fp16_accumulation --extra-model-paths-config "%PC_YAML%" --input-directory "%H3_PINKCHERRY_COMFY_INPUT%" --output-directory "%H3_PINKCHERRY_COMFY_OUTPUT%" --temp-directory "%H3_PINKCHERRY_COMFY_HOME%\temp"
echo 等待 PinkCherry ComfyUI 就绪...
set /a _n=0
:wait_comfy
timeout /t 2 /nobreak >nul
curl.exe -s -o nul -m 3 http://127.0.0.1:8189/system_stats
if not errorlevel 1 goto comfy_ok
set /a _n+=1
if !_n! GEQ 90 (
  echo PinkCherry ComfyUI 90 秒还没起来，请看「PinkCherry ComfyUI :8189」窗口。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
goto wait_comfy

:comfy_ok
echo PinkCherry ComfyUI 已就绪  -^>  http://127.0.0.1:8189
echo PinkCherry INT8 边车  -^>  http://127.0.0.1:30013
echo 工坊选「H3 PinkCherry INT8」，推理模式 auto。
echo.
cd /d "%AISHOW_ROOT%"
"%COMFY_PY%" "%AISHOW_ROOT%\backend\python\pinkcherry_int8_server.py"
echo.
echo PinkCherry INT8 边车已退出。
if /i not "%AISHOW_HEADLESS%"=="1" pause
exit /b 0

:linkfile
if exist "%~2" exit /b 0
if not exist "%~1" exit /b 0
mklink /H "%~2" "%~1" >nul 2>&1
if exist "%~2" exit /b 0
echo 无法硬链接，跳过 %~nx1（PinkCherry 目录里需要这份文件）
exit /b 0
