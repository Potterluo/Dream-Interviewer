# AGENTS.md — operating guide for AI coding agents

This file is the contract between this repository and AI coding agents
(ZCode, Claude Code, Codex, Cursor, …). Read it before writing code. It
tells you how the project is shaped, which rules are load-bearing, and
which command does a task for you so you don't have to hand-copy code.

## What this repo is

**Dream Interviewer** — a self-hosted AI interviewer. A Go backend and a
Next.js frontend compiled into ONE static binary (the frontend is an
`output: "export"` static build embedded via `go:embed`). SQLite by
default (pure-Go driver, no CGO), PostgreSQL optional. Same-origin: no
CORS, no API base URL config.

It was scaffolded from the `web-starter` template, and the template's
example domains (`items`, mock `chat`) have been **deleted** — the
interview domain is the real entity now. The generator and its
`--- gen:* ---` markers still work.

The product: the user supplies a target role, résumé and JD; an LLM plans
questions, the user answers one at a time, each answer is graded on named
dimensions with adaptive follow-ups, and the session ends in a report.

## Commands

| Task | Command |
|---|---|
| Backend build + vet | `go build ./... && go vet ./...` |
| Backend test | `go test ./...` |
| Frontend build (static export) | `cd web && pnpm build` |
| Frontend lint | `cd web && pnpm lint` |
| End-to-end smoke test (219 checks) | `pwsh -NoProfile -File scripts/smoke.ps1` |
| Full single binary | `make build` (frontend → embed → go build) |
| Lite binary (no Shiki markdown, −11MB) | `make build MARKDOWN=lite` |
| Desktop app (Wails native window) | `make desktop` |
| Dev with hot reload | `make dev-frontend` + `make dev` (browser on :8080) |
| Rebrand a copy of this template | `go run ./cmd/generator init --module <path> --name <Name>` |
| Generate an entity's full CRUD | `go run ./cmd/generator entity <name> --field title:string ...` |

**`make` does not exist on Windows.** There, the full build is:

```powershell
cd web; pnpm install; $env:MARKDOWN_FULL="1"; pnpm build; cd ..
Remove-Item -Recurse -Force internal\server\dist   # replace, never merge
Copy-Item -Recurse web\out internal\server\dist
go build -ldflags "-s -w" -o bin/app.exe ./cmd/server
```

Definition of done for any code change: `go build ./...`, `go vet ./...`,
`go test ./...`, and `cd web && pnpm build` all pass. Frontend type errors
surface in `pnpm build` (no separate tsc step). `pnpm lint` must have zero
errors — note that `eslint-config-next` 16's `react-hooks/set-state-in-effect`
is an error, so a synchronous `reload()` called from `useEffect` will fail
lint; wrap it in an async IIFE.

## Layout map

```
cmd/server/            entry point (flags: -port -data-dir -dev-proxy -version)
cmd/desktop/           Wails shell over the same server.BuildHandler
cmd/generator/         scaffolding CLI (init / entity) — templates in templates/
internal/config/       env + .env bootstrap config (APP_* variables, no credentials)
internal/llm/          OpenAI-compatible client (Chat / ChatStream / ChatJSON)
internal/interview/    the product's brain: types, prompts, rubric, offline engine
internal/store/        Store interface + sqlite/postgres drivers + migrations
internal/auth/         sessions, API keys, bcrypt, Identity + middleware
internal/events/       in-process pub/sub hub (SSE/WS fan-out)
internal/server/       HTTP layer: routes in server.go, one handler file per domain
internal/server/settings.go  model-config resolution + admin endpoints + swappable engine
internal/server/scope.go     whose rows a list/aggregate request may see
internal/server/dist/  build output for go:embed (never edit; make build-web fills it)
web/src/app/           Next.js App Router pages (all client components, static export)
web/src/components/ui/ shadcn-style primitives on @base-ui/react (NOT Radix)
web/src/lib/api.ts     central typed API client (apiFetch, envelope, SSE helpers)
web/src/lib/api/<x>.ts per-domain typed clients
docs/API.md            the frozen HTTP contract — read it before touching either side
scripts/smoke.ps1      end-to-end HTTP smoke test against a live server
```

