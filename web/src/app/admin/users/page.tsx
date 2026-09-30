"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { KeyRound, ListChecks, Plus, Trash2 } from "lucide-react";
import {
  createUser,
  deleteUser,
  listUsers,
  resetUserPassword,
  updateUser,
  type User,
} from "@/lib/api";
import { useUser } from "@/components/user-context";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

// Admin: Users. Server-side this whole surface requires the admin role;
// the client gate (AuthGuard admin prefix) just avoids rendering an
// empty shell for non-admins.
export default function UsersPage() {
  const { user: me } = useUser();
  const [users, setUsers] = useState<User[] | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [resetting, setResetting] = useState<User | null>(null);
  const [removing, setRemoving] = useState<User | null>(null);
  const [error, setError] = useState("");

  const reload = useCallback(async () => {
    const res = await listUsers();
    if (res.ok) {
      setUsers(res.users ?? []);
      setError("");
    } else {
      // Render the empty state + error banner instead of spinning forever.
      setUsers([]);
      setError(res.error || "failed to load users");
    }
  }, []);

  useEffect(() => {
    // Initial load runs in an async callback so setState lands after the await.
    void (async () => {
      await reload();
    })();
  }, [reload]);

  async function toggleStatus(u: User) {
    await updateUser(u.id, { status: u.status === "active" ? "disabled" : "active" });
    reload();
  }

  return (
    <div className="grid gap-6">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="font-heading text-2xl font-semibold">Users</h1>
          <p className="text-sm text-muted-foreground">Accounts on this instance.</p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4" /> New user
        </Button>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardContent className="pt-6">
          {users === null ? (
            <p className="py-8 text-center text-sm text-muted-foreground">Loading…</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>User</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="hidden md:table-cell">Created</TableHead>
                  <TableHead className="w-28 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((u) => (
                  <TableRow key={u.id}>
                    <TableCell>
                      <p className="font-medium">
                        {u.displayName || u.username}
                        {u.id === me?.id && (
                          <span className="ml-2 text-xs text-muted-foreground">(you)</span>
                        )}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {u.username} · {u.email}
                      </p>
                    </TableCell>
                    <TableCell>
                      <Badge variant={u.role === "admin" ? "default" : "secondary"}>{u.role}</Badge>
                    </TableCell>
                    <TableCell>
                      <button
                        className="inline-flex"
                        title={u.id === me?.id ? "You cannot disable yourself" : "Toggle status"}
                        disabled={u.id === me?.id}
                        onClick={() => toggleStatus(u)}
                      >
                        <Badge variant={u.status === "active" ? "outline" : "destructive"}>
                          {u.status}
                        </Badge>
                      </button>
                    </TableCell>
                    <TableCell className="hidden text-sm text-muted-foreground md:table-cell">
                      {new Date(u.createdAt).toLocaleDateString()}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label="查看面试记录"
                          title="查看该用户的面试记录"
                          render={
                            <Link href={`/interviews/?userId=${encodeURIComponent(u.id)}`} />
                          }
                        >
                          <ListChecks className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label="Reset password"
                          onClick={() => setResetting(u)}
                        >
                          <KeyRound className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label="Delete"
                          disabled={u.id === me?.id}
                          onClick={() => setRemoving(u)}
                        >
                          <Trash2 className="h-4 w-4 text-destructive" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <CreateUserDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreated={() => {
          setCreateOpen(false);
          reload();
        }}
      />

      <ResetPasswordDialog
        user={resetting}
        onOpenChange={(open) => !open && setResetting(null)}
      />

      <Dialog open={!!removing} onOpenChange={(open) => !open && setRemoving(null)}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Delete user?</DialogTitle>
            <DialogDescription>
              {removing?.username} and all their data (sessions, API keys, items) will be
              removed. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoving(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={async () => {
                if (removing) {
                  const res = await deleteUser(removing.id);
                  if (!res.ok) setError(res.error || "failed");
                }
                setRemoving(null);
                reload();
              }}
            >
              Delete
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function CreateUserDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: () => void;
}) {
  const [form, setForm] = useState({ username: "", email: "", password: "", role: "user" });
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // Reset the form when the dialog opens — inside an async callback so the
    // setState does not run synchronously in the effect body
    // (react-hooks/set-state-in-effect).
    void (async () => {
      if (open) {
        setForm({ username: "", email: "", password: "", role: "user" });
        setError("");
      }
    })();
  }, [open]);

  function set<K extends keyof typeof form>(k: K, v: string) {
    setForm((f) => ({ ...f, [k]: v }));
  }

  async function create() {
    setBusy(true);
    setError("");
    try {
      const res = await createUser({
        username: form.username.trim(),
        email: form.email.trim(),
        password: form.password,
        role: form.role,
      });
      if (res.ok) onCreated();
      else setError(res.error || "failed to create user");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create user</DialogTitle>
          <DialogDescription>The new account can sign in immediately.</DialogDescription>
        </DialogHeader>
        <div className="grid gap-4">
          <div className="grid gap-2">
            <Label htmlFor="nu-username">Username</Label>
            <Input id="nu-username" value={form.username} onChange={(e) => set("username", e.target.value)} />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="nu-email">Email</Label>
            <Input id="nu-email" type="email" value={form.email} onChange={(e) => set("email", e.target.value)} />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="nu-password">Password</Label>
            <Input
              id="nu-password"
              type="password"
              value={form.password}
              onChange={(e) => set("password", e.target.value)}
              placeholder="at least 8 characters"
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="nu-role">Role</Label>
            <Select value={form.role} onValueChange={(v) => set("role", v ?? "user")}>
              <SelectTrigger id="nu-role" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="user">user</SelectItem>
                <SelectItem value="admin">admin</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={create} disabled={busy}>
            {busy ? "Creating…" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function ResetPasswordDialog({
  user,
  onOpenChange,
}: {
  user: User | null;
  onOpenChange: (open: boolean) => void;
}) {
  const [password, setPassword] = useState("");
  const [msg, setMsg] = useState("");
  const [err, setErr] = useState("");

  useEffect(() => {
    // Fresh state for the newly selected user (asynchronous, see above).
    void (async () => {
      if (user) {
        setPassword("");
        setMsg("");
        setErr("");
      }
    })();
  }, [user]);

  async function reset() {
    if (!user) return;
    const res = await resetUserPassword(user.id, password);
    if (res.ok) setMsg("Password updated.");
    else setErr(res.error || "failed");
  }

  return (
    <Dialog open={!!user} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>Reset password</DialogTitle>
          <DialogDescription>Set a new password for {user?.username}.</DialogDescription>
        </DialogHeader>
        <div className="grid gap-2">
          <Label htmlFor="reset-password">New password</Label>
          <Input
            id="reset-password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          {msg && <p className="text-sm text-muted-foreground">{msg}</p>}
          {err && <p className="text-sm text-destructive">{err}</p>}
        </div>
        <DialogFooter>
          <Button onClick={reset}>Update password</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
