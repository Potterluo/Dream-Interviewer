# Dream Interviewer — API contract (frozen)

Every JSON endpoint returns the template envelope: `{"ok": true, ...}` or
`{"ok": false, "error": "..."}`. All routes are **authenticated** (cookie
session or `Authorization: Bearer sk_...`) unless marked public.

`?actAs=<userId>` (admin) is accepted by the shared middleware: it switches
`EffectiveUserID()` and marks the request read-only, and it is honoured by
the routes that read `EffectiveUserID()` — `GET /api/me`, interview creation
and file upload. List and aggregate endpoints **ignore** `actAs`; to read
another user's records, an admin uses `?userId=` (§6). Mutating routes reject
a read-only `actAs` caller with `403`.

Ownership rule: another user's record is a **404**, never a 403. Admins pass.
The only routes that relax ownership are the two admin filters
(`?all=true` / `?userId=`) documented in §6 — and there, a **non-admin gets an
explicit 403** rather than a silently filtered list, so an escalation attempt
is visible instead of looking like an empty result set.

---

## 1. Enums

```
status        draft | in_progress | completed | aborted
level         junior | mid | senior | expert
type          tech | behavior | mixed | system_design
difficulty    easy | normal | hard
language      zh | en
kind          question | followup
verdict       strong | ok | weak            (per answer)
recommendation strong_hire | hire | maybe | no_hire   (whole interview)
engine        llm | offline
```

`level` / `type` / `difficulty` / `language` are **free-form strings on the
wire**; the values above are the ones the UI sends. The server validates
against this list and falls back to the default rather than 400.

Status transitions, and the only thing that writes each of them:

```
POST /api/interviews         → draft
advance action=start         → in_progress
advance action=answer (last) → completed
advance action=finish        → completed   (also from completed and aborted)
advance action=abort         → aborted     (graded turns are kept; NOT from draft)
advance action=start         → in_progress (resumes an aborted interview)
```

`engine` records which implementation produced the last result: `llm`, or
`offline` when the built-in bank/rubric was used (see `Source.Note` for why).

---

## 2. Shared object shapes

### Dimension

```json
{ "key": "accuracy", "label": "技术准确性", "weight": 0.3, "desc": "..." }
```

`key` is a stable slug; `label` is for display; `weight` sums to 1.0 across
the interview's dimension set.

### PlannedQuestion

```json
{
  "seq": 1,
  "dimension": "accuracy",
  "question": "…",
  "intent": "考察候选人对 … 的理解",
  "weight": 1.0,
  "followUps": ["…"],
  "reference": "参考答案要点"
}
```

### Interview (list + detail DTO)

```json
{
  "id": "iv_xxxxxxxx",
  "userId": "u_…",
  "title": "高级后端工程师 · 技术面试",
  "role": "高级后端工程师",
  "level": "senior",
  "interviewType": "tech",
  "language": "zh",
  "difficulty": "normal",
  "questionCount": 6,
  "resumeText": "…",
  "jdText": "…",
  "status": "in_progress",
  "plan": { "dimensions": [Dimension], "questions": [PlannedQuestion], "rationale": "…" },
  "currentSeq": 2,
  "turnCount": 3,
  "overallScore": 78.5,
  "hrSatisfaction": 74.0,
  "recommendation": "hire",
  "summary": "一句话总评",
  "report": Report | null,
  "model": "Qwen/Qwen2.5-7B-Instruct",
  "engine": "llm",
  "durationSec": 412,
  "startedAt": "2025-01-01T00:00:00Z",
  "completedAt": null,
  "createdAt": "…",
  "updatedAt": "…"
}
```

`recommendation` and `hrSatisfaction` are empty/0 until the report exists, but
they are real columns, so the list DTO carries them too — the table can show a
recommendation badge without loading any report.

List responses **omit** `plan`, `report`, `resumeText`, `jdText`, `summary`
(they are set to `null`/`""`) so the table stays light; `GET` by id returns
everything.

### InterviewTurn

```json
{
  "id": "tr_xxxxxxxx",
  "interviewId": "iv_…",
  "seq": 1,
  "kind": "question",
  "dimension": "accuracy",
  "question": "…",
  "intent": "…",
  "weight": 1.0,
  "answer": "…",
  "score": 82.0,
  "grade": Grade | null,
  "feedback": "…",
  "answerSeconds": 95,
  "createdAt": "…",
  "answeredAt": "…"
}
```

