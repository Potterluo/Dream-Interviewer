"use client";

import { useCallback, useEffect, useState } from "react";
import { Lock, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import {
  getInterviewPresets,
  optionsFor,
  type Preset,
  type PresetLabels,
} from "@/lib/api/interviews";
import {
  createPreset,
  deletePreset,
  updatePreset,
  type PresetInput,
} from "@/lib/api/admin";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";

// 共享预置（仅管理员）：内置预置编译进二进制且只读，服务端对编辑/删除
// 返回 409，所以界面直接不给这两个按钮；管理员新建的预置对所有用户可见。
// 读取走公开的 GET /api/interview/presets（返回内置 + 共享预置 + 标签表），
// 写操作走 /api/admin/presets*。

const MIN_QUESTIONS = 3;
const MAX_QUESTIONS = 15;

export default function AdminPresetsPage() {
  const [presets, setPresets] = useState<Preset[] | null>(null);
  const [labels, setLabels] = useState<PresetLabels | null>(null);
  const [error, setError] = useState("");
  const [editing, setEditing] = useState<Preset | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [removing, setRemoving] = useState<Preset | null>(null);
  const [busy, setBusy] = useState(false);

  const reload = useCallback(async () => {
    try {
      const res = await getInterviewPresets();
      if (res.ok) {
        setPresets(res.presets ?? []);
        if (res.labels) setLabels(res.labels);
        setError("");
      } else {
        setPresets([]);
        setError(res.error || "无法加载预置");
      }
    } catch {
      setPresets([]);
      setError("加载失败，请稍后重试。");
    }
  }, []);

  useEffect(() => {
    // 初次加载放进 async 回调：setState 发生在 await 之后
    void (async () => {
      await reload();
    })();
  }, [reload]);

  async function confirmDelete() {
    if (!removing) return;
    setBusy(true);
    const res = await deletePreset(removing.id);
    setBusy(false);
    if (!res.ok) setError(res.error || "删除失败");
    setRemoving(null);
    await reload();
  }

  function labelOf(group: string, value: string): string {
    return optionsFor(group, labels).find((o) => o.value === value)?.label ?? (value || "—");
  }

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="font-heading text-2xl font-semibold">共享预置</h1>
          <p className="text-sm text-muted-foreground">
            所有用户都能在新建面试时选到这些岗位预置；内置预置只读。
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={reload}>
            <RefreshCw className="h-4 w-4" /> 刷新
          </Button>
          <Button onClick={() => setCreateOpen(true)}>
            <Plus className="h-4 w-4" /> 新建预置
          </Button>
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader>
          <CardTitle>预置列表</CardTitle>
          <CardDescription>
            {presets ? `${presets.length} 个预置` : "加载中…"} · 内置预置排在前面，其次是管理员新建的预置。
          </CardDescription>
        </CardHeader>
        <CardContent>
          {presets === null ? (
            <div className="grid gap-2">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </div>
          ) : presets.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">还没有预置。</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>岗位</TableHead>
                  <TableHead className="hidden sm:table-cell">级别</TableHead>
                  <TableHead className="hidden md:table-cell">类型</TableHead>
                  <TableHead className="hidden md:table-cell">难度</TableHead>
                  <TableHead className="text-right">题量</TableHead>
                  <TableHead className="hidden lg:table-cell">考察方向</TableHead>
                  <TableHead className="hidden sm:table-cell">来源</TableHead>
                  <TableHead className="w-24 text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {presets.map((p) => (
                  <TableRow key={p.id}>
                    <TableCell className="max-w-64">
                      <p className="truncate font-medium" title={p.role}>
                        {p.role}
                      </p>
                      {p.description && (
                        <p className="truncate text-xs text-muted-foreground" title={p.description}>
                          {p.description}
                        </p>
                      )}
                    </TableCell>
                    <TableCell className="hidden text-sm text-muted-foreground sm:table-cell">
                      {labelOf("level", p.level)}
                    </TableCell>
                    <TableCell className="hidden text-sm text-muted-foreground md:table-cell">
                      {labelOf("type", p.interviewType)}
                    </TableCell>
                    <TableCell className="hidden text-sm text-muted-foreground md:table-cell">
                      {labelOf("difficulty", p.difficulty)}
                    </TableCell>
                    <TableCell className="text-right text-sm text-muted-foreground">
                      {p.questionCount}
                    </TableCell>
                    <TableCell className="hidden text-xs text-muted-foreground lg:table-cell">
                      {p.focusAreas && p.focusAreas.length > 0 ? (
                        // The truncation has to happen on a BLOCK element with its
                        // own max-width. `truncate` on the inline <span> was a
                        // no-op, and `max-w-56` on a <td> is only a hint under
                        // auto table layout — so a long 考察方向 list spilled out
                        // of the cell and painted over the 来源 badge and the
                        // action buttons.
                        <div className="max-w-56 truncate" title={p.focusAreas.join("・")}>
                          {p.focusAreas.join("・")}
                        </div>
                      ) : (
                        "—"
                      )}
                    </TableCell>
                    <TableCell className="hidden sm:table-cell">
                      {p.builtin ? (
                        <Badge variant="secondary" className="gap-1">
                          <Lock className="h-3 w-3" /> 内置
                        </Badge>
                      ) : (
                        <Badge variant="outline">共享</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      {p.builtin ? (
                        <div className="flex justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="内置预置不可修改"
                            title="内置预置不可修改"
                            disabled
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="内置预置不可删除"
                            title="内置预置不可删除"
                            disabled
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      ) : (
                        <div className="flex justify-end gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="编辑"
                            onClick={() => setEditing(p)}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="删除"
                            onClick={() => setRemoving(p)}
                          >
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        </div>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <PresetDialog
        open={createOpen || !!editing}
        preset={editing}
        labels={labels}
        onOpenChange={(open) => {
          if (!open) {
            setCreateOpen(false);
            setEditing(null);
          }
        }}
        onSaved={async () => {
          setCreateOpen(false);
          setEditing(null);
          await reload();
        }}
        onError={setError}
      />

      <AlertDialog open={!!removing} onOpenChange={(open) => !open && setRemoving(null)}>
        <AlertDialogContent className="sm:max-w-sm">
          <AlertDialogHeader>
            <AlertDialogTitle>删除这个预置？</AlertDialogTitle>
            <AlertDialogDescription>
              「{removing?.role}」将从共享预置中移除；已经用它创建的面试不受影响。
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

function PresetDialog({
  open,
  preset,
  labels,
  onOpenChange,
  onSaved,
  onError,
}: {
  open: boolean;
  preset: Preset | null;
  labels: PresetLabels | null;
  onOpenChange: (open: boolean) => void;
  onSaved: () => void | Promise<void>;
  onError: (msg: string) => void;
}) {
  const [role, setRole] = useState("");
  const [level, setLevel] = useState("senior");
  const [interviewType, setInterviewType] = useState("tech");
  const [difficulty, setDifficulty] = useState("normal");
  const [questionCount, setQuestionCount] = useState(6);
  const [focusAreas, setFocusAreas] = useState("");
  const [jdSample, setJdSample] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // 打开时按当前预置重置表单；放进 async 回调避免在 effect 体内同步 setState
    // （react-hooks/set-state-in-effect）。
    void (async () => {
      if (!open) return;
      setRole(preset?.role ?? "");
      setLevel(preset?.level || "senior");
      setInterviewType(preset?.interviewType || "tech");
      setDifficulty(preset?.difficulty || "normal");
      setQuestionCount(clamp(preset?.questionCount ?? 6));
      setFocusAreas((preset?.focusAreas ?? []).join("，"));
      setJdSample(preset?.jdSample ?? "");
      setDescription(preset?.description ?? "");
      setError("");
    })();
  }, [open, preset]);

  async function submit() {
    setError("");
    if (!role.trim()) {
      setError("请填写岗位名称");
      return;
    }
    const body: PresetInput = {
      role: role.trim(),
      level,
      interviewType,
      difficulty,
      questionCount: clamp(questionCount),
      focusAreas: parseFocusAreas(focusAreas),
      jdSample: jdSample.trim(),
      description: description.trim(),
    };
    setBusy(true);
    try {
      const res = preset
        ? await updatePreset(preset.id, body)
        : await createPreset(body);
      if (res.ok) {
        await onSaved();
      } else {
        setError(res.error || "保存失败");
        onError(res.error || "保存失败");
      }
    } catch {
      setError("保存失败（网络错误）");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{preset ? "编辑预置" : "新建共享预置"}</DialogTitle>
          <DialogDescription>
            预置会作为新建面试时的快捷选项，对所有用户可见。
          </DialogDescription>
        </DialogHeader>

        <div className="grid max-h-[60vh] gap-4 overflow-y-auto pr-1">
          <div className="grid gap-2">
            <Label htmlFor="preset-role">岗位 *</Label>
            <Input
              id="preset-role"
              value={role}
              onChange={(e) => setRole(e.target.value)}
              placeholder="例如：资深数据工程师"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-3">
            <div className="grid gap-2">
              <Label htmlFor="preset-level">级别</Label>
              <Select value={level} onValueChange={(v) => setLevel(v ?? "senior")}>
                <SelectTrigger id="preset-level" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {optionsFor("level", labels).map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="preset-type">面试类型</Label>
              <Select value={interviewType} onValueChange={(v) => setInterviewType(v ?? "tech")}>
                <SelectTrigger id="preset-type" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {optionsFor("type", labels).map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="preset-difficulty">难度</Label>
              <Select value={difficulty} onValueChange={(v) => setDifficulty(v ?? "normal")}>
                <SelectTrigger id="preset-difficulty" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {optionsFor("difficulty", labels).map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="preset-count">题目数量（{MIN_QUESTIONS}–{MAX_QUESTIONS}）</Label>
            <Input
              id="preset-count"
              type="number"
              min={MIN_QUESTIONS}
              max={MAX_QUESTIONS}
              value={questionCount}
              onChange={(e) => setQuestionCount(clamp(Number(e.target.value)))}
              className="w-28"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="preset-focus">考察方向</Label>
            <Input
              id="preset-focus"
              value={focusAreas}
              onChange={(e) => setFocusAreas(e.target.value)}
              placeholder="用逗号分隔，例如：并发，数据库，系统设计"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="preset-description">说明</Label>
            <Input
              id="preset-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="一句话说明这个预置适合什么岗位"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="preset-jd">示例 JD</Label>
            <Textarea
              id="preset-jd"
              rows={4}
              value={jdSample}
              onChange={(e) => setJdSample(e.target.value)}
              placeholder="选择这个预置时会自动填入 JD，用户可以再修改。"
            />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button onClick={submit} disabled={busy}>
            {busy ? "保存中…" : preset ? "保存修改" : "创建预置"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// "并发，数据库, 系统设计" → ["并发", "数据库", "系统设计"]
function parseFocusAreas(raw: string): string[] {
  return raw
    .split(/[,，、]/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function clamp(n: number): number {
  if (!Number.isFinite(n)) return MIN_QUESTIONS;
  return Math.min(MAX_QUESTIONS, Math.max(MIN_QUESTIONS, Math.round(n)));
}
