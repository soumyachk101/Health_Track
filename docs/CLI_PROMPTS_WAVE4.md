# 6-CLI Prompts — WAVE 4 (queue-chatbot-and-otp-mailer branch)

Branch: `queue-chatbot-and-otp-mailer`. Verified at writing: tree clean+idle,
`go build/vet/test` green, frontend 67/67 + build green. OTP mailer is already
queue-wired (`auth.go` uses `queue.DispatchMail`). The chatbot still blocks the
HTTP request on the AI call (`chatbot.go:106` `ai.ChatWithInstruction`).
The 5 wave-3 fix-pass defects are STILL OPEN (cache tests, gofmt ×5, uploads
gitignore, AI-UI tests, DSN note).

## RPM math (all 6 CLIs must obey)
Shared ceiling: **60 API calls/minute**. 6 CLIs → **~10 tool calls/minute each**.
On any rate-limit error: stop, wait ~60s, resume. No subagents (they multiply
load). No pushing, deploying, or merging from any CLI.

**gstack (all CLIs):** gstack plugin + skills are installed on this machine.
- EVERY CLI: end its lane by invoking the gstack **`/verify`** skill in its own
  session. In this repo `detect` reports no declared checks, so when /verify
  does nothing, the lane's own build/test commands ARE the evidence — put them
  in the board entry.
- CLI 6 only: after the gate is green, invoke gstack **`/ship`** (review +
  commit + PR; instruct it to STOP after PR creation — no deploy, no
  `/land-and-deploy`, no merge).
- Do NOT invoke other gstack skills (`/qa`, `/office-hours`, `/cso` …). If a
  gstack preamble asks about telemetry/upgrade, accept the shown default and
  move on — one call max.

**Shared rules:** read `docs/CLI_HANDOFF.md` FIRST (the MAIN 13:3x entry
supersedes older lane maps); append one dated entry when your lane is done;
message the 5 sibling sessions when done or blocked; CLIs 1–5 leave their work
UNCOMMITTED (CLI 6 ships it via /ship); re-read shared files (`router.go`,
`.gitignore`) immediately before editing them; keep edits on disjoint lines.

Lane map (disjoint):
| CLI | Lane |
|---|---|
| 1 | chat-queue backend: `queue/tasks.go`, `cmd/worker/handlers.go`, `hands/chatbot.go`, `hands/jobs_v1.go`, `router.go` (1 line), `backend/test/`, gofmt `cmd/worker/handlers.go` |
| 2 | `backend/internal/platform/cache/` tests only |
| 3 | backend hygiene: gofmt ×4, `backend/uploads/` + `.gitignore`, DSN/integration note |
| 4 | frontend only (chat-poll UI, AI-UI specs, nav) |
| 5 | docs + read-only audit (`docs/PROGRESS.md`, board, supabase/uploads/secret scans) |
| 6 | cross-stack gate + gstack `/verify` + `/ship` |

---

## PROMPT 1 — CLI 1: async chatbot via queue (backend, chat lane)

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker, branch queue-chatbot-and-otp-mailer.
READ FIRST: docs/CLI_HANDOFF.md (latest MAIN entry), docs/ADR-0002-r2-object-storage-and-async-worker.md.
PACING: ~10 tool calls/min (6 CLIs share a 60 RPM ceiling); pause 60s on
rate-limit. No subagents. Do NOT commit (CLI 6 ships). Edit ONLY your lane:
backend/internal/platform/queue/tasks.go, backend/cmd/worker/handlers.go,
backend/internal/hands/chatbot.go, backend/internal/hands/jobs_v1.go,
backend/internal/hands/router.go (ONE shared line: producer into
ChatbotRouter), backend/test/ (new tests).

TASK — move the chatbot AI call off the HTTP path (ADR-0002 step 4):
1. tasks.go: add TypeChat = "ai:chat" (queue "ai", same retry/timeout policy
   as ai:analyze_document), a ChatPayload {message, share_chart, instruction}
   builder + idempotency key "ai:chat:<uuid>". Keep existing task types intact.