### Grade

```json
{
  "score": 82.0,
  "dimensions": { "accuracy": 85, "depth": 78, "clarity": 88, "relevance": 80 },
  "strengths": ["…"],
  "weaknesses": ["…"],
  "feedback": "点评正文（Markdown）",
  "reference": "参考答案要点",
  "followUp": "",
  "verdict": "strong"
}
```

`followUp` non-empty means the interviewer wants one adaptive follow-up.

### Report

```json
{
  "overallScore": 78.5,
  "hrSatisfaction": 74.0,
  "dimensions": [ { "key": "…", "label": "…", "score": 80, "weight": 0.3, "comment": "…" } ],
  "summary": "总评（Markdown）",
  "strengths": ["…"],
  "weaknesses": ["…"],
  "suggestions": ["…"],
  "resumeSuggestions": ["…"],
  "interviewStrategies": ["…"],
  "learningPlan": [ { "topic": "…", "why": "…", "how": "…" } ],
  "recommendation": "hire",
  "recommendationReason": "…",
  "highlights": ["候选人原话摘录"],
  "risks": ["…"]
}
```

All array fields are always present (never `null`) — the UI maps over them
directly. `hrSatisfaction` is the reference project's "predicted HR
satisfaction" signal: a 0-100 estimate of how an HR screener would react,
which is not the same thing as the technical score.

> **Counts.** The report is built from turns that were actually answered
> (`answeredAt` set). An **open** question — the one on screen when the
> candidate stopped or gave up — is not part of the evaluation and is not
> counted as 未作答. A **skipped** question IS answered (with no text), so it
> is counted as 未作答 and scored 0. This means the report's `本场共 N 题`
> can be smaller than the interview's `turnCount`, which counts every
> question ever asked. That is intentional: `turnCount` is a row count,
> while the report describes what was assessed.

---

## 3. Endpoints

### `GET /api/interviews`

Query: `all=true` (admin).

```json
{ "ok": true, "interviews": [Interview (light)] }
```

### `POST /api/interviews`

```json
{
  "title": "可选；缺省由 role/type 生成",
  "role": "高级后端工程师",
  "level": "senior",
  "interviewType": "tech",
  "language": "zh",
  "difficulty": "normal",
  "questionCount": 6,
  "resumeText": "…",
  "jdText": "…"
}
```

Creates a `draft` interview (no LLM call). Responds `201`:

```json
{ "ok": true, "interview": Interview }
```

`role` is required (400 otherwise). `questionCount` is clamped to 3..15.

### `GET /api/interviews/{id}`

```json
{ "ok": true, "interview": Interview, "turns": [InterviewTurn] }
```

### `PUT /api/interviews/{id}`

Any subset of the create fields plus `title`. Only allowed while `draft`
(409 otherwise). Returns `{ "ok": true, "interview": Interview }`.

> The template's own `GET /api/stats` endpoint (items + files demo
> aggregates) was **removed** along with the `items` and mock-`chat`
> example domains. Analytics reads `GET /api/interview/stats` below.

### `DELETE /api/interviews/{id}`

Deletes the interview and all its turns. `{ "ok": true }`.

### `POST /api/interviews/{id}/advance` — **the streaming endpoint**

This single endpoint advances the interview state machine and streams the
model's output. Request body:

```json
{
  "action": "start | answer | skip | finish | abort",
  "answer": "候选人作答（action=answer 时必填）",
  "elapsedSec": 95,
  "turnId": "tr_xxxxxxxx"
}
```

`elapsedSec` is optional and recorded on the turn as `answerSeconds`; the UI
derives it from the open turn's `createdAt` rather than a local timer, so it
survives a page reload. `answerSeconds` is `0` for turns nobody answered.

`turnId` is optional and binds the submit to the question the client was
showing (`answer` / `skip` only). If it does not match the currently open
turn the request is rejected with an `error` frame. Send it: without it, a
late retry — a resend after a timeout, a user going back and resubmitting —
would be scored against whatever question is open *now*.

Response: `text/event-stream`, one JSON object per `data:` frame
(consumed by `streamSSE` in `web/src/lib/api.ts`).

