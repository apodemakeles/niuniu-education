# 牛牛教辅系统

给自家孩子用的英语单词教辅系统。当前为 **MVP 阶段（家长端单词库管理）**。

> 设计文档：[`design/docs/architecture.md`](design/docs/architecture.md) · 需求：[`design/docs/parent-word-library-management.md`](design/docs/parent-word-library-management.md)

## 目录结构

```
niuniu-education/
├── design/      # 设计内容：需求、架构文档与原型
│   ├── docs/
│   └── prototype/
├── frontend/    # 前端：Vue 3 + TypeScript + Vite
├── backend/     # 后端：Go + chi + SQLite
├── bin/         # 构建产物（gitignore）
└── Makefile
```

## 技术栈

| | 选型 |
|---|---|
| 前端 | Vue 3 + TypeScript + Vite + Pinia + Vue Router |
| 后端 | Go + chi + SQLite（纯 Go 驱动 `modernc.org/sqlite`，`CGO_ENABLED=0`） |
| 部署 | **前后端独立部署，本地同机**：前端静态托管，后端单 API 二进制 |

## 快速开始（开发模式）

需要 Go ≥ 1.22 与 Node ≥ 20。

**1. 构建后端二进制**

```bash
make backend       # 产物：bin/niuniu-api
```

**2. 启动后端**（终端 A）

```bash
make dev-backend   # 监听 127.0.0.1:8787
```

首次启动会在当前目录创建 `data/`（含 `config.yaml`、`niuniu.db`、`images/`、`exports/`）。

**3. 启动前端**（终端 B）

```bash
cd frontend && npm install   # 首次
make dev-frontend            # vite dev server，监听 5173，/api 代理到 8787
```

**4. 访问**

打开浏览器 `http://localhost:5173`。

> vite dev server 仅监听 IPv6 的 localhost，请用 `localhost` 而非 `127.0.0.1` 访问。

## 验证联通

```bash
curl http://localhost:8787/api/v1/health        # 后端健康检查
curl http://localhost:8787/api/v1/words         # 单词列表（含预置示例）
curl http://localhost:5173/api/v1/words         # 经 vite proxy 转发
```

## 配置

配置文件位于 `data/config.yaml`（首次启动自动生成，可手动编辑后重启）：

- `dataDir`：数据目录（默认 `./data`）
- `server.port`：后端端口（默认 8787）
- `server.cors.allowedOrigins`：放行的前端来源（含 5173/4173/8080）
- `ocr.provider`：OCR 提供方（MVP 仅 `mock`）

也可用环境变量覆盖：`NIUNIU_DATA_DIR`、`NIUNIU_PORT`、`NIUNIU_OCR_PROVIDER`。

## 当前进度（M0 骨架）

- ✅ 前后端独立工程脚手架
- ✅ 后端：config、SQLite 连接与迁移（预置原型示例数据）、chi 路由、CORS、健康检查
- ✅ 前端：Vue3+TS、Pinia、Router、vite proxy、API client、单词表渲染
- ✅ 前后端联调跑通

下一步 M1：单词 CRUD 数据层与完整接口。
