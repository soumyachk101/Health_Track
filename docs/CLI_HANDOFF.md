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
