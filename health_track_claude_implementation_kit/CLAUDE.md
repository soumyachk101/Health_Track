# Claude Code Project Instructions — Health_Track

## Mission
Evolve the existing Health_Track product into a secure, maintainable, observable, and horizontally scalable healthcare platform through incremental engineering. Do not replace the application wholesale.

## Known baseline — re-verify against the current checkout
The public repository was statically reviewed on 17 November 2026. At that time it contained:
- Frontend: Next.js 15 / React 19 / TypeScript.
- Backend: Go with Gin.
- Database: PostgreSQL through pgx / pgxpool.
- Authentication: Supabase Auth integration and legacy custom JWT compatibility paths.
- Product domains: patients, health records, medicines, prescriptions, mental-health and lifestyle logs, appointments, service providers, insurance, admin, and AI/chat features.
- Frontend and backend folders in one repository.
- Existing frontend and Go tests.
This is only a baseline. The current working tree, lockfiles, configuration, and deployment state are authoritative.

## Mandatory working rules
1. Before editing, inspect the repository, git status, current branch, recent commits, package manifests, Go module, environment examples, migrations, tests, workflows, and deployment files.
2. Never overwrite uncommitted user work. Stop and explain if the working tree has unexpected changes.
3. Work on a feature branch. Never push, deploy, merge, rotate secrets, or modify production resources unless explicitly instructed.
4. Make small, reviewable changes. Avoid broad mechanical rewrites.
5. Preserve API behavior unless a versioned migration and compatibility plan is approved.
6. Do not invent product requirements, clinical rules, legal compliance claims, or test results.
7. Never log or expose passwords, JWTs, API keys, OTP values, session cookies, or identifiable patient health information.
8. Do not add secrets to source control. Keep `.env` files ignored and use placeholders in examples.
9. Do not run destructive migration, reset, truncate, drop, or seed commands against production.
10. Every implementation change must include appropriate tests and documentation.
11. Prefer standard-library and already-approved dependencies; justify new dependencies.
12. Avoid microservices until measured scale or team boundaries justify extraction. Prefer a modular monolith initially.
13. Do not place business logic in HTTP handlers. Use handler → service/use-case → repository boundaries.
14. Use database constraints and backend authorization as the source of truth. Frontend route guards are UX, not security.
15. For health-related functionality, treat data confidentiality, integrity, consent, access auditing, retention, and recovery as core product requirements. Do not claim HIPAA, GDPR, DPDP Act, or other compliance without a formal assessment.

## Required execution workflow
For each phase:
- State scope, assumptions, and acceptance criteria.
- Inspect affected files before editing.
- Propose a concise implementation plan.
- Implement only that phase.
- Run formatting, lint/type checks, unit tests, integration tests where available, and relevant security checks.
- Inspect `git diff` for accidental changes, secrets, generated files, and compatibility breaks.
- Update `docs/PROGRESS.md`, relevant architecture docs, and migration notes.
- Report exact commands run, pass/fail status, skipped checks, and residual risks.
- Stop at the phase boundary and wait for review before starting the next phase.

## Quality expectations
- Go: `gofmt`, `go vet`, race tests where practical, context-aware I/O, explicit errors, bounded requests, graceful shutdown, and deterministic tests.
- TypeScript: strict typing, no unnecessary `any`, accessible UI, centralized API client, robust async states, and tests.
- PostgreSQL: versioned migrations, explicit constraints, indexes backed by query patterns, transactions for multi-step operations, and backward-compatible rollout.
- Security: deny by default, least privilege, object-level authorization, secure session handling, input validation, rate limiting, audit logs, and safe error messages.
- Operations: health/readiness checks, structured logs, metrics, tracing where practical, backups, rollback, and runbooks.

## Source-of-truth hierarchy
1. Current checked-out source code and runtime behavior.
2. Current tests and CI configuration.
3. Current deployment configuration and actual environment documentation.
4. This kit's design guidance.
5. Earlier repository review notes.

If this kit conflicts with the current code, explain the conflict and update the plan; do not blindly follow stale assumptions.
