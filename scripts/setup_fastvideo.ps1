# Install FastVideo + download FastH3 Preview v0.2. Safe to rerun.

. "$PSScriptRoot\_env.ps1"
$ErrorActionPreference = "Stop"
$models = Require-AishowEnv "MODELS_ROOT"
$fvRoot = Get-AishowEnv "FASTVIDEO_ROOT"
if (-not $fvRoot) { $fvRoot = Join-Path $models "FastVideo" }
$ModelDir = Get-AishowEnv "FASTH3_LOCAL_DIR"
if (-not $ModelDir) { $ModelDir = Join-Path $models "FastVideo-Minimax-FastH3-Preview-v0.2" }
if ($Proxy) {
    $env:HTTP_PROXY = $Proxy
    $env:HTTPS_PROXY = $Proxy
    $env:ALL_PROXY = $Proxy
}
$env:HF_HUB_DISABLE_XET = "1"
$env:UV_TORCH_BACKEND = "cu126"
$env:UV_HTTP_TIMEOUT = "300"

$Uv = Join-Path $env:USERPROFILE ".local\bin\uv.exe"
$VenvPy = Join-Path $fvRoot ".venv\Scripts\python.exe"

New-Item -ItemType Directory -Force -Path $fvRoot, $models | Out-Null

if (-not (Test-Path $VenvPy)) {
    & $Uv venv --python 3.12 --seed --directory $fvRoot
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
    --extra-index-url https://download.pytorch.org/whl/cu126 `
    fastvideo

Write-Host "Done. Start with scripts\start_fasth3.bat"
