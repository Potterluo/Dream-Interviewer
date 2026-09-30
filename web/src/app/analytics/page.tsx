"use client";

import { useEffect, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  getInterviewStats,
  labelFor,
  type InterviewStats,
} from "@/lib/api/interviews";
import { CHART_TOOLTIP, useChartColors } from "@/components/chart-colors";
import { DimensionRadar } from "@/components/interview-ui";
import { useUser, isAdmin } from "@/components/user-context";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { formatDuration, formatScore } from "@/lib/format";

// 分析页：全部图表来自 GET /api/interview/stats —— 得分趋势（折线）、
// 维度雷达、岗位分布（柱状）、录用建议分布（饼图）。图表颜色在 JS 里从
// 主题 token 解析（SVG 属性不支持 var()）。
//
// 模板原有的 items 图表随 Items 领域一起删除了，这里替换成面试分析。

export default function AnalyticsPage() {
  const { user } = useUser();
  const CHART = useChartColors();
  const [stats, setStats] = useState<InterviewStats | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const res = await getInterviewStats(isAdmin(user));
        if (res.ok && res.stats) setStats(res.stats);
        else setError(res.error || "无法加载统计数据");
      } catch {
        setError("加载失败，请稍后重试。");
      }
    })();
  }, [user]);

  const trend = stats?.avgScoreByDay ?? [];
  const roles = stats?.roleBreakdown ?? [];
  const recommendations = (stats?.recommendationBreakdown ?? []).map((r) => ({
    name: labelFor("recommendation", r.label),
    value: r.count,
  }));
  const dimensions = (stats?.dimensionAvg ?? []).map((d) => ({
    label: d.label,
    score: d.score,
  }));

  return (
    <div className="grid gap-6">
      <div>
        <h1 className="font-heading text-2xl font-semibold">分析</h1>
        <p className="text-sm text-muted-foreground">
          {isAdmin(user) ? "全部用户的面试数据。" : "你的面试数据。"} 数据来自{" "}
          <code className="rounded bg-muted px-1 py-0.5 text-xs">GET /api/interview/stats</code>。
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <StatCard label="总面试数" value={stats ? String(stats.total) : undefined} hint={`已完成 ${stats?.completed ?? 0}`} />
        <StatCard label="进行中" value={stats ? String(stats.inProgress) : undefined} hint={`已中止 ${stats?.aborted ?? 0}`} />
        <StatCard label="平均分" value={stats ? formatScore(stats.avgScore) : undefined} hint={`最高 ${stats ? formatScore(stats.bestScore) : "…"}`} />
        <StatCard label="平均轮次" value={stats ? stats.avgTurns.toFixed(1) : undefined} hint="每场面试的问答轮数" />
        <StatCard label="平均时长" value={stats ? formatDuration(stats.avgDurationSec) : undefined} hint="从开始到完成" />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>得分趋势</CardTitle>
          <CardDescription>按天统计的平均总分（满分 100）</CardDescription>
        </CardHeader>
        <CardContent className="h-64">
          {stats === null ? (
            <Skeleton className="h-full w-full" />
          ) : trend.length === 0 ? (
            <Empty text="还没有完成的面试。" />
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={trend} margin={{ left: -20, right: 8 }}>
                <CartesianGrid stroke={CHART.grid} strokeDasharray="3 3" vertical={false} />
                <XAxis
                  dataKey="date"
                  tick={{ fill: CHART.axis, fontSize: 11 }}
                  tickFormatter={(d: string) => d.slice(5)}
                  tickLine={false}
                  axisLine={false}
                  minTickGap={24}
                />
                <YAxis
                  domain={[0, 100]}
                  tick={{ fill: CHART.axis, fontSize: 11 }}
                  tickLine={false}
                  axisLine={false}
                />
                <Tooltip contentStyle={CHART_TOOLTIP} />
                <Line
                  type="monotone"
                  dataKey="count"
                  name="平均分"
                  stroke={CHART.c1}
                  strokeWidth={2}
                  dot={{ r: 3, fill: CHART.c1 }}
                  activeDot={{ r: 5 }}
                />
              </LineChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>能力维度</CardTitle>
            <CardDescription>各维度平均分</CardDescription>
          </CardHeader>
          <CardContent>
            {stats === null ? (
              <Skeleton className="h-64 w-full" />
            ) : dimensions.length === 0 ? (
              <Empty text="完成一场面试后会出现维度分析。" />
            ) : (
              <DimensionRadar data={dimensions} height={280} />
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>岗位分布</CardTitle>
            <CardDescription>面试数量按岗位</CardDescription>
          </CardHeader>
          <CardContent className="h-72">
            {stats === null ? (
              <Skeleton className="h-full w-full" />
            ) : roles.length === 0 ? (
              <Empty text="还没有面试记录。" />
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={roles} margin={{ left: -20, right: 8 }}>
                  <CartesianGrid stroke={CHART.grid} strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="label"
                    tick={{ fill: CHART.axis, fontSize: 11 }}
                    tickLine={false}
                    axisLine={false}
                    interval={0}
                    height={48}
                    angle={-15}
                    textAnchor="end"
                  />
                  <YAxis
                    allowDecimals={false}
                    tick={{ fill: CHART.axis, fontSize: 11 }}
                    tickLine={false}
                    axisLine={false}
                  />
                  <Tooltip cursor={{ fill: "var(--muted)" }} contentStyle={CHART_TOOLTIP} />
                  <Bar dataKey="count" name="面试数" fill={CHART.c2} radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>录用建议分布</CardTitle>
          <CardDescription>已生成报告的面试按建议分类</CardDescription>
        </CardHeader>
        <CardContent className="h-72">
          {stats === null ? (
            <Skeleton className="h-full w-full" />
          ) : recommendations.length === 0 ? (
            <Empty text="还没有生成报告的面试。" />
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={recommendations}
                  dataKey="value"
                  nameKey="name"
                  innerRadius={60}
                  outerRadius={96}
                  paddingAngle={3}
                >
                  {recommendations.map((entry, i) => (
                    <Cell
                      key={entry.name}
                      fill={[CHART.c1, CHART.c2, CHART.c3, CHART.c4, CHART.c5][i % 5]}
                      stroke="none"
                    />
                  ))}
                </Pie>
                <Tooltip contentStyle={CHART_TOOLTIP} />
              </PieChart>
            </ResponsiveContainer>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

function StatCard({ label, value, hint }: { label: string; value?: string; hint?: string }) {
  return (
    <Card>
      <CardHeader>
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-2xl tabular-nums">{value ?? "…"}</CardTitle>
      </CardHeader>
      <CardContent className="text-xs text-muted-foreground">{hint ?? "\u00a0"}</CardContent>
    </Card>
  );
}

function Empty({ text }: { text: string }) {
  return (
    <p className="flex h-full items-center justify-center text-sm text-muted-foreground">
      {text}
    </p>
  );
}
