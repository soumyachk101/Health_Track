# 4-CLI Prompts — paste one per CLI (revised 11:45 against the verified tree)

Coordination: all four CLIs share `docs/CLI_HANDOFF.md` (append-only board — read FIRST,
append after every milestone, message the 3 sibling sessions when unblocked/blocked).
Pacing: ≤ ~15 tool calls/minute each (shared 60 RPM); on any rate-limit error pause ~60s.
Nobody commits/pushes — the human commits at the end.

**Current verified state (11:44 board entry):** backend build+vet+unit tests GREEN;
frontend 67/67 + build + tsc GREEN; supabase removed; worker/storage/files/jobs handlers
landed; stray artifacts gone. The prompts below target ONLY the verified remaining gaps.

Common READ-FIRST for every CLI: `docs/CLI_HANDOFF.md` (especially the 11:44 MAIN
snapshot — it supersedes earlier entries), `docs/CLI_ORCHESTRATION.md` (ownership +
pinned API surface), `docs/PROGRESS.md`, `docs/ADR-0001-pure-go-auth.md` +
`docs/ADR-0002-r2-object-storage-and-async-worker.md`.

---

## PROMPT 1 — CLI-1: backend test gaps (auth/cache) + gofmt

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker (branch phase1-pure-go-auth).
READ FIRST: docs/CLI_HANDOFF.md (append entries + message siblings), docs/CLI_ORCHESTRATION.md,
docs/ADR-0001-pure-go-auth.md. PACING: ~15 tool calls/min; pause 60s on rate-limit. No commits.
The tree is GREEN — your job is the remaining test + hygiene gaps, NOT rebuilding:
1. Write backend/internal/platform/cache/cache_test.go (package cache): against
   NewMemCache, cover — Set/Get/TTL expiry, SetNX semantics, Incr window TTL,
   Del, and Eval (in-memory only supports the rotation Lua script: drive the
   rotation path through the exported rotation API in scripts.go).
