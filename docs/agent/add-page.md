# Recipe: add a page (frontend)

Static-export constraints shape everything here: pages are client
components, there is no SSR, no `next/link` middleware, no API routes,
and routing is file-based under `web/src/app` with `trailingSlash: true`
(link to `/tasks/`, not `/tasks`).

## Simple page

1. Create `web/src/app/<route>/page.tsx` starting with `"use client"`.
2. If it needs the signed-in user, call `useUser()` from
   `@/components/user-context` — AuthGuard has already resolved the
   session before any page renders.
3. Data access through `@/lib/api` helpers (`apiJSON`, or a typed
   function). Authenticated calls must use `apiFetch`-based helpers —
   they attach credentials and mirror `?actAs=`.
4. Admin-only pages: add the route prefix to `ADMIN_PATH_PREFIXES` in
   `web/src/components/auth-guard.tsx` (client gate) AND enforce the
   role server-side (`s.adminOnly`) — the client gate is UX, the server
   is the authority.
5. Register it in `NAV` inside `web/src/components/app-shell.tsx`.

## Realtime data

`subscribeEvents((evt) => ...)` returns an unsubscribe function — call
it in the `useEffect` cleanup. Event types are
`<entity>.created|updated|deleted` published by the backend handlers,
plus `hello`/`ping` keep-alives (ignore them).

## UI primitives

- `@/components/ui/*` are shadcn-style components on **@base-ui/react**
  (not Radix). Composition uses the `render` prop:
  `<Button render={<Link href="/x/" />}>Label</Button>` — there is no
  `asChild`.
- Design tokens: use semantic Tailwind classes (`bg-background`,
  `text-muted-foreground`, `border-border`); the palette lives in
  `web/src/app/globals.css` (`:root` + `.dark`). Chart colors come from
  `--chart-1..5`, resolved in JS when fed to SVG (no `var()` in SVG
  attributes).

## Streaming UI

For LLM-style streaming, copy the pattern in `web/src/app/interview/page.tsx`:
`streamSSE` from `@/lib/api` (or the typed wrapper in
`web/src/lib/api/interviews.ts`) with an `AbortController` for the Stop
button, and `<Markdown>` from `@/components/markdown` for the streamed text.

Two rules that page demonstrates and that are easy to get wrong:

- A streamed stage is not a document. Reset the accumulated buffer when a
  `stage` frame arrives, otherwise the planning text bleeds into the grade.
- The terminal `done` frame carries authoritative server state — replace
  local state with it rather than trusting what you accumulated, and
  re-fetch after an abort so an interrupted stream still shows the truth
  that was already persisted.
