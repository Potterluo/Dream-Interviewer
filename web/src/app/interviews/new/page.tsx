"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft, Check, Sparkles } from "lucide-react";
import {
  createInterview,
  getInterviewPresets,
  optionsFor,
  type Preset,
  type PresetLabels,
} from "@/lib/api/interviews";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

// 新建面试：选预设 → 改岗位/级别/类型/难度/语言/题量 → 贴 JD 与简历 →
// POST /api/interviews 后直接进入面试房间（静态导出，所以用查询参数）。
// 所有下拉选项的中文标签都来自 GET /api/interview/presets 的 labels，
// 加载中或接口不可用时回落到 interviews.ts 里的内置中文表。

const MIN_QUESTIONS = 3;
const MAX_QUESTIONS = 15;

export default function NewInterviewPage() {
  const router = useRouter();
  const [presets, setPresets] = useState<Preset[] | null>(null);
  const [labels, setLabels] = useState<PresetLabels | null>(null);
  const [presetId, setPresetId] = useState("");

  const [title, setTitle] = useState("");
  const [role, setRole] = useState("");
  const [level, setLevel] = useState("senior");
  const [type, setType] = useState("tech");
  const [difficulty, setDifficulty] = useState("normal");
  const [language, setLanguage] = useState("zh");
  const [questionCount, setQuestionCount] = useState(6);
  const [jdText, setJdText] = useState("");
  const [resumeText, setResumeText] = useState("");

  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let alive = true;
    (async () => {
      let list: Preset[] = [];
      try {
        const res = await getInterviewPresets();
        if (!alive) return;
        list = res.presets ?? [];
        setPresets(list);
        if (res.labels) setLabels(res.labels);
      } catch {
        if (alive) setPresets([]);
        return;
      }
      if (!alive) return;
      // 默认选中第一个预设，用户不动手也能直接开始。
      const first = list[0];
      if (first) {
        setPresetId(first.id);
        setRole(first.role);
        if (first.level) setLevel(first.level);
        if (first.interviewType) setType(first.interviewType);
        if (first.difficulty) setDifficulty(first.difficulty);
        if (typeof first.questionCount === "number") {
          setQuestionCount(clamp(first.questionCount));
        }
        setJdText(first.jdSample ?? "");
      }
    })();
    return () => {
      alive = false;
    };
  }, []);

  function applyPreset(p: Preset) {
    setPresetId(p.id);
    setRole(p.role);
    if (p.level) setLevel(p.level);
    if (p.interviewType) setType(p.interviewType);
    if (p.difficulty) setDifficulty(p.difficulty);
    if (typeof p.questionCount === "number") setQuestionCount(clamp(p.questionCount));
    setJdText(p.jdSample ?? "");
  }

  async function submit() {
    setError("");
    if (!role.trim()) {
      setError("请填写面试岗位");
      return;
    }
    setBusy(true);
    try {
      const res = await createInterview({
        role: role.trim(),
        level,
        interviewType: type,
        language,
        difficulty,
        questionCount: clamp(questionCount),
        title: title.trim() || undefined,
        jdText: jdText.trim() || undefined,
        resumeText: resumeText.trim() || undefined,
      });
      if (!res.ok || !res.interview) {
        setError(res.error || "创建失败");
        return;
      }
      router.push(`/interview/?id=${encodeURIComponent(res.interview.id)}`);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-6">
      <div>
        <Button variant="ghost" size="sm" className="mb-2 -ml-2" render={<Link href="/interviews/" />}>
          <ArrowLeft className="h-4 w-4" /> 返回面试记录
        </Button>
        <h1 className="font-heading text-2xl font-semibold">新建面试</h1>
        <p className="text-sm text-muted-foreground">
          选择岗位预设或自定义，AI 面试官会据此生成面试计划并逐题追问。
        </p>
      </div>

      {/* 岗位预设 */}
      <Card>
        <CardHeader>
          <CardTitle>岗位预设</CardTitle>
          <CardDescription>选择预设会自动填入岗位、级别、题量与示例 JD，之后都可以修改。</CardDescription>
        </CardHeader>
        <CardContent>
          {presets === null ? (
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              <Skeleton className="h-28 w-full" />
              <Skeleton className="h-28 w-full" />
              <Skeleton className="h-28 w-full" />
            </div>
          ) : presets.length === 0 ? (
            <p className="text-sm text-muted-foreground">暂无预设，直接填写下面的岗位信息即可。</p>
          ) : (
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {presets.map((p) => (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => applyPreset(p)}
                  aria-pressed={presetId === p.id}
                  className={cn(
                    "flex flex-col gap-1.5 rounded-xl border p-3 text-left transition-colors hover:bg-accent/40",
                    presetId === p.id ? "border-primary ring-1 ring-primary" : "border-border"
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-medium">{p.role}</span>
                    {presetId === p.id && <Check className="h-4 w-4 text-primary" />}
                  </div>
                  <div className="flex flex-wrap gap-1">
                    <Badge variant="secondary">{labelOf("level", p.level)}</Badge>
                    <Badge variant="outline">{labelOf("type", p.interviewType)}</Badge>
                    <Badge variant="outline">{p.questionCount} 题</Badge>
                  </div>
                  {p.description && (
                    <p className="text-xs text-muted-foreground">{p.description}</p>
                  )}
                  {p.focusAreas && p.focusAreas.length > 0 && (
                    <p className="text-xs text-muted-foreground">考察：{p.focusAreas.join(" · ")}</p>
                  )}
                </button>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* 面试配置 */}
      <Card>
        <CardHeader>
          <CardTitle>面试配置</CardTitle>
          <CardDescription>岗位为必填，其余字段留空时使用默认值。</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-5">
          <div className="grid gap-2">
            <Label htmlFor="role">岗位 *</Label>
            <Input
              id="role"
              value={role}
              onChange={(e) => setRole(e.target.value)}
              placeholder="例如：高级后端工程师"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="title">标题（可选）</Label>
            <Input
              id="title"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="留空则根据岗位与类型自动生成"
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div className="grid gap-2">
              <Label htmlFor="level">级别</Label>
              <Select value={level} onValueChange={(v) => setLevel(v ?? "senior")}>
                <SelectTrigger id="level" className="w-full">
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
              <Label htmlFor="type">面试类型</Label>
              <Select value={type} onValueChange={(v) => setType(v ?? "tech")}>
                <SelectTrigger id="type" className="w-full">
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
              <Label htmlFor="difficulty">难度</Label>
              <Select value={difficulty} onValueChange={(v) => setDifficulty(v ?? "normal")}>
                <SelectTrigger id="difficulty" className="w-full">
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

            <div className="grid gap-2">
              <Label htmlFor="language">语言</Label>
              <Select value={language} onValueChange={(v) => setLanguage(v ?? "zh")}>
                <SelectTrigger id="language" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {optionsFor("language", labels).map((o) => (
                    <SelectItem key={o.value} value={o.value}>
                      {o.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="grid gap-2">
            <div className="flex items-center justify-between">
              <Label htmlFor="questionCount">题目数量</Label>
              <span className="text-sm text-muted-foreground">
                {questionCount} 题（{MIN_QUESTIONS}–{MAX_QUESTIONS}）
              </span>
            </div>
            <div className="flex items-center gap-3">
              <input
                id="questionCount"
                type="range"
                min={MIN_QUESTIONS}
                max={MAX_QUESTIONS}
                step={1}
                value={questionCount}
                onChange={(e) => setQuestionCount(clamp(Number(e.target.value)))}
                className="h-2 flex-1 cursor-pointer appearance-none rounded-full bg-muted accent-primary"
              />
              <Input
                type="number"
                min={MIN_QUESTIONS}
                max={MAX_QUESTIONS}
                value={questionCount}
                onChange={(e) => setQuestionCount(clamp(Number(e.target.value)))}
                className="w-20"
                aria-label="题目数量"
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="jd">职位描述（JD，可选）</Label>
            <Textarea
              id="jd"
              rows={5}
              value={jdText}
              onChange={(e) => setJdText(e.target.value)}
              placeholder="粘贴 JD，AI 会针对岗位要求出题。"
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="resume">简历 / 自我介绍（可选）</Label>
            <Textarea
              id="resume"
              rows={6}
              value={resumeText}
              onChange={(e) => setResumeText(e.target.value)}
              placeholder="粘贴简历或项目经历，AI 会据此追问细节。"
            />
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
        </CardContent>
      </Card>

      <div className="flex items-center gap-3">
        <Button size="lg" onClick={submit} disabled={busy}>
          <Sparkles className="h-4 w-4" /> {busy ? "正在创建…" : "创建并开始面试"}
        </Button>
        <span className="text-xs text-muted-foreground">
          创建后进入面试房间，点击「开始面试」才会调用模型。
        </span>
      </div>
    </div>
  );

  function labelOf(group: string, value: string): string {
    const opts = optionsFor(group, labels);
    return opts.find((o) => o.value === value)?.label ?? value;
  }
}

function clamp(n: number): number {
  if (!Number.isFinite(n)) return MIN_QUESTIONS;
  return Math.min(MAX_QUESTIONS, Math.max(MIN_QUESTIONS, Math.round(n)));
}
