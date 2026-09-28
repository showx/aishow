@echo off
setlocal
rem Qwen-Image-2.1 Uncensored GGUF via stable-diffusion.cpp (sd-server).
rem Weights: download_qwen_image_uc.ps1    Binary: download_sdcpp.ps1

call "%~dp0_env.bat"
if not defined MODELS_ROOT (
  echo 缺少 MODELS_ROOT。请复制 scripts\paths.example.bat 为 scripts\paths.bat。
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not defined SDCPP_ROOT set "SDCPP_ROOT=%MODELS_ROOT%\stable-diffusion.cpp"
if not defined QWEN_IMAGE_UC_ROOT set "QWEN_IMAGE_UC_ROOT=%MODELS_ROOT%\qwen-image-2.1-uc"
if not defined QWEN_IMAGE_UC_HOST set "QWEN_IMAGE_UC_HOST=127.0.0.1"
if not defined QWEN_IMAGE_UC_PORT set "QWEN_IMAGE_UC_PORT=30024"

set "SD_SERVER=%SDCPP_ROOT%\sd-server.exe"
set "UNET=%QWEN_IMAGE_UC_ROOT%\diffusion_models\qwen-image-2.1-UC-Q4_K_M.gguf"
set "VAE=%QWEN_IMAGE_UC_ROOT%\vae\qwen_image_2.1_vae_bf16.safetensors"
set "LLM=%QWEN_IMAGE_UC_ROOT%\text_encoders\qwen3vl_8b_int8_convrot.safetensors"

if not exist "%SD_SERVER%" (
  echo 找不到 sd-server: %SD_SERVER%
  echo 请先运行: powershell -ExecutionPolicy Bypass -File scripts\download_sdcpp.ps1
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%UNET%" (
  echo 找不到扩散模型: %UNET%
  echo 请先运行: powershell -ExecutionPolicy Bypass -File scripts\download_qwen_image_uc.ps1
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%VAE%" (
  echo 找不到 VAE: %VAE%
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%LLM%" (
  echo 找不到文本编码器: %LLM%
  if /i not "%AISHOW_HEADLESS%"=="1" pause
  exit /b 1
)
if not exist "%QWEN_IMAGE_UC_ROOT%\lora" mkdir "%QWEN_IMAGE_UC_ROOT%\lora"

echo 扩散: %UNET%
echo 文本: %LLM%
echo VAE:  %VAE%
echo 地址: http://%QWEN_IMAGE_UC_HOST%:%QWEN_IMAGE_UC_PORT%/
echo 24GB：文本编码器放内存，扩散模型留在显卡。

cd /d "%SDCPP_ROOT%"
"%SD_SERVER%" ^
  --diffusion-model "%UNET%" ^
  --vae "%VAE%" ^
  --llm "%LLM%" ^
  --lora-model-dir "%QWEN_IMAGE_UC_ROOT%\lora" ^
  --params-backend te=cpu,diffusion=CUDA0,vae=CUDA0 ^
  --diffusion-fa ^
  --cfg-scale 6 ^
  --sampling-method euler ^
  --steps 20 ^
  -W 1024 -H 1024 ^
  --listen-ip %QWEN_IMAGE_UC_HOST% ^
  --listen-port %QWEN_IMAGE_UC_PORT%

echo.
echo sd-server 已退出。
if /i not "%AISHOW_HEADLESS%"=="1" pause
