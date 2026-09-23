# Dedicated venv for HunyuanVideo-1.5. Needs a recent diffusers with HunyuanVideo15Pipeline.

. "$PSScriptRoot\_env.ps1"
$Models = Get-AishowEnv "MODELS_ROOT"
if (-not $Models) {
    Write-Host "缺少 MODELS_ROOT。请复制 scripts\paths.example.bat 为 scripts\paths.bat。"
    exit 1
}
$Venv = Get-AishowEnv "HUNYUAN_VENV"
if (-not $Venv) { $Venv = Join-Path $Models "hunyuan-video-venv" }
$Py = Join-Path $Venv "Scripts\python.exe"
$BasePy = Get-AishowEnv "HUNYUAN_BASE_PYTHON"
if (-not $BasePy) { $BasePy = "python" }

if (-not (Test-Path $Py)) {
    Write-Host ">> create venv $Venv"
    & $BasePy -m venv $Venv
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $Py)) {
        Write-Host "创建 venv 失败。请先安装本机 Python 3.10+。"
        exit 1
    }
}

Write-Host ">> pip / setuptools"
& $Py -m pip install -U pip setuptools wheel
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$torchOk = & $Py -c "import torch; print(torch.cuda.is_available())" 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Host ">> install torch + torchvision (CUDA 12.4 wheel)"
    & $Py -m pip install torch torchvision --index-url https://download.pytorch.org/whl/cu124
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} else {
    Write-Host ">> torch already present (cuda=$torchOk)"
}

Write-Host ">> diffusers / transformers / accelerate / imageio"
& $Py -m pip install "transformers>=4.57" accelerate pillow imageio imageio-ffmpeg
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& $Py -m pip install "git+https://github.com/huggingface/diffusers"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ">> check HunyuanVideo15Pipeline"
& $Py -c "from diffusers import HunyuanVideo15Pipeline, HunyuanVideo15ImageToVideoPipeline; print('HunyuanVideo15 OK')"
if ($LASTEXITCODE -ne 0) {
    Write-Host "diffusers 还没有 HunyuanVideo15Pipeline。确认 git 版 diffusers 已装上。"
    exit 1
}

Write-Host "`nvenv: $Venv"
Write-Host "python: $Py"
