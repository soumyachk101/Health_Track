# 4-CLI Orchestration — Phase 1 (Pure Go Auth) + Phase 2/5 (R2 + Async Worker)

Runbook for 4 parallel Claude Code CLIs working in this one checkout. Each CLI gets exactly
one prompt (from the user) and owns a disjoint file lane. This file is the shared contract:
read it before coding, never edit it.

## Ground rules
1. One shared checkout, disjoint file ownership (table below). If a file is not yours, do not
   edit it. If you need something from another lane, code against the pinned contract in this
   file and record it in HANDOFF_STATUS.md.
2. Pace: max 3–4 parallel tool calls per turn, ~15 API calls/minute per CLI (global 60 RPM).
   On a 429 / rate-limit message: stop, wait ~60s, then continue. No subagents.
3. No `git commit` until your lane is fully green (tests pass). If `index.lock` errors appear,
   wait 30s and retry. The owner commits all lanes at the end.
4. Before starting: read this file + `docs/HANDOFF_STATUS.md` + the two ADRs
   (`docs/ADR-0001-pure-go-auth.md`, `docs/ADR-0002-r2-object-storage-and-async-worker.md`).
   Before finishing: append a handoff block to `docs/HANDOFF_STATUS.md` (format at bottom).
5. Never print secrets. `.env.example` files get placeholders only.

