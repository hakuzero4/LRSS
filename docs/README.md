# LRSS documentation

Technical docs for contributors and advanced users. Day-to-day usage lives in the app UI and the root READMEs: [English](../README.md) · [中文](../README.zh-CN.md).

## Guides

| Document | Description |
| --- | --- |
| [database.md](database.md) | SQLite layout, FTS5, migrations, data path |
| [embedding.md](embedding.md) | Embedding config, search modes, sqlite-vector notes |
| [llm.md](llm.md) | OpenAI-compatible LLM settings and AI features |

## Brand & screenshots

| Path | Description |
| --- | --- |
| [brand/lrss-icon.png](brand/lrss-icon.png) | App icon source (transparent PNG) |
| [screenshots/main-reader.png](screenshots/main-reader.png) | Main three-pane UI (used in root README) |
| [screenshots/folder-cards.png](screenshots/folder-cards.png) | Folder image-card layout (used in root README) |

Platform icons are generated into `build/` (`appicon.png`, Windows `.ico`, etc.) and `frontend/public/`.

## Agent / stage plans

Multi-agent stage plans are **not** published product docs. When working a roadmap stage locally, write a plan under `docs/dev/plans/` (gitignored). See [AGENTS.md](../AGENTS.md).
