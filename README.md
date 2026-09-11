# Aishow

本地 MiniMax-H3 影像工坊。Go 控制面负责任务队列、素材与成片；Vue 工坊负责投递生成；推理由本机边车（SGLang / DiffSynth / ComfyUI / LLaDA-Image）完成。

没有 GPU 也能先把队列和界面跑通（`mock` 模式）。

## 能力

- 文生影像 `t2va`、首帧 / 尾帧 / 首尾帧 `fl2va`、参考生成 `ref2va`
- 开源文生图 / 指令编辑：`LLaDA-Image`（`t2i` / `i2i`）
- SQLite 持久化队列：排队、插队、取消、重试、实时进度
- 对接 SGLang / 本机边车：`POST /v1/videos` → 轮询 → 下载 MP4
- 对接 LLaDA-Image 边车：`POST /v1/images` → 轮询 → 下载 PNG
- 可选调用官方 H3-Context-IR 做提示增强
- `mock` 模式：没有 GPU 也能把队列和工坊跑通

## 5 分钟上手（无需 GPU）

需要 [Go](https://go.dev/dl/) 1.22+ 和 [Node.js](https://nodejs.org/) 20+。

```bat
scripts\dev.bat
```

或手动：

```bash
# 后端
cd backend
cp .env.example .env    # Windows: copy .env.example .env
go mod tidy
go run ./cmd/server

# 前端（另开一个终端）
cd frontend
cp .env.example .env
npm install
npm run dev
```

浏览器打开 http://127.0.0.1:5173 。后端默认 http://127.0.0.1:9808 。

首次访问会创建账号，该账号自动成为管理员。之后在「用户管理」里开账号、改角色、重置密码。任务、素材和成片按账号隔离。

默认推理模式是 `mock`：提交任务会走完整队列，但不加载模型。

## 接上本机推理

控制面不加载权重。本机路径写进一份**不入库**的文件：

```bat
copy scripts\paths.example.bat scripts\paths.bat
```

至少填写：

| 变量 | 含义 |
| --- | --- |
| `H3_ROOT` | MiniMax-H3 / DiffSynth / ComfyUI 所在根目录 |
| `COMFY_ROOT` | ComfyUI 便携版目录（含 `python_embeded`） |
| `MODELS_ROOT` | FastH3 GGUF、PinkCherry、LLaDA 等权重根目录 |

可选：`LLADA_REPO`、`ARIA2C`、`AISHOW_DOWNLOAD_PROXY`（下载脚本默认直连 Hugging Face，只有填了代理才会走代理）。

然后按下面表格下载权重、启动边车，再在「推理节点」把模式改成 `auto`。

## 引擎一览

| 引擎 | 大约显存 | 能力 | 下载 | 启动 | 端口 |
| --- | --- | --- | --- | --- | --- |
| H3-Base NF4 | 24GB 可跑 | 文生 / 首尾帧 | DiffSynth NF4 | `scripts\start_h3_nf4.bat` | `30010` |
| H3 Turbo LoRA | 24GB 可跑 | 文生 / 首尾帧，4 步 | `download_h3_turbo_lora.ps1` | `start_h3_turbo_lora.bat` | `30012` |
| H3 PinkCherry INT8 | 24GB 可跑 | 文生 / 首尾帧 | `download_h3_pinkcherry_int8.ps1` | `start_h3_pinkcherry_int8.bat` | `30013` / Comfy `8189` |
| H3 Ref2VA INT8 | 24GB 可跑 | 参考生成 | `download_h3_ref2va.ps1` | `start_h3_ref2va_int8.bat` | `30011` / Comfy `8188` |
| FastH3 GGUF Q4 | 24GB 推荐 | 仅文生，4 步 | `download_fasth3_gguf.ps1` | `start_fasth3_gguf.bat` | `8000` / Comfy `8188` |
| LLaDA-Image | 视 Turbo / Base | 文生图 / 指令编辑 | 见下方 | `start_llada_image.bat` | `30020` |
| FastH3 全量 bf16 | 远超 24GB | 仅文生 | `download_fasth3.ps1` | `start_fasth3.bat` | `8000` |
| 官方 SGLang H3-Base | 多卡 | 文生 / 首尾帧 / 参考 | Hugging Face `MiniMaxAI/MiniMax-H3` | 见下方 | `30010` / `30011` |

24GB 单卡不要同时加载两个大模型。打开「排队时自动切换模型」后，队列可以混着堆：当前引擎跑完再切。

PinkCherry 是独立一套（自己的权重目录 + ComfyUI `8189`），不要和 FastH3 / Ref2VA 的 `8188` 混用。

### LLaDA-Image

```bash
git clone https://github.com/inclusionAI/LLaDA-Image.git
# 在 scripts\paths.bat 里设置 LLADA_REPO、LLADA_MODEL
scripts\start_llada_image.bat
```

默认 Turbo（4 步）。要高品质 Base，把 `LLADA_MODEL` 指到 `inclusionAI/LLaDA-Image` 的本地目录。

### 官方 SGLang（多卡）

```bash
sglang serve --model-path MiniMaxAI/MiniMax-H3 --num-gpus 4 --ulysses-degree 4 --performance-mode speed --host 0.0.0.0 --port 30010 --model-variant fl2va
sglang serve --model-path MiniMaxAI/MiniMax-H3 --num-gpus 4 --ulysses-degree 4 --performance-mode speed --host 0.0.0.0 --port 30011 --model-variant ref2va
```

把 `backend/data/media` 挂到推理容器的 `/data/minimax-h3`，与默认 `file://` 前缀对齐。本地边车一般用 `http` 回源即可。

## 局域网

后端默认监听 `0.0.0.0:9808`（`AISHOW_HOST`），前端开发服务器默认 `0.0.0.0:5173`（`VITE_DEV_HOST`）。同一网段打开 `http://<本机局域网IP>:9808`（已构建 `frontend/dist`）或 `:5173`（`npm run dev`）。

`AISHOW_PUBLIC_BASE_URL` 给本机推理回源用，保持 `127.0.0.1`。仅本机调试时把 `AISHOW_HOST` / `VITE_DEV_HOST` 改成 `127.0.0.1`。Windows 需放行对应端口。

可在 `backend/.env` 用 `AISHOW_ADMIN_USER` / `AISHOW_ADMIN_PASSWORD` 预置管理员。公开注册开关在用户管理页。

## 目录

```
backend/    Go API · 队列 · Worker · Python 边车
frontend/   Vue 3 工坊界面
scripts/    启动 / 下载脚本（本机路径见 paths.bat）
```

## 配置

- 控制面：`backend/.env.example` → `backend/.env`
- 前端开发：`frontend/.env.example` → `frontend/.env`
- 推理路径：`scripts/paths.example.bat` → `scripts/paths.bat`（已加入 `.gitignore`）

本地 H3 输出短边 768、24fps、32kHz 立体声。2K 再生仍走官方 H3-Regenerate-2K，未做进默认队列。