## Load-bearing conventions (break these and the design leaks)

1. **One response envelope.** Every JSON endpoint returns
   `{"ok": true, ...}` or `{"ok": false, "error": "..."}`. Use
   `writeOK`/`writeError`/`readJSON` in `internal/server/respond.go`.
2. **Ownership: 404, not 403.** A single row belonging to another user is
   `not found` — never reveal existence. Admins bypass via
   `ident.IsAdmin()`. For LIST/AGGREGATE widening (`?all=true`,
   `?userId=<id>`) a non-admin gets an explicit **403**, not a silently
   filtered list: an escalation attempt must be visible rather than look
   like an empty result set. Both rules live in
   `internal/server/scope.go` (`resolveScope`) — put any new widening
   there, never inline in a handler.
3. **Auth wrappers pick the gate.** In server.go route registrations:
   `s.public` (no auth), `s.protected` (any identity), `s.writable`
   (rejects read-only actAs — REQUIRED for mutating routes),
   `s.adminOnly` (admin role).
4. **Store: one interface, two dialects.** Entity files define a
   `<Name>Store` sub-interface embedded into `Store` (store.go) and
   DBStore methods. Write SQL as plain `?` literals and wrap with
   `d.rebind(...)` (rewrites to `$n` for postgres). Missing rows →
   `store.ErrNotFound`; unique violations → `store.ErrDuplicate`.
5. **Migrations are layered.** New tables: add `CREATE TABLE IF NOT
   EXISTS` to `migrationSQL()` in internal/store/db.go. Changing an
   EXISTING table: add a `tableHasColumn`-guarded imperative step (see
   `migrateBackfillInterviewTurnCount` for the shape). Never destructive
   without a migration.
6. **Mutations publish events.** Handlers publish
   `<entity>.created|updated|deleted` via `s.hub.PublishTo(ownerID, ...)`;
   pages subscribe with `subscribeEvents` and refresh.
7. **Frontend API calls go through apiFetch/apiJSON** (`web/src/lib/api.ts`)
   — they carry the cookie/bearer and mirror `?actAs=`. Never call bare
   `fetch` for authenticated endpoints.
7b. **Interview state lives on the server.** Read `docs/API.md` before
    touching either side: the whole interview is advanced by ONE
    streaming endpoint (`POST /api/interviews/{id}/advance`) with actions
    `start|answer|skip|finish|abort`. Do not add ask/grade/report
    endpoints — the point of the single endpoint is that the state machine
    has exactly one definition. The terminal `done` frame carries
    authoritative state; never trust client-side accumulation over it, and
    make sure the counters in that frame were refreshed from the rows you
    just read.
7c. **Model artifacts are JSON columns; queryable fields are real
    columns.** `plan_json` / `grade_json` / `report_json` are owned by
    `internal/interview` and change with prompt iterations. Anything that
    is filtered, sorted or aggregated (`status`, `overall_score`,
    `recommendation`, `role`, `duration_sec`) must be a real column so
    analytics never parses JSON. See `internal/store/interviews.go`.
7d. **Never present fallback output as model output.** Whenever the
    engine degrades it returns a `Source` with `Engine: "offline"` and a
    `Note`; `sseWriter.noteFallback` surfaces that to the candidate in
    the same stream, and the offline feedback text says so too. Keep it
    that way — silently passing heuristics off as a model's judgement is
    the worst failure mode this product has.
7e. **A turn is "open" iff `answered_at IS NULL`.** Not "answer is
    empty" — a skipped turn keeps an empty answer but is answered. Every
    write path must set `answered_at`, and the answer write goes through
    `store.AnswerInterviewTurn` (a compare-and-set) so a duplicated submit
    cannot grade one question twice. `advanceLocks.TryLock` refuses a
    *simultaneous* duplicate with 409, and the optional `turnId` binds a
    submit to the question the client was showing so a LATE retry cannot
    land on the next question. Blocking on the lock instead of failing
    fast is a known regression path: the loser would wake to a different
    open question and write the stale answer onto it.
