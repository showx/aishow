# Qwen/Qwen-Image-2.1 local snapshot (~33 GB).

. "$PSScriptRoot\_env.ps1"
$Root = Get-AishowEnv "QWEN_IMAGE_MODEL"
if (-not $Root) {
    $models = Require-AishowEnv "MODELS_ROOT"
    $Root = Join-Path $models "Qwen-Image-2.1"
}
$Base = "https://huggingface.co/Qwen/Qwen-Image-2.1/resolve/main"

$jobs = @(
    @{ Rel = "model_index.json"; Size = 447 },
    @{ Rel = "LICENSE"; Size = 7831 },
    @{ Rel = "README.md"; Size = 5358 },
    @{ Rel = "scheduler/scheduler_config.json"; Size = 485 },
    @{ Rel = "processor/added_tokens.json"; Size = 707 },
    @{ Rel = "processor/chat_template.jinja"; Size = 5292 },
    @{ Rel = "processor/merges.txt"; Size = 1671853 },
    @{ Rel = "processor/preprocessor_config.json"; Size = 782 },
    @{ Rel = "processor/special_tokens_map.json"; Size = 613 },
    @{ Rel = "processor/tokenizer.json"; Size = 11422654 },
    @{ Rel = "processor/tokenizer_config.json"; Size = 5445 },
    @{ Rel = "processor/video_preprocessor_config.json"; Size = 817 },
    @{ Rel = "processor/vocab.json"; Size = 2776833 },
    @{ Rel = "transformer/config.json"; Size = 370 },
    @{ Rel = "transformer/diffusion_pytorch_model.safetensors.index.json"; Size = 30283 },
    @{ Rel = "transformer/diffusion_pytorch_model-00001-of-00002.safetensors"; Size = 9968332504 },
    @{ Rel = "transformer/diffusion_pytorch_model-00002-of-00002.safetensors"; Size = 4261951904 },
    @{ Rel = "text_encoder/config.json"; Size = 1517 },
    @{ Rel = "text_encoder/generation_config.json"; Size = 213 },
    @{ Rel = "text_encoder/model.safetensors.index.json"; Size = 67795 },
    @{ Rel = "text_encoder/model-00001-of-00004.safetensors"; Size = 4998056552 },
    @{ Rel = "text_encoder/model-00002-of-00004.safetensors"; Size = 4915962464 },
    @{ Rel = "text_encoder/model-00003-of-00004.safetensors"; Size = 4915962496 },
    @{ Rel = "text_encoder/model-00004-of-00004.safetensors"; Size = 2704357976 },
    @{ Rel = "vae/config.json"; Size = 2079 },
    @{ Rel = "vae/diffusion_pytorch_model.safetensors"; Size = 1350989512 }
)

foreach ($j in $jobs) {
    $relWin = $j.Rel -replace "/", "\"
    $dir = Join-Path $Root (Split-Path $relWin -Parent)
    if (-not (Split-Path $relWin -Parent)) { $dir = $Root }
    $out = Split-Path $relWin -Leaf
    $url = "$Base/$($j.Rel)"
    Write-Host ">> $($j.Rel)"
    $null = Invoke-AishowDownload -Dir $dir -Out $out -Url $url -Size $j.Size
}

Write-Host "`n=== result ==="
Get-ChildItem $Root -Recurse -File | Where-Object { $_.Name -notmatch "^\." } | ForEach-Object {
    "{0,8:N2} GB  {1}" -f ($_.Length / 1GB), $_.FullName
}
Write-Host "`nweights: $Root"
Write-Host "paths.bat: set `"QWEN_IMAGE_MODEL=$Root`""
Write-Host "then: powershell -File scripts\setup_qwen_image.ps1"
Write-Host "then: scripts\start_qwen_image.bat"
