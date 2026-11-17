# Health_Track Modernization Progress

Update this document at the end of every approved phase.

## Status
- [x] Phase 0 — Audit and plan (complete 2026-11-17; delivered for review)
- [ ] Phase 1 — Security and correctness (in flight)
- [ ] Phase 2 — Database integrity and migrations
- [ ] Phase 3 — Backend modularization and API contracts
- [ ] Phase 4 — Frontend architecture
- [ ] Phase 5 — Files and background jobs
- [ ] Phase 6 — Deployment and infrastructure
- [ ] Phase 7 — Reliability and release readiness

> Numbering note: the **research-aligned plan is authoritative** for product order and **supersedes** this kit's generic Phase 0–7 numbering where they conflict. Phase 0 here maps to the research plan's "Reconcile instructions + verified baseline" phase.

## Current phase
**Phase 0 complete (2026-11-17), delivered for product-owner review. Stopping at the phase boundary per the plan — Phase 1 not started.** No application source, schema, dependency, or deployment change was made in Phase 0 (docs-only, verified via `git status`).

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
Status: **not started — awaiting Phase 0 review + two blocking platform decisions.**
- [ ] 1.1 `internal/modules/auth/{handler,service,repository,middleware}.go` + `internal/platform/cache/` (Redis/`REDIS_URL`)
- [ ] 1.2 Migrations: `users.role/status/password_hash`, `oauth_accounts`, `patient_consents`, `audit_logs`; remove startup DDL
- [ ] 1.3 Access JWT 15m + opaque refresh 14d (Lua rotation, reuse→family invalidation, `user:revoked_before:<id>`), argon2id, atomic hashed OTP/reset
- [ ] 1.4 `AuthRequired()` + `RequireRoles(...)` + object-level ownership/consent (IDOR), strict JSON binding→400, CORS allowlist, Redis rate limiter
- [ ] 1.5 Frontend: drop `@supabase/supabase-js`, central typed client (`credentials:"include"`, silent 401→refresh), session-based route guards on all four route groups
- [ ] 1.6 AI endpoint controls (auth + quotas + general-assistant instruction + chart-consent)
- **Blocking open items (record as decisions before 1.1):** ✅ **both decided 2026-11-17 (product owner)**
  - [x] Same-site vs cross-origin → **same-site, different subdomains** (`app.*` / `api.*`); `SameSite=Lax` HttpOnly cookies work natively, **no BFF**. CORS allowlist-only.
  - [x] Where Redis lives → **Go API + worker move to Cloud Run with managed Redis** (Vercel Go-function replaced for the API; frontend unchanged). See ADR-0001 / ADR-0002 "Platform".

### Phase 3/5 tracking (implementation prompt #2 — R2 + async worker)
Status: **not started — depends on Phase 1 (consent model) + worker-hosting decision.**
- [ ] 3.1 `internal/platform/storage/` R2 (private, S3-compatible): `upload-url` → `confirm` → `download-url`
- [ ] 3.2 Migrations: `medical_files`, `background_jobs`
- [ ] 3.3 `internal/platform/queue/` + `cmd/worker/main.go` (asynq): `email:send`, `ai:analyze_document`, `files:cleanup_stale`; `GET /api/v1/jobs/:id`
- [ ] 3.4 Remove `backend/uploads/` writes + public `r.Static("/uploads", ...)` mount (finding S2)
- [ ] 3.5 Frontend direct-to-R2 upload + job polling
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

## Known blockers
1. **Postgres DSN for integration tests** — the `test/integration_test.go` suite cannot run without a disposable database; obtain or provision before Phase 1's test gate.
2. **Same-site feasibility check** — whether production frontend (Vercel/Netlify) and API (Vercel Go function) are same-site in production still needs a runtime look at the actual deploy URLs (see PHASE0_AUDIT.md §4). ADR-0001 rollout step 2 depends on it.
3. **`REDIS_URL` / `R2_*` are greenfield** — no Redis client, queue, or storage package exists yet; Phase 1 must introduce the Redis client before refresh-rotation can be implemented/tested.

## Deferred items
- **Postgres integration suite** — deferred from Phase 0 by design (no live DB, no production changes allowed). Must run on a disposable database in Phase 1/2.
- **AGENTS.md relabel/archive** — stale May-2026 Express/Node context; recommended in PHASE0_AUDIT.md §2 but not executed (docs-only housekeeping, safe to defer).
- **gofmt fixes for 3 pre-existing files** — deferred to Phase 1's first commit (see Completed work above).
