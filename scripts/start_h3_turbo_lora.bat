@echo off
setlocal
rem DiffSynth NF4 + lightx2v MiniMax-H3 Turbo LoRA (4-step).
call "%~dp0_env.bat"
if not defined H3_ROOT (
  echo 缺少 MODELS_ROOT。请复制 scripts\paths.example.bat 为 scripts\paths.bat，并把 MiniMax-H3 放在该目录下。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not defined PYTHON (
  echo 缺少 PYTHON / COMFY_ROOT。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

set "H3_HOST=127.0.0.1"
set "H3_PORT=30012"
set "H3_TURBO=1"
set "H3_MAX_SHORT_EDGE=768"
set "PYTHONUNBUFFERED=1"
set "DIFFSYNTH_SKIP_DOWNLOAD=True"
if defined DIFFSYNTH_ROOT set "PYTHONPATH=%DIFFSYNTH_ROOT%;%PYTHONPATH%"

if not exist "%H3_LORA_DIR%\minimax_h3_fl2v_turbo_4step_v1.2_768p_bf16.safetensors" (
  if not exist "%H3_LORA_DIR%\minimax_h3_fl2v_turbo_4step_v1.0_768p_bf16.safetensors" (
    if not exist "%H3_LORA_DIR%\minimax_h3_fl2v_turbo_4step_v0.1.safetensors" (
      if not exist "%H3_LORA_DIR%\minimax_h3_turbo_4step_ema_ckpt850.safetensors" (
        echo 找不到 Turbo LoRA。请先跑 scripts\download_h3_turbo_lora.ps1
        if /i not "%AISHOW_HEADLESS%"=="1" pause
        exit /b 1
      )
    )
  )
)

cd /d "%AISHOW_ROOT%"
"%PYTHON%" "%AISHOW_ROOT%\backend\python\diffsynth_server.py"
echo.
echo Turbo LoRA 边车已退出。看到 ready / listening 之前请不要关窗口。
if /i not "%AISHOW_HEADLESS%"=="1" pause
