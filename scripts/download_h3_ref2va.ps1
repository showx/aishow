# Resume-safe Ref2VA weights via Clash + aria2.
#   1) DiffSynth NF4  -> E:\MiniMax-H3\models\MiniMax-H3-NF4
#   2) Comfy-Org pruned INT8 ConvRot -> F:\models\fasth3-gguf\diffusion_models
#      (ComfyUI extra_model_paths already points there)

$ErrorActionPreference = "Continue"
$Aria = "F:\tools\aria2\aria2-1.37.0-win-64bit-build1\aria2c.exe"
$Proxy = "http://127.0.0.1:7897"
Remove-Item env:HTTP_PROXY, env:HTTPS_PROXY, env:ALL_PROXY, env:http_proxy, env:https_proxy -EA SilentlyContinue

$jobs = @(
    @{
        Name = "DiffSynth NF4 Ref2VA"
        Dir  = "E:\MiniMax-H3\models\MiniMax-H3-NF4"
        Out  = "minimax-h3-ref2va-nf4.safetensors"
        Url  = "https://huggingface.co/DiffSynth-Studio/MiniMax-H3-NF4/resolve/main/minimax-h3-ref2va-nf4.safetensors"
        Size = [int64]17162138284
    },
    @{
        Name = "Comfy-Org pruned INT8 ConvRot Ref2VA"
        Dir  = "F:\models\fasth3-gguf\diffusion_models"
        Out  = "minimax_h3_ref2va_pruned_int8_convrot.safetensors"
        Url  = "https://huggingface.co/Comfy-Org/MiniMax-H3/resolve/main/diffusion_models/minimax_h3_ref2va_pruned_int8_convrot.safetensors"
        Size = [int64]20970379616
    }
)

function Invoke-Aria($job) {
    New-Item -ItemType Directory -Force -Path $job.Dir | Out-Null
    $dest = Join-Path $job.Dir $job.Out
    $have = if (Test-Path $dest) { (Get-Item $dest).Length } else { 0 }
    if ($have -eq $job.Size) {
        Write-Host ("SKIP already complete {0:N2} GB  {1}" -f ($have/1GB), $dest)
        return 0
    }
    Write-Host ("==== {0}  {1:N2}/{2:N2} GB ====" -f $job.Name, ($have/1GB), ($job.Size/1GB))
    & $Aria --console-log-level=notice --summary-interval=15 `
        --max-connection-per-server=16 --split=16 --min-split-size=8M `
        --continue=true --auto-file-renaming=false --allow-overwrite=false `
        --file-allocation=none --max-tries=0 --retry-wait=3 `
        --connect-timeout=20 --timeout=120 `
        --all-proxy=$Proxy --dir=$($job.Dir) --out=$($job.Out) $job.Url
    return $LASTEXITCODE
}

$code = 0
foreach ($j in $jobs) {
    $c = Invoke-Aria $j
    if ($c -ne 0) { $code = $c }
}

Write-Host "`n=== result ==="
foreach ($j in $jobs) {
    $dest = Join-Path $j.Dir $j.Out
    $now = if (Test-Path $dest) { (Get-Item $dest).Length } else { 0 }
    $ok = if ($now -eq $j.Size) { "OK" } else { "INCOMPLETE" }
    "{0,-12} {1,8:N2}/{2,8:N2} GB  {3}" -f $ok, ($now/1GB), ($j.Size/1GB), $dest
}
exit $code
