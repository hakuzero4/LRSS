# LRSS

<p align="center">
  <img src="docs/brand/lrss-source-22.png" alt="LRSS icon" width="96" height="96" />
</p>

<p align="center">
  <strong>Local-first desktop RSS reader</strong><br />
  Subscribe, sync, search, optional AI, YouTube-friendly reading, and optional browser access on your LAN.
</p>

<p align="center">
  <a href="README.zh-CN.md">中文说明</a>
</p>

---

## Screenshot

Three-pane library · article list · reader (summary deck, selection translate, folders & smart lists)

![LRSS main UI — three-pane library, article list, and reader with AI summary and selection translate](docs/screenshots/main-reader.png)

Folder **image-card** layout — covers in a grid, switch from the list header or the folder context menu

![Folder image-card layout — article covers in a grid next to the reader](docs/screenshots/folder-cards.png)

---

## Features

### Library & navigation

| Capability | Details |
| --- | --- |
| **Three-pane layout** | Sidebar (smart lists / folders / feeds) · article list · reader |
| **Smart lists** | Unread · Today · Starred · All — with live counts |
| **Folders** | Nested-style organization; create / rename / delete; mark all read; refresh folder; move feeds; [list or image-card display](#folder-display) per folder |
| **Feeds** | Favicons, unread badges, context menus, per-feed editor |
| **Office / NSFW mode** | Mark feeds or folders sensitive; hide them from sidebar & smart lists when office mode is on |
| **Zen mode** | Hide sidebar + list; focus on the article (`z`) |
| **Resizable panes** | Layout sizes persist across sessions |

### Folder display

Some folders are all image feeds. Set **Display → Image cards** on the folder (right-click), or use the grid / list icon in the article-list header (next to refresh).

- **List** — existing title + teaser rows
- **Image cards** — cover grid (feed image, or the first `<img>` in the body). Click a card to open the reader
- Saved on the folder; feeds inside inherit it. Smart lists stay as a list

![Image-card folder view](docs/screenshots/folder-cards.png)

### Subscriptions

| Capability | Details |
| --- | --- |
| **Add feeds** | Single or multi-line RSS/Atom URLs; optional mark all as NSFW on import |
| **Refresh** | Single feed, folder, or library-wide; respects ETag / Last-Modified |
| **Auto-refresh** | Configurable global interval (minutes → hours); per-feed interval or pause |
| **OPML** | Import / export (OPML 2.0); folders preserved; re-import merges |
| **Per-feed options** | Display title (user-locked), folder, refresh interval, keep-days, pause, NSFW |
| **Failed feeds** | Filter errors, one-click remove invalid subscriptions |
| **Retention** | Global and per-feed keep-days for non-starred articles; manual purge; **starred never purged** |

### Reading

| Capability | Details |
| --- | --- |
| **Read / star** | Toggle read state and favorites; mark all read on lists/folders |
| **Mark-on-open / mark-on-scroll** | Configurable read behavior |
| **Unread-only list filter** | Hide read items (starred collection exempt) |
| **Open original** | System browser (desktop) or new tab (web access) |
| **In-body links** | Honor “open links in browser” preference |
| **HTML safety** | Article HTML sanitized with bluemonday before store/display |
| **Fetch full content** | Toolbar, paced background queue, or optional on-open auto-fetch — [how it works](#fetch-full-content) |
| **Typography** | Font size (sm/md/lg), system font picker, reader width (narrow → fill) |
| **Reader toolbar** | Configurable: zen, star, read, summarize, translate, AI menu, fetch full, Markdown panel, open original |
| **Markdown panel** | Side panel for Markdown conversion of the article |

### Fetch full content

Many feeds only ship a teaser. LRSS can download the original page and replace the stored body.

**When it runs**

| Trigger | Where | What happens |
| --- | --- | --- |
| Manual | Reader toolbar | Fetch this article’s URL now |
| New articles | **Settings → Feeds → Fetch full content** | After a refresh, new items that look truncated are queued; a paced drain (separate from feed refresh) fetches them |
| On open | **Settings → AI features → Auto fetch full** | Opening an article runs a conservative fullness check; only then fetch. Off by default |

**Pipeline** (`internal/fulltext`)

1. **URL policy** — `http`/`https` only. Block loopback, RFC1918, IPv6 ULA, CGNAT (`100.64/10`), link-local, `.localhost` / `.local`, and cloud metadata hosts. Every redirect is re-checked (SSRF-style egress).
2. **Download** — outbound [`enetx/surf`](https://github.com/enetx/surf) via `internal/httpx.Std` (fingerprint-friendly TLS/HTTP; not a bare `http.Client`). Timeout 45s, body cap 8 MiB.
3. **Extract** — [Mozilla Readability](https://github.com/mozilla/readability) algorithm via [`codeberg.org/readeck/go-readability/v2`](https://codeberg.org/readeck/go-readability). Main article HTML + plain text; chrome/nav/ads stripped.
4. **Sanitize** — [bluemonday](https://github.com/microcosm-cc/bluemonday) UGC policy before SQLite / UI (same as feed HTML).
5. **Store** — overwrite `content_html` / `content_text`, set `fullContentFetched`, refresh FTS.

**Truncation heuristic** (queue + auto-fetch) is local and conservative: empty body, “read more” style cues, or body ≈ feed summary. Ambiguous short articles are left alone; use the toolbar. YouTube watch URLs skip Readability — captions use InnerTube → kkdai → optional `yt-dlp`, plus the in-reader embed.

### Search & filters

| Capability | Details |
| --- | --- |
| **FTS5 full-text** | Always available offline on title / summary / body text |
| **Semantic search (optional)** | OpenAI-compatible embeddings; vector / hybrid modes when configured |
| **Search modes** | auto · fts · vector · hybrid |
| **List search** | Filter current collection; backend FTS when connected |
| **Duplicate titles** | Optional hide across list |
| **Block keywords** | Comma-separated title/summary mute list |

### AI (optional, OpenAI-compatible)

Requires **Settings → Models** chat endpoint. Feature toggles under **AI features**.

| Feature | Entry | Notes |
| --- | --- | --- |
| **Summarize** | Toolbar / ✨ | Streaming summary deck above the body; optional auto-summarize on open |
| **Translate** | Language icon | Original + translation side-by-side; original body never deleted |
| **Selection translate** | Select text in body | Popup short-text translation |
| **Ask / explain** | ✨ menu | Q&A on the current article |
| **Folder / tag suggest** | ✨ menu | Local keywords + LLM; one-click move feed to folder |
| **Promo / soft-ad classify** | ✨ menu | Manual organic / promo / unclear |
| **Auto full-fetch** | AI features toggle | Opens article → conservative partial check → fetch page |
| **Cache** | Local SQLite | `article + feature + model + content fingerprint + locale` |
| **Locale-aware prompts** | UI language | zh-CN UI → Chinese prompts/replies |

Embedding (vector search) and chat (LLM) are **configured separately** (different models/gateways allowed).

### YouTube

| Capability | Details |
| --- | --- |
| **Channel RSS** | Embed player for video items |
| **Captions** | Fetch when available; **timeline / timed cues** in the reader |
| **Fetch backends** | Watch-session InnerTube (ANDROID/WEB/iOS) → kkdai → optional **yt-dlp** |
| **Fail-safe** | Keep plain caption text if timed upgrade fails |
| **Cookies (optional)** | `YOUTUBE_TRANSCRIPT_COOKIES_FROM_BROWSER` for restricted videos |

### Web access (optional)

Enable in **Settings → Advanced → Web access**.

| Capability | Details |
| --- | --- |
| **Same SPA** | Browser opens the same reading UI as desktop |
| **Bind** | `localhost` (127.0.0.1) or **LAN** (0.0.0.0) |
| **Port / token** | Default port `18765`; Bearer or `?token=`; LAN empty token auto-generated |
| **Allowed** | Browse, star, mark read, mark-all-read, search, reader tools (if enabled on desktop) |
| **Blocked** | Settings UI, feed/folder CRUD, refresh, OPML, sync management |
| **Invalid token** | Full-page block only — no empty library shell |
| **Locale** | Follows desktop UIPrefs language |
| **Dev builds** | `wails3 task dev` does not embed the SPA. Web access proxies Vite (`FRONTEND_DEVSERVER_URL`) or serves `frontend/dist` (`npm run build`). Release binaries already embed the UI. |

### Desktop & preferences

| Area | Details |
| --- | --- |
| **Themes** | System / light / dark; accent presets + custom hex |
| **Windows 11 Mica** | Optional full-window Mica (Settings → Appearance). 22H2+; needs hardware acceleration. Web access stays opaque. |
| **Compact sidebar** | Denser feed list |
| **i18n** | Simplified Chinese · English |
| **Keyboard shortcuts** | j/k next-prev · s star · m read · r refresh · z zen · `,` settings (toggleable) |
| **Notifications** | New-article system notifications; sound on/off; test notification |
| **Sync (OPML only)** | WebDAV or S3-compatible (R2 / MinIO); push/pull subscription structure — **not** read/star/body |
| **System tray** | Always-on tray: open window · refresh feeds · toggle web access · quit |
| **Close to tray** | Window close hides the app; quit from the tray menu |
| **Advanced** | Launch at login, hardware acceleration, clear AI cache, developer diagnostics export, reset UI prefs |
| **Startup** | Default collection; hide-read-on-startup |

### Privacy & data

- **Local-first**: library in SQLite on disk (no cloud account required)
- **API keys** stored only on-device (masked in UI)
- **Outbound HTTP** via fingerprint-friendly client (`enetx/surf` / `internal/httpx`)
- Data path: XDG data home; on Windows typically `%LOCALAPPDATA%/LRSS/data/lrss.db`

---

## Stack

| Layer | Technology |
| --- | --- |
| Shell | [Wails v3](https://v3.wails.io) |
| Frontend | Vue 3 · TypeScript · Vite · Tailwind CSS v4 · [shadcn-vue](https://www.shadcn-vue.com) |
| Data | SQLite ([`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite)) · FTS5 · optional vectors |
| HTTP (outbound) | [`github.com/enetx/surf`](https://github.com/enetx/surf) via `internal/httpx` |
| Full-page extract | [`go-readability/v2`](https://codeberg.org/readeck/go-readability) (Mozilla Readability) · [bluemonday](https://github.com/microcosm-cc/bluemonday) |
| i18n | vue-i18n (`zh-CN` / `en-US`) |

Go module path: `lrss`.

---

## Requirements

- **Go** 1.25+ (dev often on 1.26)
- **Node.js** 20+
- **Wails CLI v3**

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

Optional for YouTube captions fallback: [`yt-dlp`](https://github.com/yt-dlp/yt-dlp) on `PATH`.

---

## Downloads / Releases

Prebuilt apps are published on GitHub Releases when a version tag is pushed:

| Asset | Platform |
| --- | --- |
| `lrss-windows-amd64.exe` | Windows x64 |
| `lrss-windows-arm64.exe` | Windows ARM64 |
| `lrss-linux-amd64` | Linux x64 |
| `LRSS-macOS-arm64.app.zip` | **macOS Apple Silicon** (M1/M2/M3…) |
| `LRSS-macOS-amd64.app.zip` | **macOS Intel** |

**Windows / Linux:** release binaries are **UPX**-packed to keep download size down (~½). Builds are **not code-signed**; Windows SmartScreen may block the first run — click **More info** → **Run anyway**.

**macOS:** real `.app` bundles (ad-hoc signed on GitHub runners; no universal fat binary; no UPX). First open: right-click → **Open** if Gatekeeper blocks.

```bash
# After pushing to GitHub, cut a release:
git tag v0.1.0
git push origin v0.1.0
```

Workflow: [`.github/workflows/release.yml`](.github/workflows/release.yml) (also **Actions → Release → Run workflow**).

Repository: [github.com/hakuzero4/LRSS](https://github.com/hakuzero4/LRSS)

---

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
wails3 task build   # → bin/
```

Frontend only:

```bash
cd frontend && npm run dev    # http://127.0.0.1:9245
cd frontend && npm run build
```

Tests:

```bash
go test ./...
```

### Commands

| Command | Description |
| --- | --- |
| `wails3 task dev` | Desktop app + hot reload |
| `wails3 task build` | Production binary |
| `wails3 generate bindings` | Regenerate frontend bindings |
| `go test ./...` | Backend tests |

---

## Configuration map

| Area | Settings path | Storage |
| --- | --- | --- |
| Refresh, read behavior, NSFW | General | SQLite UI prefs |
| Theme, accent, locale, toolbar buttons | Appearance | SQLite UI prefs |
| Font, width, unread-only, open-in-browser | Reading | SQLite UI prefs |
| OPML, feed list, retention | Feeds | SQLite + files |
| Duplicates, block keywords | Filters | SQLite UI prefs |
| Embedding + LLM endpoints | Models | SQLite settings |
| Auto-summarize, auto full-fetch, … | AI features | SQLite UI prefs |
| WebDAV / S3 OPML sync | Sync | SQLite settings |
| Shortcuts on/off | Shortcuts | SQLite UI prefs |
| New-article notifications | Notifications | SQLite UI prefs |
| Web access, cache, diagnostics | Advanced | SQLite + runtime |

Details: [docs/embedding.md](docs/embedding.md) · [docs/llm.md](docs/llm.md) · [docs/database.md](docs/database.md)

### Web access quick setup

1. **Settings → Advanced → Allow Web access**
2. Choose **localhost** or **LAN**, port, token
3. Copy local / LAN URL (includes `?token=` when set)
4. Open in a browser on the same machine or LAN

---

## Architecture

```text
main.go
├── appsvc/          Wails facades (Feed / Article / Search / Settings / AI / Web)
├── service/         Library orchestration (fetch, OPML, …)
├── repo/            SQLite + FTS / embed hooks
├── rss/             Fetch + parse (gofeed)
├── search/          FTS + optional vector
├── embed/ · llm/    OpenAI-compatible providers
├── fulltext/        Page fetch (surf) + Readability extract + host policy
├── web/             Optional browser HTTP API + SPA
├── ytcaptions/      YouTube caption backends
├── cloudsync/       OPML push/pull (WebDAV / S3)
└── db/              Open, migrate, vector probe

frontend/
├── src/             Vue app (store, reader, settings, web gate)
└── bindings/        Generated from Go (via loadAppsvc())
```

---

## Documentation

| Doc | Content |
| --- | --- |
| [README.zh-CN.md](README.zh-CN.md) | Chinese README |
| [docs/README.md](docs/README.md) | Documentation index |
| [docs/database.md](docs/database.md) | Schema, FTS, migrations, data path |
| [docs/embedding.md](docs/embedding.md) | Vector search & sqlite-vector notes |
| [docs/llm.md](docs/llm.md) | LLM config and AI feature matrix |
| [docs/screenshots/](docs/screenshots/) | UI screenshots |
| [docs/brand/](docs/brand/) | App icon source asset |
| [AGENTS.md](AGENTS.md) | Contributor / multi-agent conventions |

---

## Contributing

1. Prefer small, focused changes with `go test ./...` green.
2. After changing Wails-facing Go APIs, run `wails3 generate bindings`.
3. Use `internal/httpx` for outbound HTTP (no ad-hoc production `http.Client`).
4. Sanitize article HTML before store/display.
5. Larger roadmap work: follow [AGENTS.md](AGENTS.md) (local plans under `docs/dev/plans/`, gitignored).

---

## License

License not specified yet. All rights reserved until a license file is added.

---

Built with **Wails**, **Vue**, and **SQLite** — offline-friendly reading first.
