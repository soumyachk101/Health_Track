# cmd/worker/

> L2 | Parent: backend/ (see CLAUDE.md)

The long-lived asynq consumer process (ADR-0002). Separate binary from the
API: both deploy on Cloud Run against shared managed Redis. SIGINT/SIGTERM
drain in-flight tasks (asynq.Server.Shutdown).

## Member list

- `main.go`: process bootstrap — REDIS_URL + DATABASE_URL are hard
  requirements (refuse to start), R2_* is soft (files tasks fail per-task
  until set); builds the asynq server (concurrency 8; queues
  emails:5 / ai:3 / files:1 / default:1; pinned ExponentialBackoff),
  registers the task table, starts the 30m cleanup scheduler, and blocks
  in srv.Run until a signal triggers Shutdown.
- `handlers.go`: the task handlers (email:send → email.Service;
  ai:analyze_document → storage.GetObject + ai.Chat; files:cleanup_stale →
  purge pending > 1h, R2 delete + row mark-deleted), the failureStamp
  middleware (mirrors asynq's retry-vs-DLQ decision onto the ledger:
  failed → dead at MaxRetries), and PII-redacted logging (recipients
  masked as `j***@x.com`; secrets never logged).

## Invariants

- Every handler stamps the ledger around its work (processing → completed
  / failed); the status endpoint stays truthful without Redis access.
- email:send payloads carry structured args (kind/recipient/secret), not
  rendered HTML: a producer/worker version skew fails loudly into the DLQ.

[PROTOCOL]: Update this header on change, then check CLAUDE.md
