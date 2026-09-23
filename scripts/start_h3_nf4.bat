@echo off
setlocal
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
set "H3_PORT=30010"
set "PYTHONUNBUFFERED=1"
set "DIFFSYNTH_SKIP_DOWNLOAD=True"
if defined DIFFSYNTH_ROOT set "PYTHONPATH=%DIFFSYNTH_ROOT%;%PYTHONPATH%"

cd /d "%AISHOW_ROOT%"
"%PYTHON%" "%AISHOW_ROOT%\backend\python\diffsynth_server.py"
echo.
echo 边车已退出。看到 ready / listening 之前请不要关窗口。
if /i not "%AISHOW_HEADLESS%"=="1" pause
