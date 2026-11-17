# Health_Track — Prioritized Implementation Backlog (post-Phase 0)

Derived from: the research-aligned plan's delivery order, the two accepted implementation prompts (now [ADR-0001](./ADR-0001-pure-go-auth.md) and [ADR-0002](./ADR-0002-r2-object-storage-and-async-worker.md)), and the [security findings](./PHASE0_SECURITY_FINDINGS.md). Each item is its own reviewable phase; stop at phase boundaries for review.

## Phase 1 — Security & correctness baseline (P0) ← ADR-0001
1. **Auth module** — `internal/modules/auth/{handler,service,repository,middleware}.go` + `internal/platform/cache/` (Redis, `REDIS_URL`).
   - `users.role`/`users.status`/`users.password_hash` (argon2id), `oauth_accounts`, `patient_consents`, `audit_logs` versioned migrations in `backend/migrations/`.
   - Remove startup DDL mutation (see [database inventory D1](./PHASE0_DATABASE_INVENTORY.md)).
   - Access JWT `15m` + opaque refresh `14d` (Lua rotation, reuse→family invalidation, `user:revoked_before:<id>`).
   - `AuthRequired()` + `RequireRoles(...)`; object-level ownership/consent in repositories (closes S1, S10).
   - Atomic hashed OTP/reset (S4); strict JSON binding→400 (S9); registration in one transaction (S6); CORS allowlist-only (S8); Redis rate limiter incl. AI/upload (S7).
   - **Open decision (blocking):** same-site vs cross-origin frontend↔API in production (cookie/BFF feasibility), and where Redis lives under the current Vercel serverless API. Record before coding.
2. **Frontend migration** — remove `@supabase/supabase-js` (`lib/supabase.ts`, 5 auth pages, `Providers.tsx`, `tests/lib/supabase.test.ts`); central typed client with `credentials:"include"` + silent 401→refresh; replace every raw `localStorage 'token'` read across `(dashboard)` pages with the session endpoint; guards on all four route groups.
3. **AI endpoint controls** — auth + per-user/IP quotas + length caps + provider timeouts on `/chatbot/*`; general-assistant system instruction (no diagnosis/prescribing); emergency-intent handling; explicit consent to share chart data (default: none). Closes S3.

## Phase 2 — Data foundation (P1)
4. Versioned migration tool (Goose or golang-migrate — product decision), fresh-DB + legacy-DB baselining strategy.
5. FKs + `CHECK` status enums + typed date fields (settle timezone first) in expand→backfill→validate→contract migrations; index on `insurance_policies.end_date`, `otps(email,otp_type,expires_at)`, `appointments(doctor_id,date)`.
6. Configurable DB pool + documented Postgres connection budget; pagination contract on all growing lists.

## Phase 3 — Private storage + async worker (P0/P1) ← ADR-0002
7. `internal/platform/storage/` (R2, S3-compatible, private bucket): `upload-url` → `confirm` → `download-url` with MIME/size/ownership/consent gates; `medical_files` + `background_jobs` migrations.
8. `internal/platform/queue/` + `cmd/worker/main.go` (asynq, `REDIS_URL`): `email:send`, `ai:analyze_document`, `files:cleanup_stale`; retries/backoff/idempotency/DLQ; `GET /api/v1/jobs/:id`.
9. Remove all `backend/uploads/` writes + the public `r.Static("/uploads", ...)` mount (closes S2).
10. Frontend direct-to-R2 upload + async job polling.
11. **Open decision (blocking):** process host for the long-lived worker (and possibly API) vs current Vercel serverless — container platform decision.

## Phase 4 — Product MVP (research-backed; after P0/P1 closed)
12. `POST /api/mental-health`, `/api/lifestyle`, `/api/insurance` create routes + validated frontend forms (close the read-only API gap; see [API inventory](./PHASE0_API_INVENTORY.md)).
13. Insurance expiry computed from `end_date` + secure document view/download via R2 (Phase 3).
14. Medicine schedules + taken/skipped dose log (keep `frequency` for compat).
15. Free CSV + doctor-ready PDF export (formula-injection-safe, user-scoped).
16. Dashboard sleep-hours summary + data-timestamp labeling.

## Phase 5 — Household / consent / scaling
17. Caregiver sharing (view-only default, scoped grants, expiry + revoke, audited via `audit_logs`).
18. Appointment status-transition + slot-booking concurrency; store post-visit notes; price/cancellation only after product decision.
19. CI (Go fmt/vet/test, frontend lint/type/test/build, dep + secret scan), structured redacted logs, metrics, backup/restore tests, shared Redis quota store.
20. Health Connect / ABDM integrations — **deferred** until native bridge / legal review.

## Suggested next move (after Phase 0 review)
Start **Phase 1 step 1** (auth module + migrations). It unblocks steps 2–3 and is the P0 gate before adding more sensitive data workflows. The **blocking open items** are the two platform decisions flagged above (same-site cookie feasibility; worker hosting) — record them as decisions *before* Phase 1 coding starts.

## Effort note
Phase 0 confirmed the app is testable (unit suites exist, Vitest wired; integration suite needs a DSN) and that the read-only API gap is a wiring job, not a schema job. Do not estimate fixed hours now; re-estimate after Phase 1's open items are decided.
