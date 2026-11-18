/**
 * [INPUT]: depends on the files API from @/lib/files; depends on Button, Card,
 *          Label from @/components/ui; depends on cn from @/lib/utils.
 * [OUTPUT]: exports FilesUpload — a drop/pick zone that runs the full
 *          direct-to-R2 flow (upload-url → PUT → confirm → job poll) with
 *          progress, retry, and failure states; plus the UploadPhase type and
 *          UploadOutcome interface so pages/tests reason about state without
 *          mounting the component.
 * [POS]: the UI half of the files lane. Pages under app/(dashboard) render it
 *          inside a card; it never talks to fetch directly, only via
 *          @/lib/files, so the auth-client migration (CLI 2) needs no edits here.
 * [PROTOCOL]: Update this header on change, then check CLAUDE.md
 */
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { AlertCircle, CheckCircle2, Loader2, Paperclip, Upload, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
import {
  confirmFile,
  getDownloadUrl,
  pollJob,
  putFileToR2,
  requestUploadUrl,
  validateFile,
  ApiError,
} from "@/lib/files";

export type UploadPhase =
  | "idle"
  | "requesting" // POST /files/upload-url in flight
  | "uploading" // browser PUT to R2, progress observable
  | "confirming" // POST /files/:id/confirm
  | "polling" // GET /jobs/:id loop
  | "completed"
  | "failed";

/** The minimal snapshot pages need to drive surrounding UI from a completed
 *  upload (e.g. reload the records list). */
export interface UploadOutcome {
  fileId: number;
  job: { status: string; result?: unknown };
}

export interface FilesUploadProps {
  /** Called when the job reports completed; the page refetches its data. */
  onCompleted?: (outcome: UploadOutcome) => void;
  /** Short label under the drop zone, e.g. "Prescription PDF". */
  hint?: string;
  /** id of the input, for a11y; unique per page instance. */
  inputId?: string;
  className?: string;
}

function describeError(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 413) return "The file exceeded the server size limit.";
    if (err.status === 400) return "The file was rejected (type or size).";
    if (err.status === 403) return "You are not allowed to upload this file.";
    return err.data?.error || "The server rejected the request.";
  }
  if (err instanceof DOMException && err.name === "AbortError") return "Upload was cancelled.";
  return "Something went wrong while uploading. Check your connection and retry.";
}

