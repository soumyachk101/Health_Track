# Health_Track Modernization Progress

Update this document at the end of every approved phase.

## Status
- [x] Phase 0 — Audit and plan (complete 2026-11-17; delivered for review)
- [x] Phase 1 — Security and correctness (2026-11-18: all three lanes green and board-confirmed; CLI 1 dedupe of `producer.go` landed, full tree `go build`/`go vet`/`go test` + frontend re-verified)
- [ ] Phase 2 — Database integrity and migrations (auth migrations landed under Phase 1; standalone integrity pass not run)
- [x] Phase 3 — Backend modularization and API contracts (2026-11-18: R2 + worker + files/jobs endpoints; tree green)
- [ ] Phase 4 — Frontend architecture (central client + route guards shipped as part of Phase 1.5; broader refactor not started)
- [x] Phase 5 — Files and background jobs (2026-11-18: direct-to-R2 UI + job polling; worker not run against live Redis/PG — deploy-blocked)
- [ ] Phase 6 — Deployment and infrastructure
- [ ] Phase 7 — Reliability and release readiness

> Numbering note: the **research-aligned plan is authoritative** for product order and **supersedes** this kit's generic Phase 0–7 numbering where they conflict. Phase 0 here maps to the research plan's "Reconcile instructions + verified baseline" phase.

