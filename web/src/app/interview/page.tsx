"use client";

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  AlertTriangle,
  ArrowLeft,
  CheckCircle2,
  Clock,
  Download,
  FileText,
  Lightbulb,
  ListChecks,
  Loader2,
  Play,
  RotateCw,
  Send,
  SkipForward,
  Square,
  Target,
  XCircle,
} from "lucide-react";
import {
  advanceInterview,
  getInterview,
  getInterviewPresets,
  interviewExportUrl,
  labelFor,
  type AdvanceAction,
  type Interview,
  type InterviewFrame,
  type InterviewTurn,
  type Plan,
  type PresetLabels,
  type Report,
  type Stage,
} from "@/lib/api/interviews";
import { Markdown } from "@/components/markdown";
import {
  DimensionBars,
  DimensionChips,
  DimensionRadar,
  RecommendationBadge,
  ScoreRing,
  ScoreText,
  StatusBadge,
  VerdictBadge,
} from "@/components/interview-ui";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { elapsedSeconds, formatDateTime, formatDuration } from "@/lib/format";

// 面试房间 —— /interview/?id=…（静态导出，所以用查询参数而不是 [id] 路由）。
//
// 整个房间由 POST /api/interviews/{id}/advance 的 SSE 帧驱动：
//   stage   新阶段开始 → 清空流式缓冲
//   delta   当前阶段的 Markdown 增量 → 追加
//   plan    面试计划
//   grade   本回答已评分并落库
//   question 下一题
//   report  最终报告
//   done    终止帧：interview + turns 是权威状态，直接替换本地累积
//   error   阶段失败
// Stop 按钮通过 AbortSignal 取消请求，服务端随即停止模型调用。

export default function InterviewRoomPage() {
  return (
    <Suspense fallback={<RoomSkeleton />}>
      <InterviewRoom />
    </Suspense>
  );
}

