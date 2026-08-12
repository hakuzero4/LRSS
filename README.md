# LRSS

**Local-first desktop RSS reader** — subscribe, sync, full-text search, optional AI (summarize / translate / embeddings), and optional LAN web access for the same reading UI.

本地优先的桌面 RSS 阅读器：订阅拉取入库、全文检索、可选大模型能力，以及可选的浏览器 Web 访问。

---

## Features

- **Three-pane library** — smart lists / folders / feeds · article list · reader
- **Smart lists** — Unread · Today · Starred · All
- **Subscriptions** — RSS/Atom URL, single-feed or full refresh; ETag / Last-Modified
- **OPML** — import / export; folders in the sidebar
- **Reading** — read / star / open original; HTML sanitized (bluemonday)
- **Search** — FTS5 always on; optional embedding for vector / hybrid search
- **AI (optional)** — OpenAI-compatible Chat Completions: summarize, translate, ask, full-content helpers
- **YouTube** — channel RSS embeds; caption fetch with timeline display
- **Web access (optional)** — local HTTP server; browser can star / mark read; no settings or feed management
- **i18n** — 简体中文 / English
- **Desktop extras** — keyboard shortcuts, retention policy, system notifications, themes

## Stack

| Layer | Technology |
| --- | --- |
| Shell | [Wails v3](https://v3.wails.io) |
| Frontend | Vue 3 · TypeScript · Vite · Tailwind CSS v4 · [shadcn-vue](https://www.shadcn-vue.com) |
| Data | SQLite ([`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite)) · FTS5 · optional vectors |
| HTTP (outbound) | [`github.com/enetx/surf`](https://github.com/enetx/surf) via `internal/httpx` |
| i18n | vue-i18n (`zh-CN` / `en-US`) |

Go module path: `lrss`.

## Requirements

- **Go** 1.25+ (CI/dev currently on 1.26)
- **Node.js** 20+
- **Wails CLI v3**

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

## Quick start

```bash
# Frontend deps
cd frontend && npm install && cd ..

# Generate Go → TS bindings after appsvc changes
wails3 generate bindings

# Dev: native window + Vite HMR
wails3 task dev
```

Production build:

```bash
wails3 task build   # output under bin/
```

Frontend only (no Wails shell):

```bash
cd frontend && npm run dev    # http://127.0.0.1:9245
cd frontend && npm run build
```

Tests:

```bash
go test ./...
cd frontend && npm run build   # typecheck + production bundle
```

### Common commands

| Command | Description |
| --- | --- |
| `wails3 task dev` | Desktop app + hot reload |
| `wails3 task build` | Production binary |
| `wails3 generate bindings` | Regenerate frontend bindings |
| `go test ./...` | Backend tests |

## Data location

SQLite DB lives under the platform data directory (XDG on Unix; on Windows typically `%LOCALAPPDATA%/LRSS/data/lrss.db`).

See [docs/database.md](docs/database.md).

## Configuration (high level)

| Area | Where | Notes |
| --- | --- | --- |
| Theme / locale / reading | Settings → Appearance / Reading | Stored in SQLite UI prefs |
| Search & embeddings | Settings → Search / AI | OpenAI-compatible embedding endpoint |
| LLM features | Settings → Search / AI · AI features | Summarize, translate, etc. |
| Web access | Settings → Advanced | Bind host, port, token; copy local / LAN URL |
| Retention | Settings · per-feed options | Drop old non-starred articles |

Details: [docs/embedding.md](docs/embedding.md), [docs/llm.md](docs/llm.md).

## Web access

When enabled on the desktop app:

1. Open **Settings → Advanced → Allow Web access**
2. Choose **localhost** or **LAN**, port, and token
3. Open the copied URL in a browser (include `?token=…` when a token is set)

**Allowed:** browse library, star, mark read, reader toolbar tools that are enabled on desktop.  
**Not allowed:** settings UI, add/remove feeds, OPML, sync management.

Invalid or missing token shows a dedicated full-page message (no reader shell).

## Architecture

```text
main.go
├── appsvc/          Wails facades (Feed / Article / Search / Settings / AI / Web)
├── service/         Library orchestration (fetch, OPML, …)
├── repo/            SQLite + FTS / embed hooks
├── rss/             Fetch + parse (gofeed)
├── search/          FTS + optional vector
├── embed/ · llm/    OpenAI-compatible providers
├── web/             Optional browser HTTP API + SPA
├── ytcaptions/      YouTube caption backends
└── db/              Open, migrate, vector probe

frontend/
├── src/             Vue app (store, reader, settings)
└── bindings/        Generated from Go (prefer loadAppsvc())
```

More: [docs/README.md](docs/README.md).

## Documentation

| Doc | Content |
| --- | --- |
| [docs/README.md](docs/README.md) | Documentation index |
| [docs/database.md](docs/database.md) | Schema, FTS, paths |
| [docs/embedding.md](docs/embedding.md) | Vector search & sqlite-vector notes |
| [docs/llm.md](docs/llm.md) | LLM config and AI features |
| [AGENTS.md](AGENTS.md) | Contributor / multi-agent conventions |

## Contributing

1. Prefer small, focused PRs with tests (`go test ./...`).
2. After changing Wails-facing Go APIs, run `wails3 generate bindings`.
3. Use `internal/httpx` for outbound HTTP (not ad-hoc `http.Client` in production paths).
4. Sanitize article HTML before store/display.
5. For larger roadmap work, follow the multi-agent process in [AGENTS.md](AGENTS.md).

## License

License not specified yet. All rights reserved until a license file is added.

---

Built with Wails, Vue, and SQLite — offline-friendly reading first.
