# 4-CLI Parallel Execution — Shared Contract

Four Claude Code CLIs work on ONE checkout in parallel, split by lane. This file is the
single source of truth for **file ownership**, **the pinned API surface**, **shared-file
rules**, and **pacing**. Every CLI reads this + `docs/HANDOFF_STATUS.md` first, codes to
the contracts below (not to another CLI's in-progress code), and posts its result to
`docs/HANDOFF_STATUS.md` when its lane is done.

## Pacing (your global 60 RPM ÷ 4)
- Budget **~15 tool-call requests/minute** per CLI. Practically: batch 2-3 reads per turn,
  do at most ~10-15 tool calls, then **stop and checkpoint** — write a `docs/HANDOFF_STATUS.md`
  entry, note what's left, and end the turn. Resume in the next user turn. This keeps all 4
  under the 60 RPM ceiling and avoids context blowout.
- No subagents inside a CLI (they'd multiply the load and re-trip the limit).
- Never push, deploy, rotate secrets, or run destructive/production commands.

## Pinned API surface (all CLIs code to this; do not renegotiate)
Auth (CLI 1 backend / CLI 3 frontend):
- `GET  /api/v1/auth/session`      → current user + role + status (from HttpOnly cookie).
- `POST /api/v1/auth/login`  `POST /api/v1/auth/register`  `POST /api/v1/auth/google`
- `POST /api/v1/auth/otp`    `POST /api/v1/auth/otp/verify`
- `POST /api/v1/auth/password/forgot`  `POST /api/v1/auth/password/reset`
- `POST /api/v1/auth/refresh` (Path-scoped cookie)  `POST /api/v1/auth/logout`
- Cookies: `ht_access` (JWT 15m, HttpOnly Secure SameSite=Lax), `ht_refresh` (opaque 32B 14d,
  HttpOnly Secure SameSite=Strict, Path=/api/v1/auth/refresh).
- `users.role` ∈ {patient, doctor, provider, admin}; `users.status` ∈ {pending, active, suspended}.
Files + jobs (CLI 2 backend / CLI 4 frontend):
- `POST /api/v1/files/upload-url`  `POST /api/v1/files/:id/confirm`
- `GET  /api/v1/files/:id/download-url`
- `GET  /api/v1/jobs/:id` → `{status: queued|processing|completed|failed, ...}`
- `medical_files.status` ∈ {pending, active, deleted}; R2 pre-signed PUT 5m / GET 2m.

## File ownership (a CLI edits ONLY its own files; shared files below need care)
| Lane | Owns (create/edit) |
|---|---|
| **CLI 1 — Backend Auth** | `backend/migrations/*auth*`, `backend/internal/platform/cache/`, `backend/internal/modules/auth/{handler,service,repository,middleware}.go`, `backend/internal/middleware/*` (RBAC/strict-bind/CORS), `backend/internal/database/postgres.go` (stop startup DDL) |
| **CLI 2 — Backend Storage+Worker** | `backend/migrations/*files*`/`*jobs*`, `backend/internal/platform/storage/`, `backend/internal/platform/queue/`, `backend/internal/modules/files/*`, `backend/cmd/worker/main.go` |
| **CLI 3 — Frontend Auth** | `frontend/lib/api.ts` (+`lib/csrf.ts`), `frontend/lib/supabase.ts` (delete), auth pages, the 4 route-group layouts, `frontend/package.json` (drop supabase dep) |
| **CLI 4 — Frontend Files + Verify** | `frontend/components/FilesUpload*.tsx`, `frontend/lib/files.ts`, upload/job-poll pages, runs all tests/builds, `backend/.env.example`, `frontend/.env.example`, `docs/PROGRESS.md` |

**Shared files (coordinate, don't fight):**
- `backend/go.mod` / `go.sum` — CLI 1 adds `github.com/redis/go-redis/v9` + `golang.org/x/crypto` (argon2 already present). CLI 2 adds `github.com/aws/aws-sdk-go-v2` + `github.com/hibiken/asynq`. **Whichever CLI edits go.mod runs `go get <pkg>` (not hand-edit) so go.sum stays consistent, and re-reads go.mod first.** If both add in the same window, one re-runs `go mod tidy`.
- `backend/main.go` + `backend/internal/hands/router.go` — CLI 1 mounts the `/api/v1/auth` group + wires `AuthRequired`/`RequireRoles` into the middleware chain. CLI 2 mounts `/api/v1/files` + `/api/v1/jobs` **as separate `Group()` blocks** and does not touch CLI 1's lines; CLI 2's worker is a standalone binary (`cmd/worker`), not wired into `main.go`. Both append distinct blocks; last writer re-runs `go build`.
- `backend/internal/hands/*` (legacy handlers) + `backend/internal/services/{ai,email}` — CLI 1 adds auth guards; CLI 2 adds the async offload. If they touch the same file, the later one re-reads it.

## Coordination protocol
1. Each CLI, on start: `git pull` (no-op locally) / `git status`, read this file + `HANDOFF_STATUS.md`, confirm its files are unowned by a *finished* lane.
2. Build to the **pinned API surface above**, never to another CLI's half-written code.
3. On finishing a sub-step: append a dated entry to `docs/HANDOFF_STATUS.md` (what's done, what's left, exact test commands + pass/fail, residual risks).
4. Integration point: CLI 4 runs `go test ./...` (backend), `npm run test` + `npm run build` (frontend) LAST, after CLI 1-3 have posted, and reconciles any cross-lane type drift.
5. One developer (the user) merges CLIs' work; stop at phase boundary for review per `CLAUDE.md`.

## Starting state (as found 17-18 Nov 2026)
- Branch `phase1-pure-go-auth`. Docs at root `docs/` (kit promoted from `health_track_claude_implementation_kit/`).
- A prior subagent left partial, **not-yet-compiling** files: `backend/internal/platform/cache/{cache,mem,scripts}.go`, `backend/internal/modules/auth/{repository,service}.go`. CLI 1 should treat these as scaffolding to complete/fix, not trust them.
- `golang.org/x/crypto` (argon2/bcrypt) and `github.com/golang-jwt/jwt/v5` are already in go.mod; `go-redis`, `aws-sdk-go-v2`, `asynq` are NOT yet.