| frame | payload | meaning |
|---|---|---|
| `stage` | `"planning" \| "questioning" \| "grading" \| "reporting"` | a new stage began; **reset your streaming buffer** |
| `delta` | `"text"` | visible model text for the current stage — **Markdown**, append it |
| `plan` | `Plan` | plan finished (action=start) |
| `grade` | `{ "turn": InterviewTurn }` | the answered turn was graded and persisted |
| `question` | `{ "turn": InterviewTurn }` | the next question to answer |
| `report` | `Report` | final report (also persisted on the interview) |
| `done` | `{ "interview": Interview, "turns": [InterviewTurn] }` | terminal frame — authoritative state |
| `error` | `"message"` | the stage failed; the stream ends after this |

`questioning` is emitted before every question (start, next, and
follow-up) — the question text also arrives as a `delta`, so the stage is
what tells the UI to start a fresh buffer instead of appending to the
previous stage's prose.

Rules:

- `action=start` — requires `draft` (or a restart of something not yet
  completed). Builds the plan if there is none (`plan` frame), then emits
  the first `question`. If a question is already open it is re-emitted
  rather than replaced; if history exists but nothing is open (a previous
  request died between grading and asking) the plan resumes — **answers are
  never deleted**.
- `action=answer` — grades the pending question (`grade` frame), then
  either emits the follow-up / next `question`, or — when the plan is
  exhausted — transitions to `completed` and emits `stage:reporting`,
  `report`, `done`. Answering an already-answered turn yields an `error`
  frame (see the concurrency note below). An answer shorter than 4
  characters is scored by the built-in rubric **without calling the model**,
  and says so.
- `action=skip` — records the pending turn with an empty answer and score
  0, then emits the next `question` (or finishes). A skipped turn counts as
  answered (`answeredAt` set) but carries no answer text.
- `action=finish` — wraps up early: `stage:reporting`, `report`, `done`.
  Allowed from `in_progress`, `completed` (regenerate the report) **and
  `aborted`** (change of heart after giving up; moves the row to
  `completed`).
- `action=abort` — gives up on the interview: sets `status="aborted"`, keeps
  every recorded turn, grades nothing, and emits only `done`. This is the
  one path that writes the `aborted` state the stats page counts. Rejected
  on a `draft` (nothing has been asked, so there is nothing to abandon) and
  on a `completed` interview (409).
- `action=start` on an `aborted` interview is the "resume" path: it puts the
  status back to `in_progress` and re-emits the open question, so the
  session can actually be continued.
- Every path ends with exactly one `done` frame (or one `error` frame).
- The client aborting the request (Stop button) cancels the model call via
  `r.Context()`; already-persisted rows stay consistent and the open
  question stays open, so an identical retry works.
- **Concurrency.** Advances for one interview are serialised, and a
  *simultaneous* duplicate is refused with `409` and the message
  `这场面试正在处理上一个请求…` rather than queued. Queueing would be a
  correctness bug, not just a cost one: by the time the loser ran, the
  winner would have advanced the interview, so the loser would find a
  different open question and legitimately write the stale submission onto
  it. Combined with the `answered_at IS NULL` compare-and-set on the answer
  write and the optional `turnId` binding, a duplicated or late submit can
  neither be written twice nor land on the wrong question.
- **Which refusal gives which shape.** A *simultaneous* duplicate is a
  transport-level refusal: `409` with a JSON envelope, and **no stream is
  opened**. A *stale* `turnId` is a state-level refusal discovered while
  the request is already valid: **`200` plus an `error` frame inside the
  stream**. Clients must handle both — `streamSSE` throws on a non-2xx
  before any frame, and the `error` frame arrives on an otherwise normal
  stream.

### `GET /api/interviews/{id}/export`

`Content-Type: text/markdown`, `Content-Disposition: attachment`. Renders
the whole interview + report as Markdown.

---

### `GET /api/interview/stats`

Query: `all=true` (admin).

