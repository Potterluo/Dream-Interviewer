"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { ArrowLeft, RefreshCw, Settings2 } from "lucide-react";
import { getEngineStatus, type EngineStatus } from "@/lib/api/interviews";
import { useUser, isAdmin } from "@/components/user-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

// 引擎：只读展示「现在是谁在面试你」。任何已登录用户都能看到（被评估的人
// 有权知道是哪个模型在评判自己），但只有管理员能修改 —— 修改入口在
// /admin/model/，这里不再提供任何写操作，也不再提供测试按钮
// （连接测试属于管理员配置流程：POST /api/admin/settings/test）。

export default function EngineSettingsPage() {
  const { user } = useUser();
  const [engine, setEngine] = useState<EngineStatus | null>(null);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      const res = await getEngineStatus();
      if (res.ok && res.engine) {
        setEngine(res.engine);
        setError("");
      } else {
        setError(res.error || "无法读取引擎配置");
      }
    } catch {
      setError("无法读取引擎配置（网络错误）");
    }
  }, []);

  useEffect(() => {
    // 初次加载放进 async 回调：setState 发生在 await 之后
    void (async () => {
      await load();
    })();
  }, [load]);

  const admin = isAdmin(user);

  return (
    <div className="grid gap-6">
      <div>
        <Button variant="ghost" size="sm" className="mb-2 -ml-2" render={<Link href="/settings/" />}>
          <ArrowLeft className="h-4 w-4" /> 返回设置
        </Button>
        <h1 className="font-heading text-2xl font-semibold">引擎</h1>
        <p className="text-sm text-muted-foreground">
          面试官使用的 LLM 引擎。密钥只显示掩码，完整值从不离开服务端。
          {admin ? "" : "只有管理员可以修改它。"}
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center gap-2">
            <CardTitle>当前面试官模型</CardTitle>
            {engine && (
              <Badge variant={engine.configured ? "default" : "destructive"}>
                {engine.configured ? "已配置" : "未配置"}
              </Badge>
            )}
            {/* 密钥状态只对管理员有意义：非管理员的 apiKeySet 恒为 false，
                直接渲染会告诉他们"密钥未配置"，而这是错的。 */}
            {engine && engine.baseUrl && (
              <Badge variant={engine.apiKeySet ? "outline" : "secondary"}>
                {engine.apiKeySet ? "密钥已配置" : "密钥未配置"}
              </Badge>
            )}
            <div className="ml-auto">
              <Button variant="ghost" size="sm" onClick={load}>
                <RefreshCw className="h-4 w-4" /> 刷新
              </Button>
            </div>
          </div>
          <CardDescription>
            配置来源 <span className="font-mono text-xs">{engine?.source ?? "…"}</span>（db =
            管理员在界面里设置，env = 环境变量，default = 内置默认值）
            {engine && !engine.configured
              ? "。当前未配置模型，面试会使用内置题库 + 确定性评分，并在结果中标注。"
              : ""}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {engine === null ? (
            <div className="grid gap-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : (
            <dl className="grid gap-x-6 gap-y-3 sm:grid-cols-2">
              <Field label="提供商" value={engine.provider} mono />
              <Field label="模型" value={engine.model} mono />
              {/* 服务端只把端点与密钥掩码返回给管理员；普通用户看到的
                  baseUrl 是空字符串，所以按值判断而不是按角色判断。 */}
              {engine.baseUrl ? (
                <>
                  <Field label="Base URL" value={engine.baseUrl} mono />
                  <Field
                    label="API Key"
                    value={engine.apiKeySet ? engine.apiKeyMasked || "已配置" : "未配置"}
                    mono
                  />
                </>
              ) : (
                <div className="sm:col-span-2 rounded-md border border-dashed p-3 text-xs text-muted-foreground">
                  模型端点与密钥仅管理员可见。你随时可以看到当前使用的是哪个模型。
                </div>
              )}
              <Field label="超时" value={`${engine.timeoutSec} 秒`} />
              <Field label="最大输出" value={`${engine.maxTokens} tokens`} />
              <Field label="温度" value={`${engine.temperature}`} />
              <Field
                label="离线兜底"
                value={engine.fallbackAvailable ? "可用（无模型时也能完成面试）" : "不可用"}
              />
            </dl>
          )}
        </CardContent>
      </Card>

      {admin && (
        <Card>
          <CardHeader>
            <CardTitle>管理员</CardTitle>
            <CardDescription>
              这里的模型对所有用户生效。要修改提供商、地址、模型或密钥，请前往模型配置页面。
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button render={<Link href="/admin/model/" />}>
              <Settings2 className="h-4 w-4" /> 前往模型配置
            </Button>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function Field({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="grid gap-0.5">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className={mono ? "truncate font-mono text-sm" : "truncate text-sm"} title={value}>
        {value || "—"}
      </dd>
    </div>
  );
}
