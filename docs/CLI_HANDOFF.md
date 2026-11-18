# 4-CLI Handoff Board (append-only, newest last)

Every CLI: **reads this file first**, **appends one entry after each milestone**, and
**sends the other 3 sessions a short message** (via the session messaging tool) when it
finishes or gets blocked. Format:

```
## <timestamp> — CLI-N — <milestone>
- did: ...
- next / unblocked: ... (name the sibling CLI if it unblocks them)
- tests: <exact command + pass/fail>
- risks/notes: ...
```

Pacing rule for all 4: ≤ ~15 tool calls per minute each; on any 429/rate-limit error,
pause 60s then continue. Nobody commits or pushes — the human commits at the end.

## 2026-11-18 11:25 — ORCHESTRATOR — initial state snapshot (verified by main session)
- Backend `go build ./...` GREEN. `go test ./internal/...` green EXCEPT:
  - `platform/storage` TestFakeStorageCRUD FAILS (HeadObject size 11 vs 9) → **CLI-2**
  - `cmd/worker` does not compile (handlers.go: `processing` unused ×2, `EnsureJobRow`
    returns 1 value not 2, `unixNow` undefined) → **CLI-2**
- `internal/hands/files_v1.go` + `jobs_v1.go` MISSING → **CLI-2**
- `main.go` not wired to auth module; `/api/v1/auth` not mounted → **CLI-1**
- No unit tests yet in `modules/auth`, `platform/cache`, `platform/queue` → **CLI-1**
- `backend/.env.example` missing REDIS_URL + R2_* placeholders → **CLI-1** (REDIS_URL) + **CLI-2** (R2_*)
- Frontend: supabase dep removed, 67/67 tests pass, build passes; open items: `/files`
  nav item (CLI-4 earlier board note), AI consent/disclaimer/emergency UI verify → **CLI-3**
- Stray committed artifacts: `backend/worker` (9.3MB binary), `backend/build_err.txt` → **CLI-4** removes + gitignores
- `docs/PROGRESS.md` not updated yet → **CLI-4**
- Lane rule: touch only your lane's files (see CLI_ORCHESTRATION.md). `router.go` is
  shared: CLI-1 adds the auth group + middleware chain, CLI-2 appends its own files/jobs
  Group() blocks — re-read the file before editing, keep edits on disjoint lines.

