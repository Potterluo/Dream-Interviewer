// api/admin.ts — typed client for the admin surface (docs/API.md §6).
//
// Every endpoint here is registered with `s.adminOnly` server-side and
// re-checks the role in-handler: a non-admin gets 403 even if the client
// guard is bypassed. The client gate (ADMIN_PATH_PREFIXES) is UX only.
//
// Two contract details this module exists to make hard to get wrong:
//
//   1. PUT /api/admin/settings is a PARTIAL update with an absent-vs-"" rule:
//      an absent field keeps its stored value, "" clears the override so the
//      field falls back to env/default. `apiKey` follows the same rule, so a
//      UI must OMIT apiKey unless the admin actually typed a new key —
//      otherwise saving the form wipes the stored key.
//   2. A failed connection probe is `ok:true` at the envelope level with
//      `result.ok=false`; transport failure is a different thing entirely.

import { apiJSON, jsonInit, type Envelope, type User } from "@/lib/api";
import type { BreakdownEntry, Preset } from "@/lib/api/interviews";

// --- Settings (docs/API.md §6, §7) -----------------------------------------

/** The fields an admin can override in the database. */
export type AdminSettingsField =
  | "provider"
  | "baseUrl"
  | "model"
  | "apiKey"
  | "timeoutSec"
  | "maxTokens"
  | "temperature";

export interface AdminSettings {
  provider: string;
  baseUrl: string;
  model: string;
  /** masked for display only ("sk-a…1234"); the real key never leaves the server */
  apiKeyMasked: string;
  apiKeySet: boolean;
  timeoutSec: number;
  maxTokens: number;
  temperature: number;
  /** true when provider + baseUrl + model + key are all resolved */
  configured: boolean;
  /**
   * true when the field is stored in the DB (the admin set it in the UI),
   * false when it is falling back to the environment or a built-in default.
   */
  overridden: Record<string, boolean>;
  /** which APP_LLM_* variables exist in the process environment */
  envPresent: string[];
}

/**
 * Partial update body. Semantics are uniform per field:
 *   absent      → leave the stored value unchanged
 *   ""          → clear the stored override, revert to env/default
 *   non-empty   → store this value
 * Numbers may be sent as "" to clear them; they are clamped server-side
 * (timeoutSec 5..600, maxTokens 256..131072, temperature 0..2).
 */
export interface AdminSettingsInput {
  provider?: string;
  baseUrl?: string;
  model?: string;
  apiKey?: string;
  timeoutSec?: number | "";
  maxTokens?: number | "";
  temperature?: number | "";
}

/**
 * POST /api/admin/settings/test body. Note the contract has no temperature
 * here. Omitted fields fall back to the EFFECTIVE configuration, so a blank
 * body tests what is saved while sending values tests a candidate config
 * before saving it.
 */
export interface AdminSettingsTestInput {
  provider?: string;
  baseUrl?: string;
  model?: string;
  apiKey?: string;
  timeoutSec?: number;
  maxTokens?: number;
}

export interface AdminTestResult {
  ok: boolean;
  latencyMs?: number;
  model?: string;
  reply?: string;
  error?: string;
  /** true when the probe used at least one value supplied by the caller */
  testedOverride?: boolean;
}

export interface AdminSettingsResponse extends Envelope {
  settings?: AdminSettings;
}

export interface AdminTestResponse extends Envelope {
  result?: AdminTestResult;
}

export interface AdminPresetResponse extends Envelope {
  preset?: Preset;
}

// --- Shared preset management ----------------------------------------------

export interface PresetInput {
  role: string;
  level?: string;
  interviewType?: string;
  difficulty?: string;
  questionCount?: number;
  focusAreas?: string[];
  jdSample?: string;
  description?: string;
}

// --- Cross-user reads -------------------------------------------------------

export interface UserOverview {
  user: User;
  total: number;
  completed: number;
  inProgress: number;
  aborted: number;
  avgScore: number;
  bestScore: number;
  lastActivityAt?: string | null;
  recommendationBreakdown: BreakdownEntry[];
}

export interface UserOverviewResponse extends Envelope {
  overview?: UserOverview;
}

// --- Endpoints --------------------------------------------------------------

// Model configuration (admin only).

export async function getAdminSettings(): Promise<AdminSettingsResponse> {
  return apiJSON("/api/admin/settings");
}

export async function updateAdminSettings(
  req: AdminSettingsInput
): Promise<AdminSettingsResponse> {
  return apiJSON("/api/admin/settings", jsonInit("PUT", req));
}

export async function testAdminSettings(
  req: AdminSettingsTestInput
): Promise<AdminTestResponse> {
  return apiJSON("/api/admin/settings/test", jsonInit("POST", req));
}

export async function clearAdminSettings(): Promise<AdminSettingsResponse> {
  return apiJSON("/api/admin/settings", { method: "DELETE" });
}

// Shared presets (admin only). Built-ins are compiled in and read-only:
// editing or deleting one is a 409, so the UI must not offer it.

export async function createPreset(req: PresetInput): Promise<AdminPresetResponse> {
  return apiJSON("/api/admin/presets", jsonInit("POST", req));
}

export async function updatePreset(
  id: string,
  req: Partial<PresetInput>
): Promise<AdminPresetResponse> {
  return apiJSON(`/api/admin/presets/${encodeURIComponent(id)}`, jsonInit("PUT", req));
}

export async function deletePreset(id: string): Promise<Envelope> {
  return apiJSON(`/api/admin/presets/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// One user's counts + score summary.
export async function getUserOverview(id: string): Promise<UserOverviewResponse> {
  return apiJSON(`/api/admin/users/${encodeURIComponent(id)}/overview`);
}
