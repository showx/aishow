# Multi-connection FastH3 download via Clash + aria2.
$ErrorActionPreference = "Continue"
$Aria = "F:\tools\aria2\aria2-1.37.0-win-64bit-build1\aria2c.exe"
$Curl = "C:\Windows\System32\curl.exe"
$Proxy = "http://127.0.0.1:7897"
$Root = "F:\models\FastVideo-Minimax-FastH3-Preview-v0.2"
$Repo = "FastVideo/FastVideo-Minimax-FastH3-Preview-v0.2"
$Base = "https://huggingface.co/$Repo/resolve/main"
$Api = "https://huggingface.co/api/models/$Repo/tree/main"
Remove-Item env:HTTP_PROXY, env:HTTPS_PROXY, env:ALL_PROXY, env:http_proxy, env:https_proxy -EA SilentlyContinue

function Get-Tree([string]$Sub) {
    $url = if ($Sub) { "$Api/$Sub" } else { $Api }
    $tmp = Join-Path $env:TEMP ("fasth3-tree-{0}.json" -f ($Sub -replace '[\\/]', '-'))
    & $Curl -sL --fail --max-time 45 -x $Proxy $url -o $tmp | Out-Null
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $tmp)) { return @() }
    $items = @(Get-Content $tmp -Raw | ConvertFrom-Json)
    foreach ($it in $items) {
        if ($Sub -and $it.path -notmatch "/") {
            $it.path = "$Sub/$($it.path)"
        }
    }
    return $items
}

Write-Host "listing HuggingFace files..."
$entries = @()
$entries += Get-Tree ""
foreach ($sub in @("text_encoder", "transformer", "vae", "tokenizer", "processor", "audio_vae")) {
    $entries += Get-Tree $sub
}
$files = @($entries | Where-Object { $_.type -eq "file" -and $_.size -gt 0 } | Sort-Object path -Unique)
if ($files.Count -lt 20) {
    Write-Host "tree listing incomplete, using fallback names"
    $files = @()
    1..14 | ForEach-Object { $files += [pscustomobject]@{ path = ("text_encoder/model-{0:d5}-of-00014.safetensors" -f $_); size = 0 } }
    1..14 | ForEach-Object { $files += [pscustomobject]@{ path = ("transformer/diffusion_pytorch_model-{0:d5}-of-00014.safetensors" -f $_); size = 0 } }
    1..3 | ForEach-Object { $files += [pscustomobject]@{ path = ("vae/diffusion_pytorch_model-{0:d5}-of-00003.safetensors" -f $_); size = 0 } }
}

$todo = @()
foreach ($f in $files) {
    $rel = $f.path
    $dest = Join-Path $Root ($rel -replace '/', '\')
    $have = if (Test-Path $dest) { (Get-Item $dest).Length } else { 0 }
    if ($f.size -gt 0 -and $have -eq $f.size) { continue }
    if ($f.size -eq 0 -and $rel.EndsWith(".safetensors") -and $have -ge 4GB) { continue }
    $todo += [pscustomobject]@{ Rel = $rel; Dest = $dest; Size = [int64]$f.size; Have = $have }
}

$todo = @($todo | Sort-Object { if ($_.Size -gt 0) { $_.Size } else { 4GB } })
Write-Host ("need {0} files, {1:N1} GB remaining" -f $todo.Count, ((($todo | Measure-Object Size -Sum).Sum - ($todo | Measure-Object Have -Sum).Sum) / 1GB))
$todo | ForEach-Object { "{0,-72} {1,8:N0}/{2,8:N0} MB" -f $_.Rel, ($_.Have/1MB), ($_.Size/1MB) }

foreach ($item in $todo) {
    $dir = Split-Path $item.Dest -Parent
    $name = Split-Path $item.Dest -Leaf
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Write-Host ("==== {0} ({1:N1}/{2:N1} MB) ====" -f $item.Rel, ($item.Have/1MB), ($item.Size/1MB))
    $url = "$Base/$($item.Rel)"
    if ($item.Rel -notmatch "\.safetensors$" -or ($item.Size -gt 0 -and $item.Size -lt 50MB)) {
        & $Curl -L --fail --retry 8 --retry-delay 2 --http1.1 -x $Proxy -o $item.Dest $url
    } else {
        & $Aria --console-log-level=notice --summary-interval=8 `
            --max-connection-per-server=16 --split=16 --min-split-size=4M `
            --continue=true --auto-file-renaming=false --allow-overwrite=false `
            --file-allocation=none --max-tries=0 --retry-wait=2 `
            --connect-timeout=20 --timeout=120 `
            --all-proxy=$Proxy --dir=$dir --out=$name $url
    }
    $now = if (Test-Path $item.Dest) { (Get-Item $item.Dest).Length } else { 0 }
    if ($item.Size -gt 0 -and $now -eq $item.Size) {
        Write-Host ("OK {0}" -f $item.Rel)
    } else {
        Write-Host ("done {0} now {1:N1} MB exit {2}" -f $item.Rel, ($now/1MB), $LASTEXITCODE)
    }
}
Write-Host "aria2 pass finished"