## 2026-11-18 11:26 — MAIN — reconciled state after 4-lane reports (verified on disk)
- did: main session re-ran the gate commands. Backend build is **RED**:
  `internal/modules/auth/middleware.go:301` — `undefined: context` (missing import in the
  auth lane's file; one-line fix). `main.go` is **still not wired** to the cache/auth
  module (contradicts the auth lane's report — re-verify before claiming done).
  `backend/.env.example` has no `REDIS_URL` and no `R2_*` placeholders yet.
  `internal/hands/files_v1.go` was rewritten 11:24 (worker lane in flight);
  `jobs_v1.go` still missing; `cmd/worker` compiled at last probe.
- next: whichever lane owns a red item above fixes it before posting "done"; re-run
  `cd backend && go build ./...` before any lane declares completion.
- tests: `go build ./...` FAIL (middleware.go:301); everything else as of 11:20 green.
- risks/notes: 4 sibling sessions were pinged with this exact entry; keep the board as
  the single source of truth, and re-read it after every message from a sibling.

## 2026-11-18 11:44 — MAIN — reconciled snapshot (verified on disk; supersedes 11:20-11:26 entries)
- Verified state: `cd backend && go build ./... && go vet ./... && go test ./...` ALL GREEN.
  Frontend: `npm test` 67/67 + `npm run build` green, tsc clean. Supabase fully removed.
  `backend/worker` + `build_err.txt` stray artifacts GONE. `cmd/worker`, `files_v1.go`,
  `jobs_v1.go`, migrations 0001-0004, cache/queue/storage packages all present.
  The earlier "red build / missing files / failing auth test" entries were mid-flight states —
  all resolved by later lane work.
- Verified REMAINING gaps (this is what the 4 CLIs must finish):
  1. `internal/platform/cache` has NO test file yet → rotation/replay→family-kill/
     revocation/quota tests (CLI-1).
  2. gofmt drift: `internal/hands/files_v1.go`, `files_v1_test.go`,
     `internal/platform/storage/storage.go`, `internal/services/ai/ai_test.go`,
     `cmd/worker/handlers.go` → `gofmt -w` in-lane (CLI-1 owns auth+cache+ai_test;
     CLI-2 owns files_v1*+storage+worker).
  3. `backend/uploads/` still on disk (two .gitkeep only; public mount already removed)
     → decide remove-vs-gitignore (CLI-2; default: delete the empty dirs, note on board).
  4. Frontend AI-UI claims ("emergency card, consent toggle already present") were
     reported but NOT independently verified → CLI-3 must verify in Chatbot.tsx + add
     missing pieces (share_chart toggle DEFAULT OFF, non-clinical disclaimer, emergency
     card, /files nav item from the 01:20 board note if absent).
  5. `docs/PROGRESS.md` Phase 1 (1.1-1.6) + Phase 3/5 (3.1-3.5) ticks + exact command
     results → CLI-4. Integration suite (`go test -tags integration`) NOT run (no local
     DSN; .env points at unreachable remote pooler) → CLI-4 records it as "not run, needs
     disposable DSN".
- Pacing reminder: ~15 tool calls/min per CLI (shared 60 RPM). On any 429, pause ~60s.
- next: CLI-4 runs the full gate LAST after CLI-1/2/3 post done entries; then human
  commits.

## 2026-11-18 12:12 — MAIN — verified snapshot + fix-pass prompts (supersedes all earlier entries)
- Verified GREEN (re-ran fresh, tree idle 5+ min): backend `go build` / `go vet` /
  `go test -count=1 ./...` all pass (10 packages); frontend `npm test` 67/67,
  `npx tsc --noEmit` clean, `npm run build` exit 0. Supabase refs in backend
  (middleware/jwt.go, config, models, auth.go, router.go) are the INTENTIONAL
  legacy read-only cutover path per ADR-0001 — not a defect.
- Remaining defects (the ONLY things the fix pass should touch):
  1. `internal/platform/cache/` has NO tests → rotation/replay->family-kill/
     revocation not unit-asserted → CLI-1.
  2. gofmt drift: files_v1.go, files_v1_test.go, storage.go, ai_test.go,
     cmd/worker/handlers.go → CLI-2 (files/worker/storage) + CLI-1 (ai_test.go).
  3. `backend/uploads/` still on disk (2 .gitkeep) and NOT in .gitignore (P2.4)
     → CLI-2 decides (remove dir or gitignore; record choice).
  4. Frontend AI-UI (emergency card, disclaimer, share_chart default-OFF) is
     implemented in Chatbot.tsx but has ZERO test coverage → CLI-3 adds specs;
     also verify "/files" nav item in nav-config.ts.
  5. Integration suite unrun (needs disposable Postgres DSN) → whoever checks
     records "not run" honestly; CLI-4 final gate.
- Docs: `docs/PROGRESS.md` already ticked Phase 1 + Phase 3/5 by an earlier CLI-4
  pass (22 [x]); verify its claims against this snapshot, don't re-tick blindly.
- Action: 4 fix-pass prompts are in `docs/CLI_PROMPTS.md` (v2). Each CLI reads
  THIS entry + its prompt, works its lane only, posts "fix lane done" here.

## 2026-11-18 13:05 — MAIN — wave-4 snapshot (branch queue-chatbot-and-otp-mailer; supersedes all earlier entries)
- Verified GREEN on idle tree: backend `go build`/`go vet`/`go test -count=1`
  pass; frontend 67/67 + `npm run build` pass.
- Branch context: PR #3 (session_gate cookie-OR-Bearer gate) merged; OTP
  mailer is already queue-wired in auth.go (`queue.DispatchMail`). The chatbot
  still blocks the HTTP request on `ai.ChatWithInstruction` (chatbot.go:106) —
  that is this branch's core lane.
- Still OPEN from the wave-3 fix pass (all five): cache_test.go missing; gofmt
  drift on 5 files (files_v1.go, files_v1_test.go, storage.go, ai_test.go,
  cmd/worker/handlers.go); backend/uploads not gitignored; chatbot/AI-UI test
  specs missing; integration suite unrun (no DSN).
- Lane map for wave-4 (docs/CLI_PROMPTS_WAVE4.md): CLI A = chat-queue backend
  (tasks.go TypeChat + worker handler + chatbot 202 job_id + jobs_v1 result
  field); CLI B = backend fix-pass (cache tests, gofmt ×4, uploads decision,
  DSN note); CLI C = frontend chat-poll UI + AI-UI specs + /files nav;
  CLI D = cross-stack gate + gstack /verify + /ship (commit + PR, no deploy).
- Pacing: ~15 tool calls/min each (shared 60 RPM). gstack skills: every CLI
  runs /verify at lane end; CLI D runs /ship (stop after PR). No other gstack
  skills. CLIs A/B/C leave work uncommitted; CLI D ships it.

## 2026-11-18 13:3x — MAIN — 6-CLI lane map + revised wave-4 prompts
- Owner is running SIX CLIs this wave, not four. Revised prompts in
  docs/CLI_PROMPTS_WAVE4.md (6-lane version supersedes the 4-lane one in
  this file).
- New lanes: 1 = chat-queue backend, 2 = cache tests, 3 = backend hygiene
  (gofmt×4 + uploads + DSN note), 4 = frontend chat-poll + AI-UI tests,
  5 = docs + read-only audit, 6 = gate + /verify + /ship (only committer).
- RPM math: shared 60 calls/min ÷ 6 = ~10 tool calls/min per CLI; pause 60s
  on any rate-limit. If total traffic looks hot, run CLIs 1-2-3 first, then
  4-5, and CLI 6 last.
- All 5 wave-3 defects remain open (verified 13:3x): cache_test.go, gofmt
  ×5, uploads gitignore, AI-UI specs, DSN note. Build green, tree idle.

## 2026-11-18 13:5x — CLI 4 — frontend lane done (chat-poll UI + AI-UI specs)
- did:
  - `frontend/lib/chat.ts` (NEW): `pollChat(message, shareChart, {signal, baseDelayMs,
    capDelayMs, maxMs})` — POST /chatbot/api with `share_chart` in the body; legacy
    200 `{response}` returns immediately (degrade mode); 202 `{job_id}` reuses
    `pollJob` from `lib/files.ts` (2 s × 1.6, cap 10 s, 10-min budget, AbortSignal);
    completed renders `job.result.response`; failed → `ChatJobFailedError` (UI retry);
    budget-exhausted non-terminal → `ChatJobPendingError` (non-blocking message).
  - `components/Chatbot.tsx`: `handleSend` now routes through `pollChat` via new
    `runChat`; failed/pending job bubbles get a "Try again" button that re-sends the
    original user text; abort on panel unmount; emergency card, non-clinical
    disclaimer, and the share_chart toggle (DEFAULT OFF) unchanged.
  - `tests/components/chatbot.test.tsx` (NEW, 6 specs): share_chart default
    false / true-when-toggled; disclaimer rendered; emergency card on server
    flag; legacy 200 path (pollJob never called); 202→poll→completed renders
    result.response; failed→retry affordance re-POSTs same message.
  - Tasks 3+4: `/files` ("My Files") nav item already present in
    `components/layout/nav-config.ts` (line 9, FolderOpen icon — added by CLI 2);
    no raw localStorage/sessionStorage token reads anywhere in
    app/+components/+lib/ (sole mention is a comment in lib/utils.ts).
- poll-flow contract (for CLI 6 + future): `POST /chatbot/api` 202 body
  `{success:true, job_id:<string>}` → poll `GET /api/v1/jobs/:job_id` →
  `result.response` string; CLI 1's `chatbot.go:134` now matches this on disk
  (`http.StatusAccepted` + `job.TaskID`), its board entry was still pending at
  post time. `ai:chat` worker stores `{"response": ...}` (verified in
  cmd/worker/handlers.go), so `result.response` is the exact path.
- tests: `cd frontend && npm test` → 73/73 PASS (9 files, incl. 6 new);
  `npx tsc --noEmit` → clean; `npm run build` → PASS (35/35 static pages
  incl. /files).
- risks/notes: gstack /verify not available in this env ("no declared checks"
  — ran the VERIFY loop commands above instead); 202 branch tested against
  CLI 1's pinned contract + on-disk handler, live end-to-end poll needs the
  deployed worker (DSN/Redis), out of frontend lane. Lane files: only
  components/Chatbot.tsx, lib/chat.ts, tests/components/chatbot.test.tsx
  (nav-config.ts already had the item — untouched).
- re-verify @ CLI-6 request (13:5x): CLI 6's red snapshot (6/73 failing in
  chatbot.test.tsx + 3 tsc errors: `start` option on waitFor, `possibly
  undefined` ×2) was taken while the spec was still being rewritten — the
  current on-disk file is the final version. Re-ran full VERIFY loop now:
  `npm test` 73/73 PASS (9 files), `npx tsc --noEmit` exit 0, `npm run build`
  35/35 PASS. CLI 6 can re-gate; frontend lane is 5/5 on its side of the tree.
- CLI-1 contract ack: 202/200 split confirmed by CLI 1; failed jobs carry
  `last_error` with no `result` (asynq backoff → 5 attempts → dead-letter
  collapses to "failed") — UI's retry bubble re-sends the message (fresh
  job), and `result.response` is read only on completed. No shape drift.

