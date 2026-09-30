"use client";

import { useState } from "react";
import { login } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

// LoginScreen is rendered inline by AuthGuard (no /login route): the
// URL stays on whatever page the user requested, and after login that
// page simply appears.
export function LoginScreen({ onSuccess }: { onSuccess: () => void | Promise<void> }) {
  const [loginField, setLoginField] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      const res = await login(loginField.trim(), password);
      if (res.ok && res.user) {
        await onSuccess();
      } else {
        setError(res.error || "login failed");
      }
    } catch {
      setError("cannot reach the server");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="page-glow flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <img src="/icon.png" alt="" className="mx-auto mb-2 h-12 w-12 rounded-xl shadow-md" />
          <CardTitle className="text-xl">Sign in</CardTitle>
          <CardDescription>Use your account to continue</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="login">Username or email</Label>
              <Input
                id="login"
                autoComplete="username"
                value={loginField}
                onChange={(e) => setLoginField(e.target.value)}
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" disabled={busy}>
              {busy ? "Signing in…" : "Sign in"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
