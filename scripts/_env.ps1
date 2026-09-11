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
}

$Aria = Get-AishowEnv "ARIA2C"
$Proxy = Get-AishowEnv "AISHOW_DOWNLOAD_PROXY"
$Curl = if (Test-Path "$env:SystemRoot\System32\curl.exe") { "$env:SystemRoot\System32\curl.exe" } else { "curl.exe" }

function Require-AishowEnv([string]$Name) {
    $v = Get-AishowEnv $Name
    if (-not $v) {
        Write-Host "缺少 $Name。请复制 scripts\paths.example.bat 为 scripts\paths.bat 并填写本机路径。"
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
        [int64]$Size = 0
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
        & $Aria @ariaArgs
        return $LASTEXITCODE
    }
    $curlArgs = @(
        "-L", "--fail", "--retry", "8", "--retry-all-errors", "--retry-delay", "3",
        "-C", "-", "--connect-timeout", "30", "--max-time", "0"
    ) + (Get-CurlProxyArgs) + @($Url, "-o", $dest)
    & $Curl @curlArgs
    return $LASTEXITCODE
}
