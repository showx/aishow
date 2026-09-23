# Shared path / download helpers for scripts\*.ps1
# Usage:  . "$PSScriptRoot\_env.ps1"

$ErrorActionPreference = "Continue"
$AishowScripts = $PSScriptRoot
$AishowRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

$pathsFile = Join-Path $AishowScripts "paths.bat"
if (Test-Path $pathsFile) {
    Get-Content $pathsFile | ForEach-Object {
        $line = $_.Trim()
        if ($line -match '^(?:rem\b|#)') { return }
        if ($line -match '^\s*set\s+"?([A-Za-z0-9_]+)=(.*)$') {
            $name = $Matches[1]
            $value = $Matches[2].Trim()
            if ($value.EndsWith('"')) { $value = $value.Substring(0, $value.Length - 1) }
            [Environment]::SetEnvironmentVariable($name, $value, "Process")
        }
    }
}

function Get-AishowEnv([string]$Name, [string]$Default = "") {
    $v = [Environment]::GetEnvironmentVariable($Name, "Process")
    if ([string]::IsNullOrWhiteSpace($v)) { return $Default }
    return $v
}

if (-not (Get-AishowEnv "AISHOW_MEDIA_ROOT")) {
    $env:AISHOW_MEDIA_ROOT = Join-Path $AishowRoot "backend\data\media"
}
if (-not (Get-AishowEnv "H3_MEDIA_ROOT")) { $env:H3_MEDIA_ROOT = $env:AISHOW_MEDIA_ROOT }

$H3Root = Get-AishowEnv "H3_ROOT"
$ModelsRoot = Get-AishowEnv "MODELS_ROOT"
if ($H3Root) {
    if (-not (Get-AishowEnv "H3_NF4_DIR")) { $env:H3_NF4_DIR = Join-Path $H3Root "models\MiniMax-H3-NF4" }
    if (-not (Get-AishowEnv "H3_LORA_DIR")) { $env:H3_LORA_DIR = Join-Path $H3Root "models\loras" }
    if (-not (Get-AishowEnv "COMFY_ROOT")) { $env:COMFY_ROOT = Join-Path $H3Root "ComfyUI_windows_portable" }
}
if ($ModelsRoot) {
    if (-not (Get-AishowEnv "FASTH3_GGUF_ROOT")) { $env:FASTH3_GGUF_ROOT = Join-Path $ModelsRoot "fasth3-gguf" }
    if (-not (Get-AishowEnv "H3_PINKCHERRY_ROOT")) { $env:H3_PINKCHERRY_ROOT = Join-Path $ModelsRoot "pinkcherry-h3" }
    if (-not (Get-AishowEnv "FASTH3_LOCAL_DIR")) {
        $full = Join-Path $ModelsRoot "FastVideo-Minimax-FastH3-Preview-v0.2"
        if (Test-Path $full) { $env:FASTH3_LOCAL_DIR = $full }
    }
    if (-not (Get-AishowEnv "QWEN_IMAGE_MODEL")) {
        $env:QWEN_IMAGE_MODEL = Join-Path $ModelsRoot "Qwen-Image-2.1"
    }
    if (-not (Get-AishowEnv "HUNYUAN_T2V_MODEL")) {
        $env:HUNYUAN_T2V_MODEL = Join-Path $ModelsRoot "HunyuanVideo-1.5-480p-t2v"
    }
    if (-not (Get-AishowEnv "HUNYUAN_I2V_MODEL")) {
        $env:HUNYUAN_I2V_MODEL = Join-Path $ModelsRoot "HunyuanVideo-1.5-480p-i2v"
    }
    if (-not (Get-AishowEnv "LTX23_ROOT")) {
        $env:LTX23_ROOT = Join-Path $ModelsRoot "LTX-2.3"
    }
    if (-not (Get-AishowEnv "LTX23_GEMMA")) {
        $env:LTX23_GEMMA = Join-Path $ModelsRoot "gemma-3-12b-it-qat-q4_0-unquantized"
    }
    if (-not (Get-AishowEnv "LTX23_REPO")) {
        $env:LTX23_REPO = Join-Path $ModelsRoot "LTX-2"
    }
}

$Aria = Get-AishowEnv "ARIA2C"
$Proxy = Get-AishowEnv "AISHOW_DOWNLOAD_PROXY"
$Curl = if (Test-Path "$env:SystemRoot\System32\curl.exe") { "$env:SystemRoot\System32\curl.exe" } else { "curl.exe" }

function Require-AishowEnv([string]$Name) {
    $v = Get-AishowEnv $Name
    if (-not $v) {
        Write-Host "Missing $Name. Copy scripts\paths.example.bat to scripts\paths.bat and fill local paths."
        exit 1
    }
    return $v
}

function Get-CurlProxyArgs {
    if ($Proxy) { return @("-x", $Proxy) }
    return @()
}

