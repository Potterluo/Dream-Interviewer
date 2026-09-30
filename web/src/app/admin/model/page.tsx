"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  CheckCircle2,
  KeyRound,
  Loader2,
  PlugZap,
  RefreshCw,
  RotateCcw,
  Save,
  ShieldAlert,
  XCircle,
} from "lucide-react";
import {
  clearAdminSettings,
  getAdminSettings,
  testAdminSettings,
  updateAdminSettings,
  type AdminSettings,
  type AdminSettingsField,
  type AdminSettingsInput,
  type AdminSettingsTestInput,
  type AdminTestResult,
} from "@/lib/api/admin";
import { getEngineStatus, type EngineStatus } from "@/lib/api/interviews";
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

// 模型配置（仅管理员）。配置存数据库，优先级高于环境变量与内置默认值；
// 这里能看到每个字段「数据库覆盖 / 环境变量 / 内置默认」，以及环境里存在
// 哪些 APP_LLM_* 变量。
//
// 三条契约细节（docs/API.md §6、§7）：
//   1. PUT 是「字段缺席 = 不改动；空串 = 清除覆盖」。因此保存时只提交用户
//      真正改过的字段（dirty），否则打开表单再保存会覆盖掉未展示的存储值。
//   2. API Key 同规则：只有用户真的输入了新 Key 才提交 apiKey，留空即不提交，
//      所以保存不会清掉已存的密钥。要清空必须显式点「清除密钥」。
//   3. 测试连接用「当前表单值」发请求（缺省字段回落到生效配置），所以可以在
//      保存前先验证一遍；探测失败仍然是信封层 ok:true + result.ok=false。

const PROVIDERS: { value: string; label: string }[] = [
  { value: "siliconflow", label: "SiliconFlow" },
  { value: "custom", label: "自定义（OpenAI 兼容）" },
  { value: "openai", label: "OpenAI" },
];

// 字段 → 对应的环境变量，用于解释「这个字段为什么是现在的值」。
const ENV_KEYS: Record<AdminSettingsField, string> = {
  provider: "APP_LLM_PROVIDER",
  baseUrl: "APP_LLM_BASE_URL",
  model: "APP_LLM_MODEL",
  apiKey: "APP_LLM_API_KEY",
  timeoutSec: "APP_LLM_TIMEOUT_SEC",
  maxTokens: "APP_LLM_MAX_TOKENS",
  temperature: "APP_LLM_TEMPERATURE",
};

const NUMERIC_FIELDS: AdminSettingsField[] = ["timeoutSec", "maxTokens", "temperature"];

const RANGE: Partial<Record<AdminSettingsField, { min: number; max: number; unit: string }>> = {
  timeoutSec: { min: 5, max: 600, unit: "秒" },
  maxTokens: { min: 256, max: 131072, unit: "tokens" },
  temperature: { min: 0, max: 2, unit: "" },
};

type FormField = "provider" | "baseUrl" | "model" | "timeoutSec" | "maxTokens" | "temperature";

type FormState = Record<FormField, string>;
type DirtyState = Record<FormField, boolean>;

const EMPTY_FORM: FormState = {
  provider: "custom",
  baseUrl: "",
  model: "",
  timeoutSec: "",
  maxTokens: "",
  temperature: "",
};

const EMPTY_DIRTY: DirtyState = {
  provider: false,
  baseUrl: false,
  model: false,
  timeoutSec: false,
  maxTokens: false,
  temperature: false,
};

