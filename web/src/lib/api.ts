// api.ts — the single place the frontend talks to the backend.
//
// Layers, top to bottom:
//
//   1. Types mirroring the Go DTOs (internal/server/handlers_*.go)
//   2. apiFetch — fetch with credentials + optional bearer + actAs mirror
//   3. Typed endpoint functions (login, users, apiKeys, files, stats)
//   4. Realtime helpers (EventSource subscription + POST-SSE reader)
//
// Convention: every endpoint returns the parsed JSON envelope
// ({ok, error?, ...}) — callers branch on `ok`, never on HTTP codes.

// --- 1. Types ---------------------------------------------------------------

export interface User {
  id: string;
  username: string;
  email: string;
  displayName: string;
  role: "admin" | "user" | string;
  status: "active" | "disabled" | string;
  createdAt: string;
}

export interface FileItem {
  id: string;
  userId: string;
  name: string;
  size: number;
  contentType: string;
  createdAt: string;
}

export interface APIKey {
  id: string;
  userId: string;
  name?: string;
  /** masked ("sk_0123456789****") on list; plaintext on create/rotate */
  key: string;
  type: "admin" | "user" | string;
  createdAt: string;
}

export interface Envelope {
  ok: boolean;
  error?: string;
}

export interface MeResponse extends Envelope {
  user?: User;
  authMethod?: string;
  actAsUserId?: string;
  readOnly?: boolean;
}

export interface StatusResponse extends Envelope {
  configured: boolean;
  authenticated: boolean;
  version?: string;
  user?: User;
}

// --- 2. apiFetch ------------------------------------------------------------

// Optional bearer token for programmatic use (localStorage). The cookie
// session set by /api/login is the primary credential for the web UI;
// this exists so the same UI can talk to the API with an issued key.
let authToken = "";

export function setAuthToken(token: string) {
  authToken = token;
  if (token) {
    localStorage.setItem("app_token", token);
  } else {
    localStorage.removeItem("app_token");
  }
}

export function getAuthToken(): string {
  if (!authToken) {
    authToken = localStorage.getItem("app_token") || "";
  }
  return authToken;
}

// Wrapper around fetch that attaches the bearer token when one is set
// and always includes the session cookie. When the page URL carries
// `?actAs=<userId>` (admin auditing another user), the param is mirrored
// onto every API request; the backend makes those read-only.
export async function apiFetch(url: string, init?: RequestInit): Promise<Response> {
  const token = getAuthToken();
  const headers: Record<string, string> = {
    ...((init?.headers as Record<string, string>) || {}),
  };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }
  if (typeof window !== "undefined") {
    const pageActAs = new URLSearchParams(window.location.search).get("actAs");
    if (pageActAs && !/[?&]actAs=/.test(url)) {
      url += (url.includes("?") ? "&" : "?") + "actAs=" + encodeURIComponent(pageActAs);
    }
  }
  return fetch(url, { credentials: "same-origin", ...init, headers });
}

// jsonFetch: apiFetch + JSON body + parsed envelope. Covers 90% of calls.
export async function apiJSON<T extends Envelope>(
  url: string,
  init?: RequestInit
): Promise<T> {
  const res = await apiFetch(url, init);
  return res.json();
}

// Exported so generated per-entity API modules (web/src/lib/api/<entity>.ts)
// reuse the same request shape.
export function jsonInit(method: string, body: unknown): RequestInit {
  return {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  };
}

// --- 3. Endpoints -----------------------------------------------------------

// Session lifecycle

export async function getStatus(): Promise<StatusResponse> {
  // Plain fetch (not apiFetch): must also work before any user exists.
  const res = await fetch("/api/status", { credentials: "same-origin" });
  return res.json();
}

export async function login(loginField: string, password: string): Promise<MeResponse> {
  const res = await fetch("/api/login", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ login: loginField, password }),
  });
  return res.json();
}

export async function onboard(req: {
  username: string;
  email: string;
  password: string;
  displayName?: string;
}): Promise<MeResponse> {
  return apiJSON("/api/onboard", jsonInit("POST", req));
}

export async function logout(): Promise<void> {
  await apiFetch("/api/logout", { method: "POST" });
  setAuthToken("");
}

export async function getMe(): Promise<MeResponse> {
  return apiJSON("/api/me");
}

export async function updateMe(req: { displayName: string }): Promise<MeResponse> {
  return apiJSON("/api/me", jsonInit("PUT", req));
}

export async function changeMyPassword(req: {
  oldPassword: string;
  newPassword: string;
}): Promise<Envelope> {
  return apiJSON("/api/me/password", jsonInit("POST", req));
}

// Admin: users

export async function listUsers(): Promise<{ ok: boolean; users?: User[]; error?: string }> {
  return apiJSON("/api/users");
}

export async function createUser(req: {
  username: string;
  email: string;
  password: string;
  displayName?: string;
  role?: string;
}): Promise<{ ok: boolean; user?: User; error?: string }> {
  return apiJSON("/api/users", jsonInit("POST", req));
}

