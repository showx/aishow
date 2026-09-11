# Aishow

本机 MiniMax-H3 影像工坊。控制面负责任务、账号与素材，推理由本机边车完成，互不加载对方的权重。

没有 GPU 时用 `mock` 模式即可跑通队列和界面。

## 架构

```
Vue 工坊  ──HTTP──►  Go 控制面  ──►  SQLite / 本地文件
                         │
                         ├── 队列 / 插队 / 取消 / 重试
                         ├── 账号隔离与会话
                         └── Worker + 编排器
                                   │
                    ┌──────────────┼──────────────┐
                    ▼              ▼              ▼
              SGLang / DiffSynth   ComfyUI     LLaDA-Image
              (H3 文生 / 首尾帧)   (GGUF / INT8)  (文生图 / 指令编辑)
```

控制面默认 `http://127.0.0.1:9808`，开发界面默认 `http://127.0.0.1:5173`。

| 层 | 技术 | 职责 |
| --- | --- | --- |
| 界面 | Vue 3 + Vite | 工坊、队列、成片、推理节点、用户管理 |
| 控制面 | Go + Gin + SQLite | API、队列、Worker、编排边车启停 |
| 推理 | 本机 Python 边车 | 按任务调用已启动的引擎，不进控制面进程 |

## 能力

- **视频**：文生 `t2va`，首帧 / 尾帧 / 首尾帧 `fl2va`，参考生成 `ref2va`
- **图片**：LLaDA-Image 文生图 `t2i`、指令编辑 `i2i`
- **队列**：SQLite 持久化；排队、插队、取消、重试；进度实时回传
- **引擎编排**：24GB 单卡默认同时只加载 1 个模型；开「排队时自动切换」后，当前任务跑完再切引擎
- **账号**：首次访问创建的账号即为管理员；任务、素材、成片按账号隔离
- **可选**：官方 H3-Context-IR 提示增强
- **演示**：`mock` 走完整队列，不加载权重

本地 H3 输出短边 768、24fps、32kHz 立体声。2K 再生仍走官方 H3-Regenerate-2K，未进默认队列。

## 快速开始

需要 [Go](https://go.dev/dl/) 1.21+、[Node.js](https://nodejs.org/) 20+。无需 GPU。

```bat
scripts\dev.bat
```

脚本会补齐缺失的 `.env`，并分别拉起后端与前端。浏览器打开 http://127.0.0.1:5173 。

手动启动：

```bash
# 后端
cd backend
cp .env.example .env    # Windows: copy .env.example .env
go mod tidy
go run ./cmd/server

# 前端（另开终端）
cd frontend
cp .env.example .env
npm install
npm run dev
```

首次访问会创建管理员。之后可在「用户管理」开账号、改角色、重置密码。默认推理模式是 `mock`。

## 接上本机推理

控制面不加载权重。本机路径写进一份**不入库**的文件：

```bat
copy scripts\paths.example.bat scripts\paths.bat
```

| 变量 | 含义 |
| --- | --- |
| `H3_ROOT` | MiniMax-H3 / DiffSynth / ComfyUI 所在根目录 |
| `COMFY_ROOT` | ComfyUI 便携版目录（含 `python_embeded`） |
| `MODELS_ROOT` | FastH3 GGUF、PinkCherry、LLaDA 等权重根目录 |

可选：`LLADA_REPO`、`ARIA2C`、`AISHOW_DOWNLOAD_PROXY`。下载脚本默认直连 Hugging Face，填了代理才会走代理。

然后按下一节下载权重、启动边车，再在「推理节点」把模式改成 `auto`。

24GB 单卡不要同时加载两个大模型。PinkCherry 使用独立权重目录和 ComfyUI `8189`，不要和 FastH3 / Ref2VA 的 `8188` 混用。

## 引擎

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

视频边车协议：`POST /v1/videos` → 轮询 → 下载 MP4。LLaDA-Image：`POST /v1/images` → 轮询 → 下载 PNG。

### LLaDA-Image

```bash
git clone https://github.com/inclusionAI/LLaDA-Image.git
# 在 scripts\paths.bat 里设置 LLADA_REPO、LLADA_MODEL
scripts\start_llada_image.bat
```

默认 Turbo（4 步）。高品质 Base 把 `LLADA_MODEL` 指到 `inclusionAI/LLaDA-Image` 的本地目录。

### 官方 SGLang（多卡）

```bash
sglang serve --model-path MiniMaxAI/MiniMax-H3 --num-gpus 4 --ulysses-degree 4 --performance-mode speed --host 0.0.0.0 --port 30010 --model-variant fl2va
sglang serve --model-path MiniMaxAI/MiniMax-H3 --num-gpus 4 --ulysses-degree 4 --performance-mode speed --host 0.0.0.0 --port 30011 --model-variant ref2va
```

把 `backend/data/media` 挂到推理容器的 `/data/minimax-h3`，与默认 `file://` 前缀对齐。本地边车一般用 `http` 回源即可。

## 局域网

后端默认监听 `0.0.0.0:9808`（`AISHOW_HOST`），前端开发服务器默认 `0.0.0.0:5173`（`VITE_DEV_HOST`）。同一网段打开：

- 已构建 `frontend/dist`：`http://<本机局域网IP>:9808`
- 开发热更新：`http://<本机局域网IP>:5173`

`AISHOW_PUBLIC_BASE_URL` 给本机推理回源用，保持 `127.0.0.1`。仅本机调试时把 `AISHOW_HOST` / `VITE_DEV_HOST` 改成 `127.0.0.1`。Windows 需放行对应端口。

可在 `backend/.env` 用 `AISHOW_ADMIN_USER` / `AISHOW_ADMIN_PASSWORD` 预置管理员。公开注册开关在用户管理页。

## 配置

| 文件 | 用途 |
| --- | --- |
| `backend/.env.example` → `backend/.env` | 控制面监听、推理模式、边车地址、会话 |
| `frontend/.env.example` → `frontend/.env` | Vite 开发服务器与 API 代理 |
| `scripts/paths.example.bat` → `scripts/paths.bat` | 本机权重与工具路径（已加入 `.gitignore`） |

## 目录

```
backend/     Go API、队列、Worker、Python 边车
frontend/    Vue 3 工坊
scripts/     启动与下载脚本；本机路径见 paths.bat
```
