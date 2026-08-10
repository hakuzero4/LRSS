# S2 详细计划：sqlite-vector + Embedding 抽象 + 搜索降级

> **前置**：S1 已完成（schema / migrate / `db.Open` / FTS5 表 / `article_embeddings` 表）。  
> **本阶段目标**：在「用户可配置向量模型」的前提下，打通 **扩展加载 → 向量写入 → 近邻查询**；**未配置时产品完全依赖 FTS5**，不崩溃、不强制联网。  
> **本阶段不做**：RSS 拉取、OPML、替换前端 mock 列表（属 S3–S5）。  
> **本阶段可做**：设置 KV 读写 API、设置页「AI / 搜索」配置 UI（为 S6 铺路）。

---

## 1. 成功标准（Definition of Done）

| # | 标准 | 验收方式 |
| --- | --- | --- |
| D1 | 扩展可加载时：`SELECT vector_version()` 有返回 | 集成测试 / 启动日志 |
| D2 | 扩展不可用时：应用仍启动，`VectorAvailable=false` | 测试 mock 路径 |
| D3 | Embedding **未配置**：`SearchService.Search` 仅走 FTS5 | 单元 + 集成测试 |
| D4 | Embedding **已配置**：能 `Embed` → 写入 BLOB → `vector_init`/`quantize` → `vector_quantize_scan` 返回邻居 | 集成测试（可用假 HTTP server） |
| D5 | 模型/维度变更：旧向量标记失效并触发 rebuild 任务（至少入队） | 单元测试 |
| D6 | 设置项可读写（DB `settings` 表），前端「搜索/AI」面板可展示配置状态 | 手工 + API 测试 |
| D7 | 文档：`docs/embedding.md` + 更新 `docs/database.md` | 文档 review |

**非目标（明确砍掉）**

- 真·RSS 入库链路（S3）
- OPML（S4）
- 完整混合排序 UI（S6 可深化；S2 提供 Search API 即可）
- 本地 onnx 模型（接口预留，实现放后）

---

## 2. 关键决策与硬约束

### 2.1 sqlite-vector API 约束（来自官方 API.md）

1. **`vector_init(table, column, options)` 每个连接都要调**（需要做向量操作的连接）。
2. **必须先 `vector_quantize` 再 `vector_quantize_scan`**。
3. 目标表必须有 **rowid**（普通表即可；`TEXT PRIMARY KEY` 仍有隐式 rowid，**不要** `WITHOUT ROWID`）。
4. 扫描返回的是 **`article_embeddings.rowid`**，不是 `article_id`：

```sql
SELECT a.id, a.title, v.distance
FROM vector_quantize_scan('article_embeddings', 'embedding', ?, 20) AS v
JOIN article_embeddings AS e ON e.rowid = v.rowid
JOIN articles AS a ON a.id = e.article_id
WHERE e.status = 'ready';
```

5. 写入推荐：`vector_as_f32(json_or_blob)`；Go 侧也可用 **little-endian FLOAT32 BLOB** 直接写入（与 dimension 一致）。
6. 距离：语义搜索默认 **`distance=COSINE`**。
7. Quantize：默认 **`qtype=TURBO,qbits=4`**（速度/召回平衡）；小数据量可用 `vector_full_scan` 做测试对照。

### 2.2 驱动选型风险（最高优先级 spike）

| 方案 | 优点 | 风险 |
| --- | --- | --- |
| A. 继续 `modernc.org/sqlite` + 动态扩展 | 无 CGO | **扩展加载支持弱/不可用** 概率高 |
| B. `mattn/go-sqlite3` + CGO + `LoadExtension` | 扩展加载成熟 | 需 CGO 工具链（Windows 需 gcc） |
| C. 进程内 CGO 静态链 vector | 发布稳 | 打包复杂 |

**S2 执行顺序：**

```text
S2.0 Spike（0.5–1 天）
  → 在 Windows 上验证：能否从 Go 加载 vector.dll 并 vector_version()
  → 结论写入 docs/embedding.md
  → 失败则采用 B（mattn），并保留 modernc 作为「仅 FTS」fallback 编译标签
```

**建议默认结论预案：**

- `//go:build cgo` 路径：vector 完整能力  
- `//go:build !cgo` 或扩展缺失：`VectorAvailable=false`，仅 FTS  

