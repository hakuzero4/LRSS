# LRSS 数据库设计

本地 SQLite 库，路径由 `internal/db.DefaultPath()` 决定（XDG data home，Windows 通常为 `%LOCALAPPDATA%/LRSS/data/lrss.db`）。

## 表关系

```text
folders 1──* feeds 1──* articles 1──0..1 article_embeddings
                              │
                              └── articles_fts (FTS5，应用层同步)
settings (KV JSON)
jobs (后台任务)
schema_migrations
```

## 核心表

| 表 | 用途 |
| --- | --- |
| `folders` | 侧栏文件夹（可嵌套 parent_id） |
| `feeds` | 订阅源；`feed_url` 唯一 |
| `articles` | 文章；`(feed_id, guid)` 去重 |
| `article_embeddings` | FLOAT32 向量 BLOB；`status`=pending/ready/error/skipped |
| `articles_fts` | FTS5 全文（title/summary/content_text） |
| `settings` | 应用设置 JSON |
| `jobs` | embed / fetch / opml_import 等任务 |

## 搜索策略

1. **始终**：FTS5 全文可用（不依赖网络 / 模型）。
2. **可选**：用户在设置中配置 OpenAI 兼容 embedding 后：
   - 启用 sqlite-vector（`vector_init` / `vector_quantize`）
   - 文章入库异步写 `article_embeddings`
   - 语义检索 `vector_quantize_scan`
3. **未配置模型**：不写向量、不跑 embed job；搜索仅 FTS。

## 迁移

- SQL 文件：`internal/db/migrations/NNN_name.sql`
- 启动时 `db.Open` → `Migrate`
- 版本记录在 `schema_migrations`

## sqlite-vector（S2）

- 表结构已预留 `article_embeddings.embedding BLOB`（FLOAT32 little-endian）。
- 预编译库：`third_party/sqlite-vector/windows_amd64/vector.dll`。
- **当前驱动** `modernc.org/sqlite` **不加载** 动态扩展（避免卡死连接）；检索走 **进程内余弦**。
- 全文：`articles_fts`（含 `article_id UNINDEXED`，migration 002）。
- 详见 `docs/embedding.md`。

## PRAGMA

- `foreign_keys=ON`
- `journal_mode=WAL`
- `synchronous=NORMAL`
- 连接池：桌面端 `MaxOpenConns=1`（避免 SQLite 写锁争用）
