"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { CloudUpload, Download, Trash2 } from "lucide-react";
import { deleteFile, fileDownloadUrl, listFiles, uploadFile, type FileItem } from "@/lib/api";
import { useUser, isAdmin } from "@/components/user-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

// Files: multipart upload with progress + list/download/delete. The
// upload itself lives in lib/api.ts (uploadFile — XHR for progress
// events); this page adds the drag-and-drop surface.

const MAX_MB = 50;

export default function FilesPage() {
  const { user } = useUser();
  const [files, setFiles] = useState<FileItem[] | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const [progress, setProgress] = useState<number | null>(null); // null = idle
  const [error, setError] = useState("");
  const [removing, setRemoving] = useState<FileItem | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const reload = useCallback(async () => {
    const res = await listFiles(isAdmin(user));
    if (res.ok) setFiles(res.files ?? []);
  }, [user]);

  useEffect(() => {
    // Initial load runs in an async callback so setState lands after the await.
    void (async () => {
      await reload();
    })();
  }, [reload]);

  const upload = useCallback(
    async (list: FileList | null) => {
      if (!list || list.length === 0) return;
      setError("");
      for (const f of Array.from(list)) {
        if (f.size > MAX_MB << 20) {
          setError(`"${f.name}" exceeds the ${MAX_MB} MiB limit`);
          continue;
        }
        setProgress(0);
        const res = await uploadFile(f, setProgress);
        if (!res.ok) setError(res.error || "upload failed");
      }
      setProgress(null);
      reload();
    },
    [reload]
  );

  // Paste-to-upload: files copied to the clipboard (screenshots!) land
  // here. A small UX touch the pattern makes easy.
  useEffect(() => {
    const onPaste = (e: ClipboardEvent) => {
      if (e.clipboardData?.files?.length) upload(e.clipboardData.files);
    };
    window.addEventListener("paste", onPaste);
    return () => window.removeEventListener("paste", onPaste);
  }, [upload]);

  function formatSize(n: number): string {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / 1024 / 1024).toFixed(1)} MB`;
  }

  return (
    <div className="grid gap-6">
      <div>
        <h1 className="font-heading text-2xl font-semibold">Files</h1>
        <p className="text-sm text-muted-foreground">
          Uploads go to the server&apos;s data directory; metadata lives in the database.
          You can also paste files from the clipboard (e.g. screenshots).
        </p>
      </div>

      {/* Dropzone */}
      <div
        className={cn(
          "flex cursor-pointer flex-col items-center justify-center gap-3 rounded-xl border-2 border-dashed p-10 text-center transition-colors",
          dragOver ? "border-primary bg-primary/5" : "border-border hover:bg-accent/40"
        )}
        onClick={() => inputRef.current?.click()}
        onDragOver={(e) => {
          e.preventDefault();
          setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragOver(false);
          upload(e.dataTransfer.files);
        }}
      >
        <CloudUpload className="h-8 w-8 text-muted-foreground" />
        <div>
          <p className="text-sm font-medium">
            {progress !== null ? `Uploading… ${progress}%` : "Drop files here or click to browse"}
          </p>
          <p className="text-xs text-muted-foreground">Up to {MAX_MB} MiB per file</p>
        </div>
        {progress !== null && (
          <div className="h-1.5 w-56 overflow-hidden rounded-full bg-muted">
            <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${progress}%` }} />
          </div>
        )}
        <input
          ref={inputRef}
          type="file"
          multiple
          className="hidden"
          onChange={(e) => {
            upload(e.target.files);
            e.target.value = "";
          }}
        />
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardContent className="pt-6">
          {files === null ? (
            <p className="py-8 text-center text-sm text-muted-foreground">Loading…</p>
          ) : files.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              No files yet — drop one above.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead className="hidden sm:table-cell">Size</TableHead>
                  <TableHead className="hidden md:table-cell">Type</TableHead>
                  <TableHead className="hidden lg:table-cell">Uploaded</TableHead>
                  <TableHead className="w-24 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {files.map((f) => (
                  <TableRow key={f.id}>
                    <TableCell className="max-w-56 truncate font-medium" title={f.name}>
                      {f.name}
                    </TableCell>
                    <TableCell className="hidden text-sm text-muted-foreground sm:table-cell">
                      {formatSize(f.size)}
                    </TableCell>
                    <TableCell className="hidden font-mono text-xs text-muted-foreground md:table-cell">
                      {f.contentType}
                    </TableCell>
                    <TableCell className="hidden text-sm text-muted-foreground lg:table-cell">
                      {new Date(f.createdAt).toLocaleString()}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label="Download"
                          render={<a href={fileDownloadUrl(f)} download />}
                        >
                          <Download className="h-4 w-4" />
                        </Button>
                        <Button variant="ghost" size="icon" aria-label="Delete" onClick={() => setRemoving(f)}>
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

      <Dialog open={!!removing} onOpenChange={(open) => !open && setRemoving(null)}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Delete file?</DialogTitle>
            <DialogDescription>
              “{removing?.name}” and its stored data will be removed. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoving(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={async () => {
                if (removing) await deleteFile(removing.id);
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