### 2.3 许可

sqlite-vector 对 **OSI 开源项目免费**；闭源商用需 Elastic/商业授权。仓库若开源，在 README 注明依赖与许可。

---

## 3. 配置模型（设置）

存在 `settings` 表，key 使用点分命名，value 为 JSON 字符串。

### 3.1 Keys

| Key | 类型 | 说明 | 默认 |
| --- | --- | --- | --- |
| `embedding.provider` | string | `disabled` \| `openai_compatible` | `disabled` |
| `embedding.base_url` | string | 如 `https://api.openai.com/v1` 或 Ollama | `""` |
| `embedding.api_key` | string | 可空（本地服务） | `""` |
| `embedding.model` | string | 如 `text-embedding-3-small` | `""` |
| `embedding.dimensions` | int | 必须与模型一致 | `0` |
| `embedding.batch_size` | int | 批量 embed | `16` |
| `search.mode` | string | `auto` \| `fts` \| `vector` \| `hybrid` | `auto` |
| `search.vector_top_k` | int | 向量召回条数 | `30` |
| `search.fts_limit` | int | FTS 条数 | `50` |

### 3.2 「已配置且可用」判定

```text
Configured :=
  provider == "openai_compatible"
  && model != ""
  && dimensions > 0
  && base_url != ""          // 或允许相对默认
  // api_key 可选

VectorSearchEnabled := Configured && VectorExtensionLoaded && vector_init 成功
```

- `search.mode = auto`：若 `VectorSearchEnabled` → hybrid，否则 fts  
- `search.mode = vector` 但未启用 → **降级 FTS** + 返回 `warnings: ["vector_unavailable"]`  
- 前端展示：`semanticEnabled: boolean` + 说明文案

### 3.3 密钥

- S2：明文存本地 DB（与桌面本地优先一致）  
- 后续可改为系统钥匙串；接口层 `SettingsService` 对前端可 mask（`sk-***`）

---

## 4. 包结构（S2 新增）

```text
internal/
  db/
    open.go              # 扩展加载钩子
    vector_ext.go        # LoadVectorExtension, VectorVersion
    vector_ops.go        # Init / Quantize / Preload / Cleanup
  settings/
    store.go             # Get/Set/GetJSON
    embedding.go         # EmbeddingConfig + IsConfigured()
  embed/
    provider.go          # interface Provider
    noop.go              # NoopProvider
    openai_compat.go     # OpenAI-compatible HTTP
    fake.go              # 测试用确定性向量
    text.go              # 文章 → embed 输入文本 + 截断 + content_hash
  vector/
    blob.go              # []float32 ↔ little-endian BLOB
    index.go             # EnsureIndex(dim) / Rebuild / Scan
  search/
    fts.go               # FTS5 MATCH
    vector.go            # quantize_scan / full_scan
    hybrid.go            # RRF 融合（可选 S2 最小实现）
    service.go           # Search(ctx, query, opts) 
  job/
    embed_queue.go       # 入队 pending、worker 骨架（可单测不跑真 worker）
  service/               # 或 app 层
    settings_service.go  # Wails 暴露
    search_service.go    # Wails 暴露
```

前端（S2 可选但推荐）：

```text
frontend/src/components/settings/panels/SearchAIPanel.vue  # 或并入 Advanced
types: EmbeddingConfig
useRssStore / 新 useSettingsStore 读 Go 配置
```

---

## 5. 分任务拆解（按顺序）

### S2.0 — Spike：扩展加载（阻塞项）

**任务**