7f. **Do not ask the model to evaluate nothing.** `interview.TrivialAnswer`
    sends sub-4-character answers straight to the deterministic rubric,
    because a model handed a contentless answer invents a critique of the
    wrong thing. Same spirit as 7d: never manufacture evaluation.
8. **base-ui, not Radix.** UI primitives take a `render={<Tag />}` prop
   for composition — there is NO `asChild`. Dialog labels must sit
   inside a `DropdownMenuGroup`/Menu.Group when used in menus.
8b. **Markdown rendering: use `<Markdown>` from
   `web/src/components/markdown.tsx`** for any LLM/user-facing markdown
   (chat replies, rich content). It bundles the plugin stack — Shiki
   code highlighting (github-light/dark), CJK line breaking, remark
   streaming support — and chat-density prose styles. NEVER hand-roll
   Streamdown options, add react-markdown, or write regex-based
   highlighting; extend the component instead.
9. **SVG/attributes don't do CSS `var()`.** Chart colors are resolved in
   JS from the theme tokens (see web/src/app/analytics/page.tsx).
9b. **Markdown is a build option.** `MARKDOWN_FULL` (set by
    `make build MARKDOWN=full|lite`) switches the implementation via a
    next.config turbopack alias — `markdown-impl.tsx` (full, default) vs
    `markdown-impl-lite.tsx`. Both export the same `Markdown` props;
    import only from `@/components/markdown` and never reference an impl
    file directly.
9c. **Two delivery targets share one wiring.** `internal/app.Boot`
    assembles config/store/hub/server; `cmd/server` serves over TCP,
    `cmd/desktop` (build tag `desktop`) serves on an EPHEMERAL loopback
    port and points the Wails window at it. Streaming (SSE) must ride
    real TCP — WebView2 buffers custom-scheme responses, so do NOT
    "simplify" the desktop shell back to the asset-server fallback.
    Server changes must keep `server.BuildHandler` working.
9c-bis. **The desktop target must never hard-wire a `%APPDATA%` path, and
    must never fail silently.** It is built with `-H windowsgui`, so there
    is NO console and `slog`'s stdout writer goes nowhere — a boot error
    then looks exactly like "nothing happens when I double-click it".
    Two things depend on `%APPDATA%` and both must be redirected:
    (a) the data dir, resolved by `pickWritableDataDir()` in
    `cmd/desktop/bootstrap.go`, which honours `APP_DATA_DIR`, then tries
    `%APPDATA%` → `%LOCALAPPDATA%` → `<exe dir>/data`, and PROBES each with
    a real file (a directory can exist and still reject writes);
    (b) WebView2's profile, which Wails otherwise defaults to
    `%APPDATA%\<binary name>` — set `Windows.WebviewUserDataPath` to a
    subdir of the resolved data dir. If both are left at their defaults the
    app dies with an opaque SQLite `CANTOPEN`/`readonly` or a WebView2
    `800700aa` error wherever the profile is unavailable (roaming profile
    off, OneDrive redirection, EDR, an inherited sandbox). Fatal startup
    errors go through `fatalDesktop()`, which logs to
    `dream-interviewer-desktop.log` next to the exe and shows a native
    message box.
9d. **Brand icon.** The raster master is `build/appicon.png` (1024×1024);
    `web/src/app/icon.png` (512) is the favicon AND the in-app logo
    (app-shell / login-screen / onboard reference `/icon.png`), and
    `web/src/app/apple-icon.png` (180) is the iOS home-screen icon. The
    CANONICAL icon PNG set lives in `build/` — `icon-256.png`,
    `icon-48.png`, `icon-32.png`, `icon-16.png`. Both consumers read that
    one set: `build/windows/mkico.py` packs `build/windows/icon.ico`, and
    `build/winres.json` feeds `go run
    github.com/tc-hib/go-winres@v0.3.3 make --in build/winres.json --out
    cmd/desktop/rsrc`, which compiles `cmd/desktop/rsrc_windows_*.syso`.
    Both steps are `make icons`. The `.syso` step is NOT optional: without
    it the `.ico` updates but the desktop exe keeps its old icon, because
    the icon is linked in as a compiled resource. The GROUP_ICON MUST stay
    at resource ID 3 — the Wails runtime loads the window icon from
    `winc.AppIconID = 3` (hardcoded).
    To replace the artwork: drop the new square master into
    `build/appicon.png` and regenerate every size from it (alpha-weighted
    downscale, not bilinear), keeping the exact filenames above. If the new
    master has an opaque background, remove it by flood-filling from a ring
    just inside the edge — a global "white → transparent" rule punches holes
    through eye highlights and light clothing.