export function FilesUpload({ onCompleted, hint, inputId = "files-upload", className }: FilesUploadProps) {
  const [phase, setPhase] = useState<UploadPhase>("idle");
  const [file, setFile] = useState<File | null>(null);
  const [progress, setProgress] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [jobNote, setJobNote] = useState<string | null>(null);
  const [downloadUrl, setDownloadUrl] = useState<string | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const abortRef = useRef<AbortController | null>(null);
  const busy = phase === "requesting" || phase === "uploading" || phase === "confirming" || phase === "polling";

  useEffect(() => () => abortRef.current?.abort(), []);

  const reset = useCallback(() => {
    abortRef.current?.abort();
    abortRef.current = null;
    setPhase("idle");
    setFile(null);
    setProgress(0);
    setError(null);
    setJobNote(null);
    setDownloadUrl(null);
    if (inputRef.current) inputRef.current.value = "";
  }, []);

  const acceptFile = useCallback(
    (candidate: File) => {
      const invalid = validateFile(candidate);
      if (invalid) {
        setPhase("failed");
        setError(invalid);
        setFile(null);
        return;
      }
      setFile(candidate);
      setPhase("idle");
      setError(null);
      setProgress(0);
      setJobNote(null);
    },
    []
  );

  /** The full pipeline; `fromRetry` restarts from step 1 (fresh presigned URL). */
  const runUpload = useCallback(
    async (candidate: File) => {
      const controller = new AbortController();
      abortRef.current = controller;
      setError(null);
      setJobNote(null);
      setDownloadUrl(null);

      try {
        setPhase("requesting");
        const { file_id, put_url } = await requestUploadUrl(candidate.name, candidate.type, candidate.size);

        setPhase("uploading");
        await putFileToR2(put_url, candidate, (p) => setProgress(p), controller.signal);

        setPhase("confirming");
        const { job_id } = await confirmFile(file_id);

        if (job_id != null) {
          setPhase("polling");
          setJobNote("Uploaded. Verifying file on the server…");
          const job = await pollJob(job_id, {
            signal: controller.signal,
            onTick: (j) => setJobNote(j.status === "processing" ? "Server is verifying the file…" : "Waiting for the verification queue…"),
          });
          setJobNote(null);
          if (job.status === "failed") {
            setPhase("failed");
            setError("The server could not verify the file. Delete it or retry the upload.");
            return;
          }
          // completed, or budget exhausted but not failed: surface the snapshot.
          if (job.status !== "completed") {
            setPhase("completed");
            setJobNote("Upload finished; server verification is still running.");
          } else {
            setPhase("completed");
            try {
              const dl = await getDownloadUrl(file_id);
              setDownloadUrl(dl.get_url);
            } catch {
              // A missing download URL must not mask a successful upload.
            }
          }
          onCompleted?.({ fileId: file_id, job: { status: job.status, result: job.result } });
        } else {
          // confirm returned active with no queued job — nothing to poll.
          setPhase("completed");
          try {
            const dl = await getDownloadUrl(file_id);
            setDownloadUrl(dl.get_url);
          } catch {
            // ignore, as above
          }
          onCompleted?.({ fileId: file_id, job: { status: "active" } });
        }
      } catch (err) {
        if (controller.signal.aborted) {
          setPhase("idle");
          return;
        }
        setPhase("failed");
        setError(describeError(err));
      }
    },
    [onCompleted]
  );

  const startWithFile = useCallback(
    (candidate: File) => {
      if (busy) return;
      setFile(candidate);
      setProgress(0);
      void runUpload(candidate);
    },
    [busy, runUpload]
  );

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault();
      setDragOver(false);
      const dropped = e.dataTransfer.files?.[0];
      if (dropped && !busy) acceptFile(dropped);
    },
    [acceptFile, busy]
  );

  const handlePick = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const picked = e.target.files?.[0];
      if (picked && !busy) acceptFile(picked);
    },
    [acceptFile, busy]
  );

  const retry = useCallback(() => {
    if (!file) return;
    void runUpload(file);
  }, [file, runUpload]);

  // ---- Render per phase ----

  if (phase === "idle" && file) {
    return (
      <div className={cn("space-y-4", className)}>
        <div className="flex items-center justify-between rounded-2xl bg-muted/50 p-4">
          <div className="flex min-w-0 items-center gap-3">
            <Paperclip className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
            <div className="min-w-0">
              <p className="truncate text-sm font-medium">{file.name}</p>
              <p className="text-xs text-muted-foreground">{(file.size / 1024 / 1024).toFixed(1)} MB</p>
            </div>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <Button size="sm" variant="ghost" onClick={reset} aria-label="Remove file">
              <X className="h-4 w-4" aria-hidden="true" />
            </Button>
            <Button size="sm" onClick={() => startWithFile(file)}>
              <Upload className="mr-2 h-4 w-4" aria-hidden="true" />
              Upload
            </Button>
          </div>
        </div>
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
      </div>
    );
  }

  if (busy) {
    return (
      <div className={cn("space-y-4 rounded-2xl bg-muted/30 p-4", className)} role="status" aria-live="polite">
        <div className="flex items-center justify-between">
          <div className="flex min-w-0 items-center gap-2">
            <Loader2 className="h-4 w-4 shrink-0 animate-spin text-primary" aria-hidden="true" />
            <p className="truncate text-sm font-medium">{file?.name}</p>
          </div>
          {phase === "uploading" ? (
            <p className="text-sm tabular-nums text-muted-foreground">{progress}%</p>
          ) : (
            <Button size="sm" variant="ghost" onClick={reset} aria-label="Cancel upload">
              Cancel
            </Button>
          )}
        </div>
        {phase === "uploading" && (
          <div className="h-2 overflow-hidden rounded-full bg-muted">
            <div
              className="h-full rounded-full bg-primary transition-all duration-200"
              style={{ width: `${progress}%` }}
              role="progressbar"
              aria-valuenow={progress}
              aria-valuemin={0}
              aria-valuemax={100}
            />
          </div>
        )}
        <p className="text-sm text-muted-foreground">
          {phase === "requesting" && "Requesting an upload link…"}
          {phase === "confirming" && "Confirming the upload with the server…"}
          {phase === "polling" && (jobNote ?? "Waiting on the server…")}
        </p>
      </div>
    );
  }

  if (phase === "completed") {
    return (
      <div className={cn("space-y-3 rounded-2xl bg-muted/30 p-4", className)} role="status" aria-live="polite">
        <div className="flex items-center gap-3">
          <CheckCircle2 className="h-5 w-5 shrink-0 text-green-600" aria-hidden="true" />
          <div>
            <p className="text-sm font-medium">{file?.name} uploaded</p>
            {jobNote && <p className="text-xs text-muted-foreground">{jobNote}</p>}
          </div>
        </div>
        {downloadUrl && (
          <a
            href={downloadUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm font-medium text-primary underline-offset-2 hover:underline"
          >
            Download a copy
          </a>
        )}
        <div className="flex gap-2">
          <Button size="sm" variant="secondary" onClick={reset}>
            Upload another
          </Button>
        </div>
      </div>
    );
  }

  // "failed" — or no file yet: show the drop zone, with the error above it.
  return (
    <div className={cn("space-y-3", className)}>
      {error && (
        <div role="alert" className="flex items-start gap-2 rounded-xl bg-red-50 p-3 text-sm text-red-600">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          <div className="flex-1">
            <p>{error}</p>
            {file && (
              <div className="mt-2 flex gap-2">
                <Button size="sm" variant="outline" onClick={retry}>
                  Retry
                </Button>
                <Button size="sm" variant="ghost" onClick={reset}>
                  Choose a different file
                </Button>
              </div>
            )}
          </div>
        </div>
      )}
      <div
        onDragOver={(e) => {
          e.preventDefault();
          if (!busy && !file) setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={handleDrop}
        className={cn(
          "flex flex-col items-center justify-center gap-2 rounded-2xl border-2 border-dashed p-6 text-center transition-colors",
          dragOver ? "border-primary bg-primary/5" : "border-muted-foreground/20 bg-muted/20"
        )}
      >
        <Upload className="h-6 w-6 text-muted-foreground" aria-hidden="true" />
        <p className="text-sm text-muted-foreground">
          Drag a file here, or{" "}
          <Label htmlFor={inputId} className="cursor-pointer text-sm font-medium text-primary">
            browse
          </Label>
        </p>
        <input
          ref={inputRef}
          id={inputId}
          type="file"
          accept="application/pdf,image/png,image/jpeg,image/webp"
          className="sr-only"
          onChange={handlePick}
          aria-label="Choose a file to upload"
        />
        <p className="text-xs text-muted-foreground/70">PDF, PNG, JPEG, or WebP · up to 25 MB</p>
      </div>
    </div>
  );
}