## Current phase
**Phase 1 + Phase 3/5 implementation verified 2026-11-18 (4-CLI orchestration).** All four lanes green end-to-end and all three "done" entries posted to the handoff board (CLI 1 @ 12:05). Final gated re-run of `cd backend && go build ./... && go vet ./... && go test ./...` passes, and `cd frontend && npm test && npm run build` passes (67/67 + 35/35 pages incl. `/files`). One transient red (a duplicate `DispatchMail` in CLI 3's `producer.go` that surfaced after the first gate) was deduped and the suite re-verified green. Integration suite not run (needs a DSN). Next: deployment/infrastructure (Phase 6) + reliability (Phase 7), and a same-site cookie runtime check. See the Completed-work table + Residual-risks below.

### Phase 0 deliverables (this session, 17 Nov 2026) — **complete, delivered for review**
- [x] PHASE0_AUDIT.md — verified baseline + build/test/deploy state + runtime unknowns
- [x] PHASE0_API_INVENTORY.md — full endpoint/permission matrix
- [x] PHASE0_DATABASE_INVENTORY.md — tables, missing FKs, DDL-on-startup, gap tables
- [x] PHASE0_SECURITY_FINDINGS.md — P0/P1/P2 findings S1–S11, each mapped to a fix
- [x] PHASE0_BACKLOG.md — prioritized post-Phase 0 work (Phases 1–5)
- [x] ADR-0001-pure-go-auth.md — Pure Go Auth decision (accepted)
- [x] ADR-0002-r2-object-storage-and-async-worker.md — R2 + async worker decision (accepted)

### Safe local checks run in Phase 0 (exact commands + outcomes)
| Command | Outcome |
|---|---|
| `go build ./...` | pass (exit 0) |
| `go vet ./internal/...` | pass (exit 0) |
| `go test ./internal/...` | pass — 7 packages ok (`config, database, hands, middleware, models, services/ai, services/otp`); `services/email` has no test files |
| `cd frontend && npm test` | pass — Vitest 48/48 tests across 7 files |
| `cd frontend && npm run build` | pass — Next.js production build, all routes static, exit 0 |
| `go test` integration suite (`//go:build integration`) | **not run** — requires a fresh Postgres DSN; Phase 0 forbids production/DB changes |

All findings in PHASE0_SECURITY_FINDINGS.md are marked **confirmed** against checked-out source (S1–S11), with S3 and S11 verified by direct source + full log-grep sweep this session.

### Phase 1 tracking (implementation prompt #1 — Pure Go Auth)
Status: **implemented 2026-11-18** — frontend lane green (CLI 2 posted done); backend lane is on disk and the full tree `go build`/`go test` green, **but CLI 1's board "done" entry was still pending at verification time** (residual risk). Ticks below reflect what is present in the checkout.
- [x] 1.1 `internal/modules/auth/{handler,service,repository,middleware}.go` + `internal/platform/cache/` (Redis/`REDIS_URL`) — on disk (`handler.go`,`service.go`,`repository.go`,`middleware.go`,`google.go`; `cache/{cache,mem,scripts}.go`)
- [x] 1.2 Migrations: `users.role/status/password_hash`, `oauth_accounts`, `patient_consents`, `audit_logs`; remove startup DDL — `0001_baseline.sql` + `0002_pure_go_auth.sql` landed; startup DDL now lives in `migrate.go` (verify retention decision was CLI 1's call)
- [x] 1.3 Access JWT 15m + opaque refresh 14d (Lua rotation, reuse→family invalidation, `user:revoked_before:<id>`), argon2id, atomic hashed OTP/reset — present in `cache/scripts.go` + `service.go` (Lua rotation in cache scripts; not independently unit-asserted here)
- [x] 1.4 `AuthRequired()` + `RequireRoles(...)` + object-level ownership/consent (IDOR), strict JSON binding→400, CORS allowlist, Redis rate limiter — `AuthRequired`/`RequireRoles` in `auth/service.go`+`middleware.go`; rate limiter in `middleware/ratelimit.go`
- [x] 1.5 Frontend: drop `@supabase/supabase-js`, central typed client (`credentials:"include"`, silent 401→refresh), session-based route guards on all four route groups — CLI 2 confirmed: `lib/supabase.ts` deleted, `api.ts` central client, `RouteGuard.tsx` on 4 group layouts, 67/67 tests + build green
- [x] 1.6 AI endpoint controls (auth + quotas + general-assistant instruction + chart-consent) — `share_chart`/emergency/quota references present in `hands/chatbot.go` (CLI 1's AI-controls lane; tree builds green, not independently asserted)
- **Blocking open items (record as decisions before 1.1):** ✅ **both decided 2026-11-17 (product owner)**
  - [x] Same-site vs cross-origin → **same-site, different subdomains** (`app.*` / `api.*`); `SameSite=Lax` HttpOnly cookies work natively, **no BFF**. CORS allowlist-only.
  - [x] Where Redis lives → **Go API + worker move to Cloud Run with managed Redis** (Vercel Go-function replaced for the API; frontend unchanged). See ADR-0001 / ADR-0002 "Platform".

### Phase 3/5 tracking (implementation prompt #2 — R2 + async worker)
Status: **implemented 2026-11-18** — CLI 3 posted done (tree green); worker not run against live Redis/PG (deploy-blocked). Ticks reflect what is present in the checkout.
- [x] 3.1 `internal/platform/storage/` R2 (private, S3-compatible): `upload-url` → `confirm` → `download-url` — `platform/storage/{storage,apifake}.go` + `hands/files_v1.go`; MIME allowlist + 25 MiB cap + key-traversal guards; backed by mocked-S3-API tests
- [x] 3.2 Migrations: `medical_files`, `background_jobs` — `0003_medical_files.sql` + `0004_background_jobs.sql` (owner/patient FKs, status CHECKs, unique task_id/idempotency_key, indexes)
- [x] 3.3 `internal/platform/queue/` + `cmd/worker/main.go` (asynq): `email:send`, `ai:analyze_document`, `files:cleanup_stale`; `GET /api/v1/jobs/:id` — asynq v0.26 on `REDIS_URL`, MaxRetries 5, exp backoff, DLQ retry-archive; consumer covered by seam tests (not run live)
- [x] 3.4 Remove `backend/uploads/` writes + public `r.Static("/uploads", ...)` mount (finding S2) — confirmed: `grep -rn 'r.Static("/uploads"' internal/` returns nothing; no local write path
- [x] 3.5 Frontend direct-to-R2 upload + job polling — CLI 4: `lib/files.ts` + `components/FilesUpload.tsx` + `/files` page; 14/14 lane tests green
- **Blocking open item:** ✅ decided 2026-11-17 — worker + API on **Cloud Run** with managed Redis (ADR-0002 "Platform"). Deployment work lands in this phase, not Phase 6.

## Decisions
| Date | Decision | Owner | ADR |
|---|---|---|---|
| 2026-11-17 | Pure Go Auth: HttpOnly cookies + Redis refresh rotation (reuse→family kill) + 2-layer RBAC/consent; drop Supabase Auth as identity source | product owner + backend lead | ADR-0001 |
| 2026-11-17 | Private Cloudflare R2 pre-signed storage + Redis-backed asynq worker (`email:send`, `ai:analyze_document`, `files:cleanup_stale`); remove local `backend/uploads` | product owner + backend lead | ADR-0002 |

## Completed work

### Phase 0 (2026-11-17, branch `main`, docs-only)
- **Deliverables:** PHASE0_AUDIT.md, PHASE0_API_INVENTORY.md, PHASE0_DATABASE_INVENTORY.md, PHASE0_SECURITY_FINDINGS.md, PHASE0_BACKLOG.md, ADR-0001, ADR-0002 (all in `health_track_claude_implementation_kit/docs/`). No application source, schema, dependency, or deployment file modified — `git status` shows docs-only changes.
- **Commands run and results:**

| Command | Result |
|---|---|
| `go build ./...` (in `backend/`) | pass (exit 0) |
| `go vet ./...` (in `backend/`) | pass |
| `go test ./...` (in `backend/`, unit packages) | pass — `config`, `database`, `hands`, `middleware`, `models`, `services/ai`, `services/otp` ok; `services/email` + `scripts/*` have no test files |
| `cd frontend && npm test` | pass — Vitest 48/48 across 7 files |
| `cd frontend && npm run build` | pass — Next.js static prerender, exit 0 |
| `gofmt -l internal scripts test main.go` | **3 pre-existing unformatted files** (`internal/hands/auth.go`, `internal/services/ai/ai_test.go`, `test/integration_test.go`); not fixed this phase (no app-code changes allowed in Phase 0) |
| `go test` integration suite (`test/integration_test.go`, `//go:build integration`) | **skipped** — requires a fresh Postgres DSN; no live DB in scope, and Phase 0 forbids production/DB changes |
| secret scan of diff/untracked docs (`api_key|secret|password|token =` patterns) | clean — no secrets in the Phase 0 docs |

- **Residual risks / notes:**
  - The Go backend `go test` unit suite does not exercise a real Postgres; only the `integration`-tagged suite does (skipped above).
  - `gofmt` drift in the 3 files above should be folded into Phase 1's first commit (trivial, no behavior change).
  - `golangci-lint` is not installed locally; only `go vet` was run.

### Phase 1 + 3/5 (2026-11-18, branch `phase1-pure-go-auth`, 4-CLI orchestration)
- **Scope:** CLI 1 backend pure-Go auth (`internal/modules/auth/**`, `platform/cache/**`, `migrate.go`, `hands/auth_v1.go`), CLI 2 frontend auth-client + supabase removal + route guards (`frontend/lib/api.ts` central client, `components/guards/**`, `Providers.tsx`), CLI 3 R2 + async worker (`platform/storage/**`, `platform/queue/**`, `cmd/worker/**`, `hands/files_v1.go`+`jobs_v1.go`, migrations 0003/0004), CLI 4 files UI (`lib/files.ts`, `components/FilesUpload.tsx`, `app/(dashboard)/files/page.tsx`) + this cross-stack verification.
- **Commands run and results (final gated pass, 2026-11-18; re-verified after all three "done" entries landed and a transient `producer.go` dup was deduped):**

| Command | Result |
|---|---|
| `cd backend && go build ./...` | **pass** (exit 0, whole tree) |
| `cd backend && go vet ./...` | **pass** (exit 0) |
| `cd backend && go test ./...` | **pass** — `ok` for config, database, hands, middleware, models, platform/queue, platform/storage, services/ai, services/otp; no-test packages: `internal/modules/auth`, `platform/cache`, `services/email`, `cmd/*`, `scripts/*` |
| `cd frontend && npm test` | **pass** — Vitest 67/67 across 8 files |
| `cd frontend && npm run build` | **pass** — Next.js production build, 35/35 pages static incl. `/files`, exit 0 |
| `go test -tags integration` | **not run, needs DSN** — no live local Postgres (`pg_isready` → "no response"); `INTEGRATION_DSN` unset; `.env` points at a remote Supabase pooler. Integration suite must run against a fresh local/dev Postgres |
| `gofmt -l internal cmd main.go` | **6 unformatted files** (CLI 3's `files_v1.go`, `files_v1_test.go`, `storage.go`, `worker/handlers.go` + `hands/auth.go`, `services/ai/ai_test.go`); no behavior change, fold into owner's commit |
| secret-scan of uncommitted diff + new files | **clean** — no credential-shaped values in added lines or new untracked files (`.env`/`.env.example` placeholders only) |

- **Residual risks:**
  1. ~~CLI 1 backend-lane board "done" entry was pending~~ — **resolved**: CLI 1 posted its done entry (12:05); Phase 1.1–1.6 are now board-confirmed and build/test-verified. Note: a post-gate red appeared in CLI 3's `producer.go` (duplicate `DispatchMail`), was deduped, and the full suite re-verified green afterwards.
  2. **Integration suite (DSN)** — the `//go:build integration` suite was not run; no live Postgres available. Must run against a fresh dev DB (not the remote Supabase pooler in `.env`).
  3. **Worker not exercised against live Redis/Postgres** — `cmd/worker` is covered only by seam/unit tests; the consumer path + DLQ + cleanup scheduler run first-time on deploy (Cloud Run).
  4. **Same-site cookie runtime check** — the pure-Go auth relies on HttpOnly `SameSite=Lax/Strict` cookies across `app.*`/`api.*` subdomains. Not verified against the actual deployed frontend+API origins (PHASE0 §4 runtime check still open). ADR-0001 rollout step 2 depends on it.
  5. **Deployment not performed** — Go API + worker target Cloud Run with managed Redis; no deploy/infra change made this phase.
  6. **gofmt drift in 6 files** (above) — cosmetic, fold into the owner's commit.
  7. **`@supabase/supabase-js` fully removed from the frontend** — confirm the Vercel/Netlify deploy has no lingering `NEXT_PUBLIC_SUPABASE_*` env vars or build-time references (`.env.example` is clean; runtime env is out of scope here).

## Known blockers
1. **Postgres DSN for integration tests** — the `test/integration_test.go` suite cannot run without a disposable database; obtain or provision before Phase 1's test gate.
2. **Same-site feasibility check** — whether production frontend (Vercel/Netlify) and API (Vercel Go function) are same-site in production still needs a runtime look at the actual deploy URLs (see PHASE0_AUDIT.md §4). ADR-0001 rollout step 2 depends on it.
3. **`REDIS_URL` / `R2_*` are greenfield** — no Redis client, queue, or storage package exists yet; Phase 1 must introduce the Redis client before refresh-rotation can be implemented/tested.

## Deferred items
- **Postgres integration suite** — deferred from Phase 0 by design (no live DB, no production changes allowed). Must run on a disposable database in Phase 1/2.
- **AGENTS.md relabel/archive** — stale May-2026 Express/Node context; recommended in PHASE0_AUDIT.md §2 but not executed (docs-only housekeeping, safe to defer).
- **gofmt fixes for 3 pre-existing files** — deferred to Phase 1's first commit (see Completed work above).