10. **Two theme axes.** Mode (`.dark` class: light/dark/system) and
    accent preset (`data-theme` attribute: violet/ocean/forest/sunset/
    rose/candy/mono) are independent. Presets are MORE than color: they
    may override `--radius`, `--background`, `--card`, `--sidebar` for
    personality (mono = sharp corners, candy = extra-round pink, ocean =
    cool-tinted surfaces). Adding a preset = one `[data-theme]` CSS pair
    in globals.css + one entry in `THEME_PRESETS` (theme-provider.tsx).
    Both axes are applied pre-paint by the inline head script in
    layout.tsx — keep that script in sync with the storage keys.
11. **The client guard only routes; the server is the authority.** Every
    API call re-checks auth/roles/ownership server-side. `adminOnly`
    routes ALSO call `requireAdmin` in-handler (belt and braces).
    Admin-route and scope-widening refusals return the exact literal
    `forbidden` (branch on equality, not `-match` — a substring check
    already hid one drift); a few non-authorization 403s
    (`admin role required…`, `read-only: cannot mutate…`,
    `account disabled`) carry a specific reason. `docs/API.md` §6 lists
    them.
11a. **`IsAdmin()` requires the owner's CURRENT role AND the key tier.**
    `Role` is re-read from the database on every request for sessions and
    API keys alike, so demoting an account revokes access immediately.
    Never go back to inferring admin from `APIKeyType` alone: a demoted
    admin's admin-tier key then keeps full admin, and can re-promote its
    owner. A user-tier key held by a real admin stays deliberately narrow
    — a key may narrow its owner's access, never widen it. See
    `internal/auth/auth_test.go`.
11b. **NO credential is compiled into the binary — ever.** Not a key, not
    a token, not a private gateway URL. `internal/config` has no key
    constant and `internal/config/config_test.go` fails the build if one
    is reintroduced. Model configuration resolves per field, highest
    priority first: **database** (an admin set it in `/admin/model/`) →
    **environment / `.env`** → **built-in default** (numeric knobs only).
    `.env` is gitignored and read from the CWD then next to the binary; a
    real environment variable ALWAYS beats `.env`. The API key is never
    serialised — DTOs expose `apiKeyMasked` + `apiKeySet` only. Do not
    "simplify" this into a default credential for zero-config
    convenience: an unconfigured install is a supported state that runs
    the offline engine and says so.
11c. **Admin settings use pointer fields + `clearable*` for numbers.**
    `docs/API.md` §6 promises a uniform rule: a field absent from the
    PUT body means "leave unchanged", `""` means "remove the stored
    override". That is why numeric fields use
    `clearableInt`/`clearableFloat` (they accept a number OR `""`)
    instead of `*int` — a client clearing a number reasonably sends `""`
    and `*int` would 400 on it. Validate before writing, so a rejected
    request cannot leave half the settings applied. After any settings
    write, call `reloadEngine` so the swap is immediate.
11d. **Built-in presets are code, not rows — and are read-only.**
    `interview.Presets()` is the curated set; admin presets live in the
    `interview_presets` table. `GET /api/interview/presets` concatenates
    them and marks `builtin: true|false`. Editing or deleting a built-in
    id is a **409**, never a silent no-op and never a "materialise it
    into the DB" trick.

## Prefer the generator before hand-writing

```bash
go run ./cmd/generator entity task --field title:string --field body:text --field done:bool
```

