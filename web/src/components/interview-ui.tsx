"use client";

import {
  PolarAngleAxis,
  PolarGrid,
  PolarRadiusAxis,
  Radar,
  RadarChart,
  ResponsiveContainer,
  Tooltip,
} from "recharts";
import { Badge } from "@/components/ui/badge";
import { CHART_TOOLTIP, useChartColors } from "@/components/chart-colors";
import { labelFor, type Dimension, type PresetLabels } from "@/lib/api/interviews";
import { cn } from "@/lib/utils";
import { formatScore, scoreToneClass } from "@/lib/format";

// Shared interview visuals: score ring, score pills, dimension radar/bars
// and the status/verdict/recommendation badges. Reused by the dashboard,
// the interview room report and the analytics page so the three stay
// consistent.

// --- Badges -----------------------------------------------------------------

const STATUS_VARIANT: Record<string, "default" | "secondary" | "destructive" | "outline"> = {
  draft: "outline",
  in_progress: "secondary",
  completed: "default",
  aborted: "destructive",
};

export function StatusBadge({
  status,
  labels,
  className,
}: {
  status?: string | null;
  labels?: PresetLabels | null;
  className?: string;
}) {
  const key = status ?? "";
  return (
    <Badge variant={STATUS_VARIANT[key] ?? "outline"} className={className}>
      {labelFor("status", status, labels)}
    </Badge>
  );
}

const RECOMMENDATION_CLASS: Record<string, string> = {
  strong_hire: "border-transparent bg-emerald-600/15 text-emerald-700 dark:text-emerald-400",
  hire: "border-transparent bg-emerald-600/10 text-emerald-700 dark:text-emerald-400",
  maybe: "border-transparent bg-amber-500/15 text-amber-700 dark:text-amber-400",
  no_hire: "border-transparent bg-destructive/15 text-destructive",
};

export function RecommendationBadge({
  recommendation,
  labels,
}: {
  recommendation?: string | null;
  labels?: PresetLabels | null;
}) {
  const key = recommendation ?? "";
  return (
    <span
      className={cn(
        "inline-flex h-6 items-center rounded-full px-2.5 text-xs font-medium",
        RECOMMENDATION_CLASS[key] ?? "bg-muted text-muted-foreground"
      )}
    >
      {labelFor("recommendation", recommendation, labels)}
    </span>
  );
}

const VERDICT_CLASS: Record<string, string> = {
  strong: "border-transparent bg-emerald-600/15 text-emerald-700 dark:text-emerald-400",
  ok: "border-transparent bg-muted text-muted-foreground",
  weak: "border-transparent bg-destructive/15 text-destructive",
};

const VERDICT_LABEL: Record<string, string> = {
  strong: "表现出色",
  ok: "基本合格",
  weak: "有待提升",
};

export function VerdictBadge({ verdict }: { verdict?: string | null }) {
  if (!verdict) return null;
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center rounded-full px-2 text-xs font-medium",
        VERDICT_CLASS[verdict] ?? "bg-muted text-muted-foreground"
      )}
    >
      {VERDICT_LABEL[verdict] ?? verdict}
    </span>
  );
}

// --- Scores -----------------------------------------------------------------

export function ScoreText({
  score,
  digits = 1,
  className,
}: {
  score?: number | null;
  digits?: number;
  className?: string;
}) {
  return (
    <span className={cn("font-medium tabular-nums", scoreToneClass(score), className)}>
      {formatScore(score, digits)}
    </span>
  );
}

const RING_RADIUS_STROKE = 10;

// ScoreRing is a pure SVG ring; stroke uses Tailwind's stroke-* theme
// utilities (currentColor based), so it follows the accent preset too.
export function ScoreRing({
  score,
  caption,
  size = 148,
}: {
  score?: number | null;
  caption?: string;
  size?: number;
}) {
  const stroke = RING_RADIUS_STROKE;
  const r = (size - stroke) / 2;
  const circumference = 2 * Math.PI * r;
  const pct = Math.max(0, Math.min(100, score ?? 0));
  const offset = circumference - (pct / 100) * circumference;
  return (
    <div className="relative inline-flex shrink-0 items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} className="-rotate-90">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          strokeWidth={stroke}
          className="stroke-muted"
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          strokeWidth={stroke}
          strokeLinecap="round"
          strokeDasharray={circumference}
          strokeDashoffset={score == null ? circumference : offset}
          className="stroke-primary transition-[stroke-dashoffset] duration-700"
        />
      </svg>
      <div className="absolute flex flex-col items-center">
        <span className="font-heading text-3xl font-semibold tabular-nums">
          {score == null ? "—" : formatScore(score, 1)}
        </span>
        {caption && <span className="text-xs text-muted-foreground">{caption}</span>}
      </div>
    </div>
  );
}

// DimensionChips renders the per-dimension scores of a single Grade.
export function DimensionChips({
  dimensions,
  plan,
}: {
  dimensions?: Record<string, number> | null;
  plan?: Dimension[] | null;
}) {
  const entries = Object.entries(dimensions ?? {});
  if (entries.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-1.5">
      {entries.map(([key, value]) => {
        const label = plan?.find((d) => d.key === key)?.label ?? key;
        return (
          <span
            key={key}
            className="inline-flex items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground"
          >
            {label}
            <span className={cn("font-medium tabular-nums", scoreToneClass(value))}>{formatScore(value, 0)}</span>
          </span>
        );
      })}
    </div>
  );
}

// --- Charts -----------------------------------------------------------------

export interface DimensionDatum {
  label: string;
  score: number;
  hint?: string;
}

export function DimensionBars({ data }: { data: DimensionDatum[] }) {
  if (data.length === 0) {
    return <p className="py-6 text-center text-sm text-muted-foreground">暂无维度数据</p>;
  }
  return (
    <ul className="grid gap-3">
      {data.map((d) => {
        const pct = Math.max(0, Math.min(100, d.score));
        return (
          <li key={d.label} className="grid gap-1.5">
            <div className="flex items-baseline justify-between gap-2 text-sm">
              <span className="font-medium">{d.label}</span>
              <span className="tabular-nums text-muted-foreground">
                {d.hint ? `${d.hint} · ` : ""}
                <span className={scoreToneClass(d.score)}>{formatScore(d.score, 1)}</span>
              </span>
            </div>
            <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
              <div
                className="h-full rounded-full bg-primary transition-[width] duration-700"
                style={{ width: `${pct}%` }}
              />
            </div>
          </li>
        );
      })}
    </ul>
  );
}

// DimensionRadar needs at least three axes to read as a radar; with fewer
// points it falls back to bars.
export function DimensionRadar({
  data,
  height = 280,
}: {
  data: DimensionDatum[];
  height?: number;
}) {
  const CHART = useChartColors();
  if (data.length < 3) return <DimensionBars data={data} />;
  return (
    <div style={{ height }} className="w-full">
      <ResponsiveContainer width="100%" height="100%">
        <RadarChart data={data} outerRadius="72%">
          <PolarGrid stroke={CHART.grid} />
          <PolarAngleAxis dataKey="label" tick={{ fill: CHART.axis, fontSize: 11 }} />
          <PolarRadiusAxis
            domain={[0, 100]}
            tick={{ fill: CHART.axis, fontSize: 10 }}
            axisLine={false}
            tickCount={5}
          />
          <Radar
            name="维度得分"
            dataKey="score"
            stroke={CHART.c1}
            fill={CHART.c1}
            fillOpacity={0.3}
            strokeWidth={2}
          />
          <Tooltip contentStyle={CHART_TOOLTIP} />
        </RadarChart>
      </ResponsiveContainer>
    </div>
  );
}
