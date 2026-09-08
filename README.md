# Aishow

本地 MiniMax-H3 影像工坊。Go 控制面负责任务队列、素材与成片；Vue 工坊负责投递生成；真正的 H3-Base 推理仍由你本机的 SGLang / vLLM 提供。

## 能力

- 文生影像 `t2va`、首帧 / 尾帧 / 首尾帧 `fl2va`、参考生成 `ref2va`
- 开源文生图 / 指令编辑：`LLaDA-Image`（`t2i` / `i2i`）
- SQLite 持久化队列：排队、插队、取消、重试、实时进度
- 对接 SGLang 异步接口 `POST /v1/videos` → 轮询 → 下载 MP4
- 对接本机 LLaDA-Image 边车 `POST /v1/images` → 轮询 → 下载 PNG
- 可选调用官方 H3-Context-IR 做提示增强
- `mock` 模式：没有 GPU 也能把队列和工坊跑通

## 目录

```
backend/    Go API · 队列 · Worker
frontend/   Vue 3 工坊界面
```

## 本机 NF4（你现在这份）

`E:\MiniMax-H3\models\MiniMax-H3-NF4` 是 DiffSynth 的 4bit 量化包，给消费级显卡用，不走 SGLang。

已齐：FL2VA DiT、Text Encoder、Video VAE、Audio VAE，以及 `models\MiniMax-H3\FL2VA\processor`。
未齐：`minimax-h3-ref2va-nf4.safetensors`（还是 `.incomplete`），所以参考生成先别用。

3090 Ti 24GB 启动：

```bat
D:\code\aishow\scripts\start_h3_nf4.bat
```

它会在 `http://127.0.0.1:30010` 提供和 SGLang 一样的 `/v1/videos`。Aishow 工坊把模式设成 `auto` 即可把任务打到这份 NF4 上。

## 本机 LLaDA-Image（开源生图）

[LLaDA-Image](https://github.com/inclusionAI/LLaDA-Image) 是 inclusionAI 的 6B 开源文生图 / 指令编辑模型。控制面不加载权重，由 Python 边车跑 Diffusers pipeline。

本机已装到 `F:\LLaDA-Image`（源码 + `pydeps`），权重在 `F:\models\LLaDA-Image-Turbo`。复用 ComfyUI 的 CUDA PyTorch，不另装一份。

```bat
D:\code\aishow\scripts\start_llada_image.bat
```

默认 Turbo（4 步）。要高品质 Base：

```bat
set LLADA_MODEL=F:\models\LLaDA-Image
D:\code\aishow\scripts\start_llada_image.bat
```

边车监听 `http://127.0.0.1:30020`。工坊选「LLaDA-Image」，推理模式设成 `auto`。

## 本机 FastH3（GGUF Q4，24GB 推荐）

完整 FastVideo Preview（约 144GB bf16）这台 64GB 内存机器跑不了。24GB 走 ComfyUI + Q4 GGUF + 量化文本编码器。

```bat
D:\code\aishow\scripts\start_fasth3_gguf.bat
```

脚本会拉起 ComfyUI `http://127.0.0.1:8188`，以及 Aishow 边车 `http://127.0.0.1:8000`（`POST /v1/videos`，模型别名 `fasth3`）。工坊选「FastH3 本地」，推理模式设成 `auto`。只支持文生，4 步 + VSA。权重在 `F:\models\fasth3-gguf`。

完整 FastVideo 边车仍是 `scripts\start_fasth3.bat`。官方 4 卡路径：`fastvideo serve --config D:\code\aishow\scripts\openai_fasth3.yaml`。

## 启动

```bash
# 后端
cd backend
copy .env.example .env   # Windows
go mod tidy
go run ./cmd/server

# 前端
cd frontend
npm install
npm run dev
```

浏览器打开 `http://127.0.0.1:5173`。后端默认在 `http://127.0.0.1:9808`。没有 GPU 时，在「推理节点」把模式改成 `mock`。

局域网同事访问：后端默认监听 `0.0.0.0:9808`（`AISHOW_HOST`），前端开发服务器默认 `0.0.0.0:5173`（`frontend/.env` 的 `VITE_DEV_HOST`）。同一网段打开 `http://<本机局域网IP>:9808`（已构建 `frontend/dist`）或 `:5173`（`npm run dev`）。Windows 需放行对应端口。`AISHOW_PUBLIC_BASE_URL` 给本机推理回源用，保持 `127.0.0.1`。仅本机调试时把 `AISHOW_HOST` / `VITE_DEV_HOST` 改成 `127.0.0.1`。

首次访问会要求创建账号，该账号自动成为管理员。之后在「用户管理」里开账号、改角色、重置密码或删除用户；任务、素材和成片按账号隔离。可在 `backend/.env` 用 `AISHOW_ADMIN_USER` / `AISHOW_ADMIN_PASSWORD` 预置管理员。公开注册开关也在用户管理页。

## 对接本地 H3-Base

```bash
sglang serve --model-path MiniMaxAI/MiniMax-H3 --num-gpus 4 --ulysses-degree 4 --performance-mode speed --host 0.0.0.0 --port 30010 --model-variant fl2va
sglang serve --model-path MiniMaxAI/MiniMax-H3 --num-gpus 4 --ulysses-degree 4 --performance-mode speed --host 0.0.0.0 --port 30011 --model-variant ref2va
```

把 `backend/data/media` 挂到推理容器的 `/data/minimax-h3`，与默认 `file://` 前缀对齐。

本地 H3-Base 输出短边 768、24fps、32kHz 立体声。2K 再生仍走官方 H3-Regenerate-2K，未做进默认队列。
