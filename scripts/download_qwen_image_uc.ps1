# Qwen-Image-2.1 Uncensored GGUF (ComfyUI).
# Recommended set: Q4_K_M diffusion + Int8 text encoder + BF16 VAE (~13.6 GB).
# https://huggingface.co/abenzerps/Qwen-Image-2.1-Uncensored-GGUF

. "$PSScriptRoot\_env.ps1"
$models = Require-AishowEnv "MODELS_ROOT"
$Root = Join-Path $models "qwen-image-2.1-uc"
$Base = "https://huggingface.co/abenzerps/Qwen-Image-2.1-Uncensored-GGUF/resolve/main"

# Remote path is the Hugging Face repo layout. Local path is the ComfyUI layout.
$jobs = @(
    @{
        Rel = "diffusion_models/qwen-image-2.1-UC-Q4_K_M.gguf"
        Remote = "qwen-image-2.1-UC-Q4_K_M.gguf"
        Size = 4604558112
    },
    @{
        Rel = "text_encoders/qwen3vl_8b_int8_convrot.safetensors"
        Remote = "text_encoders/qwen3vl_8b_int8_convrot.safetensors"
        Size = 9350798360
    },
    @{
        Rel = "vae/qwen_image_2.1_vae_bf16.safetensors"
        Remote = "vae/qwen_image_2.1_vae_bf16.safetensors"
        Size = 675509688
    }
)

foreach ($j in $jobs) {
    $relWin = $j.Rel -replace "/", "\"
    $dir = Join-Path $Root (Split-Path $relWin -Parent)
    $out = Split-Path $relWin -Leaf
    $url = "$Base/$($j.Remote)"
    Write-Host ">> $($j.Rel)"
    $code = Invoke-AishowDownload -Dir $dir -Out $out -Url $url -Size $j.Size
    if ($code -ne 0) {
        Write-Host "FAIL $($j.Rel) exit=$code"
        exit $code
    }
}

Write-Host "`n=== result ==="
Get-ChildItem $Root -Recurse -File | Where-Object { $_.Name -notmatch "^\." } | ForEach-Object {
    "{0,8:N2} GB  {1}" -f ($_.Length / 1GB), $_.FullName
}
Write-Host "`nweights: $Root"
Write-Host "ComfyUI: Unet Loader (GGUF) + CLIPLoader type=qwen_image + VAELoader"
