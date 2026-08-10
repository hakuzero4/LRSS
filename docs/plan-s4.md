# S4 详细计划：OPML 导入/导出 + 文件夹管理 + 自动刷新

## 目标

1. **OPML 2.0** 导入/导出订阅树（文件夹 + RSS/Atom 源）。
2. **文件夹 CRUD** 与订阅归入文件夹（OPML 层级与侧栏管理所需）。
3. **自动刷新**（S3 遗留）：按设置间隔后台 `RefreshAll`。
4. **前端**：设置「订阅」页导入/导出；侧栏可新建文件夹；设置「刷新」写入后端。

## 相对 S3 的缺口

| 缺口 | 说明 | S4 |
| --- | --- | --- |
| OPML 解析 | outline 嵌套 → 文件夹树；`type=rss` / 有 `xmlUrl` 为源 | **必须** |
| OPML 导出 | 当前 folders + feeds → 合法 OPML 2.0 | **必须** |
| 导入幂等 | 已存在 `feed_url` **跳过**（merge），计 skipped | **必须** |
| 导入后拉取 | 默认对 **新增** 源串行/限流 Refresh（可配置 skip） | **必须** |
| 文件夹 Create/Rename/Delete | repo 已有 Create/Delete；补 Rename + 服务/API | **必须** |
| 源移动 / 暂停 | `folder_id` 变更、`is_paused` 暴露 | **必须** |
| 自动刷新 ticker | `autoRefresh` + `refreshIntervalMinutes` 持久化 + 后台循环 | **必须** |
| 云同步 / WebDAV | SyncPanel 设计稿 | **不做** |
| 全文抓取 | fetchFullContent | **不做** |
| 键盘快捷键 j/k | S5+ | **不做** |

## 包结构

```text
internal/
  opml/
    types.go      # Outline, Document
    parse.go      # Parse([]byte) → Document
    export.go     # Export(Document) → []byte
    parse_test.go
    export_test.go
  service/
    library.go    # + folder CRUD, MoveFeed, SetPaused, Import/Export OPML
    opml.go       # ImportOPML / ExportOPML 编排（可与 library 同包）
    refresh.go    # AutoRefresh loop（可选独立文件）
    ports.go      # 扩展 FolderStore / FeedStore
  repo/
    folder.go     # + Rename, Get, UpdateSort
    feed.go       # + SetFolder, (已有 SetPaused)
  appsvc/
    library.go    # Wails 方法
  settings/
    library.go    # AutoRefresh / Interval 配置（新）
```

## OPML 约定

### 导入

- 支持 OPML **1.0 / 2.0**（`version` 宽松）。
- 仅处理 `body` 下 `outline`：
  - 有 **`xmlUrl`**（或 `xmlurl`）→ 订阅源；`text`/`title` 作标题，`htmlUrl` 作 site。
  - 无 `xmlUrl` 且有子 outline → **文件夹**（`text`/`title` 为名）。
  - 无 `xmlUrl` 且无子节点 → 忽略或空文件夹（忽略即可）。
- 嵌套文件夹：最多 **合理深度**（实现上递归；建议 cap 16 防畸形）。
- **URL 规范化**：trim；拒绝非 `http`/`https`。
- **冲突**：`feed_url` 已存在 → skip（不改 folder、不改 title）。
- 新建 feed **不立即**写文章；导入结束后对 `added` 列表调用 `RefreshFeed`（失败记入 result，不中断整批）。
- 可选参数 `fetch bool`（默认 true）。

### 导出

```xml
<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head>
    <title>LRSS Subscriptions</title>
    <dateCreated>RFC1123Z</dateCreated>
  </head>
  <body>
    <!-- 文件夹：无 xmlUrl，子级为源或子文件夹 -->
    <outline text="News" title="News">
      <outline type="rss" text="Example" title="Example"
               xmlUrl="https://example.com/feed.xml"
               htmlUrl="https://example.com"/>
    </outline>
    <!-- 未分组源在 body 顶层 -->
    <outline type="rss" text="Bare" xmlUrl="https://..."/>
  </body>
</opml>
```

