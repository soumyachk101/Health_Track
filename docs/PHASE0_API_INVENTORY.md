# Health_Track — Phase 0 API Inventory

Source of truth: `backend/internal/hands/router.go` (route wiring) + `backend/internal/hands/{auth,core,admin,chatbot}.go` (handlers). Verified against checked-out `main`, 17 Nov 2026.

Middleware keys: `jwt` = `middleware.JWTRequired(db,cfg)` (Supabase-JWKS or legacy-HS256 Bearer). `admin` = `middleware.AdminRequired` (jwt + `IsSuperuser`/`UserType=="admin"`). `none` = no auth.

> Note: the two `:id/action` write routes (appointments, service-requests) already do a **one-sided** ownership check (see [security S10](./PHASE0_SECURITY_FINDINGS.md)). Global middleware order: `Recover → StripTrailingSlash → CORS → MaxBodySize(1MiB) → SecurityHeaders → RequestLogger`.

## Public (unauthenticated)

| Route | Handler | Notes |
|---|---|---|
| `GET /` | `router.go:29` | Server banner JSON |
| `GET /api/health` | `router.go:25` | Health check |
| `POST /accounts/api/login` | `auth.go:289` | HS256 issue; email-verified gate |
| `POST /accounts/api/register` | `auth.go:337` | Non-transactional (S6); sends OTP email (blocking) |
| `POST /accounts/api/verify-otp` | `auth.go:604` | Non-atomic consumption (S4) |
| `POST /accounts/api/resend-otp` | `auth.go:692` | Blocking SMTP |
| `POST /accounts/api/forgot-password` | `auth.go:734` | Non-enumerating ✔; blocking SMTP |
| `POST /accounts/api/reset-password` | `auth.go:792` | Does not check `is_used` (S4); non-atomic |
| `POST /accounts/api/google-login` | `auth.go:862` | Google ID-token (JWKS, stdlib) |
| `POST /accounts/api/register-profile` | `auth.go:461` | Supabase onboarding upsert (idempotent) |
| `POST /chatbot/api` | `chatbot.go:21` | **Unauthenticated, blocking AI** (S3) |
| `POST /chatbot/api/tts` | `chatbot.go:48` | **Unauthenticated, blocking TTS** (S3) |
| `GET /uploads/...` | `router.go:23` | **Public static mount** (S2) |

## Authenticated (`jwt`) — patient / core

| Route | Handler | Read/Write |
|---|---|---|
| `GET /api/dashboard` | `core.go` | read (aggregates) |
| `GET /api/profile` | `core.go` | read (own) |
| `POST /api/health-track/add` | `core.go` | write (own) |
| `GET /api/health-track` | `core.go` | read (own) |
| `POST /api/medicines/add` | `core.go` | write |
| `GET /api/medicines` | `core.go` | read (own) |
| `POST /api/prescriptions/add` | `core.go` | write |
| `GET /api/prescriptions` | `core.go` | read (own) |
| `GET /api/mental-health` | `core.go` | **read-only, no create route** (gap) |
| `GET /api/lifestyle` | `core.go` | **read-only, no create route** (gap) |
| `GET /api/insurance` | `core.go` | **read-only, no create route** (gap) |
| `GET /api/past-records` | `core.go` | read |
| `GET /api/appointments` | `core.go` | read |
| `POST /api/appointments` | `core.go:800` | write (books; patient = caller ✔) |
| `POST /api/appointments/:appointment_id/action` | `core.go:839` | write; 1-sided check `doctor_id==user.ID` (S10) |
| `GET /api/service-requests` | `core.go:888` | read; filters by `patient_id`/`provider_id` ✔ |
| `POST /api/service-requests` | `core.go:927` | write (patient = caller ✔) |
| `POST /api/service-requests/:request_id/action` | `core.go:971` | write; 1-sided check `provider_id==user.ID` (S10) |

**Missing write routes (read-only today):** `POST /api/mental-health`, `POST /api/lifestyle`, `POST /api/insurance`. The schema tables exist (`mental_health_logs`, `lifestyle_logs`, `insurance_policies`); only the handlers are absent. Wiring frontend buttons alone is insufficient — backend handlers + tests are required.

## Admin (`admin`)

| Route | Handler | Notes |
|---|---|---|
| `GET /admin-panel/api/stats` | `admin.go` | aggregate stats |
| `GET /admin-panel/api/users` | `admin.go` | user list |
| `POST /admin-panel/api/users/:user_id/action` | `admin.go:156` | approve/suspend; swallowed bind error (S9) |

## NoRoute
`router.go:45-49` → JSON `404 {success:false}` (correct; not 200).

## What the Pure-Go Auth migration adds (ADR-0001, not yet present)
- `GET /api/v1/auth/session` — return the current user + role from the HttpOnly access cookie.
- `POST /api/v1/auth/refresh` — atomic Redis rotation of the opaque refresh token.
- `POST /api/v1/auth/logout` — revoke family + clear cookies.
- `POST /api/v1/auth/otp`, `POST /api/v1/auth/otp/verify`, `POST /api/v1/auth/password/forgot`, `POST /api/v1/auth/password/reset`, `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/google` — same flows, now session-cookie based.
- `GET /api/v1/jobs/:id` — async job status (ADR-0002).
- File endpoints: `POST /api/v1/files/upload-url`, `POST /api/v1/files/:id/confirm`, `GET /api/v1/files/:id/download-url` (ADR-0002).

These are **new** and must coexist with the legacy `/accounts/api` + `/api` surface during migration; the plan keeps legacy routes read-only until the 7-day HS256 tokens fully expire.
