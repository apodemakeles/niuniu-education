# 牛牛教辅系统 · 架构设计文档

> 适用范围：家长端单词库管理（MVP）
> 版本：v1.2 · 2026-07-04
> 变更：前后端独立部署（去 embed）；字段与功能严格对齐原型（删除例句/难度/必背/批量/行内编辑/修改记录/设置页）
> 配套文档：[家长端单词库管理设计](./parent-word-library-management.md)、原型 `../prototype/parent-word-library/`

---

## 1. 总览

### 1.1 产品定位

给自家孩子用的英语单词教辅系统。**MVP 只实现家长端单词库管理**，目标是让家长低成本、可持续地维护孩子要学的单词内容。孩子学习端、复习算法、报表等全部预留字段、暂不实现。

### 1.2 设计目标

| 目标 | 说明 |
|------|------|
| 单机即用 | 本地同机启动前后端，浏览器访问 `http://localhost:端口`，零外部依赖 |
| 前后端分离 | 前端独立构建、独立静态托管，后端只提供 REST API，两者解耦演进 |
| 零运维 | SQLite 单文件、数据目录可配置、无外部数据库/缓存服务 |
| 易迁移 | 所有数据（库 + 原图 + 导出）集中在一个目录，复制即备份 |
| 可扩展 | 学习端、复习算法、真实 OCR 通过预留字段和 Provider 接口平滑接入 |

### 1.3 技术决策一览

| 维度 | 选型 | 理由 |
|------|------|------|
| 前端 | Vue 3 + TypeScript + Vite | 表单/弹窗密集场景友好，模板直观 |
| UI 组件 | 自研轻量组件 + 局部引用 Element Plus 按需组件 | 对齐原型风格，避免全家桶体积 |
| 后端语言 | Go | 后端编译为单个 API 二进制、交叉编译、并发模型适合本地服务 |
| HTTP 路由 | `go-chi/chi` v5 | 轻量、贴近 `net/http`、依赖少 |
| 数据库 | SQLite（`modernc.org/sqlite` 纯 Go 驱动） | 纯 Go、`CGO_ENABLED=0`、可交叉编译单二进制 |
| ORM/查询 | `sqlc` 生成类型安全代码 + `database/sql` | 编译期 SQL 正确性，无运行时反射 |
| 配置 | YAML + 环境变量覆盖 | 本地工具友好 |
| 日志 | `log/slog`（标准库） | 结构化、零依赖 |
| OCR | MVP Mock，后端 Provider 适配层 | 解耦厂商，未来接视觉模型零改动契约 |
| 部署 | 前后端**独立部署**：前端静态托管，后端单二进制 API | 职责清晰，各自独立演进、独立发版 |
| 导出 Word | 服务端生成 HTML + `application/msword` 头 | 零依赖，原型已验证可行 |

> 说明：`modernc.org/sqlite` 是 SQLite 的纯 Go 翻译实现，无需 CGO，本地单机量级性能完全够用，且保证 `CGO_ENABLED=0` 一键交叉编译。
>
> 说明：前端构建产物（`dist/`）由独立静态服务托管，**不打包进 Go 二进制**。开发期前端走 vite dev server 并代理 `/api` 到后端；本机部署期用轻量静态服务（如 `vite preview` / Nginx / `http-server`）托管前端，后端只暴露 `/api/v1/*`。

---

## 2. 运行形态与部署

### 2.1 启动方式（前后端独立部署，本地同机）

