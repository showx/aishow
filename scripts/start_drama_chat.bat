@echo off
setlocal
rem 短剧编剧：本机 Ollama 小模型（默认 qwen3.5:4b）。不占 GPU 边车槽，可和 H3 / LLaDA 同时开。
call "%~dp0_env.bat"
if not defined DRAMA_CHAT_MODEL set "DRAMA_CHAT_MODEL=qwen3.5:4b"

where ollama >nul 2>&1
if errorlevel 1 (
  echo 未找到 ollama。请先安装 https://ollama.com 后再跑本脚本。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)

curl -s -m 2 http://127.0.0.1:11434/api/tags >nul 2>&1
if errorlevel 1 (
  echo 正在启动 Ollama...
  start "Ollama" /MIN ollama serve
  timeout /t 3 /nobreak >nul
)

ollama list | findstr /i /c:"%DRAMA_CHAT_MODEL%" >nul
if errorlevel 1 (
  echo 本机还没有 %DRAMA_CHAT_MODEL%，开始拉取...
  ollama pull %DRAMA_CHAT_MODEL%
  if errorlevel 1 (
    echo 拉取失败。也可改用已有的对话模型，不要选 qwen3-vl。
    if /i not "%AISHOW_HEADLESS%"=="1" pause
    exit /b 1
  )
)

echo.
echo 短剧 Chat 已就绪
echo   地址  http://127.0.0.1:11434
echo   模型  %DRAMA_CHAT_MODEL%
echo 推理节点里本地 Chat 填以上两项。不要选 qwen3-vl（那是视觉模型，不是编剧）。
echo LLaDA-Image 只负责出图，和剧情模型不是同一个。
if /i not "%AISHOW_HEADLESS%"=="1" pause
exit /b 0
