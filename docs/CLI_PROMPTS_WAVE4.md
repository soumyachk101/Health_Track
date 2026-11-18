# 4-CLI Prompts — WAVE 4 (queue-chatbot-and-otp-mailer branch)

Branch: `queue-chatbot-and-otp-mailer`. Verified at writing: tree clean+idle,
`go build/vet/test` green, frontend 67/67 + build green. OTP mailer is already
queue-wired (`auth.go` uses `queue.DispatchMail`). The chatbot still blocks the
HTTP request on the AI call (`chatbot.go:106` `ai.ChatWithInstruction`).
The 5 wave-3 fix-pass defects are STILL OPEN (cache tests, gofmt ×5, uploads
gitignore, AI-UI tests, DSN note).

**gstack (all CLIs):** gstack plugin + skills are installed on this machine.
- EVERY CLI: end its lane by invoking the gstack **`/verify`** skill in its own
  session. In this repo `detect` reports no declared checks, so when /verify
  does nothing, the lane's own build/test commands ARE the evidence — put them
  in the board entry.
- CLI-D only: after the gate is green, invoke gstack **`/ship`** (review +
  commit + PR; instruct it to STOP after PR creation — no deploy, no
  `/land-and-deploy`, no merge).
- Do NOT invoke other gstack skills (`/qa`, `/office-hours`, `/cso` …). If a
  gstack preamble asks about telemetry/upgrade, accept the shown default and
  move on — one call max.

**Shared rules:** ≤ ~15 tool calls/min per CLI (shared 60 RPM); pause ~60s on
rate-limit; no subagents; read `docs/CLI_HANDOFF.md` FIRST, append one entry
when done; message the 3 sibling sessions (they show in ListAgents as the
other 3 CLIs); CLIs A/B/C leave their work UNCOMMITTED (CLI-D commits via
/ship); re-read shared files (`router.go`, `go.mod`, `.gitignore`) right
before editing.

---

## PROMPT A — CLI A: async chatbot via queue (backend, chat lane)

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker, branch queue-chatbot-and-otp-mailer.
READ FIRST: docs/CLI_HANDOFF.md, docs/ADR-0002-r2-object-storage-and-async-worker.md.
PACING: ~15 tool calls/min; pause 60s on rate-limit. No subagents. Do NOT commit
(CLI-D ships). Lane (edit ONLY): backend/internal/platform/queue/tasks.go,
backend/cmd/worker/handlers.go, backend/internal/hands/chatbot.go,
backend/internal/hands/jobs_v1.go (additive result field),
backend/internal/hands/router.go (ONE shared line: producer into ChatbotRouter),
backend/test/ (new tests).

TASK — move the chatbot AI call off the HTTP path (ADR-0002 step 4):
1. tasks.go: add TypeChat = "ai:chat" (queue "ai", same retry/timeout policy as
   ai:analyze_document), a ChatPayload {message, share_chart, instruction}
   builder + idempotency key "ai:chat:<uuid>". Keep existing task types intact.
2. chatbot.go POST /chatbot/api: emergency-intent reply + quota check stay
   SYNCHRONOUS (never enqueue an emergency). After quota passes: if the queue
   producer is configured → enqueue ai:chat (owner = authenticated user id) and
   respond 202 {success:true, job_id}. If REDIS_URL is unset (degrade mode) →
   run ai.ChatWithInstruction inline and respond 200 {success:true, response}
   exactly like today, so the app stays deployable without Redis.
   TTS (/chatbot/api/tts) stays synchronous — record that as a known limitation.
3. worker/handlers.go: register + implement handleAiChat: call the AI service
   with the pinned general-assistant instruction, on success stamp
   background_jobs.result = {"response": text} via ledger MarkJobResult and
   MarkJobResult completed; on failure MarkJobResult failed + last_error.
4. jobs_v1.go writeJobResponse: ADD "result" field to the 200 payload when
   background_jobs.result is non-null (additive — existing clients unaffected).
