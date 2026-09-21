# H3 latent upscaler for Timeline Director two-stage sampling.
# Puts the fp16 checkpoint into ComfyUI/models/latent_upscale_models/

. "$PSScriptRoot\_env.ps1"
$comfy = Require-AishowEnv "COMFY_ROOT"
$dir = Join-Path $comfy "ComfyUI\models\latent_upscale_models"

$jobs = @(
    @{
        Name = "MiniMax H3 Latent Upscaler 3D fp16"
        Dir  = $dir
        Out  = "minimax_h3_latent_upscaler_3d_conv_v1_fp16.safetensors"
        Url  = "https://huggingface.co/LBH-123-AI/Minimax_h3_latent_Upscaler/resolve/main/minimax_h3_latent_upscaler_3d_conv_v1/minimax_h3_latent_upscaler_3d_conv_v1_fp16.safetensors"
        Size = [int64]690592672
    }
)

$code = 0
foreach ($j in $jobs) {
    Write-Host ("==== {0} ====" -f $j.Name)
    $c = Invoke-AishowDownload -Dir $j.Dir -Out $j.Out -Url $j.Url -Size $j.Size
    if ($c -ne 0) { $code = $c }
}

Write-Host "`n=== result ==="
foreach ($j in $jobs) {
    $dest = Join-Path $j.Dir $j.Out
    $now = if (Test-Path $dest) { (Get-Item $dest).Length } else { 0 }
    $ok = if ($now -eq $j.Size) { "OK" } else { "INCOMPLETE" }
    "{0,-12} {1,8:N2}/{2,8:N2} GB  {3}" -f $ok, ($now / 1GB), ($j.Size / 1GB), $dest
}
if ($code -eq 0) {
    Write-Host "放到 ComfyUI 标准目录后重启 ComfyUI，工坊的 Timeline Director 才会开二采。"
}
exit $code
