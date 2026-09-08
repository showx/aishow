# FastH3 GGUF Q4 + quantized text encoder + Comfy-Org VAEs via curl + Clash.
# Safe to rerun; uses HTTP resume.

$ErrorActionPreference = "Continue"
$Curl = "C:\Windows\System32\curl.exe"
$Proxy = "http://127.0.0.1:7897"
$Root = "F:\models\fasth3-gguf"
Remove-Item env:HTTP_PROXY, env:HTTPS_PROXY, env:ALL_PROXY, env:http_proxy, env:https_proxy, env:PYTHONHOME -EA SilentlyContinue

$jobs = @(
    @{ Rel = "diffusion_models/FastH3-comfy-Q4_K_M.gguf"; Url = "https://huggingface.co/realrebelai/FastH3_GGUFs/resolve/main/FastH3-comfy-Q4_K_M.gguf" },
    @{ Rel = "text_encoders/qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors"; Url = "https://huggingface.co/Comfy-Org/MiniMax-H3/resolve/main/text_encoders/qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors" },
    @{ Rel = "vae/minimax_h3_video_vae_fp16.safetensors"; Url = "https://huggingface.co/Comfy-Org/MiniMax-H3/resolve/main/vae/minimax_h3_video_vae_fp16.safetensors" },
    @{ Rel = "vae/minimax_h3_audio_vae_fp32.safetensors"; Url = "https://huggingface.co/Comfy-Org/MiniMax-H3/resolve/main/vae/minimax_h3_audio_vae_fp32.safetensors" },
    @{ Rel = "workflows/FastH3 (GGUF) Workflow.json"; Url = "https://huggingface.co/realrebelai/FastH3_GGUFs/resolve/main/FastH3%20(GGUF)%20Workflow.json" }
)

foreach ($j in $jobs) {
    $dest = Join-Path $Root ($j.Rel -replace '/', '\')
    New-Item -ItemType Directory -Force -Path (Split-Path $dest) | Out-Null
    Write-Host ">> $($j.Rel)"
    & $Curl -L --fail --retry 8 --retry-all-errors --retry-delay 3 `
        -C - -x $Proxy --connect-timeout 30 --max-time 0 `
        $j.Url -o $dest
    if ($LASTEXITCODE -ne 0) { Write-Host "WARN curl exit $LASTEXITCODE for $($j.Rel)" }
}

Write-Host "`n=== result ==="
Get-ChildItem $Root -Recurse -File | Where-Object { $_.Name -notmatch '^\.' } | ForEach-Object {
    "{0,8:N2} GB  {1}" -f ($_.Length/1GB), $_.FullName
}
