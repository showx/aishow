# Isolated PinkCherry MiniMax-H3 pruned INT8. Never put this under fasth3-gguf.

. "$PSScriptRoot\_env.ps1"
$pcRoot = Require-AishowEnv "H3_PINKCHERRY_ROOT"
$Dir = Join-Path $pcRoot "diffusion_models"
$Out = "PinkCherry_fl2va_MiniMax_H3_pruned_int8_convrot-beta-0.6.safetensors"
$Url = "https://huggingface.co/SexGod1979/PinkCherry_MiniMax-H3/resolve/main/beta-0.6-fl2va/PinkCherry_fl2va_MiniMax_H3_pruned_int8_convrot-beta-0.6.safetensors"
$Size = [int64]20970380016
$ggufRoot = Get-AishowEnv "FASTH3_GGUF_ROOT"
$Old = if ($ggufRoot) { Join-Path $ggufRoot "diffusion_models\$Out" } else { "" }

New-Item -ItemType Directory -Force -Path $Dir | Out-Null
$dest = Join-Path $Dir $Out

if ($Old -and (Test-Path $Old) -and -not (Test-Path $dest)) {
    Write-Host "Moving PinkCherry weights out of fasth3-gguf into isolated dir..."
    Move-Item -LiteralPath $Old -Destination $dest
} elseif ($Old -and (Test-Path $Old) -and (Test-Path $dest)) {
    $oldLen = (Get-Item $Old).Length
    $newLen = (Get-Item $dest).Length
    if ($oldLen -eq $Size -and $newLen -ne $Size) {
        Remove-Item -LiteralPath $dest -Force
        Move-Item -LiteralPath $Old -Destination $dest
    } elseif ($newLen -eq $Size) {
        Write-Host "Isolated copy is complete; removing fasth3-gguf duplicate"
        Remove-Item -LiteralPath $Old -Force
    }
}

$code = Invoke-AishowDownload -Dir $Dir -Out $Out -Url $Url -Size $Size
$now = if (Test-Path $dest) { (Get-Item $dest).Length } else { 0 }
$ok = if ($now -eq $Size) { "OK" } else { "INCOMPLETE" }
Write-Host ""
Write-Host "=== result ==="
Write-Host ("{0,-12} {1,8:N2}/{2,8:N2} GB  {3}" -f $ok, ($now / 1GB), ($Size / 1GB), $dest)
Write-Host "Keep this file only under $pcRoot, not fasth3-gguf."
exit $code
