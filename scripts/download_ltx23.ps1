# LTX-2.3 distilled 1.1 + x2 spatial upscaler + Gemma 3 text encoder.
# Distilled is the 8-step checkpoint. The 46GB dev weights are not required.

. "$PSScriptRoot\_env.ps1"
$Models = Require-AishowEnv "MODELS_ROOT"
$Root = Get-AishowEnv "LTX23_ROOT"
if (-not $Root) { $Root = Join-Path $Models "LTX-2.3" }
$Gemma = Get-AishowEnv "LTX23_GEMMA"
if (-not $Gemma) { $Gemma = Join-Path $Models "gemma-3-12b-it-qat-q4_0-unquantized" }

Write-Host "权重: $Root"
$fail = Invoke-AishowHfRepo -Repo "Lightricks/LTX-2.3" -Dest $Root -Only @(
    "ltx-2.3-22b-distilled-1.1.safetensors",
    "ltx-2.3-spatial-upscaler-x2-1.1.safetensors"
)
Write-Host "文本编码器: $Gemma"
$fail2 = Invoke-AishowHfRepo -Repo "google/gemma-3-12b-it-qat-q4_0-unquantized" -Dest $Gemma

Write-Host "`n=== result ==="
foreach ($dir in @($Root, $Gemma)) {
    if (-not (Test-Path $dir)) { continue }
    Get-ChildItem $dir -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Length -gt 1MB } |
        ForEach-Object { "{0,8:N2} GB  {1}" -f ($_.Length / 1GB), $_.FullName }
}
if (($fail + $fail2) -ne 0) {
    Write-Host "有文件没下完。Gemma 若是 401，先在 Hugging Face 接受协议并设置 HF_TOKEN，再重跑。"
    exit 1
}
Write-Host "然后: powershell -File scripts\setup_ltx23.ps1"
Write-Host "然后: scripts\start_ltx23.bat"