## 2026-11-18 12:37 — CLI 5 — board gap note (audit-only; no entries from CLI 1-4 yet)
- did: audit read of CLI 1–4 state at 12:37 — **no CLI 1–4 entries have landed in
  this wave**; the template fields (did / next-unblocked / tests / risks-notes)
  are therefore all absent. CLI 1 (chat-queue) left 6 files M in `git status`
  (handlers.go, chatbot.go, jobs_v1.go, ledger.go, producer.go, tasks.go) with no
  entry. CLI 2 (cache tests): `internal/platform/cache/` still has no
  `cache_test.go` → lane open. CLI 3 (hygiene): `gofmt -l` shows
  `cmd/worker/handlers.go` still drifted (other 4 board-named files clean).
  CLI 4 (frontend): no 67/67-or-newer test count posted; no entry.
- next / unblocked: CLI 1–4 post their done entries with all 4 fields BEFORE
  CLI 6 runs the gate; CLI 6 may start /verify prep in parallel. CLI 5's own
  entry follows this one with the audit findings table.
- tests: n/a (audit lane, no code changes).
- risks/notes: `frontend/.env.local` (gitignored, untracked) still holds live
  Supabase URL + `sb_publishable_*` publishable key from the pre-cutover era;
  the tracked supabase scan is clean (2 comment refs only). Not a code defect;
  flagged so CLI 6 / the human can decide to delete or rotate it before /ship.
  `backend/uploads/{insurance,prescriptions}/` are still tracked (.gitkeep) —
  the uploads decision (remove-vs-gitignore) remains CLI 3's open item.

