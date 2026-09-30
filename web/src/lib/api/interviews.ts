// api/interviews.ts — typed client for the Dream Interviewer API.
//
// Mirrors docs/API.md §3 exactly. Every call goes through the shared
// helpers in @/lib/api (apiFetch/apiJSON/jsonInit/streamSSE) so the
// cookie session, the ?actAs= mirror and the SSE framing stay in one
// place — never call bare fetch from a page.
//
// Nullability note: LIST responses omit the heavy fields (plan, report,
// resumeText, jdText, summary) by sending null/"" so the table stays
// light; GET by id returns everything. Every field that can come back
// null is typed `| null` and every UI reads it defensively.

import { apiJSON, jsonInit, streamSSE, type Envelope } from "@/lib/api";

// --- Enums (docs/API.md §1) -------------------------------------------------

export type InterviewStatus = "draft" | "in_progress" | "completed" | "aborted";
export type InterviewLevel = "junior" | "mid" | "senior" | "expert";
export type InterviewType = "tech" | "behavior" | "mixed" | "system_design";
export type InterviewDifficulty = "easy" | "normal" | "hard";
export type InterviewLanguage = "zh" | "en";
export type TurnKind = "question" | "followup";
export type Verdict = "strong" | "ok" | "weak";
export type Recommendation = "strong_hire" | "hire" | "maybe" | "no_hire";
export type InterviewEngine = "llm" | "offline";

// --- Shared object shapes (docs/API.md §2) ----------------------------------

export interface Dimension {
  key: string;
  label: string;
  weight: number;
  desc?: string;
}

export interface PlannedQuestion {
  seq: number;
  dimension: string;
  question: string;
  intent?: string;
  weight?: number;
  followUps?: string[];
  reference?: string;
}

export interface Plan {
  dimensions: Dimension[];
  questions: PlannedQuestion[];
  rationale?: string;
}

export interface Grade {
  score: number;
  /** per-dimension 0..100 score, keyed by Dimension.key */
  dimensions?: Record<string, number>;
  strengths?: string[];
  weaknesses?: string[];
  /** 点评正文（Markdown） */
  feedback?: string;
  reference?: string;
  /** non-empty means the interviewer wants one adaptive follow-up */
  followUp?: string;
  verdict?: Verdict | string;
}

export interface InterviewTurn {
  id: string;
  interviewId: string;
  seq: number;
  kind: TurnKind | string;
  dimension: string;
  question: string;
  intent?: string;
  weight?: number;
  answer?: string;
  score?: number | null;
  grade?: Grade | null;
  feedback?: string;
  answerSeconds?: number;
  createdAt?: string;
  answeredAt?: string | null;
}

export interface ReportDimension {
  key: string;
  label: string;
  score: number;
  weight?: number;
  comment?: string;
}

export interface LearningPlanItem {
  topic: string;
  why?: string;
  how?: string;
}

export interface Report {
  overallScore: number;
  /** 预测 HR 满意度（0..100）；与技术得分不必一致 */
  hrSatisfaction?: number | null;
  dimensions: ReportDimension[];
  /** 总评（Markdown） */
  summary: string;
  strengths: string[];
  weaknesses: string[];
  suggestions: string[];
  /** 针对简历本身的修改建议 */
  resumeSuggestions?: string[] | null;
  /** 下一次面试就能用的策略 */
  interviewStrategies?: string[] | null;
  learningPlan: LearningPlanItem[];
  recommendation: Recommendation | string;
  recommendationReason: string;
  /** 候选人原话摘录 */
  highlights: string[];
  risks: string[];
}

export interface Interview {
  id: string;
  userId: string;
  title: string;
  role: string;
  level: InterviewLevel | string;
  interviewType: InterviewType | string;
  language: InterviewLanguage | string;
  difficulty: InterviewDifficulty | string;
  questionCount: number;
  resumeText?: string | null;
  jdText?: string | null;
  status: InterviewStatus | string;
  plan?: Plan | null;
  currentSeq: number;
  turnCount: number;
  overallScore?: number | null;
  /** 面试官满意度（0..100）；报告生成前为 0 */
  hrSatisfaction?: number | null;
  /** 录用建议；报告生成前为空串。列表 DTO 也带这个字段 */
  recommendation?: Recommendation | string | null;
  summary?: string | null;
  report?: Report | null;
  model?: string;
  engine?: InterviewEngine | string;
  durationSec?: number;
  startedAt?: string | null;
  completedAt?: string | null;
  createdAt?: string;
  updatedAt?: string;
}

// --- Stats / presets / engine (docs/API.md §3) ------------------------------

