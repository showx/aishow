# HunyuanVideo-1.5 480p text-to-video + image-to-video (Diffusers layout).
# 24GB cards should stay on 480p and CPU offload. 720p is a separate checkpoint.

. "$PSScriptRoot\_env.ps1"
$Models = Require-AishowEnv "MODELS_ROOT"
$T2V = Get-AishowEnv "HUNYUAN_T2V_MODEL"
if (-not $T2V) { $T2V = Join-Path $Models "HunyuanVideo-1.5-480p-t2v" }
$I2V = Get-AishowEnv "HUNYUAN_I2V_MODEL"
if (-not $I2V) { $I2V = Join-Path $Models "HunyuanVideo-1.5-480p-i2v" }

Write-Host "文生: $T2V"
$fail = Invoke-AishowHfRepo -Repo "hunyuanvideo-community/HunyuanVideo-1.5-Diffusers-480p_t2v" -Dest $T2V
Write-Host "图生: $I2V"
$fail2 = Invoke-AishowHfRepo -Repo "hunyuanvideo-community/HunyuanVideo-1.5-Diffusers-480p_i2v" -Dest $I2V

Write-Host "`n=== result ==="
foreach ($root in @($T2V, $I2V)) {
    if (-not (Test-Path $root)) { continue }
    $bytes = (Get-ChildItem $root -Recurse -File -ErrorAction SilentlyContinue | Measure-Object Length -Sum).Sum
    "{0,8:N2} GB  {1}" -f ($bytes / 1GB), $root
}
if (($fail + $fail2) -ne 0) {
    Write-Host "有文件没下完，直接重跑本脚本即可续传。"
    exit 1
}
Write-Host "然后: powershell -File scripts\setup_hunyuan_video.ps1"
Write-Host "然后: scripts\start_hunyuan_video.bat"
