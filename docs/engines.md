# 引擎与权重

控制面（Go）不加载模型。本机 Python 边车负责推理；工坊只负责选引擎、投队列。

24GB 单卡不要同时加载两个大模型。打开「排队时自动切换模型」后，队列可以混着堆：当前引擎跑完再切。

## 1. 写本机路径

复制一份不入库的路径文件，按你的磁盘改：

```bat
copy scripts\paths.example.bat scripts\paths.bat
```

至少填写：

| 变量 | 含义 |
| --- | --- |
| `H3_ROOT` | MiniMax-H3 / DiffSynth / ComfyUI 所在根目录 |
| `COMFY_ROOT` | ComfyUI 便携版目录（含 `python_embeded`） |
| `MODELS_ROOT` | FastH3 GGUF、PinkCherry、LLaDA 等权重根目录 |

可选：`LLADA_REPO`、`LLADA_MODEL`、`ARIA2C`、`AISHOW_DOWNLOAD_PROXY`。

下载脚本默认直连 Hugging Face。只有填了 `AISHOW_DOWNLOAD_PROXY` 才会走代理。装了 [aria2](https://github.com/aria2/aria2/releases) 并写上 `ARIA2C` 会快很多。

## 2. 按显存选引擎

| 引擎 | 大约显存 | 能力 | 下载 | 启动 | 端口 |
| --- | --- | --- | --- | --- | --- |
| H3-Base NF4 | 24GB 可跑 | 文生 / 首尾帧 | DiffSynth NF4 | `scripts\start_h3_nf4.bat` | `30010` |
| H3 Turbo LoRA | 24GB 可跑 | 文生 / 首尾帧，4 步 | `download_h3_turbo_lora.ps1` | `start_h3_turbo_lora.bat` | `30012` |
| H3 PinkCherry INT8 | 24GB 可跑 | 文生 / 首尾帧 | `download_h3_pinkcherry_int8.ps1` | `start_h3_pinkcherry_int8.bat` | `30013` / Comfy `8189` |
| H3 Ref2VA INT8 | 24GB 可跑 | 参考生成 | `download_h3_ref2va.ps1` | `start_h3_ref2va_int8.bat` | `30011` / Comfy `8188` |
| H3 Timeline Director | 24GB 可跑 | 文生 / 参考，SelfLift 二采 | `download_h3_latent_upscaler.ps1` | `start_h3_director.bat` | `30014` / Comfy `8188` |
| FastH3 GGUF Q4 | 24GB 推荐 | 仅文生，4 步 | `download_fasth3_gguf.ps1` | `start_fasth3_gguf.bat` | `8000` / Comfy `8188` |
| LLaDA-Image | 视 Turbo / Base | 文生图 / 指令编辑 | 见下方 | `start_llada_image.bat` | `30020` |
| FastH3 全量 bf16 | 远超 24GB | 仅文生 | `download_fasth3.ps1` | `start_fasth3.bat` | `8000` |
| 官方 SGLang H3-Base | 多卡 | 文生 / 首尾帧 / 参考 | Hugging Face `MiniMaxAI/MiniMax-H3` | 见下方 | `30010` / `30011` |

视频边车：`POST /v1/videos` → 轮询 → 下载 MP4。LLaDA-Image：`POST /v1/images` → 轮询 → 下载 PNG。

## 3. 下载并启动

在仓库根目录用 PowerShell，例如 FastH3 GGUF：

```powershell
powershell -ExecutionPolicy Bypass -File scripts\download_fasth3_gguf.ps1
scripts\start_fasth3_gguf.bat
```

边车窗口不要关。回到浏览器「推理节点」：

1. 推理模式改成 **auto**
2. 确认对应地址（上表端口）
3. 保存节点配置
4. 去工坊选这个引擎，先投一条短的试跑

本地 H3 输出短边 768、24fps、32kHz 立体声。2K 再生仍走官方 H3-Regenerate-2K，未进默认队列。

## Timeline Director（SelfLift 二采）

用来对照「能不能更快出片」。边车提交 [ComfyUI-MiniMaxH3-TimelineDirector](https://github.com/Songssx/ComfyUI-MiniMaxH3-TimelineDirector) 的素材规划台 + 有限分段采样。有 H3 Latent Upscaler 时，约 `75%` 步数在半分辨率跑，再 latent 抬清做剩余高清步。作者实测 `1536×832 / 29 秒` 大约 10 分钟，本机 24GB 请先用 `480p / 5 秒 / 8 步` 和原来的 Ref2VA 20 步对照。

```powershell
powershell -ExecutionPolicy Bypass -File scripts\download_h3_latent_upscaler.ps1
scripts\start_h3_director.bat
```

启动脚本会把插件克隆进 `ComfyUI/custom_nodes`。没有 Upscaler 也能跑，只是不会开二采。文生和参考生成都能走这套图；超过 15 秒自动分段续写，最长 30 秒。和 FastH3 / Ref2VA 共用 ComfyUI `8188`。

## 不要混用的两套 ComfyUI

PinkCherry 是独立一套：自己的权重目录 `H3_PINKCHERRY_ROOT` + ComfyUI **8189** + 边车 **30013**。

FastH3 GGUF、Ref2VA INT8 和 Timeline Director 共用另一套：权重在 `FASTH3_GGUF_ROOT`，ComfyUI **8188**。Director 还会把 [TimelineDirector](https://github.com/Songssx/ComfyUI-MiniMaxH3-TimelineDirector) 克隆进 `custom_nodes`，并读取 `models/latent_upscale_models/` 里的 H3 Latent Upscaler 做二采。

不要把 PinkCherry 文件丢进 `fasth3-gguf`，也不要两套抢同一个 8188。

## LLaDA-Image

```bash
git clone https://github.com/inclusionAI/LLaDA-Image.git
```

在 `scripts\paths.bat` 里设置：

```bat
set "LLADA_REPO=C:\path\to\LLaDA-Image"
set "LLADA_MODEL=%MODELS_ROOT%\LLaDA-Image-Turbo"
```

然后：

```bat
scripts\start_llada_image.bat
```

默认 Turbo（4 步）。要高品质 Base，把 `LLADA_MODEL` 指到 `inclusionAI/LLaDA-Image` 的本地目录，工坊档位也选 Base。

## 官方 SGLang（多卡）

```bash
sglang serve --model-path MiniMaxAI/MiniMax-H3 --num-gpus 4 --ulysses-degree 4 --performance-mode speed --host 0.0.0.0 --port 30010 --model-variant fl2va
sglang serve --model-path MiniMaxAI/MiniMax-H3 --num-gpus 4 --ulysses-degree 4 --performance-mode speed --host 0.0.0.0 --port 30011 --model-variant ref2va
```

把 `backend/data/media` 挂到推理容器的 `/data/minimax-h3`，与默认 `file://` 前缀对齐。本地边车一般把「素材 URI 模式」设成 `http` 即可。