5. Migration: background_jobs.result already exists (0004). Do NOT add a new
   migration unless the column check proves otherwise; record what you found.
6. Tests: enqueue → 202 + ledger row; degrade-mode inline 200; emergency never
   enqueued; handleAiChat stamps result on success/failure; jobStatus returns
   result when completed.
7. VERIFY loop: cd backend && go build ./... && go vet ./... && go test -count=1
   ./... green.
8. Finish: run gstack `/verify` in this CLI (attach its output or "no declared
   checks" note). Append CLI_HANDOFF.md entry ("CLI A chat-queue lane done",
   exact commands + results + the job_id contract for CLI C) and message CLI C.
```

## PROMPT B — CLI B: backend fix-pass (cache tests + gofmt + uploads)

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker, branch queue-chatbot-and-otp-mailer.
READ FIRST: docs/CLI_HANDOFF.md, docs/ADR-0001-pure-go-auth.md.
PACING: ~15 tool calls/min; pause 60s on rate-limit. No subagents. Do NOT commit.
Lane (edit ONLY): backend/internal/platform/cache/ (new cache_test.go),
backend/internal/hands/files_v1.go + files_v1_test.go,
backend/internal/platform/storage/storage.go,
backend/internal/services/ai/ai_test.go, .gitignore, backend/uploads/.
Do NOT touch cmd/worker/handlers.go (CLI A owns it this wave) or chatbot.go.

TASKS:
1. NEW backend/internal/platform/cache/cache_test.go against NewMemCache:
   Set/Get + TTL expiry, SetNX second-call-false, Incr window (first incr sets
   TTL), Del; then drive the rotation API in scripts.go end-to-end: happy-path
   refresh rotation; REPLAY of a consumed token invalidates the whole family;
   user:revoked_before:<user_id> revokes a live access token (the two
   ADR-0001 Validation items not yet asserted).
2. gofmt -w ONLY your 4 files (gofmt -l currently flags exactly:
   hands/files_v1.go, hands/files_v1_test.go, storage/storage.go,
   services/ai/ai_test.go — the 5th, cmd/worker/handlers.go, is CLI A's).
   Confirm `gofmt -l internal cmd` shows no files in YOUR lane when done.
3. backend/uploads/: public mount already removed; dir holds 2 .gitkeep only.
   Decide: delete the dir AND add `backend/uploads/` to .gitignore (recommended
   — closes PHASE0 P2.4), or keep + gitignore. Record the choice + reason.
4. DSN: check backend/.env for a reachable Postgres. If reachable: run
   `go test -tags integration ./test/...` and record pass/fail. Else record
   exactly "integration suite not run: no reachable DSN".
5. VERIFY loop: cd backend && go build ./... && go vet ./... && go test -count=1
   ./... green.
6. Finish: run gstack `/verify` (attach output or "no declared checks"). Append
   CLI_HANDOFF.md entry ("CLI B fix-pass done", commands + uploads decision +
   DSN note) and message CLI D.
```

## PROMPT C — CLI C: frontend chat-poll UI + AI-UI test coverage

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker, branch queue-chatbot-and-otp-mailer.
READ FIRST: docs/CLI_HANDOFF.md (esp. CLI A's job_id contract entry when it
lands; if missing, poll the board every ~2 min, max 10 checks, then code to the
pinned 202 {success, job_id} contract anyway), docs/PHASE0_SECURITY_FINDINGS.md (S3).
PACING: ~15 tool calls/min; pause 60s on rate-limit. No subagents. Do NOT commit.
Lane (edit ONLY): frontend/ (components/Chatbot.tsx, lib/, tests/,
components/layout/nav-config.ts). No backend files.

TASKS:
1. Chat polling: when POST /chatbot/api answers 202 with job_id, poll
   GET /api/v1/jobs/:id until completed/failed (reuse the backoff pattern from
   lib/files.ts pollJob: 2s × 1.6, cap 10s, 10-min budget, AbortController);
   render job.result.response as the assistant bubble; on failed show retry.
   The legacy 200 {response} shape must keep working (degrade mode).
   Emergency card + non-clinical disclaimer + share_chart toggle (DEFAULT OFF,
   send share_chart in body) stay as-is.