1. 从 [sqlite-vector Releases](https://github.com/sqliteai/sqlite-vector/releases) 下载 Windows `vector.dll`（及后续 mac/linux）。  
2. 放置策略：
   - 开发：`third_party/sqlite-vector/{windows_amd64,darwin_arm64,...}/`  
   - 运行：优先 `可执行文件旁/lib/vector.*`，其次 embed 解压到 user data。  
3. 验证脚本 / 测试：`TestVectorExtension_Load`（扩展不存在则 `t.Skip`）。  
4. 记录：modernc vs mattn 哪个能 load。

**产出**

- `internal/db/vector_ext.go`  
- `docs/embedding.md` 中「平台与构建」一节  
- 若必须 CGO：`go.mod` 条件依赖说明、`README` 构建要求  

**预估**：0.5–1 天  

---

### S2.1 — Embedding 配置读写

**任务**

1. `settings.Store`：`Get(ctx, key) (string, error)` / `Set` / `GetAll` / `GetJSON`。  
2. `EmbeddingConfig` 结构体 + `LoadEmbeddingConfig` / `SaveEmbeddingConfig`。  
3. `IsConfigured()` / `Validate()`（dimensions 范围 32–4096、URL 可解析）。  
4. Wails `SettingsService`：
   - `GetEmbeddingConfig() EmbeddingConfigDTO`
   - `SetEmbeddingConfig(cfg) error`（校验）
   - `GetSearchCapabilities() { fts: true, vector: bool, reason?: string }`

**测试**

- 空库默认 disabled  
- 写入后重开 DB 仍可读  

**预估**：0.5 天  

---

### S2.2 — Embed Provider

**接口**

```go
type Provider interface {
    Name() string
    Dimensions() int
    Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type Factory func(cfg EmbeddingConfig) (Provider, error)
```

**实现**

| 实现 | 用途 |
| --- | --- |
| `NoopProvider` | 未配置；`Embed` 返回明确错误 `ErrEmbeddingDisabled` |
| `OpenAICompatProvider` | `POST {base}/embeddings`，body: `{model, input}`；兼容 OpenAI / 多数代理 / Ollama openai 兼容层 |
| `FakeProvider` | 测试：对 hash(text) 生成固定 dimensions 的单位向量 |

**细节**

- HTTP：timeout 30s、可取消 context、错误体解析  
- 批量：按 `batch_size` 切分  
- 维度校验：返回向量长度必须 == `cfg.Dimensions`  
- 文本：`embed.BuildInput(title, summary, contentText)` 截断（默认 6000 runes）  
- `content_hash`：`sha256` of normalized input  

**测试**

- `httptest.Server` mock embeddings 响应  
- 维度不一致报错  
- Noop 不发起网络  

**预估**：1 天  

---

### S2.3 — 向量 BLOB 与 Index 生命周期

**BLOB 编解码**

```go
func Float32ToBlob(v []float32) []byte  // little-endian
func BlobToFloat32(b []byte, dim int) ([]float32, error)
```

**Index 状态（内存 + 可选 settings 缓存）**

```go
type IndexState struct {
    Initialized bool
    Dimensions  int
    Model       string
    Quantized   bool
    Preloaded   bool
}
```

**EnsureIndex(ctx, dim)**

```text
if !extensionLoaded → return ErrVectorUnavailable
if 从未 init 或 dim 变化:
    vector_init('article_embeddings','embedding',
      fmt.Sprintf('type=FLOAT32,dimension=%d,distance=COSINE', dim))
if 存在 status=ready 且 embedding NOT NULL:
    vector_quantize(..., 'qtype=TURBO,qbits=4')
    vector_quantize_preload(...)  // 可选，失败不致命
```

**RebuildIndex**

```text
当 model 或 dimensions 变更:
  1. UPDATE article_embeddings SET status='pending', embedding=NULL, ...
  2. vector_quantize_cleanup（若 API 可用）
  3. 重新 EnsureIndex
  4. 入队全量 embed job（S2 可只写 jobs 行，worker 最小实现）
```

**写入一条 ready 向量**

```sql
INSERT INTO article_embeddings(...)
  ON CONFLICT(article_id) DO UPDATE SET
    embedding=excluded.embedding,
    model=..., dimensions=..., content_hash=..., status='ready', error=NULL, updated_at=...
-- 小批量后 throttle quantize（见 5.4）
```

**注意**：每条 insert 后都 quantize 太贵 → **策略**：

- 批量 embed 结束时 quantize 一次  
- 或 pending quantize 标记，每 N 条 / 每 30s 一次  
- 单测可用 full_scan 避免 quantize 依赖  

**预估**：1 天  

---

### S2.4 — Embed Job（最小可用）

S2 不要求完善调度器，但要有清晰骨架：

```text
jobs.kind = 'embed'
payload = {"article_ids":[...]} | {"mode":"all_pending"}
```

**Worker 循环（可同步函数 + 可选后台 goroutine）**

1. 若 `!Configured` → 全部 pending 标 `skipped` 或不入队  
2. 取 `status IN ('pending','error')` 且文章仍存在的 rows（LIMIT batch）  
3. 组装文本 → Provider.Embed  
4. 写 BLOB + status=ready / error  
5. 批次结束 `vector_quantize`  

**API**

- `EmbedService.EnqueueArticle(id)`  
- `EmbedService.EnqueuePending(limit)`  
- `EmbedService.RunOnce(ctx)` — 处理一批，便于测试与手动触发  

**预估**：0.5–1 天  

---

### S2.5 — Search 服务（核心业务 API）

```go
type SearchOptions struct {
    Mode   string // auto|fts|vector|hybrid
    Limit  int
    // 后续: CollectionId, UnreadOnly — S3 再加
}

type SearchHit struct {
    ArticleID string
    Score     float64   // 归一化或 RRF 分
    Source    string    // fts|vector|hybrid
    Title     string    // 可只返回 id，S3 再 join 列表
    Snippet   string    // FTS snippet 可选
}

type SearchResult struct {
    Hits     []SearchHit
    ModeUsed string
    Warnings []string
}
```

**算法**

| Mode | 行为 |
| --- | --- |
| `fts` | `articles_fts MATCH` + bm25/rank，JOIN articles |
| `vector` | embed query → quantize_scan；失败则 FTS + warning |
| `hybrid` | FTS 与 vector 各取 TopK，**RRF** 融合（k=60） |
| `auto` | VectorSearchEnabled ? hybrid : fts |

**FTS 查询**

- 安全：用户输入转义，禁止任意 FTS 语法注入（简单策略：去掉 `"` `*` 特殊符或 quote 成 phrase）  
- `snippet(articles_fts, ...)` 可选  

**测试数据**

- 手工插入 3–5 篇文章 + FakeProvider 向量  
- 查询词靠近某篇标题 → 该篇 rank 第一  

**预估**：1–1.5 天  

---

### S2.6 — Wails 接线 + 前端设置面板

**Go Services 注册**

```go
SettingsService
SearchService   // Search(query string, mode string) SearchResult
// 可选 HealthService.VectorStatus()
```

**前端**

1. 设置左侧导航增加 **「搜索 / AI」**（或挂在「高级」下二级——推荐独立一项，文案清晰）。  
2. 表单字段：provider 开关、base_url、api_key、model、dimensions、保存/测试连接。  
3. 「测试连接」：`Embed(["ping"])` 或 SearchCapabilities。  
4. 状态条：
   - 灰色：未配置，全文检索  
   - 绿色：向量可用  
   - 黄色：已配置但扩展未加载  

**列表搜索框**（最小）

- 调用 `SearchService.Search`；结果若仅有 id，S2 可先 toast 命中数，**完整列表渲染可放到 S5/S6**。  
- 或：Search 直接返回足够字段画列表（推荐，少一次往返）。

**预估**：1 天  

---

### S2.7 — 文档与验收

1. `docs/embedding.md`：配置、provider、扩展路径、故障排查  
2. 更新 `docs/database.md`：rowid join、quantize 时机  
3. README：CGO / 扩展说明  
4. 检查清单跑通 D1–D7  

**预估**：0.5 天  

---

## 6. 时间线汇总

| 子任务 | 预估 | 依赖 |
| --- | --- | --- |
| S2.0 Spike 扩展 | 0.5–1d | — |
| S2.1 Settings | 0.5d | S1 |
| S2.2 Provider | 1d | S2.1 |
| S2.3 Vector index | 1d | S2.0 |
| S2.4 Embed job | 0.5–1d | S2.2 + S2.3 |
| S2.5 Search | 1–1.5d | S2.3 + FTS |
| S2.6 UI + Wails | 1d | S2.1 + S2.5 |
| S2.7 文档验收 | 0.5d | all |
| **合计** | **约 6–8 人日** | |

可并行：S2.1∥S2.0；S2.2∥S2.3（S2.0 完成后）。

---

## 7. 关键设计细节

### 7.1 启动流程（main）

```text
db.Open
  → Migrate
  → TryLoadVectorExtension (失败仅 log.Warn)
settings.LoadEmbeddingConfig
if Configured && VectorLoaded:
  vector.EnsureIndex(dimensions)
  // 可选：go embedWorker.Run(ctx)
register Services
app.Run
```

### 7.2 配置变更流程

```text
SetEmbeddingConfig(new)
  old := Load()
  Save(new)
  if !new.IsConfigured():
      // 可选：不删已有向量，搜索不再用
      return
  if old.Model != new.Model || old.Dimensions != new.Dimensions:
      RebuildIndex + EnqueuePending(all)
  else:
      EnsureIndex(new.Dimensions)
```

### 7.3 RRF（hybrid）

```text
score(d) = Σ 1 / (k + rank_i(d))   // k=60
取 fts_rank、vector_rank 合并去重，按 score 降序
```

### 7.4 错误类型

```go
var (
  ErrEmbeddingDisabled  = errors.New("embedding disabled")
  ErrVectorUnavailable  = errors.New("sqlite-vector extension not loaded")
  ErrVectorNotReady     = errors.New("vector index not initialized")
  ErrDimensionMismatch  = errors.New("embedding dimension mismatch")
)
```

Service 层转为用户可读中文 message。

### 7.5 与 S1 表的配合

- FTS：S2 提供 `fts.Upsert(article)` / `Delete` 供 S3 调用；S2 测试自己插入 FTS 行。  
- embeddings：`status=skipped` 用于「当时未配置」；开启配置后可重新 pending。  

---

## 8. 测试矩阵

| 用例 | 类型 |
| --- | --- |
| migrate 后无扩展，Search fts OK | 集成 |
| FakeProvider + full_scan 近邻 | 集成（可不依赖 quantize） |
| FakeProvider + quantize_scan（扩展在时） | 集成 / skip |
| 未配置 mode=vector → 降级 fts + warning | 单元 |
| OpenAICompat mock HTTP | 单元 |
| 维度变更清空 pending | 单元 |
| BuildInput 截断与 hash 稳定 | 单元 |
| Settings 持久化 | 集成 |

---

## 9. 风险与缓解

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| modernc 无法 load 扩展 | 无向量 | S2.0 定案；CGO 路径；无扩展时功能完整降级 |
| quantize 在空表上失败 | 启动报错 | 仅当 `COUNT(ready)>0` 时 quantize |
| 每连接需 vector_init | 漏 init 查询失败 | 封装在 `DB` 包装层 / 每次 Search 前 Ensure |
| API key 进库 | 本地泄露 | 文档声明；后续钥匙串 |
| 假阳性 FTS 语法 | 崩溃/空结果 | 查询消毒 |
| 大库 quantize 耗时 | UI 卡顿 | 后台 job，不阻塞主线程 |

---

## 10. 交付清单（PR 拆分建议）

| PR | 内容 |
| --- | --- |
| PR-S2a | Spike + vector_ext 加载 + 文档骨架 |
| PR-S2b | settings store + EmbeddingConfig + Wails SettingsService |
| PR-S2c | embed providers + blob + tests |
| PR-S2d | vector index ops + embed job skeleton |
| PR-S2e | search fts/vector/hybrid + SearchService |
| PR-S2f | 设置页 AI/搜索 UI + 绑定生成 |

---

## 11. S2 完成后的状态

```text
✅ 可配置 / 可关闭的向量能力
✅ 未配置 → 纯 FTS，体验完整
✅ 已配置 → 向量入库与语义检索 API 可用
✅ 为 S3（RSS 写库时 enqueue embed + fts）留好钩子
⏭ S3：Feed/Article 真实数据
⏭ S4：OPML
⏭ S5：UI 替换 mock
⏭ S6：搜索框深度体验 + 回填进度条
```

---

## 12. 建议的第一步动作（开工令）

1. 下载 Windows `vector.dll` 到 `third_party/sqlite-vector/windows_amd64/`。  
2. 实现 `TestVectorExtension_Load`（存在则跑，不存在则 Skip）。  
3. 根据结果选定 **modernc vs mattn**，冻结构建说明。  
4. 再并行推进 Settings + Provider。

---

*本文是 S2 的执行规格；实现时若 Spike 结论与预案冲突，以 Spike 记录更新本节 2.2 并调整 PR-S2a。*
