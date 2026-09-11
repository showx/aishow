# FastH3 GGUF Q4 + quantized text encoder + Comfy-Org VAEs.

. "$PSScriptRoot\_env.ps1"
$Root = Require-AishowEnv "FASTH3_GGUF_ROOT"

$jobs = @(
    @{ Rel = "diffusion_models/FastH3-comfy-Q4_K_M.gguf"; Url = "https://huggingface.co/realrebelai/FastH3_GGUFs/resolve/main/FastH3-comfy-Q4_K_M.gguf" },
    @{ Rel = "text_encoders/qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors"; Url = "https://huggingface.co/Comfy-Org/MiniMax-H3/resolve/main/text_encoders/qwen3vl_32b_minimax_h3_nvfp4_awq.safetensors" },
    @{ Rel = "vae/minimax_h3_video_vae_fp16.safetensors"; Url = "https://huggingface.co/Comfy-Org/MiniMax-H3/resolve/main/vae/minimax_h3_video_vae_fp16.safetensors" },
    @{ Rel = "vae/minimax_h3_audio_vae_fp32.safetensors"; Url = "https://huggingface.co/Comfy-Org/MiniMax-H3/resolve/main/vae/minimax_h3_audio_vae_fp32.safetensors" },
    @{ Rel = "workflows/FastH3 (GGUF) Workflow.json"; Url = "https://huggingface.co/realrebelai/FastH3_GGUFs/resolve/main/FastH3%20(GGUF)%20Workflow.json" }
)

foreach ($j in $jobs) {
    $relWin = $j.Rel -replace "/", "\"
    $dir = Join-Path $Root (Split-Path $relWin -Parent)
    $out = Split-Path $relWin -Leaf
    Write-Host ">> $($j.Rel)"
    $null = Invoke-AishowDownload -Dir $dir -Out $out -Url $j.Url
}

Write-Host "`n=== result ==="
Get-ChildItem $Root -Recurse -File | Where-Object { $_.Name -notmatch "^\." } | ForEach-Object {
    "{0,8:N2} GB  {1}" -f ($_.Length / 1GB), $_.FullName
}
