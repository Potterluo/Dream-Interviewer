"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { ArrowRight, Sparkles } from "lucide-react";
import { subscribeEvents } from "@/lib/api";
import {
  getInterviewStats,
  labelFor,
  listInterviews,
  type Interview,
  type InterviewStats,
} from "@/lib/api/interviews";
import { CHART_TOOLTIP, useChartColors } from "@/components/chart-colors";
import { RecommendationBadge, ScoreText, StatusBadge } from "@/components/interview-ui";
import { useUser, isAdmin } from "@/components/user-context";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { formatDateTime, formatScore } from "@/lib/format";

// Dashboard: the AI interviewer landing page. Reads GET /api/interview/stats
// for the aggregate cards + dimension chart and GET /api/interviews for the
// recent list; interview.* events keep it fresh while it is open.

export default function DashboardPage() {
  const { user } = useUser();
  const CHART = useChartColors();
  const [stats, setStats] = useState<InterviewStats | null>(null);
  const [recent, setRecent] = useState<Interview[] | null>(null);
  const [error, setError] = useState("");

  const reload = useCallback(async () => {
    const all = isAdmin(user);
    try {
      const [statsRes, listRes] = await Promise.all([
        getInterviewStats(all),
        listInterviews(all),
      ]);
      if (statsRes.ok && statsRes.stats) setStats(statsRes.stats);
      else setError(statsRes.error || "无法加载统计数据");
      if (listRes.ok) setRecent(listRes.interviews ?? []);
    } catch {
      setError("加载失败，请稍后重试。");
    }
  }, [user]);

  useEffect(() => {
    // 初次加载放进 async 回调：setState 发生在 await 之后
    void (async () => {
      await reload();
    })();
  }, [reload]);

  useEffect(() => subscribeEvents((evt) => {
    if (evt.type.startsWith("interview.")) reload();
  }), [reload]);

  const dimensionData = (stats?.dimensionAvg ?? []).map((d) => ({
    label: d.label,
    score: Math.round(d.score * 10) / 10,
  }));

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="font-heading text-2xl font-semibold">
            你好，{user?.displayName || user?.username}
          </h1>
          <p className="text-sm text-muted-foreground">
            让 AI 面试官陪你练习：生成面试计划、逐题追问、评分并给出提升建议。
          </p>
        </div>
        <Button size="lg" render={<Link href="/interviews/new/" />}>
          <Sparkles className="h-4 w-4" /> 开始一场面试
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {/* 统计卡片 */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="总面试数" value={stats ? String(stats.total) : undefined} hint="含草稿与已中止" />
        <StatCard
          label="已完成"
          value={stats ? String(stats.completed) : undefined}
          hint={stats ? `进行中 ${stats.inProgress} · 已中止 ${stats.aborted}` : undefined}
        />
        <StatCard
          label="平均分"
          value={stats ? formatScore(stats.avgScore) : undefined}
          hint={stats ? `平均 ${stats.avgTurns.toFixed(1)} 轮 · 平均 ${Math.round(stats.avgDurationSec / 60)} 分钟` : undefined}
        />
        <StatCard
          label="最高分"
          value={stats ? formatScore(stats.bestScore) : undefined}
          hint="历史最佳总体得分"
        />
      </div>

      <div className="grid gap-4 lg:grid-cols-5">
        {/* 最近面试 */}
        <Card className="lg:col-span-3">
          <CardHeader>
            <CardTitle>最近面试</CardTitle>
            <CardDescription>最近创建的 5 场面试</CardDescription>
          </CardHeader>
          <CardContent>
            {recent === null ? (
              <div className="grid gap-2">
                <Skeleton className="h-12 w-full" />
                <Skeleton className="h-12 w-full" />
                <Skeleton className="h-12 w-full" />
              </div>
            ) : recent.length === 0 ? (
              <div className="flex flex-col items-center gap-3 py-12 text-center">
                <p className="text-sm text-muted-foreground">还没有面试记录，先来一场练练手吧。</p>
                <Button render={<Link href="/interviews/new/" />}>
                  <Sparkles className="h-4 w-4" /> 开始一场面试
                </Button>
              </div>
            ) : (
              <ul className="divide-y">
                {recent.slice(0, 5).map((iv) => (
                  <li key={iv.id}>
                    <Link
                      href={`/interview/?id=${encodeURIComponent(iv.id)}`}
                      className="flex items-center gap-3 py-3 transition-colors hover:bg-accent/40"
                    >
                      <div className="min-w-0 flex-1">
                        <p className="truncate font-medium">{iv.title || iv.role}</p>
                        <p className="truncate text-xs text-muted-foreground">
                          {labelFor("level", iv.level)} · {labelFor("type", iv.interviewType)} ·{" "}
                          {iv.turnCount} 轮 · {formatDateTime(iv.createdAt)}
                        </p>
                      </div>
                      <StatusBadge status={iv.status} />
                      {iv.recommendation ? (
                        <RecommendationBadge recommendation={iv.recommendation} />
                      ) : null}
                      <ScoreText score={iv.overallScore} className="w-12 text-right text-sm" />
                      <ArrowRight className="h-4 w-4 shrink-0 text-muted-foreground" />
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </CardContent>
        </Card>

        {/* 维度平均分 */}
        <Card className="lg:col-span-2">
          <CardHeader>
            <CardTitle>能力维度</CardTitle>
            <CardDescription>全部已完成面试的各维度平均分</CardDescription>
          </CardHeader>
          <CardContent className="h-72">
            {stats === null ? (
              <Skeleton className="h-full w-full" />
            ) : dimensionData.length === 0 ? (
              <p className="flex h-full items-center justify-center text-sm text-muted-foreground">
                完成一场面试后这里会出现维度分析。
              </p>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={dimensionData} layout="vertical" margin={{ left: 8, right: 16 }}>
                  <CartesianGrid stroke={CHART.grid} strokeDasharray="3 3" horizontal={false} />
                  <XAxis
                    type="number"
                    domain={[0, 100]}
                    tick={{ fill: CHART.axis, fontSize: 11 }}
                    tickLine={false}
                    axisLine={false}
                  />
                  <YAxis
                    type="category"
                    dataKey="label"
                    width={92}
                    tick={{ fill: CHART.axis, fontSize: 11 }}
                    tickLine={false}
                    axisLine={false}
                  />
                  <Tooltip cursor={{ fill: "var(--muted)" }} contentStyle={CHART_TOOLTIP} />
                  <Bar dataKey="score" name="平均分" fill={CHART.c1} radius={[0, 4, 4, 0]} barSize={14} />
                </BarChart>
              </ResponsiveContainer>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

function StatCard({ label, value, hint }: { label: string; value?: string; hint?: string }) {
  return (
    <Card>
      <CardHeader>
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-3xl tabular-nums">{value ?? "…"}</CardTitle>
      </CardHeader>
      <CardContent className="text-xs text-muted-foreground">{hint ?? "\u00a0"}</CardContent>
    </Card>
  );
}
