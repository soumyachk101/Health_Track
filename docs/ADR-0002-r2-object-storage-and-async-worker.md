# ADR-0002: Private Cloudflare R2 object storage + Redis-backed Go async worker

- Status: **Accepted** (recorded Phase 0; implementation is a later phase)
- Date: 2026-11-17
- Decision owner: product owner + backend lead
- Source: implementation prompt #2 (this session)

## Context
Sensitive patient documents live on **local disk** `backend/uploads/{prescriptions,insurance}`, publicly mounted at `GET /uploads` with no auth or ownership (finding S2, P0). AI chat/TTS (Groq→OpenRouter, Sarvam) and SMTP mail (OTP, verification, reset, appointment alerts) are all **synchronous blocking calls inside HTTP handlers** (S3, P0; `services/ai/ai.go`, `services/email/service.go`). On a Vercel serverless function, blocking a request on a third-party API or SMTP dial (15s) is fragile and unbudgeted. There is **no** queue, worker, or object-storage package today.

## Decision
**Object storage:** replace local-disk uploads with **private Cloudflare R2** (S3-compatible, `aws-sdk-go-v2`), browser **direct upload** via short-lived pre-signed PUT URLs, and ownership/consent-gated pre-signed GET URLs. No public static mount.

- `POST /api/v1/files/upload-url` — validate MIME (PDF/PNG/JPEG/WEBP) + size cap + caller ownership → create a **`pending`** `medical_files` row → return a pre-signed PUT URL (`5m` TTL).
- `POST /api/v1/files/:id/confirm` — `HeadObject` verifies the object exists and matches expected size/type → mark `active` → optionally enqueue an `ai:analyze_document` job.
- `GET /api/v1/files/:id/download-url` — RBAC + patient/doctor consent check (no IDOR) → pre-signed GET URL (`2m` TTL).
- **Remove** all filesystem writes to `backend/uploads/` and the `r.Static("/uploads", ...)` mount.

**Async worker:** offload blocking work into a **Redis-backed Go worker** using `hibiken/asynq` on `REDIS_URL` (shared with ADR-0001's refresh/revocation keys and the Redis rate limiter).

- New entrypoint `backend/cmd/worker/main.go`: graceful shutdown on `SIGINT`/`SIGTERM`, structured logs with **PII/health-data redaction**, queue concurrency limits.
- Task types (typed payloads, max retries, exponential backoff, **idempotency keys**, dead-letter handling):
  - `email:send` — OTP, verification, reset, appointment alerts via SMTP (out of the HTTP path).
  - `ai:analyze_document` — Groq/OpenAI/Google/Sarvam with timeouts + failure status updates.
  - `files:cleanup_stale` — periodic purge of `pending` uploads older than 1 hour.
- Status endpoint: `GET /api/v1/jobs/:id` (authenticated) → `queued | processing | completed | failed` so the frontend can poll async AI/report jobs.

**New schema (versioned migrations, `backend/migrations/`):**
- `medical_files(id, owner_id, patient_id, object_key, mime_type, size_bytes, status[pending|active|deleted], created_at)` + FKs + check.
- `background_jobs(id, type, payload, status, idempotency_key, attempts, last_error, enqueued_at, ...)` for status + idempotency.

## Target layout (new packages)
```
backend/
  cmd/worker/main.go        # NEW — separate long-lived process
  internal/
    platform/storage/       # NEW — S3-compatible ObjectStorage (R2)
    platform/queue/         # NEW — asynq producer/consumer
    modules/files/          # NEW — handler/service/repository + consent
```

## Alternatives considered
1. **Keep local disk, add auth + private buckets in Postgres** — no: files in Postgres `bytea` or an unmounted dir doesn't solve the public mount or the Vercel stateless constraint (serverless has no durable local FS between cold starts). R2 is the right private-store primitive.
2. **Move blocking AI/SMTP into the same HTTP goroutine with a timeout** — no: doesn't survive a request, gives no retry/DLQ/idempotency, and still blocks. A queue is required for retries that outlive a request.
3. **This ADR (R2 + asynq worker)** — matches roadmap Phase 5 "private object storage + background worker for work that outlives a request." Selected.

## Consequences
- **New deps:** `aws-sdk-go-v2` (S3/R2), `hibiken/asynq` (+ its Redis driver), `REDIS_URL`. R2 needs `R2_ACCOUNT_ID, R2_BUCKET, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY`.
- **Process model:** the worker is a **long-lived process**, which is *incompatible* with a pure serverless Vercel function. **This is the single biggest architectural decision in the plan and it conflicts with the current Vercel deployment.** Options:
  - Run the worker on a container platform (Cloud Run / a VPS / Fly) while the API stays on Vercel, **or**
  - Move the API **and** worker to a container platform (Cloud Run) so Redis connection pooling is sane for both.
  - **Decision (recorded 2026-11-17, product owner):** the Go API **and** `cmd/worker` run on a container platform — **Cloud Run** — with **managed Redis**. The Redis-backed queue is the measured reason that supersedes the "keep Vercel while stabilizing" guidance *for the Go API only*; the Next.js frontend is not touched. Cloud Run gives the worker its required long-lived process and sane Redis pooling; Vercel serverless is dropped for the API.
- **Frontend:** upload components + central API client switch to request-PUT-URL → direct browser upload to R2 → `/confirm` → poll `/jobs/:id`.
- **Security:** private bucket by default, random non-guessable object keys, MIME + size validation on both PUT and confirm, ownership + consent before any pre-signed URL, short TTLs.

## Rollout & rollback
1. Add `medical_files` + `background_jobs` migrations (expand). ✔ additive
2. Ship `platform/storage` + `platform/queue` + `modules/files` behind flags; **legacy `backend/uploads` + `/uploads` mount kept read-only** until R2 cutover is verified (expand → contract).
3. Frontend flips to direct-to-R2 upload; verify MIME/size, IDOR on download URL, confirm flow.
4. Move SMTP + AI out of handlers into `email:send` / `ai:analyze_document` tasks; verify retry/DLQ/idempotency.
5. Contract: remove local-disk writes + the `/uploads` static mount + `document TEXT` column usage once R2 is authoritative.
6. Rollback = disable flags; legacy upload path still exists until contract.

## Validation
Go tests: R2 interface (mocked), MIME/size validation, IDOR on `/files/:id/download-url`, job-enqueue idempotency, retry + DLQ. Frontend: direct-upload + job-poll tests. `go test ./...`, `npm run test`, `npm run build`. Runbook: `api` and `worker` are **separate** processes; worker restart is safe (idempotent + persisted jobs).