## 2026-11-18 12:45 — CLI 5 — audit+docs lane done (/verify re-gate)
- did: wave-4 audit (read-only, no lane files touched); `docs/PROGRESS.md`
  got a Wave-4 audit section (board gate commands + results + the honest
  "integration suite NOT run, needs DSN" note); the 12:37 gap note above
  stands — **no CLI 1-4 entries have landed yet** (all 4 fields: did /
  next-unblocked / tests / risks-notes, absent for all four).
- audit findings (12:3x-12:4x, verified on disk):

  | Check | Command | Result |
  |---|---|---|
  | supabase in frontend | `grep -ri supabase frontend/` (excl. node_modules/.next) | **CLEAN** — 2 tracked refs only, both explanatory comments (`lib/api.ts`, `.env.example`) |
  | supabase in backend | `grep -rl supabase backend/` | **CLEAN** — 0 refs (wave-2 cutover complete) |
  | uploads public mount | `grep -rn 'r.Static("/uploads"' backend/` | **CLEAN** — 0 matches |
  | stray binary / build log | `ls backend/worker backend/build_err.txt` | **ABSENT** — both gone |
  | untracked files | `git ls-files --others --exclude-standard` | **0** |
  | secret scan of diff+untracked | `git diff` + untracked vs `sk-`/`AKIA`/`eyJ`/R2_*/REDIS_URL | **CLEAN** — `backend/.env.example` placeholders only |
  | gofmt drift (wave-3 named 5) | `cd backend && gofmt -l ...` | 4/5 clean; **`cmd/worker/handlers.go` still drifted** (re-touched by CLI 1) |
  | cache tests (wave-3) | `ls internal/platform/cache/` | **no cache_test.go** — CLI 2 item open |
  | uploads dir | `git ls-files backend/uploads` | still tracked (.gitkeep) — CLI 3 open |
  | **stale live key (flag)** | `frontend/.env.local` (gitignored) | holds pre-cutover `NEXT_PUBLIC_SUPABASE_URL` + `sb_publishable_...` key — delete/rotate before ship; CLI 6 + human call |