2. chatbot.go POST /chatbot/api: emergency-intent reply + quota check stay
   SYNCHRONOUS (never enqueue an emergency). After quota passes: if the queue
   producer is configured → enqueue ai:chat (owner = authenticated user id)
   and respond 202 {success:true, job_id}. If REDIS_URL is unset (degrade
   mode) → run ai.ChatWithInstruction inline and respond 200 {success:true,
   response} exactly like today, so the app stays deployable without Redis.
   TTS (/chatbot/api/tts) stays synchronous — record that as a known
   limitation on the board.
3. worker/handlers.go: register + implement handleAiChat: call the AI service
   with the pinned general-assistant instruction; on success stamp
   background_jobs.result = {"response": text} via ledger MarkJobResult,
   MarkJob completed; on failure MarkJob failed + last_error. gofmt this file
   (it's in your lane; the other 4 gofmt files are CLI 3's).
4. jobs_v1.go writeJobResponse: ADD "result" field to the 200 payload when
   background_jobs.result is non-null (additive — existing clients
   unaffected).
5. Migration: background_jobs.result already exists (0004). Do NOT add a new
   migration unless the column check proves otherwise; record what you found.
6. Tests (backend/test/ or in-package): enqueue → 202 + ledger row;
   degrade-mode inline 200; emergency never enqueued; handleAiChat stamps
   result on success/failure; jobStatus returns result when completed.
7. VERIFY loop: cd backend && go build ./... && go vet ./... && go test
   -count=1 ./... green.
