# LRSS — Agent instructions

Wails v3 + Vue 3 + shadcn-vue local RSS reader. Module path: `lrss`.

## Stack

- **Go**: services under `internal/`; Wails facades in `internal/appsvc/`; entry `main.go`
- **HTTP**: outbound via **`github.com/enetx/surf`** through `internal/httpx.Std` (returns `*http.Client` via `.Std()`). Prefer inject `HTTP` for tests (httptest). Do not add bare `http.Client{Timeout:…}` for production paths.
- **SQLite**: `modernc.org/sqlite`, migrations in `internal/db/migrations/`
- **Frontend**: `frontend/` (Vue 3, TS, Tailwind v4, **vue-i18n**, bindings in `frontend/bindings/`)
- **i18n**: `frontend/src/i18n/` — locales `zh-CN` / `en-US`; use `t('key')` / `useLocale()`; persist `localStorage` key `lrss.locale`
- **Plans**: local only under `docs/dev/plans/` (gitignored — not product docs). Public docs live in `docs/*.md` + root `README.md`.

## Multi-agent stage development (required)

When the user starts a roadmap stage, says **开干**, **multi-agent**, or runs **`/goal`** on a stage:

1. **Plan first**  
   Create or refresh `docs/dev/plans/plan-sN.md` (gitignored) with: goals, non-goals, package layout, Wails API, acceptance checklist, **agent split table**, and frozen interfaces between agents.

2. **Split by ownership, not by “layers of the same file”**  
   Prefer 3–4 agents with **non-overlapping paths** (e.g. `internal/opml/**` vs `internal/repo/**` vs `appsvc`+`main` vs `frontend/**`).  
   Shared integration files (`main.go`, bindings, one store composable) stay with **one** owner or the **main session**.

3. **Phases**  
   - Phase 1: parallel leaf packages + pure libs + tests  
   - Phase 2: orchestration / Wails API + UI (UI may code to the planned signatures before bindings exist)  
   - Phase 3 (main session): merge conflicts, `wails3 generate bindings` if needed, `go test ./...`, smoke UI, tick acceptance boxes in the plan

4. **How to run agents**  
   - Prefer **parallel subagents** or a **project workflow** (`.grok/workflows/`) matching the plan’s agent table.  
   - Prefer **`/goal`** when the user wants multi-round autonomous completion with verification.  
   - Each agent prompt must be **self-contained**: package paths, interface contracts, test commands, “do not edit outside ownership”.

5. **Quality bar before stage complete**  
   - `go test ./...` green  
   - No drive-by refactors outside the stage  
   - Update the local plan checkboxes and the root README feature/docs sections if user-facing behavior changed

6. **Do not**  
   - Start coding a new stage without a plan file  
   - Let two agents edit the same file  
   - Claim stage done without tests (or explicit waived items in the plan)

## Code conventions

- Go: small packages, interfaces at use site (`service/ports.go`), table-driven tests  
- SQLite: `MaxOpenConns=1` — **fully consume** `Rows` before further `Exec` on the same connection  
- RSS HTML: sanitize with bluemonday before store/display  
- Frontend: call `frontend/bindings` via `loadAppsvc()`; keep mock fallback when backend missing  
- Dev on Windows: Vite on `127.0.0.1` (see `build/scripts/`); prefer `wails3 task dev`

## Commands

```bash
go test ./...
cd frontend && npm run build
wails3 generate bindings
wails3 task dev
```
