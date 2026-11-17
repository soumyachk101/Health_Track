# Health_Track — Phase 0 Security Findings

All findings verified against checked-out `main` (17 Nov 2026) unless tagged **runtime**. Severity: **P0** = must fix before adding more sensitive workflows; **P1** = core maintainability/correctness; **P2** = operational maturity. Each finding maps to the target module in [ADR-0001](./ADR-0001-pure-go-auth.md) / [ADR-0002](./ADR-0002-r2-object-storage-and-async-worker.md).

## P0 — must fix first

### S1. No central account-state enforcement (P0)
- **Where:** `backend/internal/middleware/jwt.go:361-383`. `authenticate()` resolves the user but never checks `is_active`, `is_approved`, or `is_email_verified`. `AdminRequired` only checks `IsSuperuser`/`UserType=="admin"`.
- **Why it matters:** A suspended (`is_active=0`), unapproved provider, or unverified user can hold a valid token and keep calling every `JWTRequired` route. Frontend role state / Supabase `user_metadata` is the only gate today.
- **Fix map:** Pure-Go `AuthRequired()` + `RequireRoles(...)` middleware that loads `users.status` and rejects non-active centrally (target module: `backend/internal/modules/auth/middleware.go`).

### S2. Public mount of sensitive upload directory (P0)
- **Where:** `backend/internal/hands/router.go:23` — `r.Static("/uploads", "./uploads")`. Serves `backend/uploads/{prescriptions,insurance}` (and anything else dropped in) as **publicly addressable** URLs.
- **Why it matters:** Insurance/prescription documents are patient PII. Anyone who can guess/enumerate the path gets the file. No auth, no ownership check.
- **Fix map:** R2 private storage + pre-signed, authorized download URLs (ADR-0002). Remove the public static mount.

### S3. Chatbot + TTS are unauthenticated, unbounded (P0)
- **Where:** `backend/internal/hands/router.go:102-106` mounts `/chatbot/api` + `/chatbot/api/tts` with **no** `JWTRequired`. Handler `chatbot.go:21-74` only rejects empty input. `ai.Chat` / `ai.TTS` are called **synchronously** (blocking the HTTP request) against Groq→OpenRouter / Sarvam.
- **Why it matters:** Open AI spend is abusable by any IP; no per-user or per-IP quota (the in-memory limiter, see S7, does not cover these).
- **Confirmed (17 Nov 2026):** `backend/internal/services/ai/ai.go:21` — `SystemInstruction` opens "You are HealthTrack+ AI, a helpful and professional **medical assistant**" and ends with a generic "not a substitute for professional medical advice" disclaimer. There is **no** emergency-intent handling, no explicit prohibition on diagnosis/disease-ranking/dosing, and no locale/verification guard — a confirmed conflict with the research plan's general-assistant policy. TTS (`ai.TTS`) is Sarvam-only, hard-coded `hi-IN`.
- **Fix map:** Auth + quotas on AI endpoints; move AI work off the HTTP path into the async worker (ADR-0002 `ai:analyze_document`).

### S4. OTP / password-reset are not atomic (P0)
- **Where:** `backend/internal/hands/auth.go:628-641` (verify-otp: `SELECT ... is_used=0` then a **separate** `UPDATE is_used=1`), `auth.go:821-856` (reset: `SELECT` that **does not check `is_used`**, then `UPDATE users` + a **separate** `DELETE` from `otps`).
- **Storage:** OTP codes are stored as **plaintext** `otp_code TEXT` (`backend/internal/database/schema.sql:51`), and `expires_at` is a **text** field compared lexically.
- **Why it matters:** A replayed or concurrently-raced OTP can be used twice; reset can be reused within the expiry window; partial updates can leave an inconsistent state.
- **Fix map:** Hash OTP/reset tokens (SHA-256), bind to email+purpose+expiry+attempts, consume atomically (`... WHERE is_used=0 RETURNING` / row-lock / delete-returning) in one transaction (ADR-0001, target `modules/auth/service.go`).

### S5. Two active token issuance/verification paths (P0)
- **Where:** Legacy HS256 self-issued tokens — issue `auth.go:249-260` (`generateToken`, 7-day), verify `jwt.go:317-349` (`JWT_SECRET`). Supabase GoTrue RS256/ES256 via JWKS — verify `jwt.go:291-314`, user by `auth_uuid` (`models.go:105-122`).
- **Why it matters:** Two sources of truth for identity. A weak `JWT_SECRET` mints valid user tokens (there is a startup guard in `main.go:30` that refuses the dev default in production — good). Keeping both alive during migration is acceptable only with a planned cutover + rollback.
- **Fix map:** ADR-0001 — one Pure-Go session (HttpOnly access + Redis-rotated refresh). Legacy tokens read-only until their 7-day TTL expires, then removed.

## P1 — core correctness

### S6. Registration is not transactional (P1)
- **Where:** `auth.go:393-432` — `INSERT users` → `INSERT service_providers` → `INSERT otps` are separate ops. A failure mid-way leaves a user without profile or without OTP.
- **Fix map:** wrap in one DB transaction; the new `users`/`oauth_accounts`/consent migrations (ADR-0001) carry this.

