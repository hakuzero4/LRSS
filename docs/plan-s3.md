# S3 详细计划：RSS 拉取 + Feed/Article 持久化 + 接库

## 目标

用真实 SQLite 数据替代 mock：添加订阅 → 拉取解析 → 入库 → FTS 同步 →（可选）入队 embedding → UI 列表/阅读来自 Go。

## 相对原总计划的缺口补充（解析与数据）

| 缺口 | 说明 | S3 是否必须 |
| --- | --- | --- |
| RSS/Atom 统一解析 | `mmcdole/gofeed` 覆盖 RSS 1/2、Atom | **必须** |
| 缺 GUID | 用 `link` 或 `title+published` 稳定 hash 作 guid | **必须** |
| HTML → 纯文本 | `content_text` 给 FTS/embed；去标签、解码实体 | **必须** |
| content:encoded / media | gofeed 已部分处理；保留 `content_html` 原文 | 必须尽力 |
| ETag / If-Modified-Since | 减少带宽；304 不写库 | **必须** |
| User-Agent / Timeout | 礼貌爬取、防挂死 | **必须** |
| 相对 URL | 相对 site_url 解析绝对链接 | **必须** |
| 字符集 | 依赖 gofeed/http 自动；失败记 last_error | 必须 |
| 图片 | 从 enclosure/media/content 抽 `image_url` | 应有 |
| XSS | 阅读器 `v-html` 需消毒（bluemonday） | **必须** |
| 刷新并发 | worker pool + 每源串行；全量刷新限流 | **必须** |
| 集合查询 | unread/today/starred/all/feed/folder + 分页 | **必须** |
| 未读计数 | ListFeeds 聚合 | **必须** |
| 已读/收藏写库 | SetRead / SetStarred / MarkAllRead | **必须** |
| FTS 同步 | 插入/更新/删除文章时 UpsertFTS/DeleteFTS | **必须** |
| Embed 入队 | 仅 embedding 已配置时 MarkPending | **必须** |
| **全量重建向量** | 用户配置模型后可手动「重新生成全部向量」 | **必须（本迭代）** |
| 定时刷新 | 读 settings 间隔；后台 ticker | 应有（可简化） |
| 验证 Feed URL | Add 前 HEAD/GET 试拉 | 应有 |
| OPML | **S4** | 不做 |
| 全文抓取正文页 | P2 | 不做 |

## 包结构

```text
internal/
  id/id.go                 # ULID/UUID
  htmltext/strip.go        # HTML→text
  rss/client.go, parse.go  # fetch + map to model
  repo/folder.go, feed.go, article.go
  service/library.go       # orchestration
  appsvc/library.go        # Wails FeedService + ArticleService
```

## Wails API（S3）

**FeedService**

- `ListFolders() []Folder`
- `ListFeeds() []Feed`（含 unreadCount）
- `AddFeed(url string, folderId *string) (Feed, error)`
- `DeleteFeed(id string) error`
- `RefreshFeed(id string) (RefreshResult, error)`
- `RefreshAll() (RefreshAllResult, error)`

**ArticleService**

- `List(collection string, opts ListOpts) []Article`  
  collection: `unread|today|starred|all|feed:ID|folder:ID`
- `Get(id string) (Article, error)`
- `SetRead(id string, read bool) error`
- `SetStarred(id string, starred bool) error`
- `MarkAllRead(collection string) error`

**SettingsService（补）**

- `RebuildAllEmbeddings() (queued int, error)` — 全量 pending + 可选立即 RunOnce 循环
- 已有 `RunEmbedOnce`

## 前端

- `useRssStore` 改为调用 bindings；保留 mock fallback（bindings 失败时）
- AddFeed 调 `AddFeed` + `RefreshFeed`
- 侧栏计数来自 DB
- 搜索/AI：按钮「重新生成全部向量」

## 验收

- [x] 添加真实 RSS URL 可见文章（`FeedService.AddFeed` + Refresh）  
- [x] 刷新增量不重复（`(feed_id,guid)` UNIQUE + DO NOTHING）  
- [x] 已读/收藏写库  
- [x] FTS 同步插入  
- [x] 配置 embedding 后可 **重新生成全部向量**（`RebuildAllEmbeddings` + UI）  
- [x] 未配置 embedding 时刷新不入队 embed  
- [x] 解析缺口：GUID/相对 URL/HTML 文本/ETag/消毒（bluemonday）  
- [ ] 定时自动刷新 → **S4**（`docs/plan-s4.md`）  
- [ ] OPML → **S4**

## 并行分工

| Agent | 任务 |
| --- | --- |
| A | `id` + `htmltext` + `rss` fetch/parse + 测试 |
| B | `repo` folders/feeds/articles + FTS/embed hooks + 测试 |
| C | `appsvc` library services + RebuildAllEmbeddings + main 接线 |
| 主会话 | 前端 store 接真 API + 搜索/AI 重建按钮 + 集成验收 |