2. NEW test coverage (none exists today) in tests/ (new file or extend):
   chatbot spec — share_chart default false / true-when-toggled (mock apiFetch),
   emergency card on server emergency flag, disclaimer rendered, poll-flow
   (202 → poll → completed shows result.response; failed shows retry), legacy
   200 path.
3. Verify the "/files" ("My Files") item exists in components/layout/nav-config.ts
   (add { icon: FileText, label: "My Files", href: "/files" } if absent).
4. Grep frontend/app + frontend/components for raw localStorage token reads;
   report count, fix stragglers in-lane (the session client owns cookies).
5. VERIFY loop: cd frontend && npm test (new specs green) && npx tsc --noEmit
   && npm run build, all green.
6. Finish: run gstack `/verify` (attach output or "no declared checks"). Append
   CLI_HANDOFF.md entry ("CLI C frontend lane done", new test file + names,
   poll-flow contract) and message CLI D.
```

## PROMPT D — CLI D: cross-stack gate + gstack `/ship` (commit + PR)

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker, branch queue-chatbot-and-otp-mailer.
READ FIRST: docs/CLI_HANDOFF.md — gate on CLI A/B/C "done" entries (poll every
~2 min, max 15 checks; relay blockers between siblings, do NOT fix their files;
if all three land, proceed), docs/PROGRESS.md.
PACING: ~15 tool calls/min; pause 60s on rate-limit. No subagents.
Lane: docs/PROGRESS.md + full-repo verification + the /ship commit. Nobody
else commits this wave.

TASKS:
1. Cross-stack gate (run all, record exact output):
   cd backend && go build ./... && go vet ./... && go test -count=1 ./...
   cd frontend && npm test && npx tsc --noEmit && npm run build
   cd backend && go test -tags integration ./test/... → ONLY with a reachable
   DSN, else record "not run: needs disposable Postgres DSN" (check what CLI B
   found on the board).
2. Confirm the defect list closed: gofmt -l internal cmd clean;
   platform/cache/cache_test.go exists; backend/uploads decision executed
   (dir gone or gitignored); /files nav item present; chatbot specs exist;
   no supabase refs in frontend source; no r.Static("/uploads" in router.go;
   secret-scan `git diff` + untracked files (REDIS_URL/R2_*/keys = placeholders
   only, no real key patterns).
3. docs/PROGRESS.md: tick ONLY what the board + gate evidence support; keep
   the honest "integration suite not run" note when applicable; add the
   wave-4 completed-work line (commands + results).
4. gstack `/verify` in this session, then gstack `/ship`: instruct /ship to
   review the full pending diff (chat queue lane + fix-pass + frontend + docs
   incl. the untracked wave docs), commit with a clear message, open a PR on
   soumyachk101/Health_Track, then STOP — no deploy, no merge, no
   /land-and-deploy. If /ship's review flags issues, fix one-liners in your
   own lane; list the rest on the board for the owner.
5. Final CLI_HANDOFF.md entry: per-item GREEN/RED table, exact commands +
   results, PR link, residual risks (live-Redis e2e, Cloud Run deploy =
   Phase 6, same-site cookie check vs real deploy URLs, TTS still synchronous).
   Message the other 3 sessions: "gate <green|red>; PR <link>."
```

---

## Launch order (you)
1. CLIs A, B, C start together (disjoint lanes: A=chat queue, B=backend
   hygiene, C=frontend). CLI D starts too and self-gates on their board
   "done" entries, then runs gate + /verify + /ship.
2. Watch docs/CLI_HANDOFF.md for 4 entries + CLI D's PR link.
3. Out of scope this wave (say so on the board, don't start): live-Redis
   end-to-end, Cloud Run deploy (Phase 6), TTS queue-ification, integration
   DSN provisioning.
