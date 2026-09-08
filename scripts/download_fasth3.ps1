# Resume FastH3 Preview v0.2 weights (~138GB). Safe to rerun.
$env:HTTP_PROXY = "http://127.0.0.1:7897"
$env:HTTPS_PROXY = $env:HTTP_PROXY
$env:ALL_PROXY = $env:HTTP_PROXY
$env:UV_CACHE_DIR = "F:\cache\uv"
$env:HF_HOME = "F:\cache\huggingface"
$env:HF_HUB_DISABLE_XET = "1"
$env:HF_HUB_DOWNLOAD_TIMEOUT = "300"
$Uv = Join-Path $env:USERPROFILE ".local\bin\uv.exe"
& $Uv tool run --from huggingface_hub hf download `
    FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2 `
    --local-dir "F:\models\FastVideo-Minimax-FastH3-Preview-v0.2"
