# Resume FastH3 Preview v0.2 weights (~138GB). Safe to rerun.
. "$PSScriptRoot\_env.ps1"
$localDir = Get-AishowEnv "FASTH3_LOCAL_DIR"
if (-not $localDir) {
    $models = Require-AishowEnv "MODELS_ROOT"
    $localDir = Join-Path $models "FastVideo-Minimax-FastH3-Preview-v0.2"
}
if ($Proxy) {
    $env:HTTP_PROXY = $Proxy
    $env:HTTPS_PROXY = $Proxy
    $env:ALL_PROXY = $Proxy
}
$env:HF_HUB_DISABLE_XET = "1"
$env:HF_HUB_DOWNLOAD_TIMEOUT = "300"
$Uv = Join-Path $env:USERPROFILE ".local\bin\uv.exe"
& $Uv tool run --from huggingface_hub hf download `
    FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2 `
    --local-dir $localDir
