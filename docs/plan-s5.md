# S5 计划：国际化 (i18n) + HTTP 统一 enetx/surf

## 目标

1. **前端 i18n**：`vue-i18n`，至少 **zh-CN** / **en-US**；设置可切换；默认跟随系统语言，fallback `zh-CN`。
2. **后端 HTTP**：所有出站请求经 **[enetx/surf](https://github.com/enetx/surf)**（RSS 拉取、embedding API 等）。测试可用注入的 `*http.Client`（httptest）。

## 非目标

- 完整翻译 100% 长文案（但核心 UI 必须双语）
- 服务端返回消息 i18n（Wails 错误可暂英文/中文固定）
- 代理 UI 配置（surf 支持代理，S5 不暴露设置）

## 包结构

```text
internal/
  httpx/
    client.go       # NewSurfClient / StdHTTP(timeout, ua) via surf
    client_test.go  # optional smoke (skip if no network)
  rss/client.go     # use httpx default; keep HTTP *http.Client override for tests
  embed/openai_compat.go  # use httpx

frontend/src/
  i18n/
    index.ts
    locales/zh-CN.ts
    locales/en-US.ts
  composables/useLocale.ts   # get/set locale + persist
  main.ts                    # app.use(i18n)
  components/...             # $t() / t()
```

## HTTP (surf)

### 契约

```go
// internal/httpx
func Std(opts Options) *http.Client
// Options: Timeout, UserAgent, ImpersonateChrome bool (default true for RSS)

// rss.Client.HTTP 若非 nil，优先使用（tests）
// 否则 httpx.Std(...)
```

- 使用 `surf.NewClient().Builder()...Build().Unwrap().Std()` 得到标准 `*http.Client`，便于与 gofeed / 现有 Do() 路径兼容。
- 默认超时 30s；RSS 可设 Accept + UA；embedding 设 JSON + Bearer。

### 验收

- [x] `go test ./internal/rss/... ./internal/embed/... ./internal/httpx/...` 通过
- [x] 生产路径经 `httpx.Std`（surf）；可注入 `*http.Client` 做测试
- [x] go.mod 含 `github.com/enetx/surf`（当前 pin **v1.0.200**，Go 1.26 可建；更新版本需 Go ≥1.27 toolchain）

## i18n

### 语言

| Code | 名称 |
| --- | --- |
| `zh-CN` | 简体中文（默认 fallback） |
| `en-US` | English |

### 设置

- Appearance（或 General）增加「界面语言」
- 持久化：`localStorage` key `lrss.locale`（S5）；可选后续写 settings 表
- 切换立即生效（`i18n.global.locale`）

### 必须覆盖的文案域

- 侧栏智能列表 / 品牌
- 文章列表工具栏 / 空状态 / 搜索
- 阅读器操作
- 添加订阅对话框
- 设置：导航 + 各 panel 主要 title/description
- OPML 导入进度 / 清除确认
- useRssStore 用户可见状态字符串

### 验收

- [x] vue-i18n + zh-CN / en-US locale 文件 + Appearance 语言选择
- [x] 主 UI / 设置 / OPML / store 状态文案 `$t` 化
- [x] `vue-tsc --noEmit` 通过
- [x] 语言偏好 `localStorage` `lrss.locale`

## 并行分工

| Agent | 所有权 | 任务 |
| --- | --- | --- |
| **A** | `internal/httpx/**`, `internal/rss/client.go`, `internal/embed/openai_compat.go`, go.mod | surf 封装 + 迁移 |
| **B** | `frontend/src/i18n/**`, `main.ts`, `composables/useLocale.ts`, package.json | 安装 vue-i18n、locale 文件骨架、入口 |
| **C** | `frontend/src/components/**`, `composables/useRssStore.ts`, settings panels | 文案 `$t` 化 + 语言选择 UI |

依赖：B 先于或并行 C（C 依赖 `i18n` 与 key 约定）。A 独立。

### Key 约定（示例）

```text
nav.unread, nav.today, nav.starred, nav.all
settings.nav.general, settings.nav.feeds, ...
feeds.opml.import, feeds.opml.export, feeds.danger.clearAll
common.cancel, common.confirm
```

## 风险

| 风险 | 缓解 |
| --- | --- |
| surf 要求 Go ≥1.27，本机可能 1.26 | 若无法安装：pin 兼容版本或 document 升级 Go |
| Impersonate 过重 | 默认可不 Impersonate，仅 Timeout+UA；可选 Chrome |
| 嵌套 Dialog + AlertDialog 文案遗漏 | 清单式迁移核心路径 |

## 完成后

```text
✅ 中英界面可切换（设置 → 外观 → 界面语言）
✅ 出站 HTTP 经 enetx/surf（internal/httpx.Std）
⏭ 更多语言 / settings 表持久化 locale / 代理配置 UI
```

## 实现记录

| Agent | 结果 |
| --- | --- |
| A | `internal/httpx` + rss/embed 迁移；surf v1.0.200 |
| B | vue-i18n 骨架、locale、useLocale、Appearance 语言项 |
| C | 侧栏/列表/阅读/设置/store 文案迁移 |
| 主会话 | go test + vue-tsc 验收 |