function Invoke-AishowDownload {
    param(
        [Parameter(Mandatory = $true)][string]$Dir,
        [Parameter(Mandatory = $true)][string]$Out,
        [Parameter(Mandatory = $true)][string]$Url,
        [int64]$Size = 0,
        [string]$Token = ""
    )
    New-Item -ItemType Directory -Force -Path $Dir | Out-Null
    $dest = Join-Path $Dir $Out
    $have = if (Test-Path $dest) { (Get-Item $dest).Length } else { 0 }
    if ($Size -gt 0 -and $have -eq $Size) {
        Write-Host ("SKIP already complete {0:N2} GB  {1}" -f ($have / 1GB), $dest)
        return 0
    }
    if ($Size -eq 0 -and $have -gt 100MB) {
        Write-Host ("SKIP already present {0:N2} GB  {1}" -f ($have / 1GB), $dest)
        return 0
    }
    Write-Host ("==== {0} ====" -f $Out)
    if ($Aria -and (Test-Path $Aria)) {
        $ariaArgs = @(
            "--console-log-level=notice", "--summary-interval=15",
            "--max-connection-per-server=16", "--split=16", "--min-split-size=8M",
            "--continue=true", "--auto-file-renaming=false", "--allow-overwrite=false",
            "--file-allocation=none", "--max-tries=0", "--retry-wait=3",
            "--connect-timeout=20", "--timeout=120",
            "--dir=$Dir", "--out=$Out", $Url
        )
        if ($Proxy) { $ariaArgs = @("--all-proxy=$Proxy") + $ariaArgs }
        if ($Token) { $ariaArgs = @("--header=Authorization: Bearer $Token") + $ariaArgs }
        & $Aria @ariaArgs | Out-Host
        return [int]$LASTEXITCODE
    }
    $curlArgs = @(
        "-L", "--fail", "--retry", "8", "--retry-all-errors", "--retry-delay", "3",
        "-C", "-", "--connect-timeout", "30", "--max-time", "0"
    )
    if ($Token) { $curlArgs += @("-H", "Authorization: Bearer $Token") }
    $curlArgs += (Get-CurlProxyArgs) + @($Url, "-o", $dest)
    & $Curl @curlArgs | Out-Host
    return [int]$LASTEXITCODE
}

function Get-AishowHfToken {
    foreach ($name in @("HF_TOKEN", "HUGGING_FACE_HUB_TOKEN")) {
        $token = Get-AishowEnv $name
        if ($token) { return $token }
    }
    $homes = @()
    $hfHome = Get-AishowEnv "HF_HOME"
    if ($hfHome) { $homes += $hfHome }
    $homes += (Join-Path $env:USERPROFILE ".cache\huggingface")
    foreach ($hfDir in $homes) {
        foreach ($rel in @("token", "stored_tokens")) {
            $p = Join-Path $hfDir $rel
            if (-not (Test-Path $p)) { continue }
            $raw = (Get-Content $p -Raw).Trim()
            if ($raw -match '(?m)^hf_[A-Za-z0-9]+') { return $Matches[0] }
            if ($raw -match '^hf_') { return ($raw -split '\s+')[0] }
        }
    }
    return ""
}

function Invoke-AishowHfRepo {
    param(
        [Parameter(Mandatory = $true)][string]$Repo,
        [Parameter(Mandatory = $true)][string]$Dest,
        [string[]]$Only = @()
    )
    $api = "https://huggingface.co/api/models/$Repo/tree/main?recursive=1"
    $headers = @{ "User-Agent" = "aishow-download" }
    $token = Get-AishowHfToken
    if ($token) { $headers["Authorization"] = "Bearer $token" }
    $irm = @{ Uri = $api; Headers = $headers }
    if ($Proxy) { $irm["Proxy"] = $Proxy }
    try {
        $items = Invoke-RestMethod @irm
    } catch {
        Write-Host "无法列出 $Repo : $($_.Exception.Message)"
        if ($Repo -like "google/*") {
            Write-Host "Gemma 是门控仓库。请打开 https://huggingface.co/$Repo 接受协议，再设置 HF_TOKEN 后重跑。"
        }
        return 1
    }
    $failed = 0
    $count = 0
    foreach ($item in @($items)) {
        if ($item.type -ne "file") { continue }
        $rel = [string]$item.path
        if ($rel -match '(^|/)\.') { continue }
        $leaf = Split-Path ($rel -replace "/", "\") -Leaf
        if ($Only.Count -gt 0 -and ($Only -notcontains $leaf)) { continue }
        $relWin = $rel -replace "/", "\"
        $parent = Split-Path $relWin -Parent
        $dir = if ($parent) { Join-Path $Dest $parent } else { $Dest }
        $size = [int64]0
        if ($item.lfs -and $item.lfs.size) { $size = [int64]$item.lfs.size }
        elseif ($item.size) { $size = [int64]$item.size }
        $url = "https://huggingface.co/$Repo/resolve/main/$rel"
        Write-Host ">> $rel"
        $code = Invoke-AishowDownload -Dir $dir -Out $leaf -Url $url -Size $size -Token $token
        $count++
        if ($code -ne 0) {
            $failed++
            Write-Host "FAIL $rel exit=$code"
        }
    }
    if ($count -eq 0) {
        Write-Host "仓库 $Repo 没有匹配到要下载的文件。"
        return 1
    }
    return $failed
}
