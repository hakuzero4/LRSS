# S6 详细计划：快捷键 + 文章保留清理 + UI 设置持久化

> **说明**：S5（i18n + enetx/surf）已完成，见 `docs/plan-s5.md`。  
> 本阶段为路线图下一迭代（README 原「S6」）。若口语中称「S5 产品能力」，以本文为准。

## 目标

1. **键盘快捷键**（可开关）：j/k 移动、s 收藏、m 已读、r 刷新、o/Enter 打开原文、`/` 聚焦搜索、`,` 或 `Ctrl+,` 设置。
2. **文章保留清理**：按 `keepArticlesDays` 删除过期本地文章（**保留收藏**；未读也可按天数清理以免无限膨胀，默认：**非收藏且 fetched_at/published_at 超过 N 天**）。
3. **UI 设置全量持久化**：主题、字号、阅读宽度、已读行为、快捷键开关、保留天数等写入 SQLite `settings`，启动加载；与已有 `LibraryConfig` / embedding 配置并存。

## 非目标

- 自定义快捷键重绑定
- 开机启动（launchAtLogin 系统 API）
- 云同步 / 通知真正推送
- fetchFullContent 全文抓取
- 搜索框深度 AI 体验

## 包结构

```text
internal/
  settings/
    ui.go              # UIPrefs Get/Set JSON key app.ui_prefs
    retention.go       # RetentionConfig or part of UIPrefs
  repo/
    article.go         # PurgeOlderThan(ctx, days, keepStarred bool) (int, error)
  service/
    library.go         # PurgeOldArticles
  appsvc/
    settings.go        # GetUIPrefs / SetUIPrefs / PurgeOldArticles (or via FeedService)
  job/ 或 main.go      # 启动后延迟一次 purge + 每日 ticker（可选简化：启动 + Set 后触发）

frontend/src/
  composables/
    useKeyboardShortcuts.ts
    useRssStore.ts       # load/save UIPrefs, wire purge result
  layouts/AppLayout.vue  # mount shortcuts
  components/settings/panels/*  # persist on change
```

## Wails API

### SettingsService

```go
type UIPrefs struct {
    MarkAsReadOnOpen       bool   `json:"markAsReadOnOpen"`
    MarkAsReadOnScrollEnd  bool   `json:"markAsReadOnScrollEnd"`
    OpenOnStartup          string `json:"openOnStartup"` // unread|today|starred|all
    HideReadOnStartup      bool   `json:"hideReadOnStartup"`
    Theme                  string `json:"theme"` // system|light|dark
    Accent                 string `json:"accent"`
    CompactSidebar         bool   `json:"compactSidebar"`
    FontSize               string `json:"fontSize"` // sm|md|lg
    ShowUnreadOnly         bool   `json:"showUnreadOnly"`
    OpenLinksInBrowser     bool   `json:"openLinksInBrowser"`
    ReaderWidth            string `json:"readerWidth"`
    DefaultFolderId        string `json:"defaultFolderId"` // empty = null
    FetchFullContent       bool   `json:"fetchFullContent"`
    KeepArticlesDays       int    `json:"keepArticlesDays"` // 7–365, default 90
    HideDuplicateTitles    bool   `json:"hideDuplicateTitles"`
    BlockKeywords          string `json:"blockKeywords"`
    EnableKeyboardShortcuts bool  `json:"enableKeyboardShortcuts"`
    NotifyOnNewArticles    bool   `json:"notifyOnNewArticles"`
    NotifySound            bool   `json:"notifySound"`
    HardwareAcceleration   bool   `json:"hardwareAcceleration"`
    ClearCacheOnQuit       bool   `json:"clearCacheOnQuit"`
    DeveloperMode          bool   `json:"developerMode"`
    // autoRefresh / interval 仍走 LibraryConfig，加载时合并进前端 settings
}

GetUIPrefs() (UIPrefs, error)
SetUIPrefs(UIPrefs) error
PurgeOldArticles() (purged int, error) // 用当前 keepArticlesDays
```

### 清理规则

```sql
-- 伪 SQL
DELETE articles WHERE is_starred = 0
  AND date(coalesce(published_at, fetched_at)) < date('now', '-' || ? || ' days')
-- 同步 FTS / embeddings（embeddings CASCADE；FTS 需应用层或 bulk delete fts）
```

- `keepArticlesDays` clamp [7, 365]
- 收藏永不删
- 启动约 20s 后跑一次；`SetUIPrefs` 变更天数时可后台再跑

## 快捷键

| 键 | 动作 |
| --- | --- |
| `j` | 下一篇 |
| `k` | 上一篇 |
| `s` | 切换收藏 |
| `m` | 切换已读 |
| `r` | 刷新当前库 |
| `o` / `Enter` | 打开原文 |
| `/` | 聚焦搜索（preventDefault） |
| `Escape` | 关闭设置/添加订阅；清除搜索 |
| `Ctrl+,` / `Meta+,` | 打开设置 |
| `n` | 打开添加订阅（可选） |

- `enableKeyboardShortcuts=false` 时不注册
- 输入框 / contenteditable / 对话框内除 Esc 外忽略

## 前端

1. bootstrap：`GetUIPrefs` + `GetLibraryConfig` → 合并 `settings`
2. 各 panel 变更：`debounced SetUIPrefs` 或 `persistUIPrefs()`
3. `useKeyboardShortcuts` 在 `AppLayout` onMounted
4. FeedsPanel 保留天数 → 持久化；可选显示「立即清理」按钮

## 并行分工

| Agent | 所有权 | 任务 |
| --- | --- | --- |
| **A** | `settings/ui.go`, `repo` purge, `service` purge, `appsvc`, `main` 定时 | UIPrefs + 清理 |
| **B** | `useKeyboardShortcuts.ts`, AppLayout, ShortcutsPanel 校验 | 快捷键 |
| **C** | `useRssStore` 加载/保存 prefs、panels 持久化、i18n 键 | 前后端接线 |

依赖：B/C 可并行；C 依赖 A 的 API 签名（可按本计划先写）。

## 验收

- [x] UIPrefs Get/Set + 前端 bootstrap 加载 / debounced 保存
- [x] j/k/s/m/r/o/Enter//Esc/Ctrl+,/n 快捷键（可关闭）
- [x] PurgeOlderThan：非收藏过期删除 + FTS；收藏保留；启动延迟 purge + 立即清理按钮
- [x] `go test ./internal/...` + `vue-tsc` + bindings 通过

## 风险

| 风险 | 缓解 |
| --- | --- |
| 大库 purge 慢 | 单次 SQL + 限量 batch；启动延迟 |
| 快捷键与 WebView 冲突 | 仅在非输入焦点；可关闭 |
| UIPrefs JSON 膨胀 | 单 key `app.ui_prefs` |

## 实现记录

| Agent | 结果 |
| --- | --- |
| A | UIPrefs、PurgeOlderThan、appsvc、main 启动 purge |
| B | useKeyboardShortcuts + AppLayout |
| C | store 持久化、各设置 panel、立即清理 |
| 主会话 | 测试 / bindings / 验收勾选 |

## 完成后

```text
✅ 快捷键可用且可关
✅ 文章按天数清理（保留收藏）
✅ UI 设置写 SQLite 并重启恢复
⏭ 自定义快捷键 / launchAtLogin / 通知实装
```