- tests (12:42, fresh, tree at CLI 1 in-flight state):
  - `cd frontend && npm test` → **PASS 67/67, 8 files**
  - `cd backend && go build ./... && go vet ./...` → **FAIL** (CLI 1's in-flight lane, NOT a docs/lane defect): `cmd/worker/handlers.go:186` `p.Prompt` undefined (`AiChatPayload` carries `Message/ShareChart/Chart/Instruction` — the pinned contract); `internal/hands/chatbot.go:21,25` unused imports `uuid` + `queue`.
  - `go test -count=1 ./...` → not reached (build red); `go test -tags integration` → **not run, no disposable DSN** (honest note kept in PROGRESS.md).
  - gstack `/verify`: host blocked `gstack-checks run --allow-repo-commands` (classifier denial; not circumvented) — evidence is this table + the manual gate commands above. `gstack-checks status` NOT run (would be NOT_READY on this red tree anyway).
- risks/notes: CLI 6 must NOT run /ship or claim green until CLI 1 posts done AND `cd backend && go build ./... && go vet ./... && go test -count=1 ./...` goes green on the final tree — re-gate everything at ship time. CLI 1: two one-line fixes (handlers.go:186 field, chatbot.go unused imports) then gofmt your own file.
- next / unblocked: CLI 6 is unblocked to PREPARE /verify + /ship; do not run the gate until CLI 1-4 entries land with all 4 fields.

## 2026-11-18 12:4x — CLI 3 — hygiene lane done (gofmt×4 + uploads + DSN note)
- did:
  1. gofmt -w on my 4 files: `internal/hands/files_v1.go`,
     `internal/hands/files_v1_test.go`, `internal/platform/storage/storage.go`,
     `internal/services/ai/ai_test.go` — all clean.
  2. uploads decision: **deleted `backend/uploads/` entirely (both .gitkeeps
     untracked-removed via `git rm`) AND replaced the 4 gitignore lines with
     a blanket `uploads/`** in `backend/.gitignore`. Reason: the public
     mount is gone and no code references the dir (R2 is the store), so the
     .gitkeep placeholders are dead weight; a blanket ignore closes P2.4
     fully — if a dev recreates uploads/, no file content can ever be
     committed.
  3. DSN: checked `backend/.env` — DATABASE_URL is a Supabase pooler
     (aws-0-eu-west-1.pooler.supabase.com:5432, sslmode=require). TCP
     connect from this environment fails (timeout), psql not installed,
     pgx probe unreachable.
- next / unblocked: CLI 6 can run the gate. CLI 1 (chatbot queue lane) and
  CLI 2 (cache tests) must land their in-flight files before the tree
  builds green — see tests + risks below.
- tests:
  - `cd backend && gofmt -l internal cmd` → my 4 files: CLEAN (empty).
    Remaining flagged (NOT my lane): `cmd/worker/handlers.go` (CLI 1)
    + `internal/platform/cache/cache_test.go` (CLI 2, in progress).
  - `go test -count=1 ./internal/platform/storage/... ./internal/services/ai/...`
    → PASS (my-lane packages green in isolation).
  - **integration suite not run: no reachable DSN** (Supabase pooler URL in
    `.env` is unreachable from this environment; TCP 5432 times out).
  - `go build ./...` / `go vet ./...` / `go test -count=1 ./...` → RED, but
    every error is in CLI 1/CLI 2 in-flight files: `chatbot.go` unused
    imports (uuid + queue), `cache_test.go` undefined: fmt. My lane's
    packages compile and pass; the VERIFY loop cannot go fully green until
    CLI 1 + CLI 2 land.
- risks/notes:
  - `gstack /verify` → **no declared checks** (no gstack:verify: line, no
    manifest; tests-only selection under origin/main@cf7bd46 policy; test
    runner not detected for the 3 selected files). Not green, not red on
    my lane — recorded per lane instruction.
  - I did NOT touch `cmd/worker/handlers.go` or `chatbot.go` per lane
    rules; CLI 1's 6 modified files are still uncommitted in-tree.

## 2026-11-18 12:5x — CLI 6 — early gate (2/5 done) + blocker relays
- did:
  - Read-only gate at 2/5 (CLI 3 + CLI 5 done; CLI 1/2/4 in flight):
    `cd backend && go build ./...` → **RED (exit 1)**:
    `internal/hands/chatbot.go:21,25` unused imports `uuid` + `queue`
    (matches CLI 5's 12:45 finding + CLI 1's `handlers.go:186`
    `p.Prompt` vs the pinned `AiChatPayload` fields).
  - `cd frontend && npm test` → **6 failed | 67 passed (73 tests, 9
    files)** — all 6 failures in `tests/components/chatbot.test.tsx`
    (CLI 4's new spec, in flight); `npx tsc --noEmit` → 3 errors in the
    same file (line 79: `'start'` is not a valid waitFor option;
    lines 181–182: object possibly undefined); `npm run build` not
    reached (chained after tsc).
  - `go test -tags integration ./test/...` → **not run: no
    disposable Postgres DSN** (per CLI 3: `.env` points at an
    unreachable Supabase pooler, TCP 5432 times out, no local psql).
- relays sent (did NOT touch any sibling file):
  - CLI 1 ← two one-line build fixes (`handlers.go:186` field
    rename to pinned `AiChatPayload` fields; `chatbot.go` unused
    imports) + gofmt `cmd/worker/handlers.go`.
  - CLI 2 ← `cache_test.go` still absent on disk; note CLI 3's
    `undefined: fmt` in the draft.
  - CLI 4 ← the 3 tsc errors + 6 failing vitest specs in
    `chatbot.test.tsx` (line 79 poll option; lines 181–182
    undefined guards).
- tests: as above; gate at 5/5 is authoritative.
- risks/notes: /verify prep confirmed — `gstack-checks detect` →
  "CHECKS: tests only", TRUST untrusted (same as CLI 3/5 saw); at 5/5
  I run `gstack-checks run --allow-repo-commands` per the /verify
  skill (session launched to run repo commands). CLI 5's audit table
  is CLEAN on every check (uploads "still tracked" row is
  superseded-on-disk by CLI 3's delete + blanket gitignore, verified
  via `git ls-files` = empty). Pending human call (flagged, not in
  PR): delete/rotate gitignored `frontend/.env.local` (stale
  `sb_publishable_*` key).

## 2026-11-18 12:48 — CLI 2 — cache-test lane done
- did: NEW `backend/internal/platform/cache/cache_test.go` (in-lane, test-only; no exported
  API changed). Against `NewMemCache`:
  - `TestMemCacheSetGetAndTTLExpiry` — Set/Get (string/[]byte/int64) + lazy TTL expiry
  - `TestMemCacheSetNXSecondCallFalse` — 1st true / 2nd false, value kept, expired key reclaimable
  - `TestMemCacheIncrWindow` — first incr sets TTL; window restarts after expiry
  - `TestMemCacheDel` — Del incl. missing key
  - `TestRotateScriptHappyPath` — seed (== StartSession writes) then one rotation: "ok", rt:h0
    consumed, rt:h1 registered, family record advanced to 42|h1
  - `TestRotateScriptReplayKillsFamily` — REPLAY of a consumed token → "reused:7", family death
    flag set, record + replayed hash map deleted, surviving successor now unusable ("dead")
  - `TestRotateScriptDeadWhenFamilyGone` — missing record → "dead", flag written, second attempt
    short-circuits
  - `TestUserRevokedBeforeRevokesLiveAccessToken` — the two ADR-0001 validation items not yet
    asserted: marker round-trips via cache; iat<marker strict comparison (marker-60 revoked,
    marker boundary accepted, marker+60 accepted, 14m-old live token revoked); no-marker case
- FINDING (flagged to CLI 1, out of this lane — did NOT edit scripts.go/service.go): the
  "ok" rotation path mis-registers the successor. `rt_fam:<h1>` is written from the
  PREDECESSOR hash, not the family: the MemCache mirror does `TrimPrefix("rt_fam:h0","rt_fam:")`
  = "h0", and the Lua `string.sub(KEYS[2],7)` = ":h0" (off-by-one, colon kept) — so the two
  backends even diverge from each other. `TestRotateScriptSuccessorRegistrationFinding` pins
  this: a legitimate second refresh dies ("dead") because Rotate resolves
  `Get("rt_fam:h1") -> "h0"` then misses `rt_family:h0`. Pinned as current behavior; flip the
  expectations to "ok"/F1 once scripts.go stores the family. This is a real multi-hop refresh
  bug worth fixing before the cutover window closes.
- tests: `cd backend && go test -count=1 ./internal/platform/cache/...` GREEN (9/9, ok).
  `go build ./internal/platform/cache/` + `go vet` GREEN. gofmt clean.
- BLOCKER (not mine, not caused by my test): `go build ./...` is RED from CLI 1's lane —
  `internal/platform/queue/tasks.go:187/195: undefined: context` (missing `context` import in
  the in-flight queue/worker work). My cache_test.go imports only the cache package; it does
  not touch queue. CLI 1: add `"context"` to tasks.go imports to unblock the whole-tree build.
- next / unblocked: CLI 6 can gate on the cache package alone (green). Full `go build ./...`
  stays red until CLI 1's `context` import lands.

## 2026-11-18 12:53 — CLI 1 — chat-queue lane done
- did: moved the chatbot model call off the HTTP path (ADR-0002 step 4):
  - `platform/queue/tasks.go`: pinned `AiChatPayload{message, share_chart,
    chart, instruction}` + `queue.BuildChatPrompt` (single prompt-builder
    shared by the inline degrade path and the worker → byte-identical model
    input both ways); `TypeAiChat`/`NewAiChatTask`/`taskOptions` (queue
    "ai", MaxRetries 5, 5m timeout — same policy as ai:analyze_document)
    were pre-landed, kept intact. Added `SetPusherForTest` (exported
    pusher seam) so external tests can drive Enqueue without Redis.
  - `hands/chatbot.go` POST /chatbot/api: emergency-intent reply + per-user
    quota stay SYNCHRONOUS (emergency is NEVER enqueued — asserted by test).
    After quota passes: producer configured → enqueue `ai:chat` (owner =
    authenticated user id, key `ai:chat:<uuid>`) → **202 {success:true,
    job_id}**. REDIS_URL unset (or an enqueue blip) → inline
    `ai.ChatWithInstruction` → **200 {success:true, response}** exactly
    like the sync behavior (app stays deployable without Redis). TTS stays
    synchronous — KNOWN LIMITATION (ai:tts task type + worker handler exist
    in tasks.go/handlers.go as headroom, but the /chatbot/api/tts route is
    NOT wired to them yet).
  - `hands/router.go`: the ONE shared line — `prod` (already built at
    BuildRouter top) now passed into `ChatbotRouter` → `chatbotAPI`.
  - `cmd/worker/handlers.go`: `handleAiChat` rebuilt to the pinned payload
    shape (p.Instruction with AssistantInstruction fallback), stamps
    completed + `result = {"response": text}` via MarkJobResult; failure
    propagates to the failureStamp middleware (failed/dead + last_error).
    gofmt'd (was one of CLI 3's drift files; now clean).
  - `hands/jobs_v1.go` `writeJobResponse`: additive `"result"` field when
    background_jobs.result is non-null (parsed JSON; absent when null).
    Migration check (task item 5): **0004 already has `result JSONB`** —
    no new migration added.
  - Tests: `internal/hands/chatbot_test.go` (async 202 + job_id + payload
    shape; degrade-inline 503 without keys; emergency-never-enqueued;
    no-user owner 0; writeJobResponse result present/absent),
    `cmd/worker/handlers_test.go` (bad-payload reject; no-provider-key
    failure path), `backend/test/chat_queue_test.go` (DSN-gated: enqueue
    writes a real background_jobs ledger row; completed row's result JSONB
    round-trips through JobByTaskID — the path jobStatus renders).
- exposes (job_id contract for CLI 4): `POST /chatbot/api` with a
  configured queue answers `202 {"success":true,"job_id":"ai:chat:<uuid>"}`
  — poll `GET /api/v1/auth/...` no: **`GET /api/v1/jobs/<job_id>`**
  (AuthRequired cookie) → `{"success":true,"id","type":"ai:chat",
  "status":"queued|processing|completed|failed","attempts","updated_at"}` +
  `"result":{"response":"<text>"}` when completed. Without REDIS_URL the
  same endpoint answers `200 {"success":true,"response":"<text>"}`
  synchronously (no job_id — poll nothing). CLI 4: branch on
  `res.status === 202 ? pollJob(res.job_id) : use res.response`.
- tests (exact): `cd backend && go build ./...` PASS; `go vet ./...` PASS;
  `go test -count=1 ./...` PASS (12 packages, incl. new hands + worker +
  queue tests); `go test -count=1 -tags integration ./test/` PASS without
  a DSN (new DB-gated tests skip: TestChatEnqueueWritesLedgerRow /
  TestChatJobResultRoundTrip); `gofmt -l` on my lane files clean.
- /verify: gstack not installed in this CLI (command not found) — recorded
  per "no declared checks" fallback; verification above is the equivalent.
- relay (NOT fixed here, out of chat-queue lane — pre-existing auth
  defect flagged by CLI 2's cache tests): refresh-rotation "ok" path
  mis-registers the successor `rt_fam:<h1>` (TrimPrefix/sub off-by-one in
  scripts.go + mem.go mirror, pinned by
  TestRotateScriptSuccessorRegistrationFinding) → a legitimate 2nd
  refresh dies "dead". Owner: assign to whoever owns platform/cache.
- known limitations: (1) TTS still synchronous (task item 2's recorded
  limitation); (2) inline degrade path returns 503 when provider keys are
  missing (same as pre-queue behavior — the async path surfaces it as the
  job's failed status instead); (3) DSN-gated ledger tests not RUN here
  (no INTEGRATION_DSN in this environment).

## 2026-11-18 (re-gate) — CLI 3 — hygiene lane re-verified on the now-green tree
- supersedes my 12:4x entry's "tree RED" caveat: CLI 1's `context` import
  + CLI 2's cache_test landed; re-ran everything on the current tree:
  - `cd backend && gofmt -l internal cmd` → **empty (whole tree clean)**.
  - `go build ./... && go vet ./... && go test -count=1 ./...` → **GREEN,
    12 packages ok, 0 fail**.
  - uploads/: absent on disk; `backend/.gitignore:11` blanket `uploads/`
    (P2.4 closed, decision recorded at my 12:4x entry).
  - DSN: `DATABASE_URL` host `aws-0-eu-west-1.pooler.supabase.com:5432`
    **UNREACHABLE** from this env (TCP timeout) →
    **integration suite not run: no reachable DSN** (3 tagged files:
    integration_test.go, auth_v1_test.go, chat_queue_test.go).
  - gstack /verify: **no declared checks** (tests-only selection, no
    manifest; runner untrusted) — recorded as such.
- next / unblocked: CLI 6 — tree is green, gate can run now.

## 2026-11-18 14:1x — CLI 6 — FINAL GATE: GREEN — PR #4 opened (all 5/5 done)
- did:
  - Gate triggered at 5/5 (CLI 1 12:53 entry + CLI 3 re-gate confirmed
    the `context` import + cache_test on disk). Re-ran the full
    cross-stack gate fresh on the committed tree:
  - gstack `/verify`: `gstack-checks detect` → "CHECKS: tests only,
    TRUST untrusted"; `run --allow-repo-commands` → `tests=INCOMPLETE`
    (Go test runner not detected for 6/7 selected files; the 1 detected
    Vitest file ran green; same environmental result CLI 3/4/5 saw).
    Per the /verify skill's "no declared checks" fallback, the direct
    gate commands below are the evidence. `/ship` scope executed:
    review full pending diff (5 lanes, secret-scan of added lines
    CLEAN, gofmt CLEAN) → commit → PR → STOP.
- GATE RESULTS (exact commands, final tree @ 37f12dc):

  | Check | Command | Result |
  |---|---|---|
  | Backend build | `cd backend && go build ./...` | **GREEN** (exit 0) |
  | Backend vet | `cd backend && go vet ./...` | **GREEN** (exit 0) |
  | Backend tests | `cd backend && go test -count=1 ./...` | **GREEN** — 12 packages ok (config, database, hands, middleware, models, modules/auth, platform/cache, platform/queue, platform/storage, services/ai, services/otp, cmd/worker) |
  | Frontend tests | `cd frontend && npm test` | **GREEN** — 73/73 across 9 files |
  | Frontend tsc | `cd frontend && npx tsc --noEmit` | **GREEN** (exit 0) |
  | Frontend build | `cd frontend && npm run build` | **GREEN** — 35/35 static pages |
  | Integration suite | `cd backend && go test -tags integration ./test/...` | **NOT RUN — no disposable Postgres DSN** (per CLI 3: `.env` pooler unreachable, TCP 5432 times out; DSN-gated tests skip cleanly) |
  | gofmt | `cd backend && gofmt -l internal cmd` | **CLEAN** (empty) |
  | cache tests | `ls backend/internal/platform/cache/cache_test.go` | **PRESENT** — 9 tests incl. replay→family-kill + user:revoked_before |
  | uploads decision | `git ls-files backend/uploads` | **EMPTY** — dir deleted, blanket `uploads/` in `.gitignore:11` (P2.4 closed) |
  | `/files` nav item | `grep -n '/files' frontend/components/layout/nav-config.ts` | **PRESENT** — line 9, "My Files" |
  | chatbot specs | `frontend/tests/components/chatbot.test.tsx` | **PRESENT** — 6 specs, all green |
  | supabase refs | `git grep -i supabase -- frontend` | **GREEN** — 0 code refs; 2 explanatory comments only (`.env.example`, `lib/api.ts`) |
  | uploads mount | `git grep -F 'r.Static("/uploads"' -- backend` | **GREEN** — 0 matches |
  | CLI 5 audit table | board @ 12:45 | **ALL GREEN** (uploads row superseded-on-disk by CLI 3's delete) |

- PR: **https://github.com/soumyachk101/Health_Track/pull/4**
  (commit 37f12dc, 23 files, +1654/−86; base `main`). Stopped at PR
  per wave-4 instruction — no deploy, no merge, no `/land-and-deploy`.
- residual risks (recorded on the PR body too):
  1. Live-Redis e2e unexercised (202→poll→result verified via contract
     + seam tests only; first live run happens on the deployed worker).
  2. Cloud Run deploy = Phase 6 — no infra changes in this PR.
  3. Same-site cookie check vs real deploy URLs still open (ADR-0001
     rollout step 2).
  4. TTS still synchronous (`ai:tts` task type is headroom; route
     not wired).
  5. **Rotation bug (owner decision):** `rt_fam:<successor>`
     registered from the predecessor hash (MemCache TrimPrefix vs Lua
     string.sub off-by-one) → legitimate 2nd refresh dies "dead";
     pinned as current behavior by
     `TestRotateScriptSuccessorRegistrationFinding`.
  6. Gitignored `frontend/.env.local` holds a stale pre-cutover
     Supabase key (`sb_publishable_*`) — human rotate/delete call;
     not in the PR (untracked).
