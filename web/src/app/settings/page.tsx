"use client";

import { useEffect, useState } from "react";
import { changeMyPassword, updateMe } from "@/lib/api";
import { useUser } from "@/components/user-context";
import { useTheme, THEME_PRESETS, type Theme } from "@/components/theme-provider";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { Label } from "@/components/ui/label";

// Settings: profile, password, appearance. Each card saves independently
// so a failure in one doesn't touch the others.
export default function SettingsPage() {
  return (
    <div className="grid gap-6">
      <div>
        <h1 className="font-heading text-2xl font-semibold">Settings</h1>
        <p className="text-sm text-muted-foreground">Your account and preferences.</p>
      </div>
      <ProfileCard />
      <PasswordCard />
      <AppearanceCard />
    </div>
  );
}

function ProfileCard() {
  const { user, refresh } = useUser();
  const [displayName, setDisplayName] = useState("");
  const [msg, setMsg] = useState("");

  useEffect(() => {
    // Seed the form from the loaded user (asynchronous so the setState does
    // not run synchronously in the effect body).
    void (async () => {
      if (user) setDisplayName(user.displayName || "");
    })();
  }, [user]);

  async function save() {
    setMsg("");
    const res = await updateMe({ displayName });
    setMsg(res.ok ? "Saved." : res.error || "failed");
    if (res.ok) await refresh();
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Profile</CardTitle>
        <CardDescription>
          {user?.username} · {user?.email} · {user?.role}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid max-w-sm gap-2">
          <Label htmlFor="displayName">Display name</Label>
          <Input id="displayName" value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
        </div>
      </CardContent>
      <CardFooter className="gap-3">
        <Button size="sm" onClick={save}>
          Save
        </Button>
        {msg && <span className="text-sm text-muted-foreground">{msg}</span>}
      </CardFooter>
    </Card>
  );
}

function PasswordCard() {
  const [oldPassword, setOld] = useState("");
  const [newPassword, setNew] = useState("");
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");

  async function save() {
    setMsg("");
    setErr("");
    const res = await changeMyPassword({ oldPassword, newPassword });
    if (res.ok) {
      setMsg("Password changed.");
      setOld("");
      setNew("");
    } else {
      setErr(res.error || "failed");
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Password</CardTitle>
        <CardDescription>Changing your password requires the current one.</CardDescription>
      </CardHeader>
      <CardContent className="grid max-w-sm gap-4">
        <div className="grid gap-2">
          <Label htmlFor="old-password">Current password</Label>
          <Input id="old-password" type="password" value={oldPassword} onChange={(e) => setOld(e.target.value)} />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="new-password">New password</Label>
          <Input id="new-password" type="password" value={newPassword} onChange={(e) => setNew(e.target.value)} />
        </div>
      </CardContent>
      <CardFooter className="gap-3">
        <Button size="sm" onClick={save}>
          Change password
        </Button>
        {msg && <span className="text-sm text-muted-foreground">{msg}</span>}
        {err && <span className="text-sm text-destructive">{err}</span>}
      </CardFooter>
    </Card>
  );
}

function AppearanceCard() {
  const { theme, setTheme, accent, setAccent } = useTheme();
  const modes: { value: Theme; label: string }[] = [
    { value: "light", label: "Light" },
    { value: "dark", label: "Dark" },
    { value: "system", label: "System" },
  ];
  return (
    <Card>
      <CardHeader>
        <CardTitle>Appearance</CardTitle>
        <CardDescription>
          Mode (light/dark) and accent theme are independent — pick both. Persisted in this
          browser.
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-5">
        <div>
          <p className="mb-2 text-sm font-medium">Mode</p>
          <div className="flex flex-wrap gap-2">
            {modes.map((o) => (
              <Button
                key={o.value}
                size="sm"
                variant={theme === o.value ? "default" : "outline"}
                onClick={() => setTheme(o.value)}
              >
                {o.label}
              </Button>
            ))}
          </div>
        </div>
        <div>
          <p className="mb-2 text-sm font-medium">Theme</p>
          <div className="flex flex-wrap gap-2">
            {THEME_PRESETS.map((p) => (
              <button
                key={p.id}
                onClick={() => setAccent(p.id)}
                aria-pressed={accent === p.id}
                className={cn(
                  "flex items-center gap-2 rounded-lg border px-3 py-1.5 text-sm transition-colors hover:bg-accent",
                  accent === p.id ? "border-primary ring-1 ring-primary" : "border-border"
                )}
              >
                <span
                  className="h-4 w-4 rounded-full border border-black/10"
                  style={{ background: p.swatch }}
                />
                {p.label}
              </button>
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