export default function AdminModelPage() {
  const [settings, setSettings] = useState<AdminSettings | null>(null);
  const [engine, setEngine] = useState<EngineStatus | null>(null);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [dirty, setDirty] = useState<DirtyState>(EMPTY_DIRTY);
  const [apiKey, setApiKey] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState(false);
  const [result, setResult] = useState<AdminTestResult | null>(null);
  const [confirmAction, setConfirmAction] = useState<"key" | "all" | null>(null);
  // provider 是唯一无法用「空输入」表达清除的字段（枚举 Select），所以它用
  // 一个独立标记：保存时提交 provider: ""（服务端接受，且不算非法提供商）。
  const [providerClear, setProviderClear] = useState(false);

  const hydrate = useCallback((s: AdminSettings) => {
    setSettings(s);
    const numStr = (v: number) => (Number.isFinite(v) ? String(v) : "");
    setForm({
      provider: s.provider || "custom",
      baseUrl: s.baseUrl ?? "",
      model: s.model ?? "",
      timeoutSec: numStr(s.timeoutSec),
      maxTokens: numStr(s.maxTokens),
      temperature: numStr(s.temperature),
    });
    setDirty(EMPTY_DIRTY);
    setProviderClear(false);
    // 永远不回填密钥：输入框只承载「本次新输入的 Key」。
    setApiKey("");
  }, []);

  const load = useCallback(async () => {
    try {
      const res = await getAdminSettings();
      if (res.ok && res.settings) {
        hydrate(res.settings);
        setError("");
      } else {
        setError(res.error || "无法读取模型配置");
      }
    } catch {
      setError("无法读取模型配置（网络错误）");
    }
  }, [hydrate]);

  useEffect(() => {
    // 初次加载放进 async 回调：setState 发生在 await 之后
    void (async () => {
      await load();
      try {
        const res = await getEngineStatus();
        if (res.ok && res.engine) setEngine(res.engine);
      } catch {
        // 生效值只是提示，读不到不影响配置表单
      }
    })();
  }, [load]);

  function setField(k: FormField, v: string) {
    if (k === "provider") setProviderClear(false);
    setForm((f) => ({ ...f, [k]: v }));
    setDirty((d) => ({ ...d, [k]: true }));
  }

  // 清空输入框 = 清除该字段的数据库覆盖（"": 回落到环境变量/默认值）。
  // provider 用 providerClear 标记，保证提交的是 ""（清除）而不是某个具体值。
  function clearOverride(k: FormField) {
    if (k === "provider") {
      setProviderClear(true);
      setDirty((d) => ({ ...d, provider: false }));
      // 显示成清除后会真正生效的提供商，避免下拉框停在旧值上误导用户
      setForm((f) => ({ ...f, provider: engine?.provider || "custom" }));
      return;
    }
    setForm((f) => ({ ...f, [k]: "" }));
    setDirty((d) => ({ ...d, [k]: true }));
  }

  function parseNumber(k: FormField, raw: string): number | "" | null {
    const t = raw.trim();
    if (!t) return ""; // 空串 = 清除覆盖，回落到环境变量/默认值
    const n = Number(t);
    if (!Number.isFinite(n)) return null;
    const range = RANGE[k];
    if (range && (n < range.min || n > range.max)) return null;
    return n;
  }

  // 只提交用户改动过的字段：未改动的字段缺席即保持不变。
  function buildUpdate(): { body: AdminSettingsInput; error: string } {
    const body: AdminSettingsInput = {};
    const fail = (field: AdminSettingsField) => {
      const range = RANGE[field];
      return {
        body,
        error: range
          ? `${labelOfField(field)} 需要是 ${range.min}–${range.max}${
              range.unit ? " " + range.unit : ""
            } 之间的数字`
          : `${labelOfField(field)} 需要是数字`,
      };
    };

    // 缺席 = 保持不变；"" = 清除覆盖（含数值字段：清空输入框就发 "" 这个
    // 字符串，绝不省略 key，否则会退化成「保持不变」）。
    if (providerClear) body.provider = "";
    else if (dirty.provider) body.provider = form.provider;
    if (dirty.baseUrl) body.baseUrl = form.baseUrl.trim();
    if (dirty.model) body.model = form.model.trim();

    if (dirty.timeoutSec) {
      const n = parseNumber("timeoutSec", form.timeoutSec);
      if (n === null) return fail("timeoutSec");
      body.timeoutSec = n;
    }
    if (dirty.maxTokens) {
      const n = parseNumber("maxTokens", form.maxTokens);
      if (n === null) return fail("maxTokens");
      body.maxTokens = n;
    }
    if (dirty.temperature) {
      const n = parseNumber("temperature", form.temperature);
      if (n === null) return fail("temperature");
      body.temperature = n;
    }

    // 关键：只有用户真的输入了才带上 apiKey；留空表示「不修改」。
    const typedKey = apiKey.trim();
    if (typedKey) body.apiKey = typedKey;

    return { body, error: "" };
  }

  async function save() {
    setError("");
    setNotice("");
    const { body, error: invalid } = buildUpdate();
    if (invalid) {
      setError(invalid);
      return;
    }
    if (Object.keys(body).length === 0) {
      setNotice("没有需要保存的改动。");
      return;
    }
    setBusy(true);
    try {
      const res = await updateAdminSettings(body);
      if (res.ok && res.settings) {
        hydrate(res.settings);
        setNotice("已保存，下一次模型调用立即生效。");
      } else {
        setError(res.error || "保存失败");
      }
    } catch {
      setError("保存失败（网络错误）");
    } finally {
      setBusy(false);
    }
  }

  // 测试连接：用当前表单值（而不是已存的值），所以可以先验证再保存。
  function buildTest(): AdminSettingsTestInput {
    const body: AdminSettingsTestInput = { provider: form.provider };
    if (form.baseUrl.trim()) body.baseUrl = form.baseUrl.trim();
    if (form.model.trim()) body.model = form.model.trim();
    if (apiKey.trim()) body.apiKey = apiKey.trim();
    const timeout = parseNumber("timeoutSec", form.timeoutSec);
    if (typeof timeout === "number") body.timeoutSec = timeout;
    const maxTokens = parseNumber("maxTokens", form.maxTokens);
    if (typeof maxTokens === "number") body.maxTokens = maxTokens;
    return body;
  }

  async function runTest() {
    setTesting(true);
    setResult(null);
    try {
      const res = await testAdminSettings(buildTest());
      if (res.result) setResult(res.result);
      else setResult({ ok: false, error: res.error || "探测失败" });
    } catch {
      setResult({ ok: false, error: "请求失败：无法连接到服务端" });
    } finally {
      setTesting(false);
    }
  }

  async function runConfirmed() {
    const action = confirmAction;
    if (!action) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const res =
        action === "all"
          ? await clearAdminSettings()
          : await updateAdminSettings({ apiKey: "" });
      if (res.ok && res.settings) {
        hydrate(res.settings);
        setResult(null);
        setNotice(
          action === "all"
            ? "已清除全部数据库覆盖，字段回落到环境变量 / 默认值。"
            : "已清除保存的 API Key。"
        );
      } else {
        setError(res.error || "操作失败");
      }
    } catch {
      setError("操作失败（网络错误）");
    } finally {
      setBusy(false);
      setConfirmAction(null);
    }
  }

  if (settings === null && !error) {
    return (
      <div className="grid gap-6">
        <Skeleton className="h-9 w-56" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  return (
    <div className="grid gap-6">
      <div>
        <Button
          variant="ghost"
          size="sm"
          className="mb-2 -ml-2"
          render={<Link href="/settings/engine/" />}
        >
          <ArrowLeft className="h-4 w-4" /> 查看当前模型
        </Button>
        <h1 className="font-heading text-2xl font-semibold">模型配置</h1>
        <p className="text-sm text-muted-foreground">
          面试官使用的 LLM 引擎。配置保存在数据库中，优先级高于环境变量与内置默认值；密钥只显示掩码，
          完整值从不离开服务端。
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}
      {notice && <p className="text-sm text-muted-foreground">{notice}</p>}

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-2">
            <CardTitle>配置来源</CardTitle>
            {settings && (
              <Badge variant={settings.configured ? "default" : "destructive"}>
                {settings.configured ? "已配置" : "未配置"}
              </Badge>
            )}
            <div className="ml-auto">
              <Button variant="ghost" size="sm" onClick={load} disabled={busy}>
                <RefreshCw className="h-4 w-4" /> 重新读取
              </Button>
            </div>
          </div>
          <CardDescription>
            「数据库覆盖」表示该字段由管理员在界面里设置，优先于环境变量；否则回落到环境变量或内置默认值。
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-4">
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <span className="text-muted-foreground">API Key：</span>
            {settings?.apiKeySet ? (
              <Badge variant="default">已配置</Badge>
            ) : (
              <Badge variant="destructive">未配置</Badge>
            )}
            <span className="font-mono text-xs text-muted-foreground">
              {settings?.apiKeyMasked || "—"}
            </span>
          </div>

          <div className="grid gap-2">
            <p className="text-xs font-medium text-muted-foreground">
              进程环境中存在的 APP_LLM_* 变量
            </p>
            {(settings?.envPresent ?? []).length === 0 ? (
              <p className="text-sm text-muted-foreground">
                没有检测到 APP_LLM_* 环境变量，未覆盖的字段将使用内置默认值。
              </p>
            ) : (
              <div className="flex flex-wrap gap-1.5">
                {(settings?.envPresent ?? []).map((name) => (
                  <Badge key={name} variant="outline" className="font-mono">
                    {name}
                  </Badge>
                ))}
              </div>
            )}
            {engine && (
              <p className="text-xs text-muted-foreground">
                当前生效来源：<span className="font-mono">{engine.source}</span>（
                {engine.fallbackAvailable ? "离线兜底始终可用" : "无兜底"}）
              </p>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>模型参数</CardTitle>
          <CardDescription>
            只有被修改的字段会提交；清空输入框或点「清除覆盖」表示清除该字段的数据库覆盖，回落到环境变量或内置默认值。
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-5">
          <FieldRow
            id="provider"
            label="提供商"
            state={<OverrideBadge field="provider" settings={settings} />}
            hint={
              providerClear
                ? "保存后将回落到环境变量 / 内置默认值"
                : effectiveHint("provider", settings, engine)
            }
            onClear={
              settings?.overridden?.provider && !providerClear
                ? () => clearOverride("provider")
                : undefined
            }
          >
            <Select value={form.provider} onValueChange={(v) => setField("provider", v ?? "custom")}>
              <SelectTrigger id="provider" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {PROVIDERS.map((p) => (
                  <SelectItem key={p.value} value={p.value}>
                    {p.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FieldRow>

          <FieldRow
            id="baseUrl"
            label="Base URL"
            state={<OverrideBadge field="baseUrl" settings={settings} />}
            hint={effectiveHint("baseUrl", settings, engine)}
            onClear={settings?.overridden?.baseUrl ? () => clearOverride("baseUrl") : undefined}
          >
            <Input
              id="baseUrl"
              value={form.baseUrl}
              onChange={(e) => setField("baseUrl", e.target.value)}
              placeholder="https://api.example.com/v1"
              autoComplete="off"
            />
          </FieldRow>

          <FieldRow
            id="model"
            label="模型"
            state={<OverrideBadge field="model" settings={settings} />}
            hint={effectiveHint("model", settings, engine)}
            onClear={settings?.overridden?.model ? () => clearOverride("model") : undefined}
          >
            <Input
              id="model"
              value={form.model}
              onChange={(e) => setField("model", e.target.value)}
              placeholder="例如：deepseek-ai/DeepSeek-V3"
              autoComplete="off"
            />
          </FieldRow>

          <FieldRow
            id="apiKey"
            label="API Key"
            state={<OverrideBadge field="apiKey" settings={settings} />}
            hint={
              settings?.apiKeySet
                ? "已配置；留空表示不修改，只有输入新值才会替换。"
                : "尚未配置；填入后保存即可写入数据库。"
            }
            extra={
              settings?.apiKeySet ? (
                <Button
                  variant="ghost"
                  size="xs"
                  type="button"
                  onClick={() => setConfirmAction("key")}
                  disabled={busy}
                >
                  <KeyRound className="h-3 w-3" /> 清除密钥
                </Button>
              ) : undefined
            }
          >
            <Input
              id="apiKey"
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder={settings?.apiKeySet ? "已配置（留空则不修改）" : "sk-…"}
              autoComplete="new-password"
            />
          </FieldRow>

          <div className="grid gap-4 sm:grid-cols-3">
            <FieldRow
              id="timeoutSec"
              label="超时（秒）"
              state={<OverrideBadge field="timeoutSec" settings={settings} />}
              hint={`5–600 · ${effectiveHint("timeoutSec", settings, engine)}`}
              onClear={
                settings?.overridden?.timeoutSec ? () => clearOverride("timeoutSec") : undefined
              }
            >
              <Input
                id="timeoutSec"
                type="number"
                min={5}
                max={600}
                value={form.timeoutSec}
                onChange={(e) => setField("timeoutSec", e.target.value)}
              />
            </FieldRow>

            <FieldRow
              id="maxTokens"
              label="最大输出（tokens）"
              state={<OverrideBadge field="maxTokens" settings={settings} />}
              hint={`256–131072 · ${effectiveHint("maxTokens", settings, engine)}`}
              onClear={
                settings?.overridden?.maxTokens ? () => clearOverride("maxTokens") : undefined
              }
            >
              <Input
                id="maxTokens"
                type="number"
                min={256}
                max={131072}
                step={256}
                value={form.maxTokens}
                onChange={(e) => setField("maxTokens", e.target.value)}
              />
            </FieldRow>

            <FieldRow
              id="temperature"
              label="温度"
              state={<OverrideBadge field="temperature" settings={settings} />}
              hint={`0–2 · ${effectiveHint("temperature", settings, engine)}`}
              onClear={
                settings?.overridden?.temperature ? () => clearOverride("temperature") : undefined
              }
            >
              <Input
                id="temperature"
                type="number"
                min={0}
                max={2}
                step={0.1}
                value={form.temperature}
                onChange={(e) => setField("temperature", e.target.value)}
              />
            </FieldRow>
          </div>

          <div className="flex flex-wrap items-center gap-3 border-t pt-4">
            <Button onClick={save} disabled={busy || testing}>
              {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              {busy ? "保存中…" : "保存"}
            </Button>
            <Button variant="outline" onClick={runTest} disabled={testing || busy}>
              {testing ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <PlugZap className="h-4 w-4" />
              )}
              {testing ? "测试中…" : "测试连接"}
            </Button>
            <Button
              variant="destructive"
              onClick={() => setConfirmAction("all")}
              disabled={busy || testing}
            >
              <ShieldAlert className="h-4 w-4" /> 清除所有覆盖
            </Button>
            <span className="text-xs text-muted-foreground">
              测试连接使用当前表单里的值，可在保存前先验证。
            </span>
          </div>

          {result && (
            <div className="grid gap-2 rounded-lg border p-3 text-sm">
              {result.ok ? (
                <>
                  <p className="flex items-center gap-2 font-medium text-emerald-600 dark:text-emerald-400">
                    <CheckCircle2 className="h-4 w-4" /> 连接正常
                    {result.latencyMs != null && (
                      <span className="text-muted-foreground">延迟 {result.latencyMs} ms</span>
                    )}
                    {result.testedOverride && (
                      <Badge variant="outline">使用表单值</Badge>
                    )}
                  </p>
                  {result.model && (
                    <p className="text-muted-foreground">
                      模型：<span className="font-mono text-xs">{result.model}</span>
                    </p>
                  )}
                  {result.reply && (
                    <p className="text-muted-foreground">
                      回复：<span className="font-mono text-xs">{result.reply}</span>
                    </p>
                  )}
                </>
              ) : (
                <>
                  <p className="flex items-center gap-2 font-medium text-destructive">
                    <XCircle className="h-4 w-4" /> 连接失败
                  </p>
                  {result.error && (
                    <p className="break-words text-muted-foreground">{result.error}</p>
                  )}
                  {result.latencyMs != null && (
                    <p className="text-xs text-muted-foreground">耗时 {result.latencyMs} ms</p>
                  )}
                </>
              )}
            </div>
          )}
        </CardContent>
      </Card>

      <AlertDialog
        open={!!confirmAction}
        onOpenChange={(open) => !open && setConfirmAction(null)}
      >
        <AlertDialogContent className="sm:max-w-md">
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirmAction === "all" ? "清除所有数据库覆盖？" : "清除已保存的 API Key？"}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {confirmAction === "all"
                ? "所有字段都会回落到环境变量或内置默认值。模型服务可能因此不可用，但离线兜底面试仍然可用。"
                : "清除后需要重新填入密钥，模型调用才能恢复。此操作立即生效。"}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={runConfirmed} disabled={busy}>
              {busy ? "处理中…" : "确认清除"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function labelOfField(field: AdminSettingsField): string {
  switch (field) {
    case "timeoutSec":
      return "超时";
    case "maxTokens":
      return "最大输出";
    case "temperature":
      return "温度";
    case "baseUrl":
      return "Base URL";
    case "model":
      return "模型";
    case "provider":
      return "提供商";
    default:
      return "API Key";
  }
}

// OverrideBadge 回答「这个字段的值从哪来」：数据库覆盖 / 环境变量 / 内置默认。
function OverrideBadge({
  field,
  settings,
}: {
  field: AdminSettingsField;
  settings: AdminSettings | null;
}) {
  if (!settings) return null;
  if (settings.overridden?.[field]) {
    return <Badge variant="default">数据库覆盖</Badge>;
  }
  if ((settings.envPresent ?? []).includes(ENV_KEYS[field])) {
    return (
      <Badge variant="secondary" className="font-mono">
        {ENV_KEYS[field]}
      </Badge>
    );
  }
  return (
    <Badge variant="outline">
      {NUMERIC_FIELDS.includes(field) ? "内置默认" : "未设置"}
    </Badge>
  );
}

// effectiveHint 用只读的 /api/interview/engine 说明未覆盖字段的生效值。
function effectiveHint(
  field: AdminSettingsField,
  settings: AdminSettings | null,
  engine: EngineStatus | null
): string {
  if (!settings || settings.overridden?.[field]) return "由数据库中的值决定";
  switch (field) {
    case "provider":
      return engine?.provider ? `生效值：${engine.provider}` : "回落到环境变量或默认值";
    case "baseUrl":
      return engine?.baseUrl ? `生效值：${engine.baseUrl}` : "回落到环境变量";
    case "model":
      return engine?.model ? `生效值：${engine.model}` : "回落到环境变量";
    case "apiKey":
      return engine?.apiKeySet
        ? `生效值：${engine.apiKeyMasked || "已配置"}`
        : "回落到环境变量或未配置";
    case "timeoutSec":
      return typeof engine?.timeoutSec === "number"
        ? `生效值：${engine.timeoutSec} 秒`
        : "回落到内置默认值";
    case "maxTokens":
      return typeof engine?.maxTokens === "number"
        ? `生效值：${engine.maxTokens} tokens`
        : "回落到内置默认值";
    default:
      return typeof engine?.temperature === "number"
        ? `生效值：${engine.temperature}`
        : "回落到内置默认值";
  }
}

function FieldRow({
  id,
  label,
  state,
  hint,
  onClear,
  extra,
  children,
}: {
  id: string;
  label: string;
  state: React.ReactNode;
  hint?: string;
  onClear?: () => void;
  extra?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="grid gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <Label htmlFor={id}>{label}</Label>
        {state}
        <div className="ml-auto flex items-center gap-1">
          {onClear && (
            <Button variant="ghost" size="xs" type="button" onClick={onClear}>
              <RotateCcw className="h-3 w-3" /> 清除覆盖
            </Button>
          )}
          {extra}
        </div>
      </div>
      {children}
      {hint && <p className="truncate text-xs text-muted-foreground" title={hint}>{hint}</p>}
    </div>
  );
}