export interface InterviewStatsPoint {
  date: string;
  /** count for scoreByDay; the score itself for avgScoreByDay */
  count: number;
}

export interface DimensionAverage {
  key: string;
  label: string;
  score: number;
}

export interface BreakdownEntry {
  label: string;
  count: number;
}

export interface InterviewStats {
  total: number;
  completed: number;
  inProgress: number;
  aborted: number;
  avgScore: number;
  bestScore: number;
  avgDurationSec: number;
  avgTurns: number;
  scoreByDay: InterviewStatsPoint[];
  avgScoreByDay: InterviewStatsPoint[];
  dimensionAvg: DimensionAverage[];
  roleBreakdown: BreakdownEntry[];
  recommendationBreakdown: BreakdownEntry[];
  levelBreakdown: BreakdownEntry[];
}

export interface LabelOption {
  value: string;
  label: string;
}

export interface PresetLabels {
  level: LabelOption[];
  type: LabelOption[];
  difficulty: LabelOption[];
  language: LabelOption[];
  recommendation: LabelOption[];
  status: LabelOption[];
  [group: string]: LabelOption[] | undefined;
}

export interface Preset {
  id: string;
  role: string;
  level: string;
  interviewType: string;
  difficulty: string;
  questionCount: number;
  focusAreas?: string[];
  jdSample?: string;
  description?: string;
  /** compiled into the binary and read-only — editing/deleting one is a 409 */
  builtin?: boolean;
  /** owner of an admin-managed preset; "" for built-ins */
  createdBy?: string;
}

export interface EngineStatus {
  configured: boolean;
  provider: string;
  baseUrl: string;
  model: string;
  apiKeyMasked: string;
  apiKeySet: boolean;
  /** db | env | default — where the effective configuration came from */
  source: string;
  fallbackAvailable: boolean;
  timeoutSec: number;
  maxTokens: number;
  /** 生效温度；引擎接口不返回密钥，但这个不是秘密 */
  temperature: number;
}

// --- Request / response envelopes -------------------------------------------

export interface CreateInterviewRequest {
  role: string;
  level?: string;
  interviewType?: string;
  language?: string;
  difficulty?: string;
  questionCount?: number;
  title?: string;
  resumeText?: string;
  jdText?: string;
}

export type UpdateInterviewRequest = Partial<CreateInterviewRequest>;

export interface InterviewsResponse extends Envelope {
  interviews?: Interview[];
}

export interface InterviewResponse extends Envelope {
  interview?: Interview;
}

export interface InterviewDetailResponse extends Envelope {
  interview?: Interview;
  turns?: InterviewTurn[];
}

export interface InterviewStatsResponse extends Envelope {
  stats?: InterviewStats;
}

export interface PresetsResponse extends Envelope {
  presets?: Preset[];
  labels?: PresetLabels;
}

export interface EngineResponse extends Envelope {
  engine?: EngineStatus;
}

// --- Streaming frames (POST /api/interviews/{id}/advance) -------------------

export type Stage = "planning" | "questioning" | "grading" | "reporting";
export type AdvanceAction = "start" | "answer" | "skip" | "finish" | "abort";

export interface AdvanceRequest {
  action: AdvanceAction;
  answer?: string;
  /** 本题作答耗时（秒），落库到 interview_turns.answer_seconds */
  elapsedSec?: number;
  /**
   * 把这次提交绑定到你当时看到的那道题。服务端会在题目已经翻页时拒绝，
   * 否则一次迟到的重试会被算到下一道题上。
   */
  turnId?: string;
}

export type InterviewFrame =
  | { type: "stage"; data: Stage }
  | { type: "delta"; data: string }
  | { type: "plan"; data: Plan }
  | { type: "grade"; data: { turn: InterviewTurn } }
  | { type: "question"; data: { turn: InterviewTurn } }
  | { type: "report"; data: Report }
  | { type: "done"; data: { interview: Interview; turns: InterviewTurn[] } }
  | { type: "error"; data: string };

// --- Endpoints --------------------------------------------------------------

/**
 * Scope for the two ownership-relaxed endpoints (docs/API.md §6). Both
 * parameters are admin-only; a regular user sending either gets 403 rather
 * than a silently filtered list. `userId` narrows to one user and wins over
 * `all` when both are set. A plain boolean is still accepted for backwards
 * compatibility with the original `all` positional argument.
 */
export interface ListScope {
  all?: boolean;
  userId?: string;
}