function InterviewRoom() {
  const searchParams = useSearchParams();
  const id = searchParams.get("id") ?? "";

  const [interview, setInterview] = useState<Interview | null>(null);
  const [turns, setTurns] = useState<InterviewTurn[]>([]);
  const [labels, setLabels] = useState<PresetLabels | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [stage, setStage] = useState<Stage | null>(null);
  const [streamText, setStreamText] = useState("");
  const [liveReport, setLiveReport] = useState<Report | null>(null);
  const [answer, setAnswer] = useState("");
  const [tickElapsed, setTickElapsed] = useState<number | null>(null);
  const [confirmAbort, setConfirmAbort] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  // --- data ---------------------------------------------------------------

  const reload = useCallback(async () => {
    if (!id) {
      setLoading(false);
      setError("缺少面试 ID，请从面试记录进入。");
      return;
    }
    try {
      const res = await getInterview(id);
      if (res.ok && res.interview) {
        setInterview(res.interview);
        setTurns(res.turns ?? []);
        setError("");
      } else {
        setError(res.error || "无法加载该面试");
      }
    } catch {
      setError("加载失败，请稍后重试。");
    }
    setLoading(false);
  }, [id]);

  useEffect(() => {
    // 初次加载放进 async 回调：setState 发生在 await 之后
    void (async () => {
      await reload();
    })();
  }, [reload]);

  // 中文标签优先用服务端 labels；失败时 labelFor 会回落到内置中文表。
  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const res = await getInterviewPresets();
        if (alive && res.ok && res.labels) setLabels(res.labels);
      } catch {
        // 忽略：labelFor 自带中文回落表
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  // 进行中每秒刷新计时器；结束/中止后用服务端记录的时长（渲染期推导，
  // 不在 effect 体内同步 setState）。
  const status = interview?.status ?? "";
  const startedAt = interview?.startedAt ?? null;
  const completedAt = interview?.completedAt ?? null;
  const durationSec = interview?.durationSec ?? null;
  useEffect(() => {
    if (status !== "in_progress") return;
    const timer = window.setInterval(() => {
      setTickElapsed(elapsedSeconds(startedAt, completedAt));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [status, startedAt, completedAt]);
  const elapsed =
    status === "in_progress"
      ? tickElapsed ?? elapsedSeconds(startedAt, null)
      : durationSec ?? (startedAt ? elapsedSeconds(startedAt, completedAt) : null);

  // --- streaming ----------------------------------------------------------

  const upsertTurn = useCallback((turn: InterviewTurn) => {
    setTurns((list) => {
      const i = list.findIndex((t) => t.id === turn.id || t.seq === turn.seq);
      if (i < 0) return [...list, turn];
      const next = [...list];
      next[i] = turn;
      return next;
    });
  }, []);

  const applyFrame = useCallback(
    (frame: InterviewFrame) => {
      switch (frame.type) {
        case "stage":
          // 新阶段：delta 是「当前阶段」的正文，必须清空缓冲。
          setStage(frame.data);
          setStreamText("");
          break;
        case "delta":
          setStreamText((t) => t + frame.data);
          break;
        case "plan":
          setInterview((iv) => (iv ? { ...iv, plan: frame.data } : iv));
          break;
        case "grade":
        case "question":
          upsertTurn(frame.data.turn);
          break;
        case "report":
          setLiveReport(frame.data);
          break;
        case "done":
          // 终止帧是权威状态：替换本地累积的一切。
          setInterview(frame.data.interview);
          setTurns(frame.data.turns ?? []);
          setLiveReport(null);
          setStreamText("");
          setStage(null);
          break;
        case "error":
          break;
      }
    },
    [upsertTurn]
  );

  const runAdvance = useCallback(
    async (action: AdvanceAction, body?: { answer?: string }) => {
      if (!id || busy) return;
      // 作答耗时取服务端在出题时写入的 createdAt，而不是本地计时器：
      // 刷新页面或换设备后仍然准确。同时把提交绑定到这道题，迟到的重试
      // 会被服务端拒绝，而不是被算到下一道题上。
      let elapsedSec: number | undefined;
      let turnId: string | undefined;
      if (action === "answer" || action === "skip") {
        const pending = turns.find((t) => !t.answeredAt);
        if (pending) {
          turnId = pending.id;
          if (action === "answer") {
            const started = pending.createdAt ? Date.parse(pending.createdAt) : NaN;
            if (!Number.isNaN(started)) {
              elapsedSec = Math.max(0, Math.min(86400, Math.round((Date.now() - started) / 1000)));
            }
          }
        }
      }
      const controller = new AbortController();
      abortRef.current = controller;
      setBusy(true);
      setNotice("");
      setStage(null);
      setStreamText("");
      let failed = false;
      try {
        await advanceInterview(
          id,
          { action, answer: body?.answer, elapsedSec, turnId },
          (frame) => {
            if (frame.type === "error") {
              failed = true;
              setNotice(typeof frame.data === "string" && frame.data ? frame.data : "生成失败，请重试。");
              return;
            }
            applyFrame(frame);
          },
          controller.signal
        );
      } catch (err) {
        failed = true;
        if (!controller.signal.aborted) {
          setNotice((err as Error).message || "请求失败，请重试。");
        }
      } finally {
        abortRef.current = null;
        setBusy(false);
        setStage(null);
        setStreamText("");
        // 无论成功、失败还是被 Stop 中止，都回读一次权威状态
        // （中止时服务端可能已经落库了部分结果）。
        await reload();
        if (action === "answer" && !failed) setAnswer("");
      }
    },
    [id, busy, applyFrame, reload, turns]
  );

  const stop = useCallback(() => {
    abortRef.current?.abort();
  }, []);

  // --- derived state ------------------------------------------------------

  const answeredTurns = useMemo(() => turns.filter(isAnswered), [turns]);

  const currentTurn = useMemo(() => {
    if (!interview) return null;
    const bySeq = turns.find((t) => t.seq === interview.currentSeq && !isAnswered(t));
    if (bySeq) return bySeq;
    return [...turns].reverse().find((t) => !isAnswered(t)) ?? null;
  }, [turns, interview]);

  const report = interview?.report ?? liveReport;

  // --- render -------------------------------------------------------------

  if (loading) return <RoomSkeleton />;

  if (error || !interview) {
    return (
      <div className="grid gap-4">
        <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href="/interviews/" />}>
          <ArrowLeft className="h-4 w-4" /> 返回面试记录
        </Button>
        <Card>
          <CardContent className="flex flex-col items-center gap-3 py-14 text-center pt-6">
            <AlertTriangle className="h-6 w-6 text-destructive" />
            <p className="text-sm text-muted-foreground">{error || "面试不存在或无权访问。"}</p>
            <Button render={<Link href="/interviews/" />}>返回面试记录</Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  const plan = interview.plan ?? null;
  const isCompleted = interview.status === "completed";
  const isAborted = interview.status === "aborted";

  return (
    <div className="grid gap-6">
      {/* 标题栏 */}
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <Button
            variant="ghost"
            size="sm"
            className="mb-1 -ml-2"
            render={<Link href="/interviews/" />}
          >
            <ArrowLeft className="h-4 w-4" /> 面试记录
          </Button>
          <h1 className="truncate font-heading text-2xl font-semibold">
            {interview.title || interview.role}
          </h1>
          <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <StatusBadge status={interview.status} labels={labels} />
            <Badge variant="outline">{labelFor("level", interview.level, labels)}</Badge>
            <Badge variant="outline">{labelFor("type", interview.interviewType, labels)}</Badge>
            <Badge variant="outline">{labelFor("difficulty", interview.difficulty, labels)}</Badge>
            <span>{interview.role}</span>
            <span className="inline-flex items-center gap-1">
              <Clock className="h-3.5 w-3.5" /> {formatDuration(elapsed)}
            </span>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          {isCompleted && (
            <Button variant="outline" render={<a href={interviewExportUrl(interview.id)} download />}>
              <Download className="h-4 w-4" /> 导出 Markdown
            </Button>
          )}
          {isCompleted && (
            <Button variant="outline" onClick={() => runAdvance("finish")} disabled={busy}>
              <RotateCw className="h-4 w-4" /> 重新生成报告
            </Button>
          )}
        </div>
      </div>

      {/* 阶段/错误提示 */}
      {notice && (
        <div className="flex items-start gap-2 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <span>{notice}</span>
        </div>
      )}

      {busy && <StreamingPanel stage={stage} text={streamText} onStop={stop} />}

      {/* 草稿：开始 */}
      {interview.status === "draft" && (
        <Card>
          <CardHeader>
            <CardTitle>准备开始</CardTitle>
            <CardDescription>
              AI 面试官会生成本场面试的评估维度与题目计划，然后逐题提问、追问并评分。
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <InfoTile label="岗位" value={interview.role} />
              <InfoTile label="级别" value={labelFor("level", interview.level, labels)} />
              <InfoTile label="题目数量" value={`${interview.questionCount} 题`} />
              <InfoTile
                label="语言"
                value={labelFor("language", interview.language, labels)}
              />
            </div>
            {(interview.jdText || interview.resumeText) && (
              <div className="grid gap-4 sm:grid-cols-2">
                {interview.jdText && <Materials title="职位描述" text={interview.jdText} />}
                {interview.resumeText && <Materials title="简历 / 自我介绍" text={interview.resumeText} />}
              </div>
            )}
            <div>
              <Button size="lg" onClick={() => runAdvance("start")} disabled={busy}>
                {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
                {busy ? "正在准备…" : "开始面试"}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}

      {/* 进行中：答题 */}
      {interview.status === "in_progress" && (
        <div className="grid gap-4 lg:grid-cols-3">
          <div className="grid gap-4 lg:col-span-2">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Target className="h-4 w-4" /> 当前题目
                  <span className="text-sm font-normal text-muted-foreground">
                    第 {currentTurn?.seq ?? interview.currentSeq} / {interview.questionCount} 题
                  </span>
                </CardTitle>
                {currentTurn && (
                  <CardDescription>
                    {dimensionLabel(plan, currentTurn.dimension)}
                    {currentTurn.intent ? ` · ${currentTurn.intent}` : ""}
                  </CardDescription>
                )}
              </CardHeader>
              <CardContent className="grid gap-4">
                {currentTurn ? (
                  <p className="whitespace-pre-wrap text-base leading-relaxed">
                    {currentTurn.question}
                  </p>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    题目已全部作答，点击「结束并生成报告」查看结果。
                  </p>
                )}

                <Textarea
                  rows={7}
                  value={answer}
                  onChange={(e) => setAnswer(e.target.value)}
                  placeholder="在这里作答 —— 思路、取舍、具体实现都可以写；AI 会针对薄弱处追问。"
                  disabled={busy || !currentTurn}
                />

                <div className="flex flex-wrap gap-2">
                  <Button
                    onClick={() => runAdvance("answer", { answer })}
                    disabled={busy || !currentTurn || !answer.trim()}
                  >
                    <Send className="h-4 w-4" /> 提交作答
                  </Button>
                  <Button
                    variant="outline"
                    onClick={() => runAdvance("skip")}
                    disabled={busy || !currentTurn}
                  >
                    <SkipForward className="h-4 w-4" /> 跳过这题
                  </Button>
                  <Button
                    variant="secondary"
                    onClick={() => runAdvance("finish")}
                    disabled={busy}
                  >
                    <CheckCircle2 className="h-4 w-4" /> 结束并生成报告
                  </Button>
                  <Button
                    variant="ghost"
                    onClick={() => setConfirmAbort(true)}
                    disabled={busy}
                  >
                    <XCircle className="h-4 w-4" /> 放弃这场面试
                  </Button>
                </div>

                <AlertDialog open={confirmAbort} onOpenChange={setConfirmAbort}>
                  <AlertDialogContent className="sm:max-w-sm">
                    <AlertDialogHeader>
                      <AlertDialogTitle>放弃这场面试？</AlertDialogTitle>
                      <AlertDialogDescription>
                        已作答的记录会保留，但不会生成评估报告；本场会记为「已放弃」并计入统计。
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>继续面试</AlertDialogCancel>
                      <AlertDialogAction
                        variant="destructive"
                        onClick={() => {
                          setConfirmAbort(false);
                          void runAdvance("abort");
                        }}
                      >
                        确认放弃
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </CardContent>
            </Card>

            <TurnHistory turns={answeredTurns} plan={plan} title="已回答的问题" />
          </div>

          <div className="grid gap-4">
            <ProgressCard interview={interview} turns={turns} plan={plan} />
            <PlanCard plan={plan} />
          </div>
        </div>
      )}

      {/* 已中止 */}
      {isAborted && (
        <Card>
          <CardHeader>
            <CardTitle>面试已中止</CardTitle>
            <CardDescription>
              已作答的题目仍然保留。可以重新开始继续作答，也可以直接基于现有记录生成报告。
            </CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <div className="flex flex-wrap gap-2">
              <Button onClick={() => runAdvance("start")} disabled={busy}>
                <Play className="h-4 w-4" /> 重新开始面试
              </Button>
              <Button variant="secondary" onClick={() => runAdvance("finish")} disabled={busy}>
                <FileText className="h-4 w-4" /> 生成报告
              </Button>
            </div>
            <TurnHistory turns={answeredTurns} plan={plan} title="已回答的问题" />
          </CardContent>
        </Card>
      )}

      {/* 已完成：报告 */}
      {isCompleted && (
        <ReportView
          interview={interview}
          report={report}
          turns={answeredTurns}
          plan={plan}
          labels={labels}
          busy={busy}
          onRegenerate={() => runAdvance("finish")}
        />
      )}
    </div>
  );
}

// --- pieces -----------------------------------------------------------------

function isAnswered(t: InterviewTurn): boolean {
  return !!t.answeredAt || (t.answer ?? "").trim().length > 0;
}

function dimensionLabel(plan: Plan | null, key?: string | null): string {
  if (!key) return "综合";
  return plan?.dimensions?.find((d) => d.key === key)?.label ?? key;
}

const STAGE_LABEL: Record<string, string> = {
  planning: "正在规划面试题目…",
  questioning: "面试官正在提问…",
  grading: "正在评估你的回答…",
  reporting: "正在生成面试报告…",
};

function StreamingPanel({
  stage,
  text,
  onStop,
}: {
  stage: Stage | null;
  text: string;
  onStop: () => void;
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Loader2 className="h-4 w-4 animate-spin text-primary" />
          {stage ? STAGE_LABEL[stage] ?? "正在生成…" : "正在生成…"}
        </CardTitle>
        <CardDescription>模型输出实时流式返回，可随时停止。</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-3">
        {text ? (
          <Markdown streaming>{text}</Markdown>
        ) : (
          <div className="grid gap-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        )}
        <div>
          <Button variant="secondary" onClick={onStop}>
            <Square className="h-4 w-4" /> 停止
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

function InfoTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="truncate text-sm font-medium" title={value}>
        {value}
      </p>
    </div>
  );
}

function Materials({ title, text }: { title: string; text: string }) {
  return (
    <div className="grid gap-1.5">
      <p className="text-xs text-muted-foreground">{title}</p>
      <div className="max-h-40 overflow-y-auto rounded-lg border bg-muted/30 p-3">
        <p className="whitespace-pre-wrap text-xs leading-relaxed text-muted-foreground">{text}</p>
      </div>
    </div>
  );
}

function ProgressCard({
  interview,
  turns,
  plan,
}: {
  interview: Interview;
  turns: InterviewTurn[];
  plan: Plan | null;
}) {
  const answered = turns.filter(isAnswered).length;
  const scored = turns.filter((t) => isAnswered(t) && t.score != null);
  const avg = scored.length
    ? scored.reduce((sum, t) => sum + (t.score ?? 0), 0) / scored.length
    : null;
  const pct = interview.questionCount
    ? Math.min(100, Math.round((answered / interview.questionCount) * 100))
    : 0;
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ListChecks className="h-4 w-4" /> 面试进度
        </CardTitle>
        <CardDescription>
          已作答 {answered} / {interview.questionCount} 题
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
          <div className="h-full rounded-full bg-primary transition-[width] duration-500" style={{ width: `${pct}%` }} />
        </div>
        <div className="grid grid-cols-2 gap-3 text-sm">
          <div>
            <p className="text-xs text-muted-foreground">当前平均分</p>
            <p className="font-medium">
              <ScoreText score={avg} />
            </p>
          </div>
          <div>
            <p className="text-xs text-muted-foreground">模型</p>
            <p className="truncate font-mono text-xs" title={interview.model}>
              {interview.model || "—"}
            </p>
          </div>
        </div>
        {plan?.dimensions && plan.dimensions.length > 0 && (
          <div className="grid gap-2">
            <p className="text-xs text-muted-foreground">评估维度</p>
            <DimensionBars
              data={plan.dimensions.map((d) => ({
                label: d.label,
                // 计划阶段只有权重，用权重占位展示，避免空图表。
                score: Math.round(d.weight * 100),
                hint: `权重 ${Math.round(d.weight * 100)}%`,
              }))}
            />
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function PlanCard({ plan }: { plan: Plan | null }) {
  if (!plan || (!plan.questions?.length && !plan.rationale)) return null;
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Lightbulb className="h-4 w-4" /> 面试计划
        </CardTitle>
        {plan.rationale && <CardDescription>{plan.rationale}</CardDescription>}
      </CardHeader>
      <CardContent>
        {plan.questions?.length ? (
          <ol className="grid gap-2 text-sm">
            {plan.questions.map((q) => (
              <li key={`${q.seq}-${q.dimension}`} className="rounded-lg border p-2.5">
                <div className="mb-1 flex items-center gap-2 text-xs text-muted-foreground">
                  <span>第 {q.seq} 题</span>
                  <Badge variant="secondary">{dimensionLabel(plan, q.dimension)}</Badge>
                </div>
                <p className="line-clamp-3">{q.question}</p>
              </li>
            ))}
          </ol>
        ) : (
          <p className="text-sm text-muted-foreground">暂无计划内容。</p>
        )}
      </CardContent>
    </Card>
  );
}

function TurnHistory({
  turns,
  plan,
  title,
}: {
  turns: InterviewTurn[];
  plan: Plan | null;
  title: string;
}) {
  if (turns.length === 0) return null;
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{turns.length} 题</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        {turns.map((t) => {
          const feedback = t.feedback || t.grade?.feedback || "";
          return (
            <div key={t.id} className="grid gap-2 rounded-xl border p-3.5">
              <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                <span className="font-medium text-foreground">第 {t.seq} 题</span>
                <Badge variant="secondary">{dimensionLabel(plan, t.dimension)}</Badge>
                {t.kind === "followup" && <Badge variant="outline">追问</Badge>}
                <VerdictBadge verdict={t.grade?.verdict} />
                <span className="ml-auto">
                  得分 <ScoreText score={t.score} />
                  {t.answerSeconds ? ` · 用时 ${formatDuration(t.answerSeconds)}` : ""}
                </span>
              </div>
              <p className="whitespace-pre-wrap text-sm font-medium">{t.question}</p>
              {t.answer ? (
                <div className="rounded-lg bg-muted/50 p-2.5">
                  <p className="mb-1 text-xs text-muted-foreground">我的回答</p>
                  <p className="whitespace-pre-wrap text-sm">{t.answer}</p>
                </div>
              ) : (
                <p className="text-sm text-muted-foreground">（跳过，未作答）</p>
              )}
              {t.grade?.dimensions && (
                <DimensionChips dimensions={t.grade.dimensions} plan={plan?.dimensions} />
              )}
              {feedback && (
                <div>
                  <p className="mb-1 text-xs text-muted-foreground">点评</p>
                  <Markdown>{feedback}</Markdown>
                </div>
              )}
              {t.grade?.reference && (
                <div className="rounded-lg border border-dashed p-2.5">
                  <p className="mb-1 text-xs text-muted-foreground">参考答案要点</p>
                  <p className="whitespace-pre-wrap text-sm text-muted-foreground">
                    {t.grade.reference}
                  </p>
                </div>
              )}
            </div>
          );
        })}
      </CardContent>
    </Card>
  );
}

function ReportView({
  interview,
  report,
  turns,
  plan,
  labels,
  busy,
  onRegenerate,
}: {
  interview: Interview;
  report: Report | null;
  turns: InterviewTurn[];
  plan: Plan | null;
  labels: PresetLabels | null;
  busy: boolean;
  onRegenerate: () => void;
}) {
  if (!report) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>面试已完成</CardTitle>
          <CardDescription>报告还没有生成，可以现在生成一份。</CardDescription>
        </CardHeader>
        <CardContent>
          <Button onClick={onRegenerate} disabled={busy}>
            {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <RotateCw className="h-4 w-4" />}
            {busy ? "正在生成…" : "生成报告"}
          </Button>
        </CardContent>
      </Card>
    );
  }

  const dims = report.dimensions ?? [];
  return (
    <div className="grid gap-4">
      {/* 总览 */}
      <Card>
        <CardContent className="flex flex-col items-center gap-6 pt-6 sm:flex-row sm:items-start">
          <ScoreRing score={report.overallScore} caption="总体得分" />
          <div className="grid flex-1 gap-3">
            <div className="flex flex-wrap items-center gap-3">
              <RecommendationBadge recommendation={report.recommendation} labels={labels} />
              <span className="text-sm text-muted-foreground">
                {interview.role} · {labelFor("level", interview.level, labels)} ·{" "}
                {turns.length} 题 · 用时 {formatDuration(interview.durationSec ?? 0)}
              </span>
            </div>
            {report.recommendationReason && (
              <p className="text-sm text-muted-foreground">{report.recommendationReason}</p>
            )}
            {interview.hrSatisfaction != null && interview.hrSatisfaction > 0 && (
              <p className="text-sm text-muted-foreground">
                面试官满意度 <ScoreText score={interview.hrSatisfaction} />
                {interview.engine ? ` · 引擎 ${interview.engine}` : ""}
              </p>
            )}
            {report.summary && (
              <div className="rounded-lg border bg-muted/30 p-3">
                <p className="mb-1 text-xs text-muted-foreground">总评</p>
                <Markdown>{report.summary}</Markdown>
              </div>
            )}
            <p className="text-xs text-muted-foreground">
              完成于 {formatDateTime(interview.completedAt)} · 模型{" "}
              <span className="font-mono">{interview.model || "—"}</span>
            </p>
          </div>
        </CardContent>
      </Card>

      {/* 维度 */}
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>能力雷达</CardTitle>
            <CardDescription>各维度得分（满分 100）</CardDescription>
          </CardHeader>
          <CardContent>
            <DimensionRadar data={dims.map((d) => ({ label: d.label, score: d.score }))} />
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>维度明细</CardTitle>
            <CardDescription>得分与权重</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-4">
            <DimensionBars
              data={dims.map((d) => ({
                label: d.label,
                score: d.score,
                hint: d.weight != null ? `权重 ${Math.round(d.weight * 100)}%` : undefined,
              }))}
            />
            {dims.some((d) => d.comment) && (
              <ul className="grid gap-2 text-sm">
                {dims.map(
                  (d) =>
                    d.comment && (
                      <li key={d.key} className="rounded-lg border p-2.5">
                        <p className="text-xs font-medium">{d.label}</p>
                        <p className="text-muted-foreground">{d.comment}</p>
                      </li>
                    )
                )}
              </ul>
            )}
          </CardContent>
        </Card>
      </div>

      {/* 优劣势与建议 */}
      <div className="grid gap-4 lg:grid-cols-3">
        <ListCard title="优势" items={report.strengths} tone="good" />
        <ListCard title="待改进" items={report.weaknesses} tone="bad" />
        <ListCard title="提升建议" items={report.suggestions} tone="neutral" />
      </div>

      {/* 简历改进 + 面试策略：报告里最可执行的两节，直接对应原项目的
          「简历改进建议」和「面试改进策略」。 */}
      {(report.resumeSuggestions?.length ?? 0) > 0 ||
      (report.interviewStrategies?.length ?? 0) > 0 ? (
        <div className="grid gap-4 lg:grid-cols-2">
          {(report.resumeSuggestions?.length ?? 0) > 0 && (
            <ListCard
              title="简历改进建议"
              description="按这些改一版简历，下一场就能用"
              items={report.resumeSuggestions ?? []}
              tone="neutral"
            />
          )}
          {(report.interviewStrategies?.length ?? 0) > 0 && (
            <ListCard
              title="面试策略"
              description="下一次面试可以直接照做"
              items={report.interviewStrategies ?? []}
              tone="neutral"
            />
          )}
        </div>
      ) : null}

      {/* 学习计划 */}
      {report.learningPlan && report.learningPlan.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>学习计划</CardTitle>
            <CardDescription>按优先级补齐短板</CardDescription>
          </CardHeader>
          <CardContent className="grid gap-3">
            {report.learningPlan.map((item, i) => (
              <div key={`${item.topic}-${i}`} className="rounded-lg border p-3">
                <p className="font-medium">{item.topic}</p>
                {item.why && <p className="text-sm text-muted-foreground">为什么：{item.why}</p>}
                {item.how && <p className="text-sm text-muted-foreground">怎么做：{item.how}</p>}
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {/* 亮点与风险 */}
      {(report.highlights?.length ?? 0) > 0 || (report.risks?.length ?? 0) > 0 ? (
        <div className="grid gap-4 lg:grid-cols-2">
          {(report.highlights?.length ?? 0) > 0 && (
            <ListCard title="原话亮点" items={report.highlights} tone="good" quoted />
          )}
          {(report.risks?.length ?? 0) > 0 && (
            <ListCard title="风险点" items={report.risks} tone="bad" />
          )}
        </div>
      ) : null}

      {/* 逐题点评 */}
      <TurnHistory turns={turns} plan={plan} title="逐题点评" />
    </div>
  );
}

function ListCard({
  title,
  description,
  items,
  tone,
  quoted,
}: {
  title: string;
  description?: string;
  items?: string[] | null;
  tone: "good" | "bad" | "neutral";
  quoted?: boolean;
}) {
  const list = (items ?? []).filter(Boolean);
  if (list.length === 0) return null;
  const dot =
    tone === "good" ? "bg-emerald-500" : tone === "bad" ? "bg-destructive" : "bg-primary";
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        {description ? <CardDescription>{description}</CardDescription> : null}
      </CardHeader>
      <CardContent>
        <ul className="grid gap-2 text-sm">
          {list.map((item, i) => (
            <li key={i} className="flex gap-2">
              <span className={cn("mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full", dot)} />
              <span className={quoted ? "text-muted-foreground italic" : ""}>
                {quoted ? `“${item}”` : item}
              </span>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}

function RoomSkeleton() {
  return (
    <div className="grid gap-6">
      <div className="grid gap-2">
        <Skeleton className="h-7 w-64" />
        <Skeleton className="h-4 w-40" />
      </div>
      <Skeleton className="h-64 w-full" />
      <Skeleton className="h-40 w-full" />
    </div>
  );
}
