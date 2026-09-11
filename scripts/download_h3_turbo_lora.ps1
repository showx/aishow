# Resume-safe MiniMax-H3 Turbo LoRA.
# Default: lightx2v 4-step v1.2. Optional: larryvrh EMA ckpt850.

. "$PSScriptRoot\_env.ps1"
$loraDir = Require-AishowEnv "H3_LORA_DIR"

$jobs = @(
    @{
        Name = "lightx2v Turbo 4-step v1.2"
        Dir  = $loraDir
        Out  = "minimax_h3_fl2v_turbo_4step_v1.2_768p_bf16.safetensors"
        Url  = "https://huggingface.co/lightx2v/Minimax-h3-Turbo/resolve/main/minimax_h3_fl2v_turbo_4step_v1.2_768p_bf16.safetensors"
    },
    @{
        Name = "larryvrh Turbo EMA ckpt850"
        Dir  = $loraDir
        Out  = "minimax_h3_turbo_4step_ema_ckpt850.safetensors"
        Url  = "https://huggingface.co/larryvrh/MiniMax-H3-Turbo-Lora/resolve/main/minimax_h3_turbo_4step_ema_ckpt850.safetensors"
    }
)

$code = 0
foreach ($j in $jobs) {
    Write-Host ("==== {0} ====" -f $j.Name)
    $c = Invoke-AishowDownload -Dir $j.Dir -Out $j.Out -Url $j.Url
    if ($c -ne 0) { $code = $c }
}

Write-Host "`n=== result ==="
foreach ($j in $jobs) {
    $dest = Join-Path $j.Dir $j.Out
    $now = if (Test-Path $dest) { (Get-Item $dest).Length } else { 0 }
    $ok = if ($now -gt 100MB) { "OK" } else { "INCOMPLETE" }
    "{0,-12} {1,8:N2} GB  {2}" -f $ok, ($now / 1GB), $dest
}
Write-Host "`nDiffSynth 边车默认用 lightx2v。要换 larry：set H3_TURBO_LORA=$loraDir\minimax_h3_turbo_4step_ema_ckpt850.safetensors"
exit $code
