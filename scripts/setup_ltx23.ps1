# Clone Lightricks/LTX-2 and create its uv environment. The sidecar runs on that Python.

. "$PSScriptRoot\_env.ps1"
$Models = Get-AishowEnv "MODELS_ROOT"
if (-not $Models) {
    Write-Host "缺少 MODELS_ROOT。请复制 scripts\paths.example.bat 为 scripts\paths.bat。"
    exit 1
}
$Repo = Get-AishowEnv "LTX23_REPO"
if (-not $Repo) { $Repo = Join-Path $Models "LTX-2" }
$Uv = Get-AishowEnv "UV"
if (-not $Uv) { $Uv = "uv" }

if (-not (Test-Path (Join-Path $Repo ".git"))) {
    Write-Host ">> git clone LTX-2 -> $Repo"
    New-Item -ItemType Directory -Force -Path (Split-Path $Repo -Parent) | Out-Null
    git clone --depth 1 https://github.com/Lightricks/LTX-2.git $Repo
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} else {
    Write-Host ">> LTX-2 already cloned: $Repo"
}

$pyproject = Join-Path $Repo "pyproject.toml"
$pyText = Get-Content $pyproject -Raw
if ($pyText -notmatch 'environments = \["sys_platform == ''win32''"\]') {
    $pyText = $pyText -replace '(?m)^\[tool\.uv\]\r?\n', "[tool.uv]`r`nenvironments = [""sys_platform == 'win32'""]`r`n"
    Set-Content -Path $pyproject -Value $pyText -NoNewline
}

Push-Location $Repo
Write-Host ">> uv sync"
& $Uv sync --python-platform windows
if ($LASTEXITCODE -ne 0) {
    Pop-Location
    Write-Host "uv sync 失败。确认 uv 已安装：https://docs.astral.sh/uv/"
    exit $LASTEXITCODE
}
Pop-Location

$Py = Join-Path $Repo ".venv\Scripts\python.exe"
if (-not (Test-Path $Py)) {
    Write-Host "找不到 $Py"
    exit 1
}
& $Py -c "import ltx_pipelines; print('ltx_pipelines OK')"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "`nrepo: $Repo"
Write-Host "python: $Py"
