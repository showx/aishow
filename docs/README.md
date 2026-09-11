# 文档

给下载下来自己跑的人看。先读仓库根目录的 [README](../README.md) 做安装，再按需要点进来。

| 文档 | 内容 |
| --- | --- |
| [使用说明](usage.md) | 登录、指挥台、工坊、队列、作品库、节点、用户（含截图） |
| [引擎与权重](engines.md) | `paths.bat`、按显存选引擎、下载与启动 |
| [配置与局域网](config.md) | `.env`、账号、CORS、数据备份、常见问题 |

界面截图在 [screenshots/](screenshots/)。本机前后端起来后，可用 `npm install` + `node capture-screenshots.mjs` 重新截（会读 `backend/.env` 里的管理员账号，不要把密码写进仓库）。
