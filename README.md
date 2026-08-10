# LRSS — Local RSS Reader

Desktop RSS tool scaffolded with **Wails v3 + Vue 3 + shadcn-vue**, UI inspired by Apple Mail / Notes (three-column library).

| Layer | Stack |
| --- | --- |
| Shell | [Wails v3](https://v3.wails.io) |
| Frontend | Vue 3 · TypeScript · Vue Router · Vite |
| UI | [shadcn-vue](https://www.shadcn-vue.com) · Tailwind CSS v4 |

**Current stage:** S1–S3 — SQLite + RSS 拉取入库 + 可选向量搜索（未配置则 FTS）。OPML 为 S4。

## Screens (designed)

| Surface | What it is |
| --- | --- |
| **Library** | 3-column: sidebar · article list · reader |
| **Smart lists** | Unread · Today · Starred · All Articles |
| **Folders / feeds** | Grouped navigation + unread counts |
| **Article list** | Search, refresh, mark-all-read |
| **Reader** | Typography-focused body, star / read / open original |
| **Add Feed** | Dialog for RSS/Atom URL |
| **Settings** | Modal dialog: theme, reading size, read behavior, refresh interval |

## Prerequisites

- Go **1.25+**
- Node.js **20+**
- Wails CLI v3: `go install github.com/wailsapp/wails/v3/cmd/wails3@latest`

## Quick start

```bash
cd frontend && npm install && cd ..
wails3 generate bindings   # optional until Go services expand
wails3 task dev
```

Production:

```bash
wails3 task build   # → bin/
```

Frontend only:

```bash
cd frontend && npm run dev    # Vite on http://127.0.0.1:9245
# Dev note: wails3 task dev waits for Vite before launching the exe, and forces
# FRONTEND_DEVSERVER_URL=http://127.0.0.1:<port> (avoids Windows localhost→::1 issues).
cd frontend && npm run build
```

## Frontend layout

```text
frontend/src/
├── layouts/AppLayout.vue       # Sidebar shell + add-feed dialog
├── views/
│   └── ReaderView.vue          # List + reader columns
├── components/
│   ├── layout/AppSidebar.vue
│   ├── article/                # List, item, reader
│   ├── feed/AddFeedDialog.vue
│   ├── settings/SettingsDialog.vue
│   └── ui/                     # shadcn-vue
├── composables/
│   ├── useRssStore.ts          # Mock library state
│   └── useTheme.ts
├── data/mock.ts
└── types/rss.ts
```

## Design notes (Apple-oriented)

- **Specific labels** — Unread / Today / Starred, not a vague “Home”
- **Translucent chrome** — sidebar + pane headers use `backdrop-filter`
- **Press feedback** — active scale on nav / list rows
- **Type** — system stack, tight tracking on titles, comfortable reader leading
- **A11y** — `prefers-reduced-motion`, `reduced-transparency`, `more` contrast hooks

## Roadmap (backend)

1. Replace mock store with Go services (feeds, articles, SQLite)
2. Background refresh + events into the UI
3. OPML import/export
4. Keyboard shortcuts (j/k, star, open)

## Common commands

| Command | Description |
| --- | --- |
| `wails3 task dev` | Native window + hot reload |
| `wails3 task build` | Production binary |
| `wails3 generate bindings` | Go → JS bindings |
