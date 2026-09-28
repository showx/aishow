# leejet/stable-diffusion.cpp Windows CUDA 12 build.
# https://github.com/leejet/stable-diffusion.cpp/releases/tag/master-929-3f8527a

. "$PSScriptRoot\_env.ps1"
$models = Require-AishowEnv "MODELS_ROOT"
$Root = Join-Path $models "stable-diffusion.cpp"
$ZipName = "sd-master-3f8527a-bin-win-cuda12-x64.zip"
$Url = "https://github.com/leejet/stable-diffusion.cpp/releases/download/master-929-3f8527a/$ZipName"
$Server = Join-Path $Root "sd-server.exe"
$tar = Join-Path $env:SystemRoot "System32\tar.exe"
New-Item -ItemType Directory -Force -Path $Root | Out-Null

if (-not (Test-Path $Server)) {
    Write-Host ">> $ZipName"
    $code = Invoke-AishowDownload -Dir $Root -Out $ZipName -Url $Url
    if ($code -ne 0) {
        Write-Host "FAIL $ZipName exit=$code"
        exit $code
    }
    $zip = Join-Path $Root $ZipName
    Write-Host "extract $zip"
    & $tar -xf $zip -C $Root
    if (-not (Test-Path $Server)) {
        Write-Host "解压后找不到 sd-server.exe"
        exit 1
    }
} else {
    Write-Host "SKIP already installed $Server"
}

# ggml-cuda.dll needs the CUDA 12 runtime. It is a separate release asset.
$CudartName = "cudart-sd-bin-win-cu12-x64.zip"
$CudartDll = Join-Path $Root "cudart64_12.dll"
if (-not (Test-Path $CudartDll)) {
    $cudartUrl = "https://github.com/leejet/stable-diffusion.cpp/releases/download/master-929-3f8527a/$CudartName"
    Write-Host ">> $CudartName"
    $code = Invoke-AishowDownload -Dir $Root -Out $CudartName -Url $cudartUrl
    if ($code -ne 0) {
        Write-Host "FAIL $CudartName exit=$code"
        exit $code
    }
    Write-Host "extract $CudartName"
    & $tar -xf (Join-Path $Root $CudartName) -C $Root
    if (-not (Test-Path $CudartDll)) {
        $found = Get-ChildItem $Root -Recurse -Filter "cudart64_12.dll" | Select-Object -First 1
        if (-not $found) {
            Write-Host "解压后找不到 cudart64_12.dll"
            Get-ChildItem $Root -Filter "*.dll" | Select-Object -ExpandProperty Name
            exit 1
        }
    }
} else {
    Write-Host "SKIP already installed $CudartDll"
}

Get-ChildItem $Root -Filter "sd-*.exe" | ForEach-Object { $_.FullName }
Write-Host "`nbin: $Root"
