/**
 * [INPUT]: depends on apiFetch and ApiError from @/lib/api (central client; CLI 2
 *          migrates it to the /api/v1/auth cookie scheme, the call shape stays).
 * [OUTPUT]: exports the pinned files+jobs API surface for the browser —
 *          requestUploadUrl, putFileToR2, confirmFile, getDownloadUrl, getJob,
 *          pollJob (backoff), validateFile, and the shared limit constants.
 * [POS]: the sole frontend boundary for the R2 direct-upload flow. Components in
 *          @/components/FilesUpload*.tsx call these; nothing else in the app may
 *          PUT to object storage. The flow follows docs/CLI_ORCHESTRATION.md:
 *          POST /api/v1/files/upload-url → browser PUT to R2 → POST
 *          /api/v1/files/:id/confirm → poll GET /api/v1/jobs/:id.
 * [PROTOCOL]: Update this header on change, then check CLAUDE.md
 */
import { apiFetch, ApiError } from "./api";

// ---- Pinned contracts (CLI 3 builds, this file consumes) ----

export interface UploadUrlResponse {
  file_id: number;
  put_url: string; // presigned R2 URL — the browser PUTs the bytes directly
  expires_in: number; // seconds
}

export interface FileConfirmResponse {
  file_id: number;
  status: string; // "active" once the worker's HeadObject verify lands
  job_id?: string | number;
}

export type JobStatus = "queued" | "processing" | "completed" | "failed";

export interface JobInfo {
  id: string | number;
  type: string;
  status: JobStatus;
  result?: unknown;
  updated_at?: string;
}

export interface DownloadUrlResponse {
  get_url: string;
  expires_in: number; // seconds
}

// The backend gates uploads to these types; the UI pre-validates the same set so
// users never burn a request quota on a rejected file.
export const ALLOWED_FILE_TYPES = ["application/pdf", "image/png", "image/jpeg", "image/webp"] as const;
export const MAX_FILE_SIZE_BYTES = 25 * 1024 * 1024; // 25 MiB, matching the backend cap

/** Returns an error message, or null when the file passes both gates. */
export function validateFile(file: File): string | null {
  if (!ALLOWED_FILE_TYPES.includes(file.type as (typeof ALLOWED_FILE_TYPES)[number])) {
    return "Unsupported file type. Use a PDF, PNG, JPEG, or WebP file.";
  }
  if (file.size > MAX_FILE_SIZE_BYTES) {
    return "File is larger than 25 MB. Compress or split it and retry.";
  }
  return null;
}

/** Step 1 — ask the backend for a presigned R2 PUT url (owner_id comes from the token). */
export function requestUploadUrl(fileName: string, mimeType: string, sizeBytes: number): Promise<UploadUrlResponse> {
  return apiFetch<UploadUrlResponse>("/api/v1/files/upload-url", {
    method: "POST",
    body: { file_name: fileName, mime_type: mimeType, size_bytes: sizeBytes },
  });
}

/** Step 2 — PUT the raw bytes straight to R2. XHR (not fetch) so upload progress
 *  is observable; R2 serves CORS for the signed URL. Progress: 0–100 integer. */
export function putFileToR2(
  putUrl: string,
  file: Blob,
  onProgress?: (percent: number) => void,
  signal?: AbortSignal
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", putUrl);
    if (xhr.upload) {
      xhr.upload.onprogress = (e: ProgressEvent) => {
        if (e.lengthComputable && onProgress) onProgress(Math.round((e.loaded / e.total) * 100));
      };
    }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new Error(`Upload to storage failed with status ${xhr.status}`));
    };
    xhr.onerror = () => reject(new Error("Upload to storage failed (network or CORS error)"));
    xhr.onabort = () => reject(new DOMException("Aborted", "AbortError"));
    const abort = () => xhr.abort();
    if (signal) {
      if (signal.aborted) abort();
      else signal.addEventListener("abort", abort, { once: true });
    }
    xhr.send(file);
  });
}

/** Step 3 — tell the backend the PUT landed; it verifies the object (HeadObject). */
export function confirmFile(fileId: number): Promise<FileConfirmResponse> {
  return apiFetch<FileConfirmResponse>(`/api/v1/files/${fileId}/confirm`, { method: "POST" });
}

/** Short-lived URL for downloading one's own file (IDOR-gated server-side). */
export function getDownloadUrl(fileId: number): Promise<DownloadUrlResponse> {
  return apiFetch<DownloadUrlResponse>(`/api/v1/files/${fileId}/download-url`);
}

/** One read of a background job (confirm may enqueue the worker's verify task). */
export async function getJob(jobId: string | number, signal?: AbortSignal): Promise<JobInfo> {
  if (signal?.aborted) throw new DOMException("Aborted", "AbortError");
  try {
    return await apiFetch<JobInfo>(`/api/v1/jobs/${encodeURIComponent(String(jobId))}`);
  } catch (err) {
    if (signal?.aborted) throw new DOMException("Aborted", "AbortError");
    throw err;
  }
}

/** Step 4 — poll a job with exponential backoff (2 s base, ×1.6, capped at 10 s,
 *  hard stop after maxMs). Resolves on completed/failed; resolves with the latest
 *  snapshot when the budget runs out so callers decide what "still busy" means. */
export interface PollJobOptions {
  baseDelayMs?: number; // default 2000
  capDelayMs?: number; // default 10000
  maxMs?: number; // default 10 minutes
  signal?: AbortSignal;
  onTick?: (job: JobInfo) => void;
}

export async function pollJob(jobId: string | number, opts: PollJobOptions = {}): Promise<JobInfo> {
  const { baseDelayMs = 2000, capDelayMs = 10000, maxMs = 10 * 60 * 1000, signal, onTick } = opts;
  const startedAt = Date.now();
  let delay = baseDelayMs;
  for (;;) {
    if (signal?.aborted) throw new DOMException("Aborted", "AbortError");
    const job = await getJob(jobId, signal);
    onTick?.(job);
    if (job.status === "completed" || job.status === "failed") return job;
    if (Date.now() - startedAt >= maxMs) return job;
    await sleep(delay, signal);
    delay = Math.min(delay * 1.6, capDelayMs);
  }
}

function sleep(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    const t = setTimeout(resolve, ms);
    const onAbort = () => {
      clearTimeout(t);
      reject(new DOMException("Aborted", "AbortError"));
    };
    if (signal) {
      if (signal.aborted) onAbort();
      else signal.addEventListener("abort", onAbort, { once: true });
    }
  });
}

/** Re-exported so UI components catch backend rejections (403 IDOR, 400 caps)
 *  without importing the auth client twice. */
export { ApiError };
