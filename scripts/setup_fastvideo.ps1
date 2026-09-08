# Install FastVideo + download FastH3 Preview v0.2.
# Uses Clash Verge at 127.0.0.1:7897. Safe to rerun; downloads resume.

$ErrorActionPreference = "Stop"
$Proxy = "http://127.0.0.1:7897"
$env:HTTP_PROXY = $Proxy
$env:HTTPS_PROXY = $Proxy
$env:ALL_PROXY = $Proxy
$env:UV_CACHE_DIR = "F:\cache\uv"
$env:UV_PYTHON_INSTALL_DIR = "F:\cache\uv-python"
$env:HF_HOME = "F:\cache\huggingface"
$env:HF_HUB_DISABLE_XET = "1"
$env:UV_TORCH_BACKEND = "cu126"
$env:UV_HTTP_TIMEOUT = "300"

$Uv = Join-Path $env:USERPROFILE ".local\bin\uv.exe"
$VenvPy = "F:\FastVideo\.venv\Scripts\python.exe"
$ModelDir = "F:\models\FastVideo-Minimax-FastH3-Preview-v0.2"
$WheelDir = "F:\cache\wheels"

New-Item -ItemType Directory -Force -Path "F:\FastVideo", "F:\models", $WheelDir, $env:UV_CACHE_DIR, $env:HF_HOME | Out-Null

if (-not (Test-Path $VenvPy)) {
    & $Uv venv --python 3.12 --seed --directory F:\FastVideo
}

Write-Host "Downloading FastH3 weights to $ModelDir (resume-safe, ~138GB)..."
Start-Process -FilePath $Uv -ArgumentList @(
    "tool", "run", "--from", "huggingface_hub",
    "hf", "download",
    "FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2",
    "--local-dir", $ModelDir
) -NoNewWindow -Wait

Write-Host "Installing fastvideo into $VenvPy ..."
& $Uv pip install --python $VenvPy `
    --default-index https://pypi.tuna.tsinghua.edu.cn/simple `
    --extra-index-url https://download.pytorch.org/whl/cu126 `
    fastvideo

Write-Host "Done. Start with D:\code\aishow\scripts\start_fasth3.bat"