- `parent_id` 嵌套：S4 **扁平一级文件夹** 足够（DB 支持 parent，导出时若有 parent 则递归；导入写 parent）。
- 暂停源仍导出（不丢订阅）。

### ImportResult / Export

```go
type OPMLImportResult struct {
    FoldersCreated int      `json:"foldersCreated"`
    FeedsAdded     int      `json:"feedsAdded"`
    FeedsSkipped   int      `json:"feedsSkipped"`
    FeedsFailed    int      `json:"feedsFailed"`   // refresh 失败
    Errors         []string `json:"errors"`        // 截断最多 20 条
}
```

导出返回 **OPML XML 字符串**（UTF-8）。

## Wails API（S4）

### FeedService 扩展

| 方法 | 说明 |
| --- | --- |
| `CreateFolder(name, parentId string) (Folder, error)` | parentId 空 = 根 |
| `RenameFolder(id, name string) error` | |
| `DeleteFolder(id string) error` | feeds.folder_id SET NULL（已有 FK） |
| `MoveFeed(feedId, folderId string) error` | folderId 空 = 未分组 |
| `SetFeedPaused(id string, paused bool) error` | |
| `ImportOPML(xml string, fetch bool) (OPMLImportResult, error)` | |
| `ExportOPML() (string, error)` | 返回 XML 文本 |

### SettingsService 扩展

| 方法 | 说明 |
| --- | --- |
| `GetLibraryConfig() (LibraryConfig, error)` | autoRefresh, refreshIntervalMinutes |
| `SetLibraryConfig(cfg LibraryConfig) error` | 校验 interval ∈ [5, 180] |

```go
type LibraryConfig struct {
    AutoRefresh            bool `json:"autoRefresh"`
    RefreshIntervalMinutes int  `json:"refreshIntervalMinutes"`
}
// defaults: true, 30
```

### 自动刷新

- `main` 启动 goroutine：读 `LibraryConfig`；`autoRefresh` 时每 N 分钟 `library.RefreshAll`。
- 变更配置后：`SetLibraryConfig` 触发 **reload channel** 或下次 tick 读库即可（简单：每次 sleep 后重新 Load）。
- 与手动 RefreshAll **可重叠**：用 `sync.Mutex` 或 `singleflight` 保证同时只跑一轮。
- 日志：`log.Printf("auto-refresh: ok=%d err=%d added=%d", ...)`。

## 前端

### 设置 · 订阅（`FeedsPanel`）

- 「导入 OPML」：`<input type="file" accept=".opml,.xml">` → 读文本 → `ImportOPML(xml, true)` → toast 结果 → `reloadLibrary`。
- 「导出 OPML」：`ExportOPML()` → 浏览器 download `lrss-subscriptions.opml`。
- 保留现有默认文件夹 / 保留天数等（本地 UI 即可；保留天数清理可 S5）。

### 设置 · 通用刷新（`GeneralPanel`）

- `autoRefresh` / `refreshIntervalMinutes`：**bootstrap 时** `GetLibraryConfig`；变更时 `SetLibraryConfig`（debounce 可无，blur/change 写即可）。

### 侧栏 / store

- `createFolder(name)`、`deleteFolder`（可选：右键/后续；S4 至少设置页或侧栏 + 按钮「新建文件夹」）。
- 导入导出后 `reloadLibrary()`。
- mock fallback 保持：无 backend 时导出 mock 结构为 OPML 可省略。

### bindings

- 实现后主会话执行：`wails3 generate bindings`（或 dev 时自动）。

## 验收清单

