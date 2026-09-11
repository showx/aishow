# Resume-safe Ref2VA weights:
#   1) DiffSynth NF4  -> %H3_NF4_DIR%
#   2) Comfy-Org pruned INT8 ConvRot -> %FASTH3_GGUF_ROOT%\diffusion_models

. "$PSScriptRoot\_env.ps1"
$nf4 = Require-AishowEnv "H3_NF4_DIR"
$gguf = Require-AishowEnv "FASTH3_GGUF_ROOT"

$jobs = @(
    @{
        Name = "DiffSynth NF4 Ref2VA"
        Dir  = $nf4
        Out  = "minimax-h3-ref2va-nf4.safetensors"
        Url  = "https://huggingface.co/DiffSynth-Studio/MiniMax-H3-NF4/resolve/main/minimax-h3-ref2va-nf4.safetensors"
        Size = [int64]17162138284
    },
    @{
        Name = "Comfy-Org pruned INT8 ConvRot Ref2VA"
        Dir  = (Join-Path $gguf "diffusion_models")
        Out  = "minimax_h3_ref2va_pruned_int8_convrot.safetensors"
        Url  = "https://huggingface.co/Comfy-Org/MiniMax-H3/resolve/main/diffusion_models/minimax_h3_ref2va_pruned_int8_convrot.safetensors"
        Size = [int64]20970379616
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
exit $code