### S7. Rate limiting is in-process and auth-only (P1)
- **Where:** `backend/internal/middleware/ratelimit.go:26-31` — `DefaultAuthLimiter` is a process-wide `map[string][]time.Time`. It only fronts the auth endpoints; it does **not** coordinate across Vercel function instances and does **not** cover chatbot/TTS/uploads.
- **Why it matters:** On serverless, per-instance memory means the effective limit scales with instance count. No shared store = no real quota.
- **Fix map:** Redis-backed limiter (reuses the `REDIS_URL` client from ADR-0002), shared across API + worker.

### S8. CORS accepts any Vercel/Netlify subdomain (P1)
- **Where:** `backend/internal/middleware/cors.go:38-43` — allows any `*.vercel.app` and `*.netlify.app` plus `healthtrack.store` subdomains, in addition to the configured allowlist.
- **Why it matters:** A malicious or misconfigured `*.vercel.app` project can be a CORS origin. Overly wide origin policy.
- **Fix map:** explicit origin allowlist from config only; drop the wildcard subdomain suffixes.

### S9. Malformed JSON silently becomes zero-value requests (P1)
- **Where:** swallowed binding errors (`_ = c.ShouldBindJSON(&b)`) at `auth.go:296,353,489,613,700,740,800,869`, `chatbot.go:28,55`, `admin.go:156`, `core.go:354,390,440,802,847,938,979`.
- **Why it matters:** Malformed/oversized JSON degrades to zero-value fields instead of a clean 400; combined with S4 this feeds the OTP/reset logic with empty inputs.
- **Fix map:** a shared strict-binding helper that returns 400 on error (ADR-0001 / ADR-0002, target `shared/validation`).

### S10. IDOR exposure on cross-user action endpoints (P1)
- **Where:** `core.go:849-861` (appointment action — checks `doctor_id != user.ID`), `core.go:981-993` (service-request action — checks `provider_id != user.ID`). The two *action* routes check one side of the relationship. **Not yet verified** that every *list/read* route (`past-records`, dashboard, provider lists) filters on the caller's `user_id`/consent — the `service_requests` list at `core.go:900-904` filters by `patient_id`/`provider_id = user.ID` ✔.
- **Why it matters:** Any read/list route that accepts a client-supplied `user_id` without an ownership/consent predicate is a cross-user data leak.
- **Status:** partially confirmed; full route-by-route IDOR sweep is **runtime/Phase 1** work (two-unrelated-user regression tests).
- **Fix map:** object-level ownership + `patient_consents` predicate in every repository query (ADR-0001 target `modules/auth` object checks + per-domain repository).

### S11. PII / secret handling in logs (P1) — **confirmed**
- **Where (sweep complete, 17 Nov 2026):**
  - `internal/services/email/service.go:143,190,230` — `log.Printf("email: EMAIL_HOST_USER not configured; ... not sent", to)` **logs the recipient email address** (PII) on every config-missing path.
  - `internal/middleware/ratelimit.go:130` — logs the lockout key (client IP + identifier) after 5 failed attempts.
  - `internal/middleware/other.go:45` — request logger is clean (method/path/status/latency only).
  - **No** OTP codes, passwords, tokens, reset links, or health payloads reached any `log.Printf`/`Fatal` site in `internal/hands/`, `internal/services/`, `internal/middleware/`, or `internal/database/` (full grep sweep, non-test files, 17 Nov 2026).
- **Fix map:** redact the recipient email in `email/service.go` (log a hash or nothing); structured redacted logging per ADR-0002.

## P2 — operational

- **P2.1** No committed CI (`.github/` absent) and no Docker/Compose. *Runtime check for external CI.*
- **P2.2** Hardcoded DB pool `MaxConns = 5` (`postgres.go:38`) with no serverless connection-budget doc.
- **P2.3** No versioned migrations; startup runs embedded DDL every boot (see [database inventory](./PHASE0_DATABASE_INVENTORY.md) D1).
- **P2.4** `backend/uploads/` is **not** covered by `.gitignore` (only legacy `server/uploads/*` is) — local files could be committed.

## Summary table

| ID | Severity | Area | One-line | Fix → |
|---|---|---|---|---|
| S1 | P0 | Auth | No central active/approved/verified enforcement | ADR-0001 middleware |
| S2 | P0 | Files | Public `/uploads` static mount | ADR-0002 R2 |
| S3 | P0 | AI | Unauthenticated, unbounded, blocking chatbot/TTS | ADR-0002 worker |
| S4 | P0 | Auth | OTP/reset non-atomic, plaintext, text expiry | ADR-0001 service |
| S5 | P0 | Auth | Dual token paths (Supabase + legacy HS256) | ADR-0001 |
| S6 | P1 | Auth | Registration not transactional | ADR-0001 |
| S7 | P1 | Auth | In-process, auth-only rate limit | ADR-0002 Redis |
| S8 | P1 | Auth | Wildcard Vercel/Netlify CORS | ADR-0001/0002 |
| S9 | P1 | Input | Swallowed JSON binding errors | shared/validation |
| S10 | P1 | AuthZ | Partial IDOR on action routes; read sweep pending | ADR-0001 repos |
| S11 | P1 | Logs | PII/secret log sweep needed | ADR-0002 logging |
