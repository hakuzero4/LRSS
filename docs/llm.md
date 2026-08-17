# LLM 配置与后续能力规划

## 当前能力

### 配置

**设置 → 搜索/AI → 大语言模型** 可接入 OpenAI 兼容的 **Chat Completions** API：

| 配置项 | 说明 |
| --- | --- |
| `llm.provider` | `disabled` \| `openai_compatible` |
| `llm.base_url` | 如 `https://api.openai.com/v1`、`http://127.0.0.1:11434/v1` |
| `llm.api_key` | 可选（本地 Ollama 可空） |
| `llm.model` | 如 `gpt-4o-mini`、`deepseek-chat`、`llama3.2` |
| `llm.temperature` | 0–2，默认 0.3 |
| `llm.max_tokens` | 完成上限，默认 2048；0 表示交给服务商 |
| `llm.system_prompt` | 可选默认 system（后续功能可覆盖） |

### API（Wails）

- `SettingsService.GetLLMConfig` / `SetLLMConfig`（返回时 key 脱敏）
- `SettingsService.TestLLMConfig` — 发极短 completion，校验连通与鉴权
- **`AIService`**（用户显式触发）  
  - `Summarize` / `Translate` / `Ask`  
  - `ChatSend` / `ChatHistory` / `ChatClear` / `ChatCancel`（可附加文章、搜订阅库）  
  - `SuggestFolders` / `ApplySuggestedFolder`  
  - `ClassifyPromo` / `DetectContentFullness` / `EnsureFullContent`  
  - `TranslateSelection`  
  - `IsLLMConfigured`

Web 访问走同一套 `AIService`：`GET/POST /api/ai/chat`、`POST /api/ai/chat/clear|cancel`，流式用 `GET /api/ai/stream`（SSE，`llm:stream`）。

### 已上线 AI 功能（设置 → 模型 / AI 功能）

| 功能 | 入口 | 说明 |
| --- | --- | --- |
| **摘要** | 阅读器工具栏「生成摘要」/ ✨ | 流式写入正文上方 deck |
| **翻译** | 工具栏语言图标 | 原文+译文对照，原文不覆盖 |
| **划词翻译** | 选中正文 | 短文本固定 prompt |
| **阅读助手** | 侧栏「阅读助手」（桌面 + Web 访问） | 全局多轮对话；+ 添加文章并可过滤/搜索订阅库；划词可「问这个」。Web 走 `/api/ai/chat` + `/api/ai/stream` |
| **标签 / 文件夹建议** | 助手建议条 | 本地关键词 + LLM；可一键移文件夹 |
| **广告 / 软文判断** | 助手建议条 | organic / promo / unclear |
| **自动请求全文** | 设置 → AI 功能（可选） | 打开文章时判断 partial 并抓取 |

**共性**

- 模型配置在 **设置 → 模型**；功能开关在 **设置 → AI 功能**
- **自动摘要**（可选）：打开文章时调用摘要，输出语言 = **当前界面语言**
- 摘要 / 问答 / 建议 / 分类 的 prompt 与回复语言跟随 UI 语言（`zh-CN` → 简体中文）
- 摘要等一次性结果仍可缓存；阅读助手对话存在 `ai_chat_*`，不走功能缓存
- 缓存：`llm_feature_cache`（article + feature + model + content 指纹 + locale）
- 正文预算：`BuildArticleBundle` + `BudgetText`
- 出站：`internal/httpx`（surf）→ OpenAI 兼容 `/chat/completions`

### 实现

- 出站 HTTP：`internal/httpx`（enetx/surf 指纹客户端）
- 协议：`POST {baseUrl}/chat/completions`（非流式）
- 与 **Embedding 配置分离**：向量模型与对话模型通常不同，可分别指向不同网关

---

## 后续工作（P2+）

### 已完成的 P0 / P1

见上表。

### P0 — 阅读增强（历史规划，已实现）

| 功能 | 说明 | 依赖 |
| --- | --- | --- |
| **摘要** | 一键生成文章摘要（短 / 要点列表） | LLM 配置 + 正文截断策略 |
| **翻译** | 译为中文/英文等目标语言 | 同上 |
| **解释 / 问答** | 「这篇文章在说什么？」「风险点？」 | 可选带对话历史 |

**实现要点**

- 统一 `internal/llm` 客户端；UI 侧 loading / 取消 / 错误 toast
- 输入预算：`title + summary + plainText(body)` 按 token 粗切（字符/4 启发式即可）
- 结果可缓存：`article_id + feature + model + content_hash` → SQLite，避免重复计费
- 输出优先 **Markdown**，复用现有 Markdown 面板或阅读器内卡片

### P1 — 库级智能（历史规划，已实现）

| 功能 | 说明 |
| --- | --- |
| **智能标签 / 文件夹建议** | 根据正文建议标签（先本地规则，再 LLM 兜底） |
| **过滤增强** | 用 LLM 判断是否广告/软文（与关键词过滤互补，需明确开销与离线策略） |

### P2 — 深度能力

| 功能 | 说明 |
| --- | --- |
| **多轮文章问答** | 已上线：`ChatSend` + 右侧助手栏（第一期） |
| **Ask-your-library** | RAG：向量/FTS Top-K → 出处可点（第二期） |
| **流式输出** | 已有 `llm:stream`（摘要 / 翻译 / 助手） |
| **工具调用** | 加星、标已读、移文件夹等，须确认（第三期） |
| **多模态** | 配图描述（不做） |

### 架构建议

```
Settings(LLMConfig)
       │
       ▼
 internal/llm.Client  ──httpx/surf──►  OpenAI-compatible API
       │
       ├── feature: summarize / translate / ask / select_translate
       ├── feature: content_fullness / suggest / classify
       └── (later) rag.Query(library, q)
```

- **不要**把 API Key 打进日志或诊断导出（已有 Masked）
- 功能开关：可在 `UIPrefs` 或 `llm.features` JSON 里逐步加，默认全关直到配置通过「测试连接」
- 与全文抓取（`FetchFullContent`）协同：摘要/翻译前可先拉全文，提高质量

### 非目标（短期）

- 内置闭源 SDK 绑定（保持 OpenAI 兼容一层即可）
- 云端账号体系 / 订阅计费
- 默认强制联网的后台静默 LLM 调用（须用户显式触发）

---

## 与向量搜索的关系

| 层 | 配置 | 用途 |
| --- | --- | --- |
| Embedding | 设置 → 向量模型 | 语义检索、未来 RAG 取文档 |
| Chat LLM | 设置 → 大语言模型 | 生成、翻译、摘要、问答 |

两者可共用同一网关不同 path（`/embeddings` vs `/chat/completions`），也可完全分离。