function scopeQuery(scope: boolean | ListScope): string {
  const all = typeof scope === "boolean" ? scope : !!scope.all;
  const userId = typeof scope === "boolean" ? "" : (scope.userId ?? "").trim();
  const params = new URLSearchParams();
  if (userId) params.set("userId", userId);
  else if (all) params.set("all", "true");
  const q = params.toString();
  return q ? `?${q}` : "";
}

export async function listInterviews(
  scope: boolean | ListScope = false
): Promise<InterviewsResponse> {
  return apiJSON(`/api/interviews${scopeQuery(scope)}`);
}

export async function createInterview(
  req: CreateInterviewRequest
): Promise<InterviewResponse> {
  return apiJSON("/api/interviews", jsonInit("POST", req));
}

export async function getInterview(id: string): Promise<InterviewDetailResponse> {
  return apiJSON(`/api/interviews/${encodeURIComponent(id)}`);
}

export async function updateInterview(
  id: string,
  req: UpdateInterviewRequest
): Promise<InterviewResponse> {
  return apiJSON(`/api/interviews/${encodeURIComponent(id)}`, jsonInit("PUT", req));
}

export async function deleteInterview(id: string): Promise<Envelope> {
  return apiJSON(`/api/interviews/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Markdown export is a plain navigation (Content-Disposition: attachment);
// pass a token for programmatic use — <a href> cannot set headers.
export function interviewExportUrl(id: string, token?: string): string {
  const base = `/api/interviews/${encodeURIComponent(id)}/export`;
  return token ? `${base}?token=${encodeURIComponent(token)}` : base;
}

// advanceInterview drives the whole room: one POST-SSE request per user
// action, one typed frame per callback. Abort with `signal` (Stop button)
// and the server cancels the model call — already-persisted rows stay
// consistent, which is why the caller should re-GET after an abort.
export async function advanceInterview(
  id: string,
  req: AdvanceRequest,
  onFrame: (frame: InterviewFrame) => void,
  signal?: AbortSignal
): Promise<void> {
  await streamSSE(
    `/api/interviews/${encodeURIComponent(id)}/advance`,
    { ...jsonInit("POST", req), signal },
    (evt) => onFrame(evt as unknown as InterviewFrame)
  );
}

export async function getInterviewStats(
  scope: boolean | ListScope = false
): Promise<InterviewStatsResponse> {
  return apiJSON(`/api/interview/stats${scopeQuery(scope)}`);
}

export async function getInterviewPresets(): Promise<PresetsResponse> {
  return apiJSON("/api/interview/presets");
}

export async function getEngineStatus(): Promise<EngineResponse> {
  return apiJSON("/api/interview/engine");
}

// --- Labels -----------------------------------------------------------------

// Chinese copy lives in one place: the server's /api/interview/presets
// label tables. These fallbacks keep the UI readable while presets are
// still loading (or if that endpoint is unreachable).
export const FALLBACK_LABELS: PresetLabels = {
  status: [
    { value: "draft", label: "草稿" },
    { value: "in_progress", label: "进行中" },
    { value: "completed", label: "已完成" },
    { value: "aborted", label: "已中止" },
  ],
  level: [
    { value: "junior", label: "初级" },
    { value: "mid", label: "中级" },
    { value: "senior", label: "高级" },
    { value: "expert", label: "专家" },
  ],
  type: [
    { value: "tech", label: "技术面试" },
    { value: "behavior", label: "行为面试" },
    { value: "mixed", label: "综合面试" },
    { value: "system_design", label: "系统设计" },
  ],
  difficulty: [
    { value: "easy", label: "简单" },
    { value: "normal", label: "常规" },
    { value: "hard", label: "困难" },
  ],
  language: [
    { value: "zh", label: "中文" },
    { value: "en", label: "英文" },
  ],
  recommendation: [
    { value: "strong_hire", label: "强烈推荐" },
    { value: "hire", label: "推荐录用" },
    { value: "maybe", label: "待定" },
    { value: "no_hire", label: "不建议录用" },
  ],
};

// labelFor resolves a wire value to its Chinese label, preferring the
// server-provided table and falling back to FALLBACK_LABELS, then to the
// raw value (free-form strings are legal on the wire).
export function labelFor(
  group: string,
  value?: string | null,
  table?: PresetLabels | null
): string {
  if (!value) return "—";
  const options = table?.[group]?.length ? table[group] : FALLBACK_LABELS[group];
  return options?.find((o) => o.value === value)?.label ?? value;
}

export function optionsFor(
  group: string,
  table?: PresetLabels | null
): LabelOption[] {
  return table?.[group]?.length ? (table[group] as LabelOption[]) : FALLBACK_LABELS[group] ?? [];
}
