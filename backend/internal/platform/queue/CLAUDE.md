# platform/queue/

> L2 | Parent: backend/ (see CLAUDE.md)

Redis-backed async task plane on hibiken/asynq (ADR-0002). Redis holds the
live queue; Postgres `background_jobs` holds the user-visible status
(ledger). The producer degrades safely when REDIS_URL is unset — the app
stays deployable and callers fall back to their sync path.

## Member list

- `tasks.go`: task-type constants (`email:send`, `ai:analyze_document`,
  `files:cleanup_stale`), typed payloads, the task builders with
  idempotency keys + per-type queue/retry/timeout policy, ExponentialBackoff
  (1m*2^n, cap 4h, MaxRetries=5), RetryOutcome (the pure DLQ decision:
  retries >= max → dead), IdempotencyKey (`<type>:<uuid-or-file-id>`).
- `producer.go`: Producer (NewProducer from REDIS_URL, Enqueue with an
  injectable pusher seam, EnqueueScheduled), JobRecord, job lookups
  (JobByTaskID / JobByIdempotencyKey). ErrTaskIDConflict returns the
  existing job, not an error (idempotent re-enqueue).
- `ledger.go`: the Postgres stamps — MarkJob (status+attempts),
  MarkJobStatus (status only), MarkJobResult, EnsureJobRow (crash
  recovery: worker creates the row it missed between push and insert);
  logWarn hook; JobStatus constants (queued/processing/completed/failed/dead).

## Invariants

- Idempotency key == asynq TaskID == background_jobs.idempotency_key:
  one string, three stores.
- The ledger is best-effort: a lost stamp logs, never fails a task that
  itself succeeded.
- `dead` is internal (DLQ archive); the status endpoint collapses it to
  `failed` with attempts carrying the count.

[PROTOCOL]: Update this header on change, then check CLAUDE.md
