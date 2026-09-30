// format.ts — small display helpers shared by the interview pages.
// Kept separate from lib/utils.ts (cn) so the template's helper file stays
// untouched.

// formatDateTime renders an ISO timestamp in the browser's locale.
export function formatDateTime(iso?: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString();
}

export function formatDate(iso?: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleDateString();
}

// formatDuration turns seconds into "6 分 52 秒" / "45 秒".
export function formatDuration(sec?: number | null): string {
  if (sec == null || !Number.isFinite(sec) || sec < 0) return "—";
  const total = Math.round(sec);
  const m = Math.floor(total / 60);
  const s = total % 60;
  if (m === 0) return `${s} 秒`;
  return `${m} 分 ${s} 秒`;
}

export function formatScore(score?: number | null, digits = 1): string {
  if (score == null || !Number.isFinite(score)) return "—";
  return score.toFixed(digits);
}

// scoreTone maps a 0..100 score to a semantic tone used for colors.
export function scoreTone(score?: number | null): "good" | "mid" | "low" | "none" {
  if (score == null || !Number.isFinite(score)) return "none";
  if (score >= 80) return "good";
  if (score >= 60) return "mid";
  return "low";
}

export function scoreToneClass(score?: number | null): string {
  switch (scoreTone(score)) {
    case "good":
      return "text-emerald-600 dark:text-emerald-400";
    case "mid":
      return "text-amber-600 dark:text-amber-400";
    case "low":
      return "text-destructive";
    default:
      return "text-muted-foreground";
  }
}

// elapsedSeconds measures from an ISO start timestamp to now (or to `end`).
export function elapsedSeconds(start?: string | null, end?: string | null): number | null {
  if (!start) return null;
  const s = new Date(start).getTime();
  if (Number.isNaN(s)) return null;
  const e = end ? new Date(end).getTime() : Date.now();
  if (Number.isNaN(e)) return null;
  return Math.max(0, Math.round((e - s) / 1000));
}