## File ownership (exclusive)
| Lane | Owns (edit only these) |
|---|---|
| **CLI 1 — BACKEND-AUTH** | `backend/internal/modules/auth/**`, `backend/internal/platform/cache/**`, `backend/internal/database/**` (incl. `schema.sql` retention decision), `backend/migrations/**` (auth tables + runner), `backend/main.go`, `backend/internal/config/config.go`, `backend/internal/middleware/**`, `backend/internal/hands/auth_v1.go` (NEW), `backend/internal/hands/router.go` (auth-v1 mount only), `backend/internal/hands/chatbot.go` (AI controls only), `backend/test/**`, `backend/.env.example` |
| **CLI 2 — FRONTEND-AUTH** | `frontend/**` EXCEPT `frontend/lib/files.ts` and `frontend/components/FilesUpload*` (CLI 4), `frontend/.env.example` |
| **CLI 3 — R2+WORKER** | `backend/internal/platform/storage/**`, `backend/internal/platform/queue/**`, `backend/cmd/worker/**`, `backend/internal/hands/files_v1.go` (NEW) + `jobs_v1.go` (NEW), migrations `backend/migrations/0003_medical_files.sql` + `0004_background_jobs.sql` (NEW files only — never edit CLI 1's migration files), `backend/go.mod`/`go.sum` (aws-sdk-go-v2 + asynq only) |
| **CLI 4 — FILES-UI + VERIFY** | `frontend/lib/files.ts`, `frontend/components/FilesUpload*.tsx`, upload/job-poll pages, `docs/PROGRESS.md`, final verification runs (`go test ./...`, `npm test`, `npm run build`) |

Notes:
- `backend/go.mod`: CLI 1 adds `github.com/redis/go-redis/v9` (+ `golang.org/x/crypto/argon2`
  if not present). CLI 3 adds `aws-sdk-go-v2` + `hibiken/asynq`. Different lines; if a
  `go mod tidy` clobber happens, re-add your dep lines and re-tidy.
- `router.go`: CLI 1 mounts `/api/v1/auth/*` from `auth_v1.go`; CLI 3 mounts
  `/api/v1/files/*` + `/api/v1/jobs/*` from `files_v1.go`/`jobs_v1.go`. Two small mount
  additions, different lines.
- The tree may contain partial files from an earlier stalled attempt
  (`backend/migrations/0001..0002`, `backend/internal/platform/cache/*`,
  `backend/internal/database/migrate.go`, modified `config.go`/`postgres.go`). CLI 1 triages
  them: keep what compiles, discard what doesn't. Other lanes ignore them.

## Pinned contracts (build against these, do not re-decide)

### Auth (CLI 1 builds, CLI 2 consumes)
Cookies (same-site subdomains `app.*`/`api.*`; `Secure` always on; No flag for `Secure` in dev):
- `ht_access` — HttpOnly; Secure; SameSite=Lax; Path=/api — JWT (HS256, `JWT_SECRET`), 15 min,
  claims: `sub` (user id, int), `username`, `email`, `role` (admin|doctor|provider|patient),
  `status` (active|pending|suspended), `type`=`ht_access`, `iat`, `exp`, `rev` (revocation epoch).
- `ht_refresh` — HttpOnly; Secure; SameSite=Strict; Path=/api/v1/auth/refresh — opaque 32-byte
  hex, 14 days. Redis stores SHA-256 hashes only: `rt:<hash>` (→ family, user, iat) and
  `rt_family:<family_id>` (SET of live hashes). Atomic Lua rotation; replay of a consumed hash
  deletes the whole family. `user:revoked_before:<user_id>` (epoch seconds, set on
  suspend/role-change) makes `AuthRequired` reject access tokens with `iat < revoked_before`.

Endpoints (group `/api/v1/auth`):
| Method + path | Body | 200/2xx response |
|---|---|---|
| GET `/session` | — (cookie) | `{user:{id,username,email,first_name,last_name,role,status}, access_expires_at:ISO}` or 401 `{success:false,error:"unauthenticated"}` |
| POST `/login` | `{username,password}` | `{success:true,user}` + sets both cookies; wrong creds → 401 `{success:false,error:"Invalid credentials"}` (generic, no enumeration) |
| POST `/register` | `{username,email,password,first_name,last_name,role?,provider_type?,business_name?,license_number?,specialization?}` | 201 `{success:true,message:"Please check your email"}` (always same, enumeration-safe) |
| POST `/otp/verify` | `{email,code,otp_type}` | `{success:true,user}` + cookies |
| POST `/password/forgot` | `{email}` | 202 `{success:true}` always |
| POST `/password/reset` | `{email,code,new_password}` | `{success:true}` |
| POST `/google` | `{credential}` | `{success:true,user}` + cookies |
| POST `/refresh` | — (cookie) | rotates; 401 + clears both cookies on reuse |
| POST `/logout` | — | `{success:true}` + clears + revokes family |

Legacy `/accounts/api/*` + Bearer HS256 stays alive (read-only cutover window). All legacy
handlers get strict JSON binding → 400 on malformed bodies. `AuthRequired()` rejects
`status != active` centrally (findings S1, S10). Chatbot/TTS: `AuthRequired` + per-user Redis
quota + 8 KB message cap; system instruction becomes general-assistant (no diagnosis/
prescribing), emergency-intent branch BEFORE any model call (112/108 copy, non-diagnostic),
chart data shared with AI only when client sends `share_chart:true` (default false, ADR rule).

### Files + jobs (CLI 3 builds, CLI 4 consumes)
| Method + path | Gating | Response |
|---|---|---|
| POST `/api/v1/files/upload-url` | auth + owner (`owner_id` from token, never client) | `{file_id, put_url, expires_in:300}`; MIME ∈ pdf/png/jpeg/webp; size ≤ 25 MiB; creates `pending` row |
| POST `/api/v1/files/:id/confirm` | auth + owner/consent | HeadObject verify → status `active`; `{file_id,status, job_id?}` |
| GET `/api/v1/files/:id/download-url` | auth + owner/consent (IDOR → 403) | `{get_url, expires_in:120}` |
| GET `/api/v1/jobs/:id` | auth + job owner | `{id,type,status:queued\|processing\|completed\|failed, result?, updated_at}` |

Worker: `cmd/worker` (asynq, `REDIS_URL`), tasks `email:send` / `ai:analyze_document` /
`files:cleanup_stale` (purge `pending` > 1h), max retries 5, exp backoff, idempotency key
`<type>:<uuid-or-file-id>`, DLQ via asynq retry-archive. SIGINT/SIGTERM graceful shutdown.
No local `backend/uploads` writes; public `r.Static("/uploads")` removed (CLI 1 keeps it
untouched until CLI 3's mount removal lands — CLI 3 removes the line in `router.go`, its own
lane note: this is the ONE shared-line exception, coordinate via status board).

Env placeholders (both `.env.example`s): `REDIS_URL`, `R2_ACCOUNT_ID`, `R2_BUCKET`,
`R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`.

## Status board
All lanes append to `docs/HANDOFF_STATUS.md`:
```
## YYYY-MM-DD HH:MM — <LANE> — <started|done|blocked>
- did: <what>
- exposes: <exported symbols / endpoints / files the next lane can call>
- needs-from-others: <anything pending, with the contract section it follows>
- tests: <exact commands + pass/fail>
```

## Lane order (recommended)
CLI 1 and CLI 3 start together (disjoint backend lanes). CLI 2 starts immediately (codes
against the auth contract, not against CLI 1's finished code). CLI 4 waits for CLI 1+2+3
status entries, then does the files UI + full verification + PROGRESS.md.
