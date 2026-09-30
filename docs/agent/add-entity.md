# Recipe: add an entity (full CRUD vertical)

> The generator automates everything in this document:
>
> ```bash
> go run ./cmd/generator entity task --field title:string --field body:text --field done:bool --field estimate:int
> ```
>
> Use the manual recipe when you need something the generator doesn't
> emit (extra endpoints, custom queries, computed fields, different UI).

Goal: a new domain object with list/create/read/update/delete over HTTP,
per-user ownership, live SSE updates, and a table UI. Example entity
name: `task`, fields: title (string), done (bool).

## 1. Store slice — `internal/store/tasks.go`

Copy the shape of `internal/store/interviews.go`:

- Record struct: `ID`, `UserID`, your fields, `CreatedAt`, `UpdatedAt`.
- A `TaskStore` sub-interface with Create/Get/List/Update/Delete. Add
  `TaskStore` to the `Store` interface in `store.go` (generated code
  embeds it at the `--- gen:store-interfaces ---` marker).
- DBStore methods with plain `?` SQL wrapped in `d.rebind(...)`.
- `ListTasks(ctx, userID)`: empty `userID` means ALL rows — that path is
  admin-only and enforced in the handler, not the store.

Two conventions worth copying from `interviews.go`: keep a `ListXSummaries`
projection for list/aggregate reads so a table never drags large text
columns through memory, and do multi-table deletes inside `inTx` (there
are no foreign keys by design, so nothing cascades for you).

## 2. Schema — `internal/store/db.go`

Add to `migrationSQL()` (fresh installs):

```go
`CREATE TABLE IF NOT EXISTS tasks (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    done       INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`,
`CREATE INDEX IF NOT EXISTS idx_tasks_user ON tasks (user_id)`,
```

Column type map: string/text → `TEXT NOT NULL DEFAULT ''`, bool/int →
`INTEGER NOT NULL DEFAULT 0`, timestamps stay `TIMESTAMP`.

## 3. Handlers — `internal/server/handlers_tasks.go`

Copy the shape of `handlers_interviews.go`:

- `taskDTO` with json camelCase tags — the DTO is the only wire shape;
  never expose `store.Task` directly. Have separate full/list projections
  if some columns are heavy (`toInterviewDTO(iv, full bool)` is the
  pattern).
- `handleListTasks` (`s.protected`): own rows; `?all=true` for admins.
- `handleCreateTask` (`s.writable`): `randomID("task_")`,
  `ident.EffectiveUserID()` as owner, publish `task.created` via
  `s.hub.PublishTo(row.UserID, ...)` → 201.
- `loadOwnedTask` helper: strangers get 404.
- `handleUpdateTask` (`s.writable`): pointer fields for partial update,
  publish `task.updated`.
- `handleDeleteTask` (`s.writable`): publish `task.deleted`.

## 4. Routes — `internal/server/server.go`

```go
api.HandleFunc("GET /api/tasks", s.protected(s.handleListTasks))
api.HandleFunc("POST /api/tasks", s.writable(s.handleCreateTask))
api.HandleFunc("GET /api/tasks/{id}", s.protected(s.handleGetTask))
api.HandleFunc("PUT /api/tasks/{id}", s.writable(s.handleUpdateTask))
api.HandleFunc("DELETE /api/tasks/{id}", s.writable(s.handleDeleteTask))
```

Mutating routes MUST use `s.writable` (it enforces read-only actAs).

## 5. Frontend API client — `web/src/lib/api/tasks.ts`

Copy the shape of `web/src/lib/api/interviews.ts`: `Task` interface
(camelCase), `listTasks/createTask/updateTask/deleteTask` via
`apiJSON`/`jsonInit` imported from `../api`.

## 6. Page — `web/src/app/tasks/page.tsx`

Copy `web/src/app/interviews/page.tsx`: `"use client"`, table + create/edit
dialog + delete confirm, `subscribeEvents` filtered on
`evt.type.startsWith("task.")` → `reload()`.

Note the lint rule: do not call `reload()` synchronously inside
`useEffect` (`react-hooks/set-state-in-effect` is an error in
eslint-config-next 16) — use `void (async () => { await reload() })()`.

## 7. Navigation — `web/src/components/app-shell.tsx`

Add to the `NAV` Main section (generated code uses the
`--- gen:nav ---` marker) plus the lucide icon import.

## 8. Verify

```bash
go build ./... && go vet ./...
cd web && pnpm build
make build && ./bin/app   # exercise the flow in the browser
```