```json
{
  "ok": true,
  "stats": {
    "total": 12,
    "completed": 9,
    "inProgress": 2,
    "aborted": 1,
    "avgScore": 76.4,
    "bestScore": 91.0,
    "avgDurationSec": 640,
    "avgTurns": 5.8,
    "scoreByDay":    [ { "date": "2025-01-01", "count": 3 } ],
    "avgScoreByDay": [ { "date": "2025-01-01", "count": 74.0 } ],
    "dimensionAvg":  [ { "key": "accuracy", "label": "技术准确性", "score": 79.2 } ],
    "roleBreakdown": [ { "label": "高级后端工程师", "count": 4 } ],
    "recommendationBreakdown": [ { "label": "推荐录用", "count": 5 } ],
    "levelBreakdown": [ { "label": "高级", "count": 6 } ]
  }
}
```

All arrays are non-null (empty arrays when there is no data). `scoreByDay`
reuses `{date, count}`; `count` is a `number` (a score for `avgScoreByDay`).
The breakdown labels are **display labels**, already translated through the
same label tables `GET /api/interview/presets` returns — not raw slugs.
`recommendationBreakdown` and `levelBreakdown` keep a fixed order with
`count: 0` entries so a chart legend never reshuffles between reloads.

### `GET /api/interview/presets`