2. Write rotation/reuse/revocation tests where the ADR's "Validation" list demands
   them (see ADR-0001): refresh-rotation happy path, replay-consumed-token ->
   whole family invalidated, user:revoked_before:<user_id> revokes live access
   tokens. Put them in internal/platform/cache/ or internal/modules/auth/
   (modules/auth already has auth_test.go — add, don't rewrite it). No live
   Redis/Postgres: use the in-memory fallback.
3. gofmt -w on YOUR lane only: internal/platform/cache/**, internal/modules/auth/**,
   internal/services/ai/ai_test.go. Do NOT touch files_v1*, storage.go, or
   cmd/worker (CLI-2's lane).
4. Verify: cd backend && gofmt -l internal/platform/cache internal/modules/auth
   internal/services/ai -> empty; go build ./... && go test ./internal/platform/...
   ./internal/modules/... green. Append a CLI_HANDOFF.md entry + message CLI-4 that
   the auth/cache test lane is done.
```

## PROMPT 2 — CLI-2: R2/worker gofmt + uploads decision + storage test gap

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker (branch phase1-pure-go-auth).
READ FIRST: docs/CLI_HANDOFF.md (append + message siblings), docs/CLI_ORCHESTRATION.md,
docs/ADR-0002-r2-object-storage-and-async-worker.md. PACING: ~15 tool calls/min;
pause 60s on rate-limit. No commits. Tree is GREEN — remaining gaps only:
1. gofmt -w on YOUR lane: internal/hands/files_v1.go, internal/hands/files_v1_test.go,
   internal/platform/storage/storage.go, cmd/worker/handlers.go. Do NOT touch
   cache/, modules/auth/, or ai_test.go (CLI-1).
2. If internal/platform/storage/storage_test.go lacks: MIME/size validation,
   key-traversal guards, and the download-url IDOR two-unrelated-users case — add
   them (files_v1_test.go may already cover some; extend, don't duplicate).
3. Decide + record the backend/uploads/ question: the public r.Static("/uploads")
   mount is already removed and only two .gitkeep files remain. Default: delete the
   two .gitkeep + empty dirs (`git rm` is NOT allowed for you — just `rm` the empty
   local dirs; the human commits). If you think a gitignore entry is safer, do that
   instead and say why. Note the choice in your CLI_HANDOFF.md entry.
4. Verify: cd backend && go build ./... && go vet ./... && go test ./... green;
   gofmt -l internal/hands internal/platform/storage cmd/worker -> empty (files you
   own). Append CLI_HANDOFF.md entry + message CLI-4.
```

## PROMPT 3 — CLI-3: verify + finish the AI-UI claims (no re-build)

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker (branch phase1-pure-go-auth).
READ FIRST: docs/CLI_HANDOFF.md (append + message siblings), docs/CLI_ORCHESTRATION.md,
docs/ADR-0001 (AI section), docs/PHASE0_SECURITY_FINDINGS.md (S3). PACING: ~15 tool
calls/min; pause 60s on rate-limit. No commits. The frontend is green — VERIFY claims,
don't redo them:
1. In frontend/components/Chatbot.tsx, verify each claimed feature is actually present
   and test-covered: (a) "share my chart data with AI" toggle DEFAULT OFF mapping to
   share_chart in the request body; (b) non-clinical disclaimer rendered; (c)
   emergency card (911/112/108 + "contact emergency services / qualified clinicians",
   never diagnose) on both local regex hit AND server emergency flag; (d) message
   length cap. For each missing one: implement it in Chatbot.tsx only.
2. Verify the /files nav item (frontend/components/layout/nav-config.ts, per the 01:20
   board note) and that the FilesUpload component + /files page render with the
   session client. Add the nav item if absent.
3. Add/extend vitest coverage for anything you implemented in step 1.
4. Verify: cd frontend && npm test && npx tsc --noEmit && npm run build all green.
   Append CLI_HANDOFF.md entry stating each S3 item: verified-present / added — with
   the test names — and message CLI-4.
```

## PROMPT 4 — CLI-4: docs + final cross-stack verification (run LAST)

```
Repo: /Users/soumyachakraborty/Documents/HealthTracker (branch phase1-pure-go-auth).
READ FIRST: docs/CLI_HANDOFF.md (you gate on CLI-1/2/3 "done" entries — poll the board
every ~2 min, max ~15 checks; if a lane is blocked, message it, relay to its sibling,
note on board, do NOT fix their files), docs/CLI_ORCHESTRATION.md, docs/PROGRESS.md.
PACING: ~15 tool calls/min; pause 60s on rate-limit. No commits (human commits).
Touch NOTHING under backend/internal, backend/cmd, or frontend/ — docs + verification
only (plus .gitignore if CLI-2's entry says to add backend/uploads/).
1. docs/PROGRESS.md: tick Phase 1 items 1.1-1.6 and Phase 3/5 items 3.1-3.5 EXACTLY as
   the board evidences (mark any unverified item as such, don't tick it). Add a
   "Commands run + pass/fail" table: go build/vet/test, npm test/build, and
   "go test -tags integration — NOT RUN (no disposable Postgres DSN; .env points at
   an unreachable remote pooler)".
2. Full cross-stack verification: cd backend && go build ./... && go vet ./... &&
   go test ./...; cd frontend && npm test && npx tsc --noEmit && npm run build.
3. Secret/secret-scan: grep the full diff + untracked files for real keys
   (REDIS_URL/R2_* must be placeholders only); confirm no NEXT_PUBLIC_SUPABASE_*
   anywhere; confirm no r.Static("/uploads" remains.
4. Final CLI_HANDOFF.md entry: overall GREEN/RED per item, exact commands + results,
   residual risks (Cloud Run deploy of api+worker = Phase 6; same-site cookie check
   against real deploy URLs; integration suite needs a disposable DSN).
```

---

## Launch checklist (you, in this order)
1. All 4 terminals: `cd /Users/soumyachakraborty/Documents/HealthTracker && git status`
   (sanity: branch phase1-pure-go-auth, tree as the 11:44 board entry describes).
2. Paste PROMPT 1 / 2 / 3 into CLIs 1-3 **together** (disjoint lanes), PROMPT 4 last —
   it self-gates on the other three's board entries.
3. Watch `docs/CLI_HANDOFF.md` grow; each CLI also messages its siblings by session name.
4. When CLI-4 posts its final GREEN entry: review the full `git status` + `git diff`,
   spot-check the 4 board "done" entries, then commit yourself (no CLI commits).
