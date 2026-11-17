# Phase 0 — Verified Repository Audit

Status: **Draft for review** (Phase 0, docs-only — no application source, schema, dependency, or deployment change made)
Reviewed: 17 November 2026 · branch `main`
Companions: [API inventory](./PHASE0_API_INVENTORY.md) · [Database inventory](./PHASE0_DATABASE_INVENTORY.md) · [Security findings](./PHASE0_SECURITY_FINDINGS.md) · [Backlog](./PHASE0_BACKLOG.md)

## 1. Method

Static review of the checked-out source plus safe local build/test checks. No production environment, live database, secrets, or deployed runtime was touched. Every claim below is tagged **confirmed** (verified against source this session) or **runtime-verification-needed**.

## 2. Verified baseline

| Area | Finding | Tag |
|---|---|---|
| Stack | Go 1.27.3 / Gin backend; Next.js + React 19 / TypeScript frontend; PostgreSQL via pgx/pgxpool | confirmed |
| Layout | Monorepo: `backend/` (module `healthtrack`), `frontend/` (Next.js App Router) | confirmed |
| Auth | Dual token paths: Supabase GoTrue RS256/ES256 (JWKS) + legacy self-issued HS256 (7-day, `JWT_SECRET`); user resolved by `auth_uuid` or `user_id` | confirmed |
| DB | `internal/database/Connect` executes embedded `schema.sql` DDL at startup (`postgres.go:50-55`); no versioned migrations | confirmed |
| Uploads | `r.Static("/uploads", "./uploads")` mounted publicly (`router.go:23`) | confirmed |
| AI | Chatbot + TTS endpoints unauthenticated, no quota (`router.go:102-106`); system prompt self-identifies as "medical assistant" (`ai.go:21`) | confirmed |
| Deploy | Backend is a Vercel Go function (`backend/vercel.json`: `@vercel/go`, `GO117`, port 8000). Frontend has Vercel + Netlify config | confirmed |
| CI | No committed `.github/` workflow; no Docker/Compose | confirmed (runtime check for external CI) |
| Queue/Redis | No Redis client, queue, worker, or object-storage package exists | confirmed |

## 3. Documentation reconciliation (plan tasks 1–5)

- Root `CLAUDE.md` added from the kit. **Done**
- Kit docs now live under root `docs/` (no duplicate nested copy). **Done**
- `research.md` kept at root; named as product-strategy source in `CLAUDE.md`. **Done**
- `IMPLEMENTATION_MASTER_PROMPT.md` still says "backend runs as a Vercel Go function" (line 7) and has no name of `research.md` or the execution plan. **Pending — task 4**
- `AGENTS.md` is the stale May-2026 Express/Node agent-memory dump. **Pending — task 5 (label as historical)**

## 4. Build / test / deploy state (commands run, safe only)

| Check | Command | Result |
|---|---|---|
| Go build | `cd backend && go build ./...` | pass (exit 0) |
| Go vet | `cd backend && go vet ./...` | pass |
| Go unit tests | `cd backend && go test ./...` | pass (7 packages ok; `services/email`, `scripts/*` have no test files) |
| Go integration tests | `cd backend && go test -tags integration ./...` | **not run** — requires a fresh disposable Postgres DSN (`test/integration_test.go`, build-tagged) |
| Frontend tests | `cd frontend && npm test` | pass — Vitest 48/48 across 7 files |
| Frontend build | `cd frontend && npm run build` | pass — static prerender, exit 0 |
| gofmt | `gofmt -l internal scripts test main.go` (in `backend/`) | **3 pre-existing unformatted files**: `internal/hands/auth.go`, `internal/services/ai/ai_test.go`, `test/integration_test.go` |
| golangci-lint | not installed locally | skipped — `go vet` used instead |
| Secret scan of docs diff | grep for `api_key|secret|password|token =` patterns | clean |

## 5. Runtime-verification unknowns

| Unknown | How to resolve |
|---|---|
| Same-site feasibility: whether the deployed frontend (Vercel/Netlify) and API (Vercel Go function) share a registrable domain in production | Check actual deploy URLs; gates the HttpOnly-cookie vs BFF decision in [ADR-0001](./ADR-0001-pure-go-auth.md) |
| Long-lived worker hosting under current Vercel serverless (a Vercel Go function is ephemeral — it cannot run the ADR-0002 Redis worker) | Product decision: container platform (e.g. Cloud Run) for worker/API vs keep Vercel; see [backlog](./PHASE0_BACKLOG.md) blocking items |
| External CI that may already gate `main` | Confirm with repo owner |
| Live schema vs `schema.sql` drift on the production DB | Run migration-baseline check on a copy, never prod |
| `users.auth_uuid` population completeness (Supabase trigger lag) | Query a staging copy |

## 6. Residual risks / notes

- Unit Go suite does not exercise a real Postgres; only the `integration`-tagged suite does.
- `gofmt` drift in the 3 files above should fold into Phase 1's first commit (trivial, no behavior change).
- `golangci-lint` not installed; only `go vet` was run.