```
本机 (localhost)
┌─────────────────────────────────────────────────────────────┐
│                                                              │
│  ┌──────────────────────┐        ┌────────────────────────┐ │
│  │  前端静态服务         │        │  后端 API 服务          │ │
│  │  (vite preview /     │  /api  │  niuniu-api (Go 二进制) │ │
│  │   Nginx / http-server│ ──────>│  chi router             │ │
│  │   托管 dist/)         │  proxy │  /api/v1/*              │ │
│  │  端口 5173 / 80       │        │  端口 8787              │ │
│  └──────────┬───────────┘        └───────────┬────────────┘ │
│             │                                │              │
│             │  浏览器加载 SPA                 │              │
│             ▼                                ▼              │
│        ┌───────────────┐          ┌────────────────────┐    │
│        │  浏览器 (用户)  │<─────────│  SQLite + 文件系统  │    │
│        └───────────────┘  JSON    │  data/niuniu.db     │    │
│                                    │  data/images/        │    │
│                                    │  data/exports/       │    │
│                                    └────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

- **后端**：运行 `./niuniu-api`，启动 HTTP API 服务（默认 `127.0.0.1:8787`），只暴露 `/api/v1/*`，不托管任何前端静态资源。
- **前端**：`npm run build` 产出 `dist/`，由独立的静态服务托管（本机推荐 `vite preview` 或 `npx http-server dist`；长期可用 Nginx）。浏览器访问前端端口。
- **前后端通信**：前端通过 `/api/v1/*` 调后端。开发期用 vite proxy 解决跨域；本机部署期由前端静态服务（Nginx 等）反向代理 `/api` 到后端，或后端开启本机 CORS（见 §7.3）。
- 两个进程各自独立启停、独立发版，互不打包。

### 2.2 数据目录

默认在**程序工作目录**下创建 `./data/`，结构如下：

```
data/
├── niuniu.db          # SQLite 主库
├── niuniu.db-wal      # SQLite WAL 文件（运行时）
├── niuniu.db-shm
├── config.yaml        # 用户配置（首次启动自动生成）
├── images/            # OCR 原图与缩略图
│   └── 2026/07/
│       └── <uuid>.jpg
└── exports/           # 导出的默写表等文件
    └── dictation-20260704-xxxx.doc
```

**路径可配置**（满足"路径做成配置"要求）：
- 优先级：启动参数 `-data-dir` > 环境变量 `NIUNIU_DATA_DIR` > `config.yaml:dataDir` > 默认 `./data`。
- 配置文件本身的默认值即"当前工作目录"，首次启动写入 `config.yaml`。
- 启动时校验：目录不存在则创建；权限不可写则给出明确错误退出。

---

## 3. 目录结构

### 3.1 仓库总览

```
niuniu-education/
├── design/                        # 设计内容
│   ├── docs/                      # 需求与设计文档
│   │   ├── parent-word-library-management.md
│   │   └── architecture.md        # 本文档
│   └── prototype/                 # 原型（仅参考，不参与构建）
│       └── parent-word-library/
├── frontend/                      # 前端源码
└── backend/                       # 后端源码
```

### 3.2 后端结构（Go）

采用**分层 + 模块化**结构，避免过度抽象。一个 `wordlibrary` 业务模块贯穿 handler→service→store。

```
backend/
├── cmd/
│   └── niuniu/
│       └── main.go                # 入口：加载配置、初始化、启动 HTTP
├── internal/
│   ├── config/                    # 配置定义与加载
│   │   └── config.go
│   ├── server/                    # HTTP 服务装配（chi 路由、中间件）
│   │   ├── server.go
│   │   └── middleware.go          # 日志、Recover、CORS(本机)、请求ID
│   ├── platform/                  # 平台层（与具体业务无关的基础设施）
│   │   ├── storage/               # SQLite 连接、迁移
│   │   │   ├── sqlite.go
│   │   │   └── migrations/        # 嵌入式 SQL 迁移脚本
│   │   │       ├── 001_init.sql
│   │   │       └── ...
│   │   ├── fs/                    # 数据目录管理（images/exports 读写）
│   │   └── ocr/                   # OCR Provider 适配层
│   │       ├── provider.go        # Provider 接口定义
│   │       ├── mock.go            # 离线 Mock 实现
│   │       ├── siliconflow.go     # 硅基流动 OpenAI 兼容接口(Qwen3-VL/DeepSeek-OCR 等)
│   │       └── registry.go        # 按 config 选择 provider
│   ├── module/
│   │   └── wordlibrary/           # 单词库业务模块
│   │       ├── handler.go         # HTTP handler（chi handler）
│   │       ├── service.go         # 业务规则（去重、默认状态、删除策略）
│   │       ├── store.go           # 数据访问（sqlc 生成的查询包装）
│   │       ├── dto.go             # 请求/响应 DTO
│   │       ├── parser.go          # 粘贴文本解析
│   │       ├── draft.go           # 草稿领域逻辑（导入预览/确认）
│   │       └── export.go          # 默写表 Word 导出
│   ├── sqlc/                      # sqlc 生成代码（不手改）
│   │   ├── queries/               # 手写的 SQL 查询
│   │   │   └── word.sql
│   │   ├── gen/                   # 生成的 Go 代码
│   │   └── sqlc.yaml
│   └── version/                   # 版本信息（构建时注入）
├── go.mod
├── go.sum
└── Makefile                       # build / dev / migrate / test
```

**分层约定**：
- `handler` 只做 HTTP 编解码与参数校验，不写业务规则。
- `service` 承载业务规则（去重判断、默认学习状态、物理/逻辑删除判定）。
- `store` 只做数据访问，方法语义化命名（`CreateWord`、`SoftDeleteWord` 等）。
- 跨模块复用的基础设施放 `platform/`，业务专属放 `module/<name>/`。

### 3.3 前端结构（Vue 3）

```
frontend/
├── src/
│   ├── main.ts                    # 应用入口
│   ├── App.vue
│   ├── router/
│   │   └── index.ts               # 路由：单词库 / 设置
│   ├── api/                       # 后端 API 客户端（封装 fetch）
│   │   ├── client.ts              # 基础请求封装、错误处理
│   │   ├── words.ts               # 单词相关接口
│   │   ├── import.ts              # 导入(粘贴/OCR)与草稿接口
│   │   └── export.ts              # 导出接口
│   ├── stores/                    # Pinia store
│   │   ├── library.ts             # 单词库列表/筛选/统计
│   │   └── draft.ts               # 草稿预览状态（导入流程）
│   ├── views/
│   │   ├── LibraryView.vue        # 单词库详情页（主页面）
│   │   ├── ImportDraftView.vue    # 导入草稿预览页（手动/粘贴/拍照共用）
│   │   └── ExportView.vue         # 导出预览页
│   ├── components/
│   │   ├── layout/                # Topbar、AppShell、CategorySidebar
│   │   ├── word/                  # WordTable、WordTableRow、WordEditModal、
│   │   │                          # StatusBadge、TypeTag
│   │   ├── import/                # PasteImportModal、PhotoImportModal、
│   │   │                          # DraftTable、DraftRow、ConfidenceMark
│   │   ├── export/                # DictationPreview、ScopeSelector
│   │   └── common/                # Modal、Toast、ConfirmDialog、Button 等
│   ├── composables/               # useToast、useConfirm、useShortcut
│   ├── types/                     # 与后端 DTO 对齐的 TS 类型
│   ├── utils/                     # 文本解析、格式化
│   └── assets/                    # 全局样式（基于原型 styles.css）
├── index.html
├── vite.config.ts
├── tsconfig.json
└── package.json
```

---

## 4. 数据模型

### 4.1 设计要点

> **字段范围以原型为准**：单词主体保留原型出现的字段（英文、中文、音标、类型、状态）。例句已作为学习展示数据独立存入 `word_examples`；难度、是否必背、修改记录等仍不纳入 MVP，需要时随对应原型一起迭代。

- **唯一词库**：MVP 只有 1 个词库。建 `libraries` 表仅为预留，不暴露给用户维护。
- **学习状态**：由家长或后续学习系统维护；MVP 仅支持家长手动设置（与原型状态 pill 一致）。
- **删除策略**：未学物理删除，其余逻辑删除（`deleted_at` 软删除），不污染历史。
- **学习进度预留**：`first_learned_at`/`last_reviewed_at`/`review_count` 仅作孩子端预留字段，MVP 不写、不展示。
- **导入草稿**：草稿在确认入库前不入正式表，作为前端临时状态（见 §5.4）。

### 4.2 表结构

```sql
-- 001_init.sql

-- 词库（MVP 固定单库，name 字段预留，不向用户暴露编辑）
CREATE TABLE libraries (
    id            TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(8)))),
    name          TEXT NOT NULL DEFAULT '默认词库',
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 单词
CREATE TABLE words (
    id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(8)))),
    library_id      TEXT NOT NULL REFERENCES libraries(id),
    text            TEXT NOT NULL,                 -- 英文单词（创建后不可改）
    meaning_zh      TEXT NOT NULL DEFAULT '',      -- 中文释义
    phonetic        TEXT NOT NULL DEFAULT '',      -- 音标
    word_type       TEXT NOT NULL DEFAULT 'new'    -- new 新词 / mistake 易错词
                    CHECK (word_type IN ('new','mistake')),
    status          TEXT NOT NULL DEFAULT 'unlearned'
                    CHECK (status IN ('unlearned','learning','reinforce','mastered')),
    -- 学习进度预留（MVP 不写，留给学习端）
    first_learned_at TEXT,
    last_reviewed_at TEXT,
    review_count    INTEGER NOT NULL DEFAULT 0,
    -- 审计
    created_at      TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at      TEXT NOT NULL DEFAULT (datetime('now')),
    last_edited_at  TEXT,                          -- 最近一次属性编辑时间
    source          TEXT NOT NULL DEFAULT 'manual',-- manual/paste/photo 录入来源
    deleted_at      TEXT                           -- 软删除标记；NULL=未删除
);

-- 唯一约束：(词库内) text + meaning_zh 唯一，支撑 PRD 的去重规则
-- 注意：仅未软删除的行参与唯一性，故用 部分索引
CREATE UNIQUE INDEX uniq_word_text_meaning
    ON words(library_id, text, meaning_zh)
    WHERE deleted_at IS NULL;

-- 常用筛选索引
CREATE INDEX idx_words_type_status
    ON words(library_id, word_type, status)
    WHERE deleted_at IS NULL;

-- OCR 导入的原图记录（PRD：保留原图一段时间便于排查）
CREATE TABLE import_images (
    id            TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(8)))),
    library_id    TEXT NOT NULL REFERENCES libraries(id),
    file_path     TEXT NOT NULL,                   -- 相对 data 目录的路径
    mime_type     TEXT NOT NULL,
    uploaded_at   TEXT NOT NULL DEFAULT (datetime('now')),
    ocr_raw_text  TEXT,                            -- OCR 原始文本（排查用）
    provider      TEXT,                            -- 识别所用 provider
    status        TEXT NOT NULL DEFAULT 'pending'  -- pending/confirmed/abandoned
);

-- Schema 版本（迁移用）
CREATE TABLE schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO schema_migrations(version) VALUES ('001');
```

### 4.3 枚举与取值

| 字段 | 取值 | 中文 | 默认值规则 |
|------|------|------|-----------|
| `word_type` | `new` / `mistake` | 新词 / 易错词 | 手动录入默认 `new`；从易错词入口默认 `mistake` |
| `status` | `unlearned` / `learning` / `reinforce` / `mastered` | 未学 / 学习中 / 需强化 / 已掌握 | 新词→`unlearned`；易错词→`reinforce` |
| `source` | `manual` / `paste` / `photo` | 手动 / 粘贴 / 拍照 | — |

### 4.4 关键业务规则落库

| 规则（PRD） | 实现 |
|------|------|
| 重复按 `text + meaning_zh` 判定 | 部分唯一索引 `uniq_word_text_meaning`（仅未软删除行） |
| 仅 `text` 相同只提示不强拦 | service 层查 `text` 命中但释义不同 → 返回提示，仍允许保存 |
| 英文单词创建后不可改 | handler/service 层拒绝更新 `text` 字段 |
| 未学物理删除 | `status='unlearned'` 时 `DELETE`；否则置 `deleted_at` |
| 学习中/需强化/已掌握逻辑删除 | 置 `deleted_at`，保留学习记录字段 |
| 普通字段修改不重置学习进度 | service 仅更新被改字段，不触碰 `status`/`review_count` 等 |

---

## 5. API 设计

### 5.1 约定

- Base path：`/api/v1`
- 编码：`application/json; charset=utf-8`
- ID：字符串（SQLite 生成的 hex）
- 错误格式统一：

```json
{ "error": { "code": "WORD_DUPLICATE", "message": "词库内已存在该单词", "details": {...} } }
```

- 成功响应直接返回数据对象或 `{ "data": [...] }`。
- 时间：ISO 8601 字符串（SQLite 以 TEXT 存储）。

### 5.2 端点总览

#### 单词库与单词

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/libraries/current` | 获取当前唯一词库（含统计：总数/新词/易错词/最近更新） |
| GET | `/libraries/current/words` | 单词列表（支持筛选/搜索/分页） |
| POST | `/libraries/current/words` | 手动新增单个单词 |
| GET | `/words/{id}` | 获取单词详情 |
| PUT | `/words/{id}` | 编辑单词属性（不含 `text`） |
| DELETE | `/words/{id}` | 删除单词（按状态走物理/逻辑删除） |

#### 查询参数（GET 单词列表）

| 参数 | 取值 | 说明 |
|------|------|------|
| `type` | `all`/`new`/`mistake` | 分类筛选，默认 `all` |
| `status` | `all`/`unlearned`/`learning`/`reinforce`/`mastered` | 状态筛选 |
| `q` | 字符串 | 在 text/meaning_zh/phonetic 中模糊匹配 |
| `page`/`pageSize` | 整数 | 分页，默认 1 / 50 |

#### 导入（统一进入草稿流程）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/imports/parse` | 解析粘贴文本，返回结构化草稿行（不入库） |
| POST | `/imports/ocr` | 上传图片执行 OCR，返回草稿行（MVP 走 Mock） |
| POST | `/imports/confirm` | 确认草稿入库（校验、去重、按类型设默认状态、返回结果统计） |

#### 查重与导出

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/words/check-duplicate` | 录入前检查 text/meaningZh 是否已存在，返回提示级别 |
| POST | `/exports/dictation` | 导出默写表 Word（返回文件流） |
| POST | `/exports/dictation:preview` | 预览数据（前端渲染预览表） |

### 5.3 关键 DTO

**单词对象（响应）**

```ts
interface WordDTO {
  id: string;
  text: string;
  meaningZh: string;
  phonetic: string;
  wordType: 'new' | 'mistake';
  status: 'unlearned' | 'learning' | 'reinforce' | 'mastered';
  source: 'manual' | 'paste' | 'photo';
  createdAt: string;
  updatedAt: string;
  lastEditedAt: string | null;
}
```

**草稿行（导入流程核心载体）**

```ts
interface DraftRowDTO {
  rowId: string;          // 草稿内临时 ID（前端生成或后端返回）
  text: string;
  meaningZh: string;
  phonetic: string;
  wordType: 'new' | 'mistake';
  // 导入校验信息（由后端在 parse/ocr/confirm 阶段填充）
  confidence?: number;        // 0~1，OCR 置信度（photo 来源）
  issues?: DraftIssue[];      // 问题标记：low_confidence / missing_meaning / duplicate
  rowStatus?: 'pending' | 'modified' | 'invalid';
}
```

### 5.4 草稿流程（核心）

三种导入方式（手动/粘贴/拍照）**最终都汇入同一个草稿预览确认流程**（PRD 明确要求）。

```
┌─────────────┐   ┌─────────────┐   ┌─────────────┐
│  手动录入    │   │  粘贴文本    │   │  拍照/图片   │
│ (前端表单)   │   │ POST /parse │   │ POST /ocr   │
└──────┬──────┘   └──────┬──────┘   └──────┬──────┘
       │                 │                  │
       └────────┬────────┴──────────┬───────┘
                ▼                   ▼
        ┌─────────────────────────────────┐
        │   统一草稿预览表（前端状态）        │
        │   - 编辑/删除/增加行（对齐原型）     │
        │   - 后端实时校验重复/缺字段         │
        └────────────────┬────────────────┘
                         │ POST /imports/confirm
                         ▼
        ┌─────────────────────────────────┐
        │   后端：逐行校验 → 去重 →         │
        │   按类型设默认状态 → 写入 words    │
        │   返回：新增N / 跳过M / 更新K      │
        └─────────────────────────────────┘
```

**草稿存哪？**
- MVP：草稿存**前端内存/Pinia**，不落库。`confirm` 时把整批草稿行 POST 给后端一次性处理。
- 理由：草稿是临时态，未确认不入库；前端持有更简单，刷新丢失可接受（PRD 优先级低）。
- 预留：`import_images` 表已记录 OCR 原图与原始文本，便于排查，不存草稿行本身。

**确认入库的后端规则**（PRD "确认入库前的校验"）：

| 校验项 | 处理 |
|--------|------|
| 英文单词为空 | 拒绝入库，标记 `invalid`，返回该行错误 |
| 中文释义为空 | 允许入库但高亮提示（需前端确认） |
| 重复（text+meaning_zh 命中） | 默认跳过；前端可选"更新已有词" |
| 低置信度字段 | 不阻塞，但 `issues` 标记 `low_confidence` |
| 按类型设默认状态 | 新词→`unlearned`，易错词→`reinforce` |

**入库结果反馈**（PRD 要求）：
```ts
interface ImportResultDTO {
  added: number;
  skipped: number;     // 重复跳过
  updated: number;     // 更新已有词
  invalid: number;     // 未处理（校验失败）
  details: { rowId: string; result: 'added'|'skipped'|'updated'|'invalid'; wordId?: string; reason?: string }[];
}
```

---

## 6. OCR Provider 适配层

### 6.1 设计目标

把 OCR/视觉识别与业务解耦。MVP 用 Mock，未来接真实视觉模型时**前后端契约不变**（草稿行结构固定），只替换 `ocr.Provider` 实现。

### 6.2 接口定义

```go
// backend/internal/platform/ocr/provider.go

package ocr

// DraftRow 是 OCR/解析共同产出的草稿行（与前端 DraftRowDTO 对齐）。
type DraftRow struct {
    Text       string  `json:"text"`
    MeaningZh  string  `json:"meaningZh"`
    Phonetic   string  `json:"phonetic"`
    WordType   string  `json:"wordType"`   // 默认 "new"，由调用方覆盖
    Confidence float64 `json:"confidence"` // 0~1
}

// Result 是一次识别的完整结果。
type Result struct {
    Rows       []DraftRow `json:"rows"`
    RawText    string     `json:"rawText"`    // 原始文本（排查用）
    Unparsed   []string   `json:"unparsed"`   // 无法识别的行
}

// Provider 适配不同视觉模型厂商。
type Provider interface {
    Name() string
    // Recognize 对图片字节执行识别，返回结构化草稿。
    Recognize(ctx context.Context, image []byte, mimeType string) (*Result, error)
}
```

### 6.3 注册与选择

```go
// backend/internal/platform/ocr/registry.go

func New(cfg Config) (Provider, error) {
    switch cfg.Provider {
    case "mock":
        return NewMockProvider(), nil
    case "siliconflow":
        return NewSiliconFlowProvider(cfg.APIKey, cfg.Model), nil
    // 预留：glm4v / qwen-vl / ...
    default:
        return nil, fmt.Errorf("unknown ocr provider: %s", cfg.Provider)
    }
}
```

- `config.yaml` 中配置 `ocr.provider`、`ocr.apiKey`、`ocr.model`、`ocr.endpoint`。
- MVP 仅 `mock` 可选；切换真实 provider 通过编辑 `config.yaml` 后重启生效（MVP 无设置页）。

### 6.4 Mock 实现要点

复刻原型 `app.js` 的 mock 数据行为，保证前端体验一致：
- 输入任意图片 → 返回固定的 3 行示例（含一个低置信度词如 `clirnb` 供家长修正演示）。
- 不真实识别图片内容，但接口形态、返回结构、置信度字段完全与真实 provider 一致。

---

## 7. 配置设计

### 7.1 `config.yaml`（首次启动生成）

```yaml
# 服务
server:
  host: "127.0.0.1"
  port: 8787              # API 服务端口
  cors:
    enabled: true                 # 本机前后端分离必须开启（见 §7.3）
    allowedOrigins:               # 允许的前端来源
      - "http://localhost:5173"   # vite dev
      - "http://localhost:4173"   # vite preview
      - "http://localhost:8080"   # 其它本机静态服务

# 数据目录（相对程序工作目录；可改）
dataDir: "./data"

# 数据库
database:
  journalMode: "WAL"
  busyTimeout: 5000

# OCR
ocr:
  provider: "mock"        # mock / siliconflow / glm4v / qwen-vl
  endpoint: ""
  apiKey: ""
  model: ""

# 导出
export:
  retentionDays: 7        # exports/ 目录清理周期

# 图片保留
image:
  retentionDays: 30       # OCR 原图保留天数（排查用）

# 调试模式（正式给孩子用时应保持关闭）
debug:
  enabled: false          # true 时延伸阅读可跳过最短停留，立即完成

# 是否启动时自动打开浏览器（指向前端地址，见 openBrowserUrl）
openBrowser: true
openBrowserUrl: "http://localhost:5173"
```

### 7.2 配置加载优先级

启动参数 > 环境变量 > `config.yaml` > 代码默认值。

| 来源 | 示例 |
|------|------|
| 参数 | `-data-dir /path -port 9000` |
| 环境变量 | `NIUNIU_DATA_DIR`、`NIUNIU_PORT`、`NIUNIU_OCR_PROVIDER`、`NIUNIU_CORS_ALLOWED_ORIGINS`、`NIUNIU_DEBUG` |
| 配置文件 | `config.yaml` |

### 7.3 跨域（CORS）与本机部署

前后端分离部署后，前端（静态服务端口）与后端（API 端口）不同源，必须处理跨域。两种方式二选一：

| 方式 | 说明 | 适用 |
|------|------|------|
| **后端 CORS（默认推荐）** | 后端开启 CORS 中间件，放行 `allowedOrigins` 中的本机前端地址；前端直连后端 `http://127.0.0.1:8787/api` | 开发期、本机简单部署 |
| **前端静态服务反代 `/api`** | 前端用 Nginx 等把 `/api/*` 反向代理到后端，浏览器同源访问，无需 CORS | 长期/正式本机部署 |

MVP 默认走 **CORS 方式**（配置项见 §7.1 的 `server.cors`），开发期与 `vite preview` 部署期都适用，零额外组件。若后续用 Nginx 托管前端，则关闭 CORS 改走反代。

### 7.4 配置修改方式

- MVP：直接编辑 `config.yaml` 后重启生效（无 UI 设置页）。你要求的"数据目录做成配置"即通过此文件实现，默认值为当前工作目录。
- 安全性：本机工具，不做配置加密；API Key 明文存配置文件。

---

## 8. 前端架构

### 8.1 前后端连接（API base）

前端通过环境变量配置后端地址，构建时注入，避免硬编码：

```bash
# frontend/.env.development（开发期，配合 vite proxy，可写相对路径）
VITE_API_BASE=/api/v1

# frontend/.env.production（本机部署期，直连后端）
VITE_API_BASE=http://127.0.0.1:8787/api/v1
```

- 开发期：`VITE_API_BASE=/api/v1`，由 vite proxy 把 `/api` 转发到后端 `127.0.0.1:8787`，浏览器同源无需 CORS。
- 部署期：前端构建产物部署到独立静态服务，`VITE_API_BASE` 指向后端绝对地址，由后端 CORS 放行（§7.3）。
- `api/client.ts` 读取 `import.meta.env.VITE_API_BASE` 作为统一请求前缀。

### 8.2 路由

```
/                  → 重定向到 /library
/library           → 单词库详情页（主页）
```

导入草稿预览、编辑、导出均以**模态**形态叠加在主页之上（与原型一致），不单独占路由。MVP 无设置页（配置通过手编 `config.yaml`，见 §7）。

### 8.3 状态管理（Pinia）

- `useLibraryStore`：当前分类、筛选条件、分页、单词列表、统计数；封装加载与刷新。
- `useDraftStore`：草稿行数组、增删改、校验状态、确认提交；三种导入方式共用。
- `useToastStore` / `useConfirmStore`：全局轻提示与确认弹窗（替代 `window.confirm`）。

### 8.4 视觉与样式

- 以原型 `styles.css` 为**设计基线**：配色（绿 `#2f6f4f` / 米白 `#f7f4ea` / 橙 `#d76735`）、字体、表格、Tag、状态 Pill、弹窗等直接迁移。
- 全局样式以 CSS 变量维护，便于后续主题化。
- 表格密集页面用原生 `<table>` + 自定义组件（与原型一致），不引入重型表格组件。

### 8.5 交互实现原则

> **以原型为唯一视觉与交互基线**：MVP 不实现原型中不存在的功能（无批量操作、无行内编辑、无修改记录、无设置页）。原型 `app.js` 中的交互均 1:1 还原：

- 分类卡片切换（全部/新词/易错词）联动表格筛选。
- 搜索框 + 状态筛选下拉联动表格。
- 逐个录入：弹窗表单，保存不关闭、回车保存下一个，单词类型默认联动当前分类，重复词 `confirm` 提示。
- 编辑：弹窗，英文单词只读。
- 删除：未学物理删，其余软删（与原型 `deleteSavedWord` 一致）。
- 粘贴导入：文本框 → 解析草稿表 → 编辑/增删行 → 确认入库。
- 拍照导入：选图 + Mock OCR → 草稿表（含置信度低词如 `clirnb`）→ 修正 → 确认入库。
- 导出：弹窗选择范围 → 默写表预览（中文可临时编辑）→ 导出 Word。

---

## 9. 打包与构建

前后端**各自独立构建**，互不打包。前端产物由独立静态服务托管，后端是纯 API 二进制。

### 9.1 后端构建

```
cd backend
CGO_ENABLED=0 go build -o niuniu-api ./cmd/niuniu
```

- 产物：单个可执行文件 `niuniu-api`，仅暴露 `/api/v1/*`，不含任何前端资源。
- 纯 Go（`modernc.org/sqlite`），`CGO_ENABLED=0` 可直接交叉编译到 darwin/linux/windows。

### 9.2 前端构建

```
cd frontend
npm run build      # 产物输出到 frontend/dist/
```

- 产物：`dist/` 纯静态文件，由独立静态服务托管。
- 本机托管方式（任选其一）：
  - `npx vite preview --port 4173`（最简单，Vite 自带）
  - `npx http-server dist -p 8080`（零依赖 Node 静态服务）
  - Nginx 容器/服务（长期正式部署，配合 `/api` 反代）

### 9.3 产物布局

```
deploy/
├── niuniu-api                  # 后端二进制
├── data/                       # 数据目录（运行时生成）
└── frontend-dist/              # 前端静态产物（由静态服务托管）
    ├── index.html
    └── assets/
```

### 9.4 Makefile 目标

```makefile
dev:        # 启动前端 dev (vite:5173) + 后端 (air:8787)，前端 proxy 到后端
frontend:   # 仅前端 build → frontend/dist
backend:    # 仅后端 build → niuniu-api
build:      # frontend + backend 两者都构建
migrate:    # 执行/生成迁移
test:       # 后端单测 + 前端单测
preview:    # 前端 vite preview 托管 dist + 启动后端（本机联调）
```

### 9.5 开发模式

- 前端：`vite` dev server（5173），`vite.config.ts` 配置 proxy 把 `/api` 转发到后端 `127.0.0.1:8787`，开发期同源无需 CORS。
- 后端：`air` 或 `go run ./cmd/niuniu` 热重载，监听 8787。
- 两个进程独立运行、独立热重载。

---

## 10. MVP 范围与里程碑

### 10.1 MVP 必做（严格对齐原型）

1. ✅ 单词库总览（统计：总词数/新词/易错词 + 分类卡片：全部/新词/易错词）
2. ✅ 单词列表 + 搜索 + 状态筛选（与原型表格列、工具栏一致）
3. ✅ 手动逐个录入（连续录入、回车下一个、类型默认联动、重复提示）
4. ✅ 单词编辑（弹窗，英文不可改）+ 删除（未学物理删，其余软删）
5. ✅ 粘贴文本导入 → 草稿预览 → 确认入库
6. ✅ 拍照导入（Mock OCR）→ 草稿预览 → 修正 → 确认入库
7. ✅ 统一草稿预览（重复词、缺字段、低置信度提示，编辑/增删行）
8. ✅ 重复词处理（text+meaningZh 判定，仅 text 相同只提示）
9. ✅ 导出默写表 Word（预览 + 中文临时编辑 + 导出）
10. ✅ 本地同机部署：前端静态托管 + 后端 API 二进制 + 数据目录可配置（config.yaml）

### 10.2 MVP 不做（原型未体现，一律推迟）

- 例句、难度、是否必背字段
- 批量操作、行内编辑
- 修改记录（word_revisions）
- 设置页（配置走 config.yaml 文件）
- 真实 OCR（仅 mock，provider 接口就位）
- 发音、音标自动补全、例句自动补全
- 孩子学习端、复习算法、遗忘曲线、报表
- 多家庭/多用户/账号体系
- 教材内置词库模板

### 10.3 里程碑建议

| 里程碑 | 内容 | 产出 |
|--------|------|------|
| M0 骨架 | 前后端工程脚手架、配置、SQLite 迁移、CORS、前后端分离可跑通（vite proxy） | 空白页能启动并调通 API |
| M1 数据层 | 表结构、sqlc、store、单词 CRUD API | 接口可调通 |
| M2 核心 UI | 单词库页（列表/分类/筛选/搜索/统计）、手动新增/编辑/删除 | 原型主流程可用 |
| M3 导入流程 | 粘贴解析、Mock OCR、统一草稿预览、确认入库 | 三种导入打通 |
| M4 导出与打磨 | 默写表导出、删除策略、错误提示、视觉 1:1 对齐原型 | MVP 可交付 |

---

## 11. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| OCR 真实接入质量差 | 拍照导入体验差 | MVP 先 mock，契约固定；真实接入放迭代，且修正体验优先于自动化（PRD 强调） |
| SQLite 并发写 | 本地单用户基本无风险 | 启用 WAL + busyTimeout；单连接池 |
| 数据目录权限/磁盘满 | 启动失败、写入失败 | 启动校验 + 友好错误提示 |
| 前后端跨域配置错误 | 前端调不通 API | MVP 默认开启后端 CORS 并放行本机前端地址（§7.3）；提供 Nginx 反代备选方案 |
| 用户需同时启动两个进程 | 启动繁琐 | 提供统一启动脚本/Makefile `preview` 目标一键拉起前后端 |
| 配置文件被误删 | 设置丢失 | 启动时若不存在则用默认值重建 |
| 单词删除误操作 | 数据丢失 | 逻辑删除优先；物理删除（未学）二次确认 |
| 孩子/家长隐私（图片上传第三方） | 合规 | OCR provider 在 config.yaml 注明数据去向；mock 不外传 |

---

## 12. 待确认 / 后续细化

以下在编码阶段进一步细化，不阻塞架构落地：

1. 默认端口确定值（暂定 8787）。
2. 默认词库的 `id` 生成时机（首次迁移时插入固定记录 vs 启动时 ensure）。
3. 草稿是否需要"暂存到后端"以防刷新丢失（MVP 暂不做）。
4. 导出 Word 的中文临时编辑是否需要持久化（MVP 不持久化，仅当次导出生效）。

---

*本文档为架构基线，编码过程中如遇与 PRD 冲突，以 PRD 为准并回写本文档。*
