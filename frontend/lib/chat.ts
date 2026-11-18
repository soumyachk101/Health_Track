/**
 * [INPUT]: depends on apiFetch from @/lib/api (cookie central client) and
 *          pollJob/JobInfo from @/lib/files (shared job-poll backoff:
 *          2 s × 1.6, cap 10 s, 10-min budget, AbortController support).
 * [OUTPUT]: exports pollChat — POST /chatbot/api with both response shapes:
 *          the legacy 200 {response} (degrade mode) and the queue 202
 *          {success, job_id} → poll GET /api/v1/jobs/:id until terminal and
 *          render result.response. Plus ChatJobFailedError for the failed
 *          state (UI shows retry) and ChatJobPendingError for budget
 *          exhaustion while still non-terminal.
 * [POS]: the sole frontend boundary for the AI chat flow. Chatbot.tsx calls
 *          pollChat; nothing else may POST to /chatbot/api. The 202 contract
 *          is the pinned wave-4 surface (CLI 1 builds the backend; when it
 *          is not deployed the legacy 200 path keeps working unchanged).
 * [PROTOCOL]: Update this header on change, then check CLAUDE.md
 */
import { apiFetch } from "./api";
import { pollJob, type JobInfo } from "./files";

export interface ChatCompletion {
  response: string;
  emergency?: boolean;
}

/** The queued job ended in `failed`; the UI should offer a retry. */
export class ChatJobFailedError extends Error {
  jobId: string | number;

  constructor(job: JobInfo) {
    super("The assistant could not answer this. Please retry.");
    this.name = "ChatJobFailedError";
    this.jobId = job.id;
  }
}

/** The poll budget ran out while the job was still queued/processing. */
export class ChatJobPendingError extends Error {
  jobId: string | number;
  status: string;

  constructor(job: JobInfo) {
    super("Your answer is still being prepared. It will finish in the background — come back in a moment.");
    this.name = "ChatJobPendingError";
    this.jobId = job.id;
    this.status = job.status;
  }
}

/** Pinned 202 response: {success, job_id}. Legacy 200: {response, emergency?}. */
interface ChatResponse {
  success?: boolean;
  job_id?: string | number;
  response?: string;
  emergency?: boolean;
  response_emergency?: boolean;
}

export interface PollChatOptions {
  signal?: AbortSignal;
  // Forwarded to pollJob; the defaults already match the files lane.
  baseDelayMs?: number;
  capDelayMs?: number;
  maxMs?: number;
}

/**
 * Send one chat message through the queue-aware flow.
 *
 * - 200 {response}            → returned immediately (degrade mode).
 * - 202 {job_id}             → poll until terminal; completed returns
 *   result.response; failed throws ChatJobFailedError; budget-exhausted
 *   non-terminal throws ChatJobPendingError.
 */
export async function pollChat(
  message: string,
  shareChart: boolean,
  opts: PollChatOptions = {}
): Promise<ChatCompletion> {
  const { signal, baseDelayMs, capDelayMs, maxMs } = opts;
  const data = await apiFetch<ChatResponse>("/chatbot/api", {
    method: "POST",
    body: { message, share_chart: shareChart },
    signal,
  });

  // Legacy synchronous shape — keep working while the queue rolls out.
  if (typeof data.response === "string") {
    return {
      response: data.response || "I didn't quite catch that.",
      emergency: Boolean(data.emergency ?? data.response_emergency),
    };
  }

  if (data.job_id != null) {
    const job = await pollJob(data.job_id, {
      signal,
      baseDelayMs,
      capDelayMs,
      maxMs,
    });
    if (job.status === "failed") throw new ChatJobFailedError(job);
    if (job.status !== "completed") throw new ChatJobPendingError(job);

    // The worker stores {"response": "..."} on the ledger row; jobs_v1
    // unmarshals it into `result` before it reaches the browser.
    const result = (job.result ?? {}) as { response?: string };
    return { response: result.response || "I didn't quite catch that.", emergency: false };
  }

  // Neither shape — treat as an empty answer rather than a hard failure.
  return { response: "I didn't quite catch that.", emergency: false };
}