- [x] 解析常见 OPML（Feedly / Inoreader / NetNewsWire 导出样例级结构）单元测试（`internal/opml`）
- [x] 导出再导入：源 URL 集合一致（第二次全 skip）（`TestLibrary_ImportOPML_NoFetch`）
- [x] 重复 `xmlUrl` 不创建第二份 feed
- [x] 文件夹创建/重命名/删除；删除后源变未分组
- [x] `MoveFeed` 改 folder 后 ListFeeds 可见
- [x] Import 支持 `fetch=true` 刷新新源（集成路径存在；无网 CI 用 fetch=false）
- [x] `Get/SetLibraryConfig` 持久化；`main.runAutoRefresh` + `TryRefreshAll`
- [x] UI：FeedsPanel 导入/导出、GeneralPanel 写 LibraryConfig、侧栏新建文件夹
- [x] `go test ./...` 通过

## 并行分工（多 Agent）

| Agent | 包/文件所有权 | 任务 |
| --- | --- | --- |
| **A** | `internal/opml/**` | Parse/Export + 表驱动测试（含嵌套、缺字段、非 http） |
| **B** | `internal/repo/folder.go`, `feed.go`（仅新增方法）, `service/ports.go`, `service` 内 folder/move/pause 方法 + 测试 | CRUD 文件夹、MoveFeed、SetPaused |
| **C** | `internal/service/opml.go` 或 library 扩展, `settings` LibraryConfig, `appsvc`, `main.go` auto-refresh | Import/Export 编排 + API + ticker |
| **D** | `frontend/src/**`（FeedsPanel, GeneralPanel, useRssStore, 可选 sidebar） | UI 接线；**不改** Go |

### 依赖顺序

```text
Phase 1 (并行): A + B
Phase 2 (并行): C（依赖 A+B 接口） + D（可先按计划中的 API 签名写，bindings 后对齐）
Phase 3 (主会话): 集成、bindings、go test、勾验收
```

### 接口冻结（Agent 之间契约）

```go
// opml
func Parse(data []byte) (*Document, error)
func Export(doc *Document) ([]byte, error)

// service.Library
func (lib *Library) CreateFolder(ctx, name string, parentID *string) (model.Folder, error)
func (lib *Library) RenameFolder(ctx, id, name string) error
func (lib *Library) DeleteFolder(ctx, id string) error
func (lib *Library) MoveFeed(ctx, feedID string, folderID *string) error
func (lib *Library) SetPaused(ctx, feedID string, paused bool) error
func (lib *Library) ImportOPML(ctx, xml string, fetch bool) (OPMLImportResult, error)
func (lib *Library) ExportOPML(ctx) (string, error)
```

## 风险

| 风险 | 缓解 |
| --- | --- |
| 巨型 OPML 一次 refresh 过久 | Import 限制并发 1；result 报告失败；UI toast 提示耗时 |
| 畸形 XML | `encoding/xml` + 明确 error；不 panic |
| MaxOpenConns=1 + 长事务 | Import 逐条 Insert，不包大事务；Refresh 沿用现逻辑 |
| 自动刷新与手动重叠 | mutex `refreshMu` |
| Windows 文件选择 | 标准 file input + FileReader 即可（webview） |

## 非目标（明确不做）

- 替换整个设置 store 为全量 AppSettings 同步
- 文章已读状态 OPML 扩展
- Google Takeout 专用格式
- 云同步

## 完成后状态

```text
✅ OPML 导入/导出（internal/opml + service + FeedService + UI）
✅ 文件夹与源归属管理 API + 侧栏新建 / 设置 OPML
✅ 自动后台刷新（LibraryConfig + runAutoRefresh）
⏭ S5：快捷键、保留天数清理、设置全量持久化、搜索体验
```

## 实现记录（多 Agent）

| Agent | 结果 |
| --- | --- |
| A | `internal/opml` Parse/Export + tests |
| B | repo/service 文件夹 CRUD、MoveFeed、SetPaused |
| C | Import/Export 编排、LibraryConfig、appsvc、main ticker |
| D | FeedsPanel / GeneralPanel / useRssStore / AppSidebar |
| 主会话 | `go test ./...` green；bindings 已含新 API |
