# LRSS

本地优先的桌面 RSS 阅读器：订阅、拉取入库、全文检索，可选 OpenAI 兼容 embedding 做语义搜索。

| 层 | 技术 |
| --- | --- |
| Shell | [Wails v3](https://v3.wails.io) |
| Frontend | Vue 3 · TypeScript · Vue Router · Vite |
| UI | [shadcn-vue](https://www.shadcn-vue.com) · Tailwind CSS v4 |
| 数据 | SQLite（`modernc.org/sqlite`）· FTS5 · 可选向量检索 |
| HTTP | [enetx/surf](https://github.com/enetx/surf)（`internal/httpx`） |
| i18n | vue-i18n · zh-CN / en-US（设置 → 外观） |

**当前阶段：S6 已落地** — 快捷键、文章保留清理、UI 设置持久化（[`docs/plan-s6.md`](docs/plan-s6.md)）。S5 i18n + surf 已完成。多 Agent 约定见 [`AGENTS.md`](AGENTS.md)。

## 功能

- **三栏资料库**：侧栏（智能列表 / 文件夹 / 订阅）· 文章列表 · 阅读器
- **智能列表**：未读 · 今日 · 收藏 · 全部
- **订阅**：添加 RSS/Atom URL，单源 / 全量刷新；ETag / Last-Modified 增量
- **OPML**：设置 → 订阅 导入/导出；侧栏新建文件夹
- **自动刷新**：设置 → 通用 开关与间隔（写入 SQLite，后台 ticker）
- **阅读**：已读 / 收藏 / 打开原文；HTML 经 bluemonday 消毒
- **搜索**：始终可用 FTS5；配置 embedding 后支持向量 / 混合模式
- **设置**：主题与阅读偏好；搜索 / AI（embedding、重建全部向量）
- **多语言**：设置 → 外观 → 界面语言（简体中文 / English）
- **快捷键**：j/k 上下篇、s 收藏、m 已读、r 刷新等（设置中可关）
- **保留策略**：过期非收藏文章自动/手动清理
- **通知**：后台/手动刷新发现新文章时系统通知（可关声音；可发测试通知）

数据文件默认在 XDG data home（Windows 一般为 `%LOCALAPPDATA%/LRSS/data/lrss.db`）。详见 [`docs/database.md`](docs/database.md)、[`docs/embedding.md`](docs/embedding.md)。

## Prerequisites

- Go **1.25+**
- Node.js **20+**
- Wails CLI v3：`go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

## Quick start

```bash
cd frontend && npm install && cd ..
wails3 generate bindings   # Go 服务变更后重新生成
wails3 task dev
```

生产构建：

```bash
wails3 task build   # → bin/
```

仅前端：

```bash
cd frontend && npm run dev    # Vite http://127.0.0.1:9245
# wails3 task dev 会等待 Vite 就绪，并设置 FRONTEND_DEVSERVER_URL（避免 Windows localhost→::1）
cd frontend && npm run build
```

后端测试：

```bash
go test ./internal/...
```

## 架构概览

```text
main.go
├── FeedService / ArticleService   # 订阅与文章（Wails）
├── SearchService                  # FTS / vector / hybrid
├── SettingsService                # embedding、搜索能力、重建向量
└── SQLite
    ├── folders / feeds / articles
    ├── articles_fts
    ├── article_embeddings         # 可选
    └── settings / jobs

internal/
├── appsvc/     # Wails 暴露层
├── service/    # 业务编排（拉取、入库）
├── repo/       # 持久化 + FTS / embed 钩子
├── rss/        # HTTP 拉取 + gofeed 解析
├── search/     # FTS + 向量检索
├── embed/      # OpenAI 兼容 / noop / fake
├── job/        # embedding worker
└── db/         # 打开库、迁移、vector 扩展探测
```

前端入口在 `frontend/src/`：`useRssStore` 优先走 Go bindings，失败时回退 mock。

## 路线图

| 阶段 | 内容 | 状态 | 文档 |
| --- | --- | --- | --- |
| S1 | Schema / 迁移 / FTS / embeddings 表 | 完成 | `docs/database.md` |
| S2 | Embedding 配置、搜索降级、向量写入 | 完成 | `docs/plan-s2.md` |
| S3 | RSS 拉取入库、列表/阅读接库、重建向量 | 完成 | `docs/plan-s3.md` |
| S4 | OPML、文件夹 CRUD、自动刷新 | 完成 | `docs/plan-s4.md` |
| S5 | i18n（zh-CN/en-US）、HTTP enetx/surf | 完成 | `docs/plan-s5.md` |
| S6 | 快捷键、保留天数清理、UI 设置持久化 | 完成 | `docs/plan-s6.md` |

## Common commands

| Command | Description |
| --- | --- |
| `wails3 task dev` | 原生窗口 + 热重载 |
| `wails3 task build` | 生产二进制 |
| `wails3 generate bindings` | Go → JS bindings |
| `go test ./internal/...` | 后端测试 |
