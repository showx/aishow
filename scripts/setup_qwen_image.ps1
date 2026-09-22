# Create a dedicated venv for Qwen-Image-2.1.
# Needs transformers>=5.17 and git-main diffusers. Do not reuse ComfyUI / LLaDA Python.

. "$PSScriptRoot\_env.ps1"
$Models = Get-AishowEnv "MODELS_ROOT"
if (-not $Models) {
    Write-Host "缺少 MODELS_ROOT。请复制 scripts\paths.example.bat 为 scripts\paths.bat。"
    exit 1
}
$Venv = Get-AishowEnv "QWEN_IMAGE_VENV"
if (-not $Venv) { $Venv = Join-Path $Models "qwen-image-venv" }
$Py = Join-Path $Venv "Scripts\python.exe"
$BasePy = Get-AishowEnv "QWEN_IMAGE_BASE_PYTHON"
if (-not $BasePy) { $BasePy = "python" }

if (-not (Test-Path $Py)) {
    Write-Host ">> create venv $Venv"
    & $BasePy -m venv $Venv
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $Py)) {
        Write-Host "创建 venv 失败。请先安装本机 Python 3.10+（带 CUDA 的 PyTorch 也可事后再装）。"
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
    & $Py -c "import torchvision" 2>$null
    if ($LASTEXITCODE -ne 0) {
        Write-Host ">> install torchvision (CUDA 12.4 wheel)"
        & $Py -m pip install torchvision --index-url https://download.pytorch.org/whl/cu124
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }
}

Write-Host ">> transformers / diffusers / accelerate / pillow"
& $Py -m pip install "transformers>=5.17" accelerate pillow
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& $Py -m pip install "git+https://github.com/huggingface/diffusers"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ">> check QwenImage21Pipeline"
& $Py -c "from diffusers import QwenImage21Pipeline; print('QwenImage21Pipeline OK')"
if ($LASTEXITCODE -ne 0) {
    Write-Host "diffusers 还没有 QwenImage21Pipeline。确认 git 版 diffusers 已装上。"
    exit 1
}

Write-Host "`nvenv: $Venv"
Write-Host "在 scripts\paths.bat 写:"
Write-Host "  set `"QWEN_IMAGE_PYTHON=$Py`""
Write-Host "  set `"QWEN_IMAGE_MODEL=$(Join-Path $Models 'Qwen-Image-2.1')`""
