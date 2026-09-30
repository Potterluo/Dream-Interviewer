"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { onboard } from "@/lib/api";
import { useUser } from "@/components/user-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

// Onboard: first-run wizard. Reached when /api/status reports
// configured=false (no users yet). The account created here becomes the
// admin; afterwards this page 409s server-side and AuthGuard stops
// routing here.
export default function OnboardPage() {
  const router = useRouter();
  const { refresh } = useUser();
  const [form, setForm] = useState({ username: "", email: "", password: "", displayName: "" });
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  function set<K extends keyof typeof form>(key: K, value: string) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const res = await onboard({
        username: form.username.trim(),
        email: form.email.trim(),
        password: form.password,
        displayName: form.displayName.trim(),
      });
      if (res.ok) {
        await refresh();
        router.replace("/");
      } else {
        setError(res.error || "setup failed");
      }
    } catch {
      setError("cannot reach the server");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex min-h-[calc(100vh-3.5rem)] items-center justify-center p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <img src="/icon.png" alt="" className="mx-auto mb-2 h-12 w-12 rounded-xl shadow-md" />
          <CardTitle className="text-xl">Set up your instance</CardTitle>
          <CardDescription>
            Create the first account. It becomes the administrator.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="grid gap-4">
            <div className="grid gap-2">
              <Label htmlFor="username">Username</Label>
              <Input
                id="username"
                value={form.username}
                onChange={(e) => set("username", e.target.value)}
                placeholder="admin"
                autoComplete="username"
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="email">Email</Label>
              <Input
                id="email"
                type="email"
                value={form.email}
                onChange={(e) => set("email", e.target.value)}
                placeholder="admin@example.com"
                autoComplete="email"
                required
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="displayName">Display name (optional)</Label>
              <Input
                id="displayName"
                value={form.displayName}
                onChange={(e) => set("displayName", e.target.value)}
                placeholder="Alice"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                value={form.password}
                onChange={(e) => set("password", e.target.value)}
                placeholder="at least 8 characters"
                autoComplete="new-password"
                required
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" disabled={busy}>
              {busy ? "Creating…" : "Create admin account"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