Role presets (built-in ones plus the admin's shared ones) and the enum label
tables. Available to **every** authenticated user — a user picks a preset,
only an admin can change the set.

```json
{
  "ok": true,
  "presets": [
    { "id": "backend-senior", "role": "高级后端工程师", "level": "senior",
      "interviewType": "tech", "difficulty": "normal", "questionCount": 6,
      "focusAreas": ["并发", "数据库", "系统设计"], "jdSample": "…",
      "description": "…", "builtin": true, "createdBy": "" }
  ],
  "labels": {
    "level": [ { "value": "senior", "label": "高级" } ],
    "type": [ { "value": "tech", "label": "技术面试" } ],
    "difficulty": [ { "value": "normal", "label": "常规" } ],
    "language": [ { "value": "zh", "label": "中文" } ],
    "recommendation": [ { "value": "hire", "label": "推荐录用" } ],
    "status": [ { "value": "completed", "label": "已完成" } ],
    "verdict": [ { "value": "strong", "label": "优秀" } ]
  }
}
```

Seven label sets are returned: `level`, `type`, `difficulty`, `language`,
`status`, `recommendation`, `verdict`.

`builtin: true` presets are compiled into the binary and are **read-only** —
attempting to edit or delete one is a `409`, never a silent no-op. Admin presets
carry `builtin: false` and their owner in `createdBy`. Built-ins come first,
then admin presets (newest first).

### `GET /api/interview/engine`

Read-only description of the model currently interviewing people. Available
to every authenticated user — you should be able to see which model assessed
you — but **only an admin can change it** (see `GET/PUT /api/admin/settings`).

```json
{
  "ok": true,
  "engine": {
    "configured": true,
    "provider": "siliconflow",
    "baseUrl": "https://api.siliconflow.cn/v1",
    "model": "XingChenAGI/Xing4.0-29B",
    "apiKeyMasked": "sk-a…1234",
    "apiKeySet": true,
    "source": "db | env | default",
    "fallbackAvailable": true,
    "timeoutSec": 90,
    "maxTokens": 8192,
    "temperature": 0.4
  }
}
```

`provider` is one of `siliconflow` / `openai` / `custom` (see §6). `source`
reports where the **effective** configuration came from: `db` (set by an admin
in the UI), `env` (from the environment or `.env`), or `default`. This is a
read-only view for every authenticated user — you should be able to see which
model assessed you — and it carries no `overridden` map, because that is an
admin concept. `fallbackAvailable` is always `true`, so an unconfigured
install still completes interviews on the bundled bank + rubric.

**`baseUrl`, `apiKeyMasked` and `apiKeySet` are admin-only.** For a non-admin
they are `""`, `""` and `false` — a regular user gets `provider`, `model`,
`source` and the numeric knobs, which is everything needed to answer "which
model interviewed me?", and nothing about the deployment's endpoint or
credential. Clients must treat those three fields as optional. The raw key is
never returned to anyone, at any role.

---

## 6. Admin (role `admin`)

Everything in this section is registered with `s.adminOnly` and additionally
enforced in-handler. A non-admin gets **`403 {"ok":false,"error":"forbidden"}`**.

**Precisely which 403s carry the bare literal `forbidden`:** every
`/api/admin/*` refusal, every admin-only route (`/api/users` CRUD, key
issuance of the admin tier via `RequireAdmin`), and every scope-widening
refusal (§6). Those are the ones a client should branch on, and the string is
exactly `forbidden` so an equality test works.

Four other `403`s carry a specific reason instead, because the reason is the
whole message — do not pattern-match them as authorization failures:

| endpoint | body |
|---|---|
| `POST /api/apikeys` (admin tier, by a non-admin) | `admin role required to issue admin keys` |
| any mutating route, by a read-only `?actAs=` caller | `read-only: cannot mutate while acting as another user` |
| `POST /api/login` on a disabled account | `account disabled` |
| `POST /api/me/password` with the wrong current password | `current password is incorrect` |

Those are the complete set of bodies the server emits with a `403`; anything
else a client sees is either the bare literal or not a permission outcome at
all (the last two are credential results, and the actAs one is an
impersonation guard).

### `GET /api/admin/settings`

The editable model configuration. Never returns the key.

```json
{
  "ok": true,
  "settings": {
    "provider": "siliconflow",
    "baseUrl": "https://api.siliconflow.cn/v1",
    "model": "XingChenAGI/Xing4.0-29B",
    "apiKeyMasked": "sk-a…1234",
    "apiKeySet": true,
    "timeoutSec": 90,
    "maxTokens": 8192,
    "temperature": 0.4,
    "configured": true,
    "overridden": { "baseUrl": true, "model": true, "apiKey": true, "provider": true,
                    "timeoutSec": false, "maxTokens": false, "temperature": false },
    "envPresent": ["APP_LLM_API_KEY", "APP_LLM_MODEL"]
  }
}
```

`overridden[field]` is `true` when a value is stored in the database (i.e. the
admin set it in the UI) and `false` when the field is falling back to the
environment or a default. `envPresent` lists which `APP_LLM_*` variables exist
in the process environment, so the UI can explain a fallback honestly.

### `PUT /api/admin/settings`

Partial update. Field semantics are uniform:

| sent value | meaning |
|---|---|
| field absent / `null` | leave the stored value unchanged |
| `""` (empty string) | **clear** the stored override, reverting to env/default |
| non-empty | store this value |

`apiKey` follows the same rule: omit it to keep the current key, send `""` to
remove it, send a value to replace it. **A UI must omit the field when the user
did not touch it** — otherwise opening the form and saving would wipe the key.

Numeric fields are **rejected, not clamped**, when out of range
(`timeoutSec` 5..600, `maxTokens` 256..131072, `temperature` 0..2), with a
`400` naming the field and its range. Rejection rather than silent clamping is
deliberate: an admin who typed 999999 wants to know, not to discover later
that the saved value is 131072. (Values already in the database are clamped on
read, as a defence for hand-edited rows.) A `400` never leaves a partial
update — all validation runs before the first write. An invalid provider is
also a `400`.

Changing settings **takes effect immediately for the next model call** — the
server swaps its engine. Returns the same shape as `GET`.

### `POST /api/admin/settings/test`

`{ "provider"?, "baseUrl"?, "model"?, "apiKey"?, "timeoutSec"?, "maxTokens"? }`

Makes one small live call. Omitted fields fall back to the **effective**
configuration, so a blank body tests what is currently saved, while sending
values tests a candidate configuration *before* saving it.

```json
{
  "ok": true,
  "result": {
    "ok": true, "latencyMs": 812, "model": "…", "reply": "PONG",
    "testedOverride": true
  }
}
```

A failed probe is still `ok:true` at the envelope level with `result.ok=false`
and `result.error`, so the UI can render a red badge without treating it as a
transport error.

### `DELETE /api/admin/settings`

Clears every stored override, reverting fully to the environment / defaults.
Returns the resulting `GET` shape. Useful when a bad value has locked the admin
out of a working model.

### `POST /api/admin/presets` · `PUT /api/admin/presets/{id}` · `DELETE /api/admin/presets/{id}`

Shared preset management. Body for create/update:

```json
{
  "role": "资深数据工程师",
  "level": "senior",
  "interviewType": "tech",
  "difficulty": "hard",
  "questionCount": 8,
  "focusAreas": ["数仓分层", "数据质量", "调度"],
  "jdSample": "…",
  "description": "…"
}
```

- `POST` returns `201` + `{ "ok": true, "preset": Preset }`. `role` is required
  (400); `questionCount` is clamped to 3..15; unknown enum values fall back to
  the same defaults as interview creation.
- `PUT` is a partial update with the usual absent-vs-`""` rule. Editing a
  built-in returns `409` (`内置预置不可修改`).
- `DELETE` returns `{ "ok": true }`; deleting a built-in returns `409`.

### Reading other users' data

These are the **only** ways to see rows you do not own, and each one is
admin-only. A regular user passing them gets `403` — not a silently filtered
list, so an attempt to escalate is visible in the logs rather than looking like
an empty result set.

| endpoint | effect |
|---|---|
| `GET /api/interviews?all=true` | every user's interviews |
| `GET /api/interviews?userId=<id>` | one user's interviews |
| `GET /api/interview/stats?all=true` | aggregates over everyone |
| `GET /api/interview/stats?userId=<id>` | aggregates for one user |
| `GET /api/files?all=true` | every user's uploaded files |
| `GET /api/files?userId=<id>` | one user's uploaded files |
| `GET /api/admin/users/{id}/overview` | that user's counts + score summary |

```json
{
  "ok": true,
  "overview": {
    "user": { "id": "u_…", "username": "alice", "displayName": "Alice",
              "role": "user", "status": "active" },
    "total": 7, "completed": 5, "inProgress": 1, "aborted": 1,
    "avgScore": 74.2, "bestScore": 91.0,
    "lastActivityAt": "2025-01-01T00:00:00Z",
    "recommendationBreakdown": [ { "label": "推荐录用", "count": 3 } ]
  }
}
```

`GET /api/interviews/{id}` and `GET /api/interviews/{id}/export` already admit
an admin for any row, so an admin can open and export anyone's report without a
separate endpoint.

Every list endpoint that can be widened routes through the same resolver
(`internal/server/scope.go`), so the rule is uniform rather than per-handler:
`?all=true` and `?userId=` are the ONLY ways ownership is relaxed, the table
above is the complete list of endpoints that honour them, and a non-admin
passing either gets the `403`.

---

## 7. Configuration and secrets

**No credential is compiled into the binary.** The build contains no API key,
no private endpoint and no token of any kind — that was a deliberate fix, and
`internal/config` has no key constant at all.

Values are resolved per field, highest precedence first:

1. **Database** — set by an admin in the UI (`GET/PUT /api/admin/settings`).
2. **Environment / `.env`** — for headless or image-based deployments.
3. **Built-in default** — only `timeoutSec` (90), `maxTokens` (8192),
   `temperature` (0.4). Endpoint/model/key have no default.

`.env` is read from the working directory at boot, then from next to the
binary. **Real environment variables always beat `.env`**, so a container can
override a baked-in file. `.env` is listed in `.gitignore`; `.env.example` is
committed as documentation. `APP_LLM_API_KEY` (and `APP_DB_DSN`) are unset from
the process environment right after boot (`config.ScrubBootSecrets`) so they
cannot leak into child processes.

`GET /api/admin/settings` reports the resulting per-field `overridden` map, so
"why is it using that model?" is answerable from the UI instead of by reading
the deployment.

---

## 8. Realtime events (existing `GET /api/events` hub)

Published to the owner's stream so lists refresh live:

```
interview.created   data: Interview (light)
interview.updated   data: Interview (light)
interview.deleted   data: { "id": "iv_…" }
```

---

## 9. Notes for the frontend

- Use `apiJSON` / `apiFetch` from `web/src/lib/api.ts`; never bare `fetch`.
- Use `streamSSE` for `/advance` — it already handles the `data:` framing
  and AbortSignal for the Stop button.
- Render any model/user Markdown with `<Markdown>` from
  `@/components/markdown.tsx`.
- Detail pages are **query-param driven** (`/interview/?id=…`), not
  `[id]` dynamic routes — the frontend is a static export, so dynamic
  segments would need `generateStaticParams`.
- Admin-only paths must be listed in `ADMIN_PATH_PREFIXES`
  (`web/src/components/auth-guard.tsx`) for routing, but that is UX only:
  every admin endpoint re-checks the role server-side. Never treat the client
  guard as the permission check.
- Hide admin navigation and admin actions for non-admins, and expect a `403`
  if the UI is bypassed.
