# ADR-0001: Pure Go Auth (drop Supabase Auth, Redis refresh rotation + 2-layer RBAC/consent)

- Status: **Accepted** (recorded Phase 0; implementation is a later phase)
- Date: 2026-11-17
- Decision owner: product owner + backend lead
- Source: implementation prompt #1 (this session)

## Context
The backend today runs **two** token paths (finding S5): Supabase GoTrue RS256/ES256 via JWKS and legacy self-issued HS256, resolved by `auth_uuid` or `user_id` (`middleware/jwt.go:280-350`). There is **no** central account-state enforcement (S1), OTP/reset are non-atomic with plaintext codes (S4), rate limiting is per-process (S7), and CORS is over-wide (S8). Supabase Auth is an external dependency the product wants to own end-to-end.

## Decision
Replace `@supabase/supabase-js` + legacy JWT with a **Pure Go** session system:
- **Identity:** single source of truth in `users` (+ `oauth_accounts` for Google). Supabase Auth is dropped as an identity provider; Supabase may remain as a DB host if that is the managed Postgres, but not as the auth engine.
- **Access token:** short-lived JWT, `15m`, claims `ht_access`, issued in an `HttpOnly; Secure; SameSite=Lax` cookie.
- **Refresh token:** opaque `32`-byte, `14d`, `HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth/refresh`.
- **Rotation & reuse detection:** store only SHA-256 hashes in Redis (`rt:<hash>` + `rt_family:<family_id>`). An atomic Lua script on `POST /api/v1/auth/refresh` rotates; a replayed (already-consumed) token **invalidates the whole `family_id`** immediately.
- **Instant revocation:** `user:revoked_before:<user_id>` in Redis so suspension / role change invalidates live access tokens without waiting for the 15m TTL.
- **Passwords:** `argon2id`.
- **OTP / reset:** SHA-256-hashed, single-use, atomic consume, bound to email+purpose+expiry+attempts, enumeration-resistant.
- **CSRF:** `Origin`/`Referer` + `X-CSRF-Token` check on all cookie-authenticated mutations.
- **RBAC (2-layer):** `AuthRequired()` (layer 1: token valid **and** `users.status == "active"`) + `RequireRoles(...)` (layer 2: role/permission). Object-level ownership + `patient_consents` checks live in the service/repository layer so IDOR is impossible by construction.

## Target layout (new packages — none exist yet)
```
backend/
  internal/
    platform/cache/      # Redis client wrapper (REDIS_URL) — NEW
    modules/auth/
      handler.go service.go repository.go middleware.go   # NEW
  migrations/            # versioned SQL (users.role/status/password_hash,
                         #  oauth_accounts, patient_consents, audit_logs)  # NEW
```

## Alternatives considered
1. **Keep Supabase Auth, harden the Go side only** — fastest, but leaves identity in an external system the product wants to own; still needs cookies+refresh. Rejected: does not meet the "own it" requirement.
2. **Opaque DB-stored sessions only (no JWT access token)** — simplest, revocation is trivial (delete row), but every request is a DB hit; fine at current scale, less clean than a short JWT + Redis revocation. Considered; chosen the JWT+Redis model for the horizontal-scaling target.
3. **This ADR (JWT access + Redis-rotated opaque refresh + 2-layer RBAC)** — matches the roadmap's "stateless API + Redis for distributed state" and the horizontal-scaling mission. Selected.

## Consequences
- **New deps:** Redis client (`REDIS_URL`), argon2id (`golang.org/x/crypto/argon2`), a Redis-backed Lua rotation. Redis becomes a hard dependency (shared with the ADR-0002 queue + limiter).
- **Frontend:** `@supabase/supabase-js` removed; central typed API client with `credentials:"include"` + silent 401→refresh + logout redirect; all route groups (`(admin) (doctor) (provider) (dashboard)`) read the unified `GET /api/v1/auth/session`.
- **Migration & rollback:** legacy `/accounts/api` + HS256 tokens stay **read-only** until the 7-day HS256 TTL lapses and Supabase sessions lapse; then removed. A feature flag gates the new cookie path so both coexist. Cutover is additive, not a hard swap.
- **Platform (recorded 2026-11-17, product owner):** Go API + `cmd/worker` move to **Cloud Run** (long-lived processes, clean Redis connection pooling) with **managed Redis**. The Vercel Go-function deployment is superseded for the API only; the Next.js frontend stays on its current host.
- **Cookie posture (recorded 2026-11-17, product owner):** production frontend and API are **same-site, different subdomains** (`app.*` / `api.*`). `SameSite=Lax` HttpOnly access cookies therefore work natively — **no BFF/proxy layer needed**. CORS stays allowlist-only (finding S8); CSRF protection = `SameSite=Lax` + `Origin`/`Referer` checks + `X-CSRF-Token` header on cookie-authenticated mutations, exactly as the refresh-token cookie's `Path=/api/v1/auth/refresh` + `SameSite=Strict` already require.

## Rollout & rollback
1. Ship new `modules/auth` behind a flag; legacy paths unchanged. ✔ additive
2. Frontend flips to the cookie client per-route-group; verify refresh + logout + OAuth + cross-origin (the plan's explicit no-half-migration gate).
3. Wait for legacy HS256 (7d) + Supabase session TTLs to expire; confirm no legacy token in use; then delete the HS256 + Supabase-JWKS code and the `auth_uuid` column (contract phase).
4. Rollback = re-enable the flag (legacy paths still exist until step 3).

## Validation
Go tests: refresh rotation, token-reuse→family-invalidation, `RequireRoles` enforcement, `pending`/inactive account blocking, IDOR (two unrelated users), OTP replay/concurrency, malformed JSON→400. Frontend: silent-refresh + logout redirect + route-guard tests. Run `go test ./...` and `npm run test` + `npm run build`.