This creates the store slice + migration DDL, handlers, routes, typed
API client, CRUD page, and nav entry in one shot, inserting code at the
`--- gen:* ---` markers (store.go, db.go, server.go,
web/src/components/app-shell.tsx).

Rules around the generator:

- **Never delete or reorder the `--- gen:* ---` marker lines.** Multiple
  entities accumulate at the same markers.
- Generated files are normal code — edit them freely AFTER generation.
  The generator only ever appends at markers and writes new files; it
  never regenerates over an edited file (it fails if the entity already
  exists).
- Field types: `string` (Input), `text` (Textarea), `bool` (toggle),
  `int` (number Input). `id`, `user_id`, timestamps, ownership, and live
  events are added automatically.
- Run `go build ./... && go vet ./... && cd web && pnpm build` after
  generating and before committing.

Manual recipes (what the generator automates, for when you need to go
beyond it): `docs/agent/add-entity.md`, `docs/agent/add-page.md`.

## Reference examples to read before similar work

| Task | Read first |
|---|---|
| Interview domain/state machine | `internal/server/handlers_interviews.go` (DTOs, routes, `advance`) |
| LLM prompt design | `internal/interview/prompts.go` (the two-part `<<<JSON>>>` protocol) |
| Scoring / rubric | `internal/interview/dims.go` + `internal/interview/offline.go` |
| LLM transport | `internal/llm/llm.go` (Chat / ChatStream / ChatJSON, budget escalation) |
| CRUD + ownership + events | the interviews handlers in `internal/server/handlers_interviews.go` |
| Store entity slice | `internal/store/interviews.go` |
| File upload | `internal/server/handlers_files.go` + `web/src/app/files/page.tsx` |
| Streaming SSE UI | `web/src/app/interview/page.tsx` + `web/src/lib/api/interviews.ts` |
| Aggregation/chart endpoint | `internal/server/handlers_interviews.go` (`handleInterviewStats`) + `web/src/app/analytics/page.tsx` |
| Auth internals | `internal/auth/auth.go` (Identity, Resolver, `?token=` allow-list) |
| Runtime model config / admin API | `internal/server/settings.go` (+ `internal/config/config.go` for `.env`) |
| Role + row-scope enforcement | `internal/server/scope.go`, then the RBAC section of `scripts/smoke.ps1` |
| Admin CRUD over a table with a read-only built-in subset | `internal/server/handlers_presets.go` + `internal/store/presets.go` |

## Environment & runtime facts

- Config is env + `.env` (`APP_*`), no config file; secrets are scrubbed
  from the process env after boot. `.env` is gitignored; `.env.example`
  is the committed template. Real env vars beat `.env`.
- Default port 8080; health probes `/healthz` `/livez` `/readyz`.
- Sessions are HttpOnly cookies (`app_session`); API keys are `sk_…`
  bearer tokens stored as SHA-256 (plaintext shown once).
- `APP_DEV_PROXY=http://localhost:3000` turns the Go server into a dev
  proxy for `next dev` (HMR websocket included) — the browser talks to
  :8080 only.
- **The interviewer model** is any OpenAI-compatible endpoint. Configure
  it as an admin at `/admin/model/` (stored in the DB, wins) or via
  `APP_LLM_PROVIDER` / `APP_LLM_BASE_URL` / `APP_LLM_API_KEY` /
  `APP_LLM_MODEL` (+ timeout, max tokens, temperature) in the env/`.env`.
  **Nothing is defaulted, and no credential ships in the repo** — the
  local machine's working endpoint lives in the gitignored `.env`. See
  convention 11b.
- **Reasoning models charge their thinking against `max_tokens`.** The
  default is 8192 for that reason, and `internal/llm` escalates the
  budget once when a reply comes back empty with
  `finish_reason: "length"` and nothing has been emitted yet. Do not
  "simplify" that away, and never forward `reasoning_content` to a user.
- **No model available is a supported state**, not an error: the engine
  falls back to the bundled bank + deterministic rubric and says so.
  Boot logs the resolved model (never the key) and says explicitly when
  nothing is configured.
