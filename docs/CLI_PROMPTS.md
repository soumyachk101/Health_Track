# 4-CLI Prompts — FIX PASS (v2, 12:12 verified tree)

All four lanes' core work is GREEN and verified: backend `go build/vet/test` pass
(fresh, -count=1), frontend 67/67 tests + `tsc --noEmit` + `npm run build` pass,
tree idle. These prompts fix ONLY the verified remaining gaps — do NOT rebuild
anything that's already done. Paste one block per CLI; all four can run at once
(disjoint files) or CLI-4 runs after 1–3 post "done".

**Shared rules (baked into each prompt):**
- Pacing: ≤ ~15 tool calls/minute each (your shared 60 RPM); on any rate-limit error
  pause ~60s then continue. No subagents.
- Coordination: read `docs/CLI_HANDOFF.md` FIRST (the 12:12 MAIN snapshot entry
  supersedes all earlier entries); append one entry after your work; message the
  other 3 sibling sessions when you finish or block.
- Never commit/push — the human commits at the end.
- Re-read any shared file (`router.go`, `.gitignore`, `go.mod`) immediately before
  editing; keep your edits on disjoint lines.

---

## PROMPT 1 — CLI-1 (backend auth lane): cache tests + gofmt + honest DSN note

```
Repo /Users/soumyachakraborty/Documents/HealthTracker, branch phase1-pure-go-auth.
READ FIRST: docs/CLI_HANDOFF.md (12:12 MAIN entry), docs/ADR-0001-pure-go-auth.md.
PACING: ~15 tool calls/min; pause 60s on rate-limit; no subagents; no commits.
Lane: internal/platform/cache/**, internal/modules/auth/** (add tests only),
internal/services/ai/ai_test.go. Backend tree is GREEN — you are adding test
coverage + hygiene, not rebuilding.

1. Write internal/platform/cache/cache_test.go (package cache) against NewMemCache:
   - Set/Get round-trip + TTL expiry (short TTL),
   - SetNX semantics (second call returns false),
   - Incr window (first incr sets TTL; counter increments),
   - Del,
   - Eval path: drive the exported rotation API in scripts.go end-to-end on the
     in-memory cache — happy-path refresh rotation, replay of a CONSUMED token
     invalidates the whole family, user:revoked_before:<user_id> revokes.
   (ADR-0001 "Validation" list: refresh rotation + reuse->family-kill are the two
   items not yet unit-asserted; if modules/auth/auth_test.go already covers the
   others, leave it.)
2. gofmt -w ONLY: internal/platform/cache/**, internal/modules/auth/**,
   internal/services/ai/ai_test.go. Leave files_v1*, storage.go, cmd/worker alone
   (CLI-2's lane).
3. DSN note: integration suite (backend/test/auth_v1_test.go + integration_test.go)
   needs a disposable Postgres. Check backend/.env for a reachable DSN — if one
   works, run `go test -tags integration ./test/...` and record pass/fail; if not,
   record exactly "not run: no reachable DSN" in your board entry (do NOT claim it).
4. VERIFY: cd backend && go build ./... && go vet ./... && go test ./... green.
5. Append CLI_HANDOFF.md entry (commands + results) + message CLI-4: "CLI-1 fix lane
   done; cache rotation/replay/revocation now unit-covered."
```

## PROMPT 2 — CLI-2 (R2 + worker lane): gofmt + uploads decision + worker smoke

```
Repo /Users/soumyachakraborty/Documents/HealthTracker, branch phase1-pure-go-auth.
READ FIRST: docs/CLI_HANDOFF.md (12:12 MAIN entry), docs/ADR-0002-r2-object-storage-and-async-worker.md.
PACING: ~15 tool calls/min; pause 60s on rate-limit; no subagents; no commits.
Lane: internal/hands/files_v1*, internal/platform/storage/**, cmd/worker/**,
backend/uploads/, .gitignore. Tree is GREEN — hygiene + verification only.

1. gofmt -w ONLY: internal/hands/files_v1.go, internal/hands/files_v1_test.go,
   internal/platform/storage/storage.go, cmd/worker/handlers.go. Do NOT touch
   cache/, modules/auth/, ai_test.go (CLI-1's lane).
2. Decide backend/uploads/: the public r.Static("/uploads") mount is already removed
   and the dir holds only two .gitkeep files. Pick ONE:
   (a) delete the dir (`rm -rf backend/uploads`) + add `backend/uploads/` to
       .gitignore (future-proof), or
   (b) keep it + add `backend/uploads/` to .gitignore.
   Record the choice + reason in your board entry. (P0 finding S2 is closed either
   way; the .gitignore entry is the P2.4 item from PHASE0_SECURITY_FINDINGS.)
3. Worker smoke: run `go run ./cmd/worker` — with REDIS_URL unset it must exit with
   a clear "not configured" message (not panic/hang). With a local redis if one is
   running, verify it starts + shuts down cleanly on SIGTERM. Record actual behavior.
4. VERIFY: cd backend && go build ./... && go vet ./... && go test ./... green;
   gofmt -l on your files empty.
5. Append CLI_HANDOFF.md entry + message CLI-4: "CLI-2 fix lane done; uploads
   decision = <a|b>; worker smoke result = <...>."
```

