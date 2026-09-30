"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Eye, FileText, Play, Plus, Trash2, UserRound } from "lucide-react";
import { subscribeEvents } from "@/lib/api";
import {
  deleteInterview,
  labelFor,
  listInterviews,
  type Interview,
} from "@/lib/api/interviews";
import { getUserOverview, type UserOverview } from "@/lib/api/admin";
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
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
import { Skeleton } from "@/components/ui/skeleton";
import { formatDateTime, formatScore } from "@/lib/format";

// 面试记录：列表 + 操作（继续 / 查看报告 / 删除）。subscribeEvents 订阅
// interview.* 事件，任一标签页的改动都会让所有打开的列表实时刷新。
//
// 管理员额外支持 /interviews/?userId=<id>（从「用户」页进入）：带过滤条件拉
// 列表，并用 GET /api/admin/users/{id}/overview 显示该用户的汇总。这个查询
// 参数只对管理员生效，普通用户看到的就是自己的记录，界面上不会出现任何痕迹。

export default function InterviewsPage() {
  // useSearchParams 需要 Suspense 边界（静态导出时会预渲染）
  return (
    <Suspense fallback={<ListSkeleton />}>
      <InterviewsList />
    </Suspense>
  );
}

function InterviewsList() {
  const searchParams = useSearchParams();
  const { user } = useUser();
  const admin = isAdmin(user);
  const targetUserId = admin ? (searchParams.get("userId") ?? "").trim() : "";

  const [interviews, setInterviews] = useState<Interview[] | null>(null);
  const [overview, setOverview] = useState<UserOverview | null>(null);
  const [error, setError] = useState("");
  const [removing, setRemoving] = useState<Interview | null>(null);
  const [busy, setBusy] = useState(false);

  const reload = useCallback(async () => {
    try {
      const res = await listInterviews(
        targetUserId ? { userId: targetUserId } : { all: admin }
      );
      if (res.ok) {
        setInterviews(res.interviews ?? []);
        setError("");
      } else {
        setError(res.error || "无法加载面试记录");
      }
    } catch {
      setError("加载失败，请稍后重试。");
    }
  }, [targetUserId, admin]);

  useEffect(() => {
    // 初次加载：放进 async IIFE，setState 发生在 await 之后
    void (async () => {
      await reload();
    })();
  }, [reload]);

  // 管理员查看某个用户时，额外拉一次该用户的汇总（仅此接口会放宽所有权）。
  useEffect(() => {
    if (!targetUserId) return;
    let alive = true;
    void (async () => {
      try {
        const res = await getUserOverview(targetUserId);
        if (!alive) return;
        if (res.ok && res.overview) setOverview(res.overview);
        else setError(res.error || "无法加载该用户的汇总");
      } catch {
        if (alive) setError("无法加载该用户的汇总");
      }
    })();
    return () => {
      alive = false;
    };
  }, [targetUserId]);

  useEffect(
    () =>
      subscribeEvents((evt) => {
        if (evt.type.startsWith("interview.")) reload();
      }),
    [reload]
  );

  async function confirmDelete() {
    if (!removing) return;
    setBusy(true);
    const res = await deleteInterview(removing.id);
    setBusy(false);
    if (!res.ok) setError(res.error || "删除失败");
    setRemoving(null);
    reload();
  }

  const ov = overview && overview.user.id === targetUserId ? overview : null;
  const viewingName =
    ov?.user.displayName || ov?.user.username || (targetUserId ? "该用户" : "");

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="font-heading text-2xl font-semibold">面试记录</h1>
          <p className="text-sm text-muted-foreground">
            {targetUserId
              ? "管理员视图：其他用户的记录只能查看与导出，不能修改。"
              : `每一场面试的进度、得分与报告都在这里。${
                  admin ? "（管理员视图：全部用户的面试）" : ""
                }`}
          </p>
        </div>
        {!targetUserId && (
          <Button render={<Link href="/interviews/new/" />}>
            <Plus className="h-4 w-4" /> 新建面试
          </Button>
        )}
      </div>

      {targetUserId && (
        <div className="flex flex-wrap items-center gap-3 rounded-lg border bg-muted/40 p-3">
          <UserRound className="h-5 w-5 shrink-0 text-muted-foreground" />
          <div className="min-w-0">
            <p className="truncate text-sm font-medium">
              正在查看 {viewingName} 的面试记录
            </p>
            <p className="text-xs text-muted-foreground">
              {ov
                ? `共 ${ov.total} 场 · 已完成 ${ov.completed} · 进行中 ${ov.inProgress} · 已中止 ${ov.aborted} · 平均分 ${formatScore(
                    ov.avgScore
                  )} · 最高分 ${formatScore(ov.bestScore)}`
                : "正在加载该用户的汇总…"}
              {ov?.lastActivityAt ? ` · 最近活动 ${formatDateTime(ov.lastActivityAt)}` : ""}
            </p>
          </div>
          <Button variant="outline" size="sm" className="ml-auto" render={<Link href="/interviews/" />}>
            返回我的记录
          </Button>
        </div>
      )}

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader>
          <CardTitle>{targetUserId ? "该用户的面试" : "全部面试"}</CardTitle>
          <CardDescription>{interviews?.length ?? "…"} 场</CardDescription>
        </CardHeader>
        <CardContent>
          {interviews === null ? (
            <div className="grid gap-2">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          ) : interviews.length === 0 ? (
            <div className="flex flex-col items-center gap-3 py-12 text-center">
              <p className="text-sm text-muted-foreground">
                {targetUserId ? "该用户还没有面试记录。" : "还没有面试记录。"}
              </p>
              {!targetUserId && (
                <Button render={<Link href="/interviews/new/" />}>
                  <Plus className="h-4 w-4" /> 创建第一场面试
                </Button>
              )}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>岗位 / 标题</TableHead>
                  <TableHead className="hidden sm:table-cell">状态</TableHead>
                  <TableHead className="hidden md:table-cell">级别</TableHead>
                  <TableHead className="hidden lg:table-cell">类型</TableHead>
                  <TableHead className="text-right">得分</TableHead>
                  <TableHead className="hidden sm:table-cell">建议</TableHead>
                  <TableHead className="hidden text-right md:table-cell">轮次</TableHead>
                  <TableHead className="hidden lg:table-cell">创建时间</TableHead>
                  <TableHead className="w-32 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {interviews.map((iv) => {
                  const done = iv.status === "completed";
                  // 所有权由服务端裁定：别人的记录 GET 可以读，写会 404。
                  const mine = !!user && iv.userId === user.id;
                  return (
                    <TableRow key={iv.id}>
                      <TableCell className="max-w-64">
                        <Link
                          href={`/interview/?id=${encodeURIComponent(iv.id)}`}
                          className="block truncate font-medium hover:underline"
                          title={iv.title || iv.role}
                        >
                          {iv.title || iv.role}
                        </Link>
                        <span className="text-xs text-muted-foreground">{iv.role}</span>
                      </TableCell>
                      <TableCell className="hidden sm:table-cell">
                        <StatusBadge status={iv.status} />
                      </TableCell>
                      <TableCell className="hidden text-sm text-muted-foreground md:table-cell">
                        {labelFor("level", iv.level)}
                      </TableCell>
                      <TableCell className="hidden text-sm text-muted-foreground lg:table-cell">
                        {labelFor("type", iv.interviewType)}
                      </TableCell>
                      <TableCell className="text-right">
                        <ScoreText score={iv.overallScore} />
                      </TableCell>
                      <TableCell className="hidden sm:table-cell">
                        {iv.recommendation ? (
                          <RecommendationBadge recommendation={iv.recommendation} />
                        ) : (
                          <span className="text-sm text-muted-foreground">—</span>
                        )}
                      </TableCell>
                      <TableCell className="hidden text-right text-sm text-muted-foreground md:table-cell">
                        {iv.turnCount}/{iv.questionCount}
                      </TableCell>
                      <TableCell className="hidden text-sm text-muted-foreground lg:table-cell">
                        {formatDateTime(iv.createdAt)}
                      </TableCell>
                      <TableCell className="text-right">
                        {mine ? (
                          <div className="flex justify-end gap-1">
                            <Button
                              variant="ghost"
                              size="sm"
                              render={
                                <Link href={`/interview/?id=${encodeURIComponent(iv.id)}`} />
                              }
                            >
                              {done ? (
                                <>
                                  <FileText className="h-4 w-4" /> 查看报告
                                </>
                              ) : (
                                <>
                                  <Play className="h-4 w-4" /> 继续面试
                                </>
                              )}
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              aria-label="删除"
                              onClick={() => setRemoving(iv)}
                            >
                              <Trash2 className="h-4 w-4 text-destructive" />
                            </Button>
                          </div>
                        ) : (
                          <Button
                            variant="ghost"
                            size="sm"
                            render={
                              <Link href={`/interview/?id=${encodeURIComponent(iv.id)}`} />
                            }
                          >
                            <Eye className="h-4 w-4" /> 查看
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <AlertDialog open={!!removing} onOpenChange={(open) => !open && setRemoving(null)}>
        <AlertDialogContent className="sm:max-w-sm">
          <AlertDialogHeader>
            <AlertDialogTitle>删除这场面试？</AlertDialogTitle>
            <AlertDialogDescription>
              「{removing?.title || removing?.role}」及其全部问答记录会被永久删除，无法撤销。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={confirmDelete} disabled={busy}>
              {busy ? "删除中…" : "删除"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function ListSkeleton() {
  return (
    <div className="grid gap-6">
      <Skeleton className="h-9 w-40" />
      <Card>
        <CardContent className="grid gap-2 pt-6">
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-9 w-full" />
          <Skeleton className="h-9 w-full" />
        </CardContent>
      </Card>
    </div>
  );
}
