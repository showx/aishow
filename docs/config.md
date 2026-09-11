# 配置与局域网

## 三份本地文件

| 文件 | 复制自 | 用途 |
| --- | --- | --- |
| `backend/.env` | `backend/.env.example` | 控制面监听、推理模式、边车地址、会话 |
| `frontend/.env` | `frontend/.env.example` | Vite 开发服务器与 API 代理 |
| `scripts/paths.bat` | `scripts/paths.example.bat` | 本机权重与工具路径 |

后两份已加入 `.gitignore`，不要提交真实路径或 Token。

`scripts\dev.bat` 在文件不存在时会自动复制两份 `.env`，默认推理模式是 `mock`。

## 控制面常用项

| 变量 | 默认 | 含义 |
| --- | --- | --- |
| `AISHOW_HOST` | `0.0.0.0` | `0.0.0.0` 允许局域网；`127.0.0.1` 仅本机 |
| `AISHOW_PORT` | `9808` | API 与已构建前端 |
| `AISHOW_PUBLIC_BASE_URL` | `http://127.0.0.1:9808` | 边车回源地址，本机请保持回环 |
| `AISHOW_INFERENCE_MODE` | `mock` | `mock` 演示队列；`auto` 调本机边车 |
| `AISHOW_AUTO_SWITCH_ENGINE` | `true` | 排队跨引擎时自动停旧启新 |
| `AISHOW_MAX_LOADED_ENGINES` | `1` | 同时驻留几个边车。24GB 保持 1 |
| `AISHOW_WORKER_CONCURRENCY` | `1` | 同时推理几条。改完需重启后端 |
| `AISHOW_URI_MODE` | `file` | `http` = 边车回源控制面；`file` = 容器内路径 |
| `AISHOW_ALLOW_REGISTER` | `true` | 也可在用户管理页随时开关 |
| `AISHOW_ADMIN_USER` / `AISHOW_ADMIN_PASSWORD` | 空 | 预置管理员；不填则首次打开页面时创建 |
| `AISHOW_MINIMAX_API_TOKEN` | 空 | 可选。H3-Context-IR 提示增强 |

边车地址：`AISHOW_SGLANG_FL2VA_URL`（30010）、`AISHOW_SGLANG_REF2VA_URL`（30011）、`AISHOW_FASTH3_URL`（8000）、`AISHOW_H3_TURBO_URL`（30012）、`AISHOW_H3_PINKCHERRY_URL`（30013）、`AISHOW_LLADA_IMAGE_URL`（30020）。

完整注释见 `backend/.env.example`。界面「推理节点」里改的项会写入 SQLite，优先于启动时的 `.env`。

## 前端开发

| 变量 | 默认 | 含义 |
| --- | --- | --- |
| `VITE_DEV_HOST` | `0.0.0.0` | 开发服务器是否对局域网开放 |
| `VITE_DEV_PORT` | `5173` | 开发端口 |
| `VITE_API_PROXY` | `http://127.0.0.1:9808` | `/api` 代理。写本机回环，不要写成局域网 IP |
| `VITE_ALLOWED_HOSTS` | `true` | 允许用机器名 / 局域网 IP 打开 Vite |

## 局域网访问

同一网段打开：

- 已构建 `frontend/dist`：`http://<本机局域网IP>:9808`
- 开发热更新：`http://<本机局域网IP>:5173`

Windows 防火墙需要放行 `9808`（以及开发时的 `5173`）。

`AISHOW_PUBLIC_BASE_URL` 仍然保持 `127.0.0.1`：那是边车在**这台机器上**回源用的，不是给同事浏览器用的。

仅本机调试时，把 `AISHOW_HOST` 和 `VITE_DEV_HOST` 改成 `127.0.0.1`。

局域网走 Vite 时，若 Origin 被拒，在 `backend/.env` 加：

```env
AISHOW_CORS_ORIGINS=http://192.168.1.10:5173
AISHOW_CORS_LAN=true
```

## 数据落在哪

默认都在 `backend/data/`：

```
backend/data/aishow.db    任务、账号、设置
backend/data/uploads/     上传的图片 / 音视频
backend/data/media/       交给边车的素材
backend/data/outputs/     成片 MP4 / PNG
```

整目录已 gitignore。备份工坊 = 备份这份目录和 `.env`。

## 常见问题

**提交后马上成功，但没有视频？**  
推理模式还是 `mock`。到「推理节点」改成 `auto`，并确认对应边车窗口还在。

**提示引擎离线 / 任务失败？**  
要么打开「排队时自动切换模型」，要么先手动跑对应 `scripts\start_*.bat`。关了自动切换就只跑已经在线的引擎。

**显存爆了 / 第二个模型起不来？**  
24GB 把「同时最多加载几个模型」保持为 1。不要同时开两个 `start_*.bat` 抢同一张卡。PinkCherry 的 8189 也不要和 FastH3 的 8188 混用。

**局域网能打开页面，但生成失败、边车报连不上？**  
检查 `AISHOW_PUBLIC_BASE_URL` 是不是被改成了局域网 IP。本机边车回源应走 `127.0.0.1:9808`。

**下载权重很慢或失败？**  
在 `scripts\paths.bat` 写 `AISHOW_DOWNLOAD_PROXY`，可选再写 `ARIA2C`。

**改了 `.env` 没生效？**  
重启 `go run ./cmd/server`。界面里保存过的节点配置以数据库为准。

**想刷新 README 截图？** 前后端起来后：

```bat
cd docs
npm install playwright
set AISHOW_URL=http://127.0.0.1:5173
node capture-screenshots.mjs
```

脚本会读 `backend/.env` 里的管理员账号登录，截图写到 `docs/screenshots/`。不要把密码写进仓库。
