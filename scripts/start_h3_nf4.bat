@echo off
setlocal
set "PYTHON=E:\MiniMax-H3\ComfyUI_windows_portable\python_embeded\python.exe"
set "PYTHONPATH=E:\MiniMax-H3\repo\DiffSynth-Studio-main;%PYTHONPATH%"
set "H3_ROOT=E:\MiniMax-H3"
set "H3_NF4_DIR=E:\MiniMax-H3\models\MiniMax-H3-NF4"
set "H3_PROCESSOR=E:\MiniMax-H3\models\MiniMax-H3\FL2VA\processor"
set "H3_OUT_DIR=E:\MiniMax-H3\tmp\aishow-jobs"
set "H3_MEDIA_ROOT=D:\code\aishow\backend\data\media"
set "H3_HOST=127.0.0.1"
set "H3_PORT=30010"
set "PYTHONUNBUFFERED=1"
set "DIFFSYNTH_SKIP_DOWNLOAD=True"
cd /d D:\code\aishow
"%PYTHON%" D:\code\aishow\backend\python\diffsynth_server.py
echo.
echo 边车已退出。看到 ready / listening 之前请不要关窗口。
if /i not "%AISHOW_HEADLESS%"=="1" pause