8. Finish: run gstack /verify in this CLI (attach its output or the "no
   declared checks" note). Append CLI_HANDOFF.md entry ("CLI 1 chat-queue
   lane done", exact commands + results + the job_id contract for CLI 4) and
   message CLI 4.
```

## PROMPT 2 — CLI 2: cache test coverage (backend, auth lane)

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker, branch queue-chatbot-and-otp-mailer.
READ FIRST: docs/CLI_HANDOFF.md (latest MAIN entry), docs/ADR-0001-pure-go-auth.md.
PACING: ~10 tool calls/min (6 CLIs share 60 RPM); pause 60s on rate-limit.
No subagents. Do NOT commit. Edit ONLY: backend/internal/platform/cache/
(new cache_test.go). Do NOT touch worker/handlers.go or chatbot.go.

TASK:
1. NEW backend/internal/platform/cache/cache_test.go against NewMemCache:
   Set/Get + TTL expiry, SetNX second-call-false, Incr window (first incr
   sets TTL), Del; then drive the rotation API in scripts.go end-to-end:
   happy-path refresh rotation; REPLAY of a consumed token invalidates the
   whole family; user:revoked_before:<user_id> revokes a live access token
   (the two ADR-0001 Validation items not yet asserted).
2. VERIFY loop: cd backend && go test -count=1 ./internal/platform/cache/...
   green, and go build ./... still green (your tests must not break other
   packages — don't change any exported API; tests only).
3. Finish: run gstack /verify (attach output or "no declared checks"). Append
   CLI_HANDOFF.md entry ("CLI 2 cache-test lane done", test names + results)
   and message CLI 6.
```

## PROMPT 3 — CLI 3: backend hygiene (gofmt + uploads + DSN note)

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker, branch queue-chatbot-and-otp-mailer.
READ FIRST: docs/CLI_HANDOFF.md (latest MAIN entry), docs/PHASE0_SECURITY_FINDINGS.md (P2.4, S7).
PACING: ~10 tool calls/min (6 CLIs share 60 RPM); pause 60s on rate-limit.
No subagents. Do NOT commit. Edit ONLY: the 4 gofmt files below, .gitignore,
backend/uploads/. Do NOT touch cmd/worker/handlers.go or chatbot.go (CLI 1's
lane).

TASKS:
1. gofmt -w ONLY these 4 files (gofmt -l currently flags 5 total; the 5th is
   CLI 1's): backend/internal/hands/files_v1.go,
   backend/internal/hands/files_v1_test.go,
   backend/internal/platform/storage/storage.go,
   backend/internal/services/ai/ai_test.go. Confirm `gofmt -l internal cmd`
   shows no files in YOUR lane when done (CLI 1's file may still show until
   CLI 1 lands).
2. backend/uploads/: public mount already removed; dir holds 2 .gitkeep only.
   Decide: delete the dir AND add `backend/uploads/` to .gitignore
   (recommended — closes P2.4), or keep dir + gitignore line. Record the
   choice + reason on the board.
3. DSN: check backend/.env for a reachable Postgres DSN. If reachable: run
   `go test -tags integration ./test/...` and record pass/fail. Else record
   exactly "integration suite not run: no reachable DSN" — never claim it.
4. VERIFY loop: cd backend && go build ./... && go vet ./... && go test
   -count=1 ./... green.
5. Finish: run gstack /verify (attach output or "no declared checks"). Append
   CLI_HANDOFF.md entry ("CLI 3 hygiene lane done", gofmt -l result, uploads
   decision, DSN/integration note) and message CLI 6.
```

## PROMPT 4 — CLI 4: frontend chat-poll UI + AI-UI test coverage

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker, branch queue-chatbot-and-otp-mailer.
READ FIRST: docs/CLI_HANDOFF.md (esp. CLI 1's job_id contract entry when it
lands; if missing, poll the board every ~2 min, max 10 checks, then code to
the pinned 202 {success, job_id} contract anyway), docs/PHASE0_SECURITY_FINDINGS.md (S3).
PACING: ~10 tool calls/min (6 CLIs share 60 RPM); pause 60s on rate-limit.
No subagents. Do NOT commit. Edit ONLY frontend/: components/Chatbot.tsx,
lib/, tests/, components/layout/nav-config.ts. No backend files.

TASKS:
1. Chat polling: when POST /chatbot/api answers 202 with job_id, poll
   GET /api/v1/jobs/:id until completed/failed (reuse the backoff pattern
   from lib/files.ts pollJob: 2s × 1.6, cap 10s, 10-min budget,
   AbortController); render job.result.response as the assistant bubble; on
   failed show retry. The legacy 200 {response} shape must keep working
   (degrade mode). Emergency card + non-clinical disclaimer + share_chart
   toggle (DEFAULT OFF, send share_chart in body) stay as-is.
2. NEW test coverage (none exists today) in tests/ (new file or extend):
   chatbot spec — share_chart default false / true-when-toggled (mock
   apiFetch), emergency card on server emergency flag, disclaimer rendered,
   poll-flow (202 → poll → completed shows result.response; failed shows
   retry), legacy 200 path.
3. Verify the "/files" ("My Files") item exists in
   components/layout/nav-config.ts (add { icon: FileText, label: "My Files",
   href: "/files" } if absent).
4. Grep frontend/app + frontend/components for raw localStorage token reads;
   report count, fix stragglers in-lane (the session client owns cookies).
5. VERIFY loop: cd frontend && npm test (new specs green) && npx tsc --noEmit
   && npm run build, all green.
6. Finish: run gstack /verify (attach output or "no declared checks"). Append
   CLI_HANDOFF.md entry ("CLI 4 frontend lane done", new test file + names,
   poll-flow contract) and message CLI 6.
```

## PROMPT 5 — CLI 5: docs + read-only audit

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker, branch queue-chatbot-and-otp-mailer.
READ FIRST: docs/CLI_HANDOFF.md (all CLI 1–4 entries when they land), docs/PROGRESS.md,
docs/RELEASE_CHECKLIST.md.
PACING: ~10 tool calls/min (6 CLIs share 60 RPM); pause 60s on rate-limit.
No subagents. Do NOT commit. Edit ONLY: docs/PROGRESS.md + append to
docs/CLI_HANDOFF.md. Everything else is READ-ONLY for you.

TASKS:
1. Audit (read-only, record results, do not fix other lanes' files):
   - supabase scan: `grep -ri supabase frontend/` (source only, no
     node_modules/lockfiles) → must be 0;
   - uploads mount: `grep -rn 'r.Static("/uploads"' backend/` → must be 0;
   - secret scan of `git diff` + all untracked files: REDIS_URL/R2_*/keys
     must be placeholders only, no real key patterns (sk-…, AKIA…, eyJ…);
   - stray artifacts: no backend/worker 9.3MB binary, no build_err.txt.
2. docs/PROGRESS.md: tick ONLY what the board entries + your audit evidence;
   keep the honest "integration suite not run" note when CLI 3 found no DSN;
   add a wave-4 completed-work line with the exact commands + pass/fail from
   the board.
3. Board discipline: confirm each CLI 1–4 entry has done/left/tests/risk
   fields; note gaps on the board (do NOT rewrite other CLIs' entries).
4. Finish: run gstack /verify (audit is your evidence; attach the scan
   results). Append CLI_HANDOFF.md entry ("CLI 5 audit+docs done", audit
   findings table) and message CLI 6.
```

## PROMPT 6 — CLI 6: cross-stack gate + gstack `/verify` + `/ship`

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker, branch queue-chatbot-and-otp-mailer.
READ FIRST: docs/CLI_HANDOFF.md — gate on CLI 1–5 "done" entries (poll every
~2 min, max 15 checks; relay blockers between siblings, do NOT fix their
files; once all five land, proceed), docs/PROGRESS.md.
PACING: ~10 tool calls/min (6 CLIs share 60 RPM); pause 60s on rate-limit.
No subagents. You are the ONLY CLI allowed to commit/PR, and only via /ship.

TASKS:
1. Cross-stack gate (run all, record exact output):
   cd backend && go build ./... && go vet ./... && go test -count=1 ./...
   cd frontend && npm test && npx tsc --noEmit && npm run build
   cd backend && go test -tags integration ./test/... → ONLY with a
   reachable DSN per CLI 3's board note; else record "not run: needs
   disposable Postgres DSN".
2. Confirm the defect list closed: gofmt -l internal cmd clean;
   platform/cache/cache_test.go exists; backend/uploads decision executed;
   /files nav item present; chatbot specs exist; no supabase refs in
   frontend source; no r.Static("/uploads" in router.go; audit findings from
   CLI 5 all GREEN.
3. gstack /verify in this session (attach result), then gstack /ship:
   instruct /ship to review the full pending diff (chat queue lane + cache
   tests + hygiene + frontend + docs), commit with a clear message, open a PR
   on soumyachk101/Health_Track, then STOP — no deploy, no merge, no
   /land-and-deploy. If /ship's review flags issues, fix one-liners in your
   own lane; list the rest on the board for the owner.
4. Final CLI_HANDOFF.md entry: per-item GREEN/RED table, exact commands +
   results, PR link, residual risks (live-Redis e2e, Cloud Run deploy =
   Phase 6, same-site cookie check vs real deploy URLs, TTS still
   synchronous). Message the other 5 sessions: "gate <green|red>; PR
   <link>."
```

---

## Launch order (you)
1. CLIs 1–5 start together (all lanes disjoint: 1=chat queue, 2=cache
   tests, 3=hygiene, 4=frontend, 5=docs/audit). CLI 6 starts too and
   self-gates on the five "done" board entries, then gate + /verify + /ship.
2. Watch docs/CLI_HANDOFF.md for 5 lane entries + CLI 6's final GREEN/RED
   table + PR link.
3. Out of scope this wave (note on board, don't start): live-Redis
   end-to-end, Cloud Run deploy (Phase 6), TTS queue-ification, integration
   DSN provisioning.

## If a CLI stalls or the RPM spikes
- Anyone: pause 60s on rate-limit, then continue — never raise the per-CLI
  budget to compensate.
- Me (main): if total traffic looks hot, start CLI 6 LAST (it's a gate, not
  a producer) and run CLIs 1–4 + 5 in a 3+2 wave (1,2,3 first; 4,5 after
  the first three post) — 3 × 10 = 30 RPM per half-wave, comfortably under 60.