export async function updateUser(
  id: string,
  req: { displayName?: string; role?: string; status?: string }
): Promise<{ ok: boolean; user?: User; error?: string }> {
  return apiJSON(`/api/users/${id}`, jsonInit("PUT", req));
}

export async function deleteUser(id: string): Promise<Envelope> {
  return apiJSON(`/api/users/${id}`, { method: "DELETE" });
}

export async function resetUserPassword(id: string, newPassword: string): Promise<Envelope> {
  return apiJSON(`/api/users/${id}/password`, jsonInit("POST", { newPassword }));
}

// API keys (per-user)

export async function listAPIKeys(): Promise<{ ok: boolean; apiKeys?: APIKey[]; error?: string }> {
  return apiJSON("/api/apikeys");
}

export async function createAPIKey(req: {
  name: string;
  type?: string;
}): Promise<{ ok: boolean; apiKey?: APIKey; error?: string }> {
  return apiJSON("/api/apikeys", jsonInit("POST", req));
}

export async function deleteAPIKey(id: string): Promise<Envelope> {
  return apiJSON(`/api/apikeys/${id}`, { method: "DELETE" });
}

export async function rotateAPIKey(id: string): Promise<{ ok: boolean; key?: string; error?: string }> {
  return apiJSON(`/api/apikeys/${id}/rotate`, { method: "POST" });
}

// --- 4. Realtime helpers ----------------------------------------------------

export interface LiveEvent {
  type: string; // "hello" | "ping" | "item.created" | "delta" | ...
  data?: unknown;
}

// subscribeEvents opens the GET /api/events SSE stream. EventSource
// sends the session cookie same-origin automatically; API-key callers
// pass a token, which is appended as ?token= (the backend allows the
// query param ONLY on this endpoint, /ws, and file downloads).
export function subscribeEvents(
  onEvent: (evt: LiveEvent) => void,
  token?: string
): () => void {
  const url = token ? `/api/events?token=${encodeURIComponent(token)}` : "/api/events";
  const es = new EventSource(url, { withCredentials: false });
  es.onmessage = (m) => {
    try {
      onEvent(JSON.parse(m.data) as LiveEvent);
    } catch {
      // malformed frame — ignore, the stream stays open
    }
  };
  return () => es.close();
}

// streamSSE reads a streaming POST response (server → client push within
// one request) and feeds each `data:` frame to onEvent. Used by the
// interview room (POST /api/interviews/{id}/advance); the same helper fits
// any streaming endpoint you add (long jobs, model output, log tails).
// Pass abort signal to wire a Stop button — the server notices the closed
// connection and stops generating.
export async function streamSSE(
  url: string,
  init: RequestInit,
  onEvent: (evt: LiveEvent) => void
): Promise<void> {
  const res = await apiFetch(url, init);
  if (!res.ok || !res.body) {
    let msg = `stream failed: ${res.status}`;
    try {
      const body = await res.json();
      if (body?.error) msg = body.error;
    } catch {
      // non-JSON error body
    }
    throw new Error(msg);
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buf = "";
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    const lines = buf.split("\n");
    buf = lines.pop() ?? "";
    for (const line of lines) {
      if (!line.startsWith("data: ")) continue;
      const payload = line.slice("data: ".length).trim();
      if (!payload) continue;
      try {
        onEvent(JSON.parse(payload) as LiveEvent);
      } catch {
        // non-JSON frame — surface as text
        onEvent({ type: "text", data: payload });
      }
    }
  }
}

// --- 5. Files ---------------------------------------------------------------

export async function listFiles(all = false): Promise<{ ok: boolean; files?: FileItem[]; error?: string }> {
  return apiJSON(`/api/files${all ? "?all=true" : ""}`);
}

export async function deleteFile(id: string): Promise<Envelope> {
  return apiJSON(`/api/files/${id}`, { method: "DELETE" });
}

// Download URL — session-cookie auth works for plain navigations; pass
// an API-key token for scripts (<a href> can't set headers).
export function fileDownloadUrl(f: FileItem, token?: string): string {
  return token ? `/api/files/${f.id}?token=${encodeURIComponent(token)}` : `/api/files/${f.id}`;
}

// uploadFile uses XHR instead of fetch for one reason: upload progress
// events. Resolve shape matches apiJSON's envelope.
export function uploadFile(
  file: globalThis.File,
  onProgress?: (percent: number) => void
): Promise<{ ok: boolean; file?: FileItem; error?: string }> {
  return new Promise((resolve) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/files");
    xhr.withCredentials = true;
    const token = getAuthToken();
    if (token) xhr.setRequestHeader("Authorization", `Bearer ${token}`);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };
    xhr.onload = () => {
      try {
        resolve(JSON.parse(xhr.responseText));
      } catch {
        resolve({ ok: false, error: `upload failed (${xhr.status})` });
      }
    };
    xhr.onerror = () => resolve({ ok: false, error: "network error during upload" });
    const form = new FormData();
    form.append("file", file);
    xhr.send(form);
  });
}
