"use client";

import { useCallback, useEffect, useState } from "react";
import { Copy, Plus, RotateCw, Trash2 } from "lucide-react";
import { createAPIKey, deleteAPIKey, listAPIKeys, rotateAPIKey, type APIKey } from "@/lib/api";
import { useUser, isAdmin } from "@/components/user-context";
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

// API Keys: manage programmatic credentials for YOUR account. The
// plaintext token is shown exactly once (right after create/rotate) —
// the backend stores only its SHA-256 hash.
export default function APIKeysPage() {
  const { user } = useUser();
  const [keys, setKeys] = useState<APIKey[] | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [issued, setIssued] = useState<{ key: APIKey; token: string } | null>(null);
  const [removing, setRemoving] = useState<APIKey | null>(null);

  const reload = useCallback(async () => {
    const res = await listAPIKeys();
    if (res.ok) setKeys(res.apiKeys ?? []);
  }, []);

  useEffect(() => {
    // Initial load runs in an async callback so setState lands after the await.
    void (async () => {
      await reload();
    })();
  }, [reload]);

  return (
    <div className="grid gap-6">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="font-heading text-2xl font-semibold">API Keys</h1>
          <p className="text-sm text-muted-foreground">
            Bearer tokens for programmatic access: <code>Authorization: Bearer sk_…</code>
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          <Plus className="h-4 w-4" /> New key
        </Button>
      </div>

      <Card>
        <CardContent className="pt-6">
          {keys === null ? (
            <p className="py-8 text-center text-sm text-muted-foreground">Loading…</p>
          ) : keys.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              No keys yet — create one to call the API from scripts.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Key</TableHead>
                  <TableHead>Type</TableHead>
                  <TableHead className="hidden sm:table-cell">Created</TableHead>
                  <TableHead className="w-24 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {keys.map((k) => (
                  <TableRow key={k.id}>
                    <TableCell className="font-medium">{k.name || "(unnamed)"}</TableCell>
                    <TableCell className="font-mono text-xs text-muted-foreground">{k.key}</TableCell>
                    <TableCell>
                      <Badge variant={k.type === "admin" ? "default" : "secondary"}>{k.type}</Badge>
                    </TableCell>
                    <TableCell className="hidden text-sm text-muted-foreground sm:table-cell">
                      {new Date(k.createdAt).toLocaleDateString()}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label="Rotate"
                          onClick={async () => {
                            const res = await rotateAPIKey(k.id);
                            if (res.ok && res.key) setIssued({ key: { ...k, key: res.key }, token: res.key });
                          }}
                        >
                          <RotateCw className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="icon" aria-label="Delete" onClick={() => setRemoving(k)}>
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

      <CreateDialog
        open={createOpen}
        adminMode={isAdmin(user)}
        onOpenChange={setCreateOpen}
        onCreated={async (apiKey, token) => {
          setCreateOpen(false);
          setIssued({ key: apiKey, token });
          await reload();
        }}
      />

      {/* One-time plaintext reveal */}
      <Dialog open={!!issued} onOpenChange={(open) => !open && setIssued(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Your new API key</DialogTitle>
            <DialogDescription>
              Copy it now — this is the only time the full token is shown.
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2 rounded-md border bg-muted/50 p-3">
            <code className="flex-1 overflow-x-auto font-mono text-sm">{issued?.token}</code>
            <Button
              variant="outline"
              size="icon"
              aria-label="Copy"
              onClick={() => navigator.clipboard.writeText(issued?.token || "")}
            >
              <Copy className="h-4 w-4" />
            </Button>
          </div>
          <DialogFooter>
            <Button onClick={() => setIssued(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete confirm */}
      <Dialog open={!!removing} onOpenChange={(open) => !open && setRemoving(null)}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Delete API key?</DialogTitle>
            <DialogDescription>
              “{removing?.name || "(unnamed)"}” stops working immediately.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoving(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={async () => {
                if (removing) await deleteAPIKey(removing.id);
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

function CreateDialog({
  open,
  adminMode,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  adminMode: boolean;
  onOpenChange: (open: boolean) => void;
  onCreated: (key: APIKey, token: string) => void | Promise<void>;
}) {
  const [name, setName] = useState("");
  const [type, setType] = useState("user");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    // Reset the form when the dialog opens (asynchronous, see above).
    void (async () => {
      if (open) {
        setName("");
        setType("user");
        setError("");
      }
    })();
  }, [open]);

  async function create() {
    setBusy(true);
    setError("");
    try {
      const res = await createAPIKey({ name: name.trim(), type });
      if (res.ok && res.apiKey) {
        await onCreated(res.apiKey, res.apiKey.key);
      } else {
        setError(res.error || "failed to create key");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Create API key</DialogTitle>
          <DialogDescription>Keys act as your account; an admin key can also manage users.</DialogDescription>
        </DialogHeader>
        <div className="grid gap-4">
          <div className="grid gap-2">
            <Label htmlFor="key-name">Name</Label>
            <Input
              id="key-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="ci-script"
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="key-type">Type</Label>
            <Select value={type} onValueChange={(v) => setType(v ?? "user")}>
              <SelectTrigger id="key-type" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="user">user — acts as you</SelectItem>
                {adminMode && <SelectItem value="admin">admin — full platform access</SelectItem>}
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
            {busy ? "Creating…" : "Create key"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