## PROMPT 3 — CLI-3 (frontend lane): AI-UI test coverage + /files nav check

```
Repo /Users/soumyachakraborty/Documents/HealthTracker, branch phase1-pure-go-auth.
READ FIRST: docs/CLI_HANDOFF.md (12:12 MAIN entry), docs/PHASE0_SECURITY_FINDINGS.md (S3),
docs/ADR-0001-pure-go-auth.md (AI section).
PACING: ~15 tool calls/min; pause 60s on rate-limit; no subagents; no commits.
Lane: frontend/ only. Frontend is GREEN (67/67 + build) — add coverage, don't
rebuild the client/guards.

1. Verify in frontend/components/layout/nav-config.ts that a "/files" ("My Files")
   item exists; if absent, add { icon: FileText, label: "My Files", href: "/files" }.
2. The AI-UI items are IMPLEMENTED in frontend/components/Chatbot.tsx (share_chart
   toggle default OFF, non-clinical disclaimer, emergency card 911/112/108 with
   "contact emergency services / qualified clinicians", message length cap) but have
   ZERO test coverage. Add vitest specs (new file tests/components/chatbot.test.tsx
   or extend an existing one) covering at minimum:
   - share_chart toggle exists and defaults to OFF, request body carries
     share_chart:false by default / true when toggled (mock apiFetch),
   - emergency card renders when the server response flags emergency,
   - non-clinical disclaimer is rendered.
3. Sanity: confirm no raw localStorage token reads remain anywhere under
   frontend/app (session client owns cookies) — grep, report count, fix stragglers
   in your lane only.
4. VERIFY: cd frontend && npm test (your new specs green) && npx tsc --noEmit &&
   npm run build.
5. Append CLI_HANDOFF.md entry (list the new test names) + message CLI-4:
   "CLI-3 fix lane done; AI-UI now test-covered."
```

## PROMPT 4 — CLI-4 (verification + docs): full cross-stack gate + PROGRESS + final entry

```
Repo /Users/soumyachakraborty/Documents/HealthTracker, branch phase1-pure-go-auth.
READ FIRST: docs/CLI_HANDOFF.md (12:12 MAIN entry + the three "fix lane done"
messages from CLI-1/2/3 — if any are missing, poll every ~2 min, max 15 checks;
relay blockers between siblings, do NOT fix their files), docs/PROGRESS.md.
PACING: ~15 tool calls/min; pause 60s on rate-limit; no subagents; no commits
(human commits). You may edit: .gitignore (only if CLI-2's entry says "keep dir"),
docs/PROGRESS.md, docs/CLI_HANDOFF.md.

1. Cross-stack gate (run all, record exact output):
   cd backend && go build ./... && go vet ./... && go test -count=1 ./...
   cd frontend && npm test && npx tsc --noEmit && npm run build
   go test -tags integration ./test/...  -> run ONLY if a reachable DSN exists,
   otherwise record "not run: needs disposable Postgres DSN" (never claim it).
2. Confirm the whole defect list is closed: gofmt -l backend clean; cache tests
   exist; backend/uploads decision executed; /files nav present; AI-UI tests exist;
   no supabase refs in frontend source; no r.Static("/uploads"); secret scan of
   `git diff` + untracked files (REDIS_URL/R2_*/keys must be placeholders only).
3. docs/PROGRESS.md: tick exactly what the board evidences (mark unverified items
   as such); keep the honest "integration suite not run" note if no DSN.
4. Final CLI_HANDOFF.md entry: per-item GREEN/RED table, exact commands + results,
   residual risks (live-Redis/live-Postgres end-to-end, Cloud Run deploy = Phase 6,
   same-site cookie check against real deploy URLs, integration suite DSN).
5. Message the other 3 sessions: "final gate <green|red>; human to commit."
```

---

## Launch order (you)
CLI-1, CLI-2, CLI-3 start together (disjoint lanes). CLI-4 can start too — it
self-gates on the three "fix lane done" board messages, so it just polls. When
CLI-4 posts its final GREEN entry: review `git status`/`git diff` here, spot-check
the board, then commit.
