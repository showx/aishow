@echo off
setlocal
call "%~dp0_env.bat"

if not exist "%AISHOW_ROOT%\backend\.env" (
  copy /y "%AISHOW_ROOT%\backend\.env.example" "%AISHOW_ROOT%\backend\.env" >nul
  echo 已创建 backend\.env（默认 mock，无需 GPU）。
)
if not exist "%AISHOW_ROOT%\frontend\.env" (
  copy /y "%AISHOW_ROOT%\frontend\.env.example" "%AISHOW_ROOT%\frontend\.env" >nul
  echo 已创建 frontend\.env。
)

echo 启动后端  http://127.0.0.1:9808
start "Aishow API" /D "%AISHOW_ROOT%\backend" cmd /k go run ./cmd/server

echo 启动前端  http://127.0.0.1:5173
start "Aishow UI" /D "%AISHOW_ROOT%\frontend" cmd /k "if not exist node_modules npm install & npm run dev"

echo.
echo 浏览器打开 http://127.0.0.1:5173
echo 首次访问会创建管理员账号。没有 GPU 时保持推理模式 mock。
echo 接本机模型：复制 scripts\paths.example.bat 为 paths.bat，再跑对应 start_*.bat。
