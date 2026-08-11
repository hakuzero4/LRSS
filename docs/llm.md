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
  - `DailyDigest`  
  - `SuggestFolders` / `ApplySuggestedFolder`  
  - `ClassifyPromo`  
  - `IsLLMConfigured`

### P0/P1 功能状态（已交付）

| 功能 | 入口 | 缓存 |
| --- | --- | --- |
| 摘要 | 阅读器 → AI 菜单 | `llm_feature_cache` |
| 翻译 zh/en | 同上 | 含目标语言 |
| 问答 | 同上（prompt） | 含问题指纹 |
| 每日简报 | 侧栏智能列表下 | 按今日 Top N 集合 |
| 标签/文件夹建议 | AI 菜单；可一键 `MoveFeed` | 本地规则可在 LLM 关闭时使用 |
| 广告/软文判断 | AI 菜单 | 返回 verdict |

- 正文预算：`BuildArticleBundle` + `BudgetText`
- 缓存键：`article + feature + model + content_hash (+ extra)`
- HTTP：`internal/httpx`（surf）

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
| **每日简报** | 汇总「今日未读」Top N 为一条 digest |
| **智能标签 / 文件夹建议** | 根据正文建议标签（先本地规则，再 LLM 兜底） |
| **过滤增强** | 用 LLM 判断是否广告/软文（与关键词过滤互补，需明确开销与离线策略） |

### P2 — 深度能力

| 功能 | 说明 |
| --- | --- |
| **多轮 Ask-your-library** | RAG：向量检索 Top-K 文章 → 拼上下文 → 回答 |
| **流式输出** | SSE / chunk 推到前端（Wails 事件） |
| **多模态** | 配图描述（可选，成本高） |
| **工具调用** | 打开原文、加星、创建文件夹等 agent 动作（需严格确认） |

### 架构建议

```
Settings(LLMConfig)
       │
       ▼
 internal/llm.Client  ──httpx/surf──►  OpenAI-compatible API
       │
       ├── feature: summarize / translate / ask
       ├── feature: digest
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
