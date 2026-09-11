# Resume FastH3 Preview v0.2 via Windows curl.
. "$PSScriptRoot\_env.ps1"
$ErrorActionPreference = "Continue"
$Root = Get-AishowEnv "FASTH3_LOCAL_DIR"
if (-not $Root) {
    $models = Require-AishowEnv "MODELS_ROOT"
    $Root = Join-Path $models "FastVideo-Minimax-FastH3-Preview-v0.2"
}
$Repo = "FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2"
$Base = "https://huggingface.co/$Repo/resolve/main"
$Api = "https://huggingface.co/api/models/$Repo/tree/main"
Remove-Item env:HTTP_PROXY, env:HTTPS_PROXY, env:ALL_PROXY, env:http_proxy, env:https_proxy -EA SilentlyContinue

function Get-Tree([string]$Sub) {
    $url = if ($Sub) { "$Api/$Sub" } else { $Api }
    $tmp = Join-Path $env:TEMP ("fasth3-tree-{0}.json" -f ($Sub -replace '[\\/]', '-'))
    & $Curl -sL --fail --max-time 45 @(Get-CurlProxyArgs) $url -o $tmp | Out-Null
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $tmp)) { return @() }
    return @(Get-Content $tmp -Raw | ConvertFrom-Json)
}

Write-Host "listing HuggingFace files..."
$entries = @()
$entries += Get-Tree ""
foreach ($sub in @("text_encoder", "transformer", "vae", "tokenizer", "processor", "audio_vae", "scheduler", "audio_scheduler")) {
    $entries += Get-Tree $sub
}
$files = @($entries | Where-Object { $_.type -eq "file" -and $_.size -gt 0 } | Sort-Object path -Unique)
$hasTransformer = @($files | Where-Object { $_.path -like "transformer/*.safetensors" }).Count -ge 14
$hasVae = @($files | Where-Object { $_.path -like "vae/*.safetensors" }).Count -ge 3
if (-not $hasTransformer -or -not $hasVae) {
    Write-Host "tree listing missing transformer/vae sizes; those shards will use fallback names"
}

$todo = @()
foreach ($f in $files) {
    $rel = $f.path
    $dest = Join-Path $Root ($rel -replace '/', '\')
    $have = if (Test-Path $dest) { (Get-Item $dest).Length } else { 0 }
    if ($f.size -gt 0 -and $have -eq $f.size) { continue }
    $todo += [pscustomobject]@{ Rel = $rel; Dest = $dest; Size = [int64]$f.size; Have = $have }
}

if ($todo.Count -eq 0 -and $hasTransformer -and $hasVae) {
    Write-Host "all listed files are complete"
    exit 0
}

if (-not $hasTransformer -or -not $hasVae) {
    $names = @()
    1..14 | ForEach-Object { $names += ("text_encoder/model-{0:d5}-of-00014.safetensors" -f $_) }
    1..14 | ForEach-Object { $names += ("transformer/diffusion_pytorch_model-{0:d5}-of-00014.safetensors" -f $_) }
    1..3 | ForEach-Object { $names += ("vae/diffusion_pytorch_model-{0:d5}-of-00003.safetensors" -f $_) }
    $names += @(
        "text_encoder/model.safetensors.index.json",
        "transformer/diffusion_pytorch_model.safetensors.index.json",
        "vae/diffusion_pytorch_model.safetensors.index.json",
        "transformer/config.json",
        "vae/config.json"
    )
    foreach ($rel in $names) {
        if ($todo.Rel -contains $rel) { continue }
        $dest = Join-Path $Root ($rel -replace '/', '\')
        $have = if (Test-Path $dest) { (Get-Item $dest).Length } else { 0 }
        if ($rel.EndsWith(".safetensors") -and $have -ge 4GB) { continue }
        if ($rel.EndsWith(".json") -and $have -gt 1KB) { continue }
        $todo += [pscustomobject]@{ Rel = $rel; Dest = $dest; Size = [int64]0; Have = $have }
    }
}

$todo = @($todo | Sort-Object Size)
Write-Host ("need {0} files, {1:N1} GB remaining" -f $todo.Count, ((($todo | Measure-Object Size -Sum).Sum - ($todo | Measure-Object Have -Sum).Sum) / 1GB))
$todo | ForEach-Object { "{0,-72} {1,8:N0}/{2,8:N0} MB" -f $_.Rel, ($_.Have/1MB), ($_.Size/1MB) }

function Invoke-Download($item) {
    $dir = Split-Path $item.Dest -Parent
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Write-Host ("==== {0} ({1:N1} MB have / {2:N1} MB need) ====" -f $item.Rel, ($item.Have/1MB), ($item.Size/1MB))
    & $Curl -L --fail -C - --retry 30 --retry-delay 2 --retry-all-errors `
        --http1.1 --speed-limit 8000 --speed-time 45 `
        @(Get-CurlProxyArgs) -o $item.Dest "$Base/$($item.Rel)"
    $code = $LASTEXITCODE
    $now = if (Test-Path $item.Dest) { (Get-Item $item.Dest).Length } else { 0 }
    if ($item.Size -gt 0 -and $now -eq $item.Size) {
        Write-Host ("OK {0}" -f $item.Rel)
        return 0
    }
    if ($code -eq 0 -and $item.Size -eq 0) {
        Write-Host ("OK {0} (no size check)" -f $item.Rel)
        return 0
    }
    Write-Host ("curl exit {0} on {1} now {2:N1} MB" -f $code, $item.Rel, ($now/1MB))
    return $code
}

foreach ($item in $todo) {
    $null = Invoke-Download $item
}
Write-Host "curl pass finished"
