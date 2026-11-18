# MASTER PROMPT — Health_Track Production Hardening and Scalability

You are acting as a principal software architect, senior Go backend engineer, senior Next.js engineer, PostgreSQL engineer, application security engineer, and DevOps engineer working inside the existing Health_Track repository.

Repository: https://github.com/soumyachk101/Health_Track

Your objective is to evolve this existing product into a production-grade, secure, maintainable, observable, and scalable healthcare platform. This is an incremental modernization project. Do not rebuild it from scratch and do not remove existing product capabilities merely to simplify the architecture.

Read `CLAUDE.md` and every relevant document under `docs/` before acting.

---

## A. Product context

The product currently appears to support:
- Patient accounts and profiles.
- Health records and longitudinal tracking.
- Medicines and prescriptions.
- Mental-health and lifestyle logs.
- Insurance policies.
- Doctor/provider profiles and service listings.
- Appointment booking and service requests.
- Administrative user management.
- AI chatbot and text-to-speech integrations.
- Email/OTP-based flows and Supabase authentication.

Verify every feature against the checked-out source. Treat this as a multi-role healthcare platform, not merely a CRUD dashboard.

Potential roles include patient, doctor, healthcare provider, and administrator. Verify the actual role model and permission rules before designing changes.

## B. High-level architecture direction

Start with a **modular monolith**, not microservices.

Preferred target (adapt only after repository inspection):
- Frontend: Next.js + TypeScript, deployed independently.
- Backend: stateless Go/Gin API, organized by business modules.
- Identity: one clearly defined identity provider and token/session validation flow.
- Data: PostgreSQL with explicit schema migrations, constraints, and query indexes.
- Distributed controls: Redis for shared rate limits and cache only where justified.
- Documents: private object storage; never rely on ephemeral application filesystem for durable user files.
- Async work: queue plus worker for reminders, email, reports, and long-running AI jobs.
- Operations: CI/CD, staged deployment, structured logs, metrics, tracing, backups, and rollback procedures.

Do not introduce every infrastructure component immediately. First establish requirements, traffic assumptions, cost boundaries, and operational ownership. Keep local development simple with Docker Compose where appropriate.

## C. Non-negotiable safety and security requirements

1. **Authorization is server-side.** Verify the authenticated user, role, resource ownership, provider relationship, patient consent, and tenant scope on every sensitive read/write.
2. **Deny by default.** Frontend route guards do not count as authorization.
3. **Account state.** Verify email, active/disabled status, and provider approval rules consistently across all protected operations.
4. **Authentication consolidation.** Identify every Supabase and legacy JWT path. Define one source of identity and a safe transition. Do not remove legacy support until migration/expiry/rollback is understood.
5. **Session storage.** Evaluate moving bearer credentials out of localStorage to an HttpOnly, Secure, SameSite cookie-backed BFF/session design. Account for CSRF protection, refresh, logout, revocation, cross-origin deployment, and existing clients. Do not create an insecure half-migration.
6. **JWT validation.** Pin acceptable algorithms; validate issuer, audience, expiry, signature, and required subject/claims. Never trust role claims supplied by a client or mutable user metadata without server-side policy.
7. **Request validation.** Never ignore JSON binding errors. Validate required fields, ranges, formats, enums, and maximum lengths. Reject malformed and oversized payloads.
8. **Rate limiting.** Use distributed rate limiting for multi-instance deployment. Keep stricter limits for login, signup, OTP, password reset, AI usage, and expensive endpoints.
9. **OTP/password reset.** Store OTPs as hashes, enforce expiry, purpose binding, attempt limits, one-time atomic consumption, and safe enumeration-resistant responses. Use transactions to prevent replay and partial updates.
10. **Secrets and logs.** No secrets in Git, responses, error pages, or logs. Redact tokens, OTPs, credentials, and patient health data.
11. **Files.** Private storage, allowlisted MIME types, file-size limits, random object names, authorization before signed URL generation, scanning where feasible, and retention/deletion policy.
12. **Audit.** Record who accessed or changed sensitive records, when, what action, and the relevant resource identifier. Avoid copying sensitive payloads into audit logs.
13. **AI.** Clearly identify AI output as non-clinical assistance. Do not diagnose, prescribe, or imply emergency triage unless a separately reviewed clinical product specification exists. Apply quotas, limits, redaction, provider timeout handling, and safe fallback behavior.
14. Do not claim regulatory compliance without an appropriate legal, security, and operational assessment.

## D. Architecture and code quality objectives

### Backend
Move toward clear boundaries:
- HTTP handlers: transport, binding, authentication context, response mapping.
- Services/use cases: business rules and orchestration.
- Repositories: database queries and persistence.
- Domain models/DTOs: explicit contracts and validation.
- Platform packages: config, database, cache, storage, observability, queue.

Organize by domain modules, for example:
- auth
- users
- patients
- providers
- healthrecords
- medicines
- prescriptions
- appointments
- insurance
- notifications
- admin
- ai

These are candidate boundaries, not a mandate to create empty packages. Preserve a coherent Go dependency graph; avoid circular dependencies and over-abstraction.

### Frontend
- One auth/session integration, with explicit hydration and refresh behavior.
- Centralized typed API client.
- Consistent server-state fetching/cache strategy.
- Consistent loading, empty, error, retry, and unauthorized states.
- Runtime validation for untrusted API responses where useful.
- Accessible components and form validation.
- Role-aware navigation, while relying on server enforcement for security.
- Avoid duplicated ad hoc `fetch` wrappers and localStorage token reads.

### Database
- Use versioned, reviewed migrations; application startup must not mutate schema.
- Establish primary/foreign keys, unique constraints, check constraints, appropriate nullability, and deletion semantics.
- Normalize identity, role, provider, patient, appointment, and consent relationships where needed.
- Prefer typed date/time columns over strings.
- Add indexes based on actual high-value queries and EXPLAIN/EXPLAIN ANALYZE, not guesswork.
- Add pagination to list endpoints.
- Use transactions for multi-step operations.
- Use optimistic concurrency or idempotency where duplicate requests can cause harm.
- Consider tenant scoping only after validating product requirements. If multiple clinics/hospitals can operate within one deployment, design explicit organization membership and isolation before launch.

## E. Required phase gates

Execute one phase at a time. **Start with Phase 0 only. Do not begin Phase 1 in the same execution unless the user explicitly says to continue after reviewing Phase 0.**

### Phase 0 — Repository audit and delivery plan
- Inspect `git status`, branch, latest commits, tree, manifests, lockfiles, source, tests, CI, environment examples, and deployment configuration.
- Map the product modules, APIs, auth flows, database tables, file handling, external services, and deployment assumptions.
- Identify actual defects with file paths and line references.
- Classify findings P0/P1/P2/P3, distinguishing confirmed defects from risks needing runtime verification.
- Produce `docs/audit/BASELINE_AUDIT.md`, `docs/audit/API_INVENTORY.md`, `docs/audit/DATA_MODEL_INVENTORY.md`, `docs/audit/SECURITY_FINDINGS.md`, and an updated `docs/PROGRESS.md`.
- Create a prioritized implementation backlog with dependencies, acceptance criteria, risk, and estimated effort.
- Propose target architecture and ADRs. Do not modify application code in this phase.
- Report tests available and tests not yet run.

### Phase 1 — Security and correctness baseline
- Fix malformed JSON handling and validation at boundaries.
- Enforce server-side account status, roles, ownership, consent, and provider approval.
- Audit all sensitive endpoints for IDOR/BOLA and privilege escalation.
- Make OTP/password reset one-time and transaction-safe.
- Harden authentication/session lifecycle with a documented migration plan.
- Introduce shared/distributed rate limiting if multi-instance deployment is planned.
- Add regression tests for every security fix.
- No breaking auth migration without compatibility and rollback plan.

### Phase 2 — PostgreSQL integrity and migrations
- Introduce versioned migrations.
- Remove schema mutation from startup only after migration workflow is ready.
- Add foreign keys and constraints after auditing/repairing legacy data.
- Normalize date/time/status representations safely.
- Add indexes based on query inventory and measured plans.
- Add safe migration, rollback, backup, and restore documentation.
- Do not drop or rewrite columns in a single risky deployment. Use expand → migrate → contract.

### Phase 3 — Backend modularization and API contracts
- Refactor handler/service/repository boundaries incrementally.
- Add typed DTOs and centralized error handling.
- Add API versioning where compatibility warrants it.
- Add pagination, filtering, sorting allowlists, and query limits.
- Use transactions and idempotency for booking/payment-like or duplicate-sensitive operations.
- Keep SQL parameterized. Never concatenate user input into SQL identifiers or clauses.
- Add OpenAPI docs and API contract tests.

### Phase 4 — Frontend architecture and experience
- Consolidate auth/session access and API client.
- Remove page-level raw token reads and duplicated network handling.
- Establish robust loading/error/empty/unauthorized states.
- Build actual admin pages only against supported backend capabilities.
- Keep role/permission-sensitive data out of unauthorized routes and caches.
- Add component, integration, and end-to-end tests.

### Phase 5 — Files and background processing
- Move durable uploads from local disk to private object storage.
- Add secure upload/download flows.
- Add queue and worker only where async behavior is required.
- Implement retry policy, idempotency, job status, dead-letter handling, and monitoring.
- Move email, reminders, report generation, and long-running AI jobs off request handlers where appropriate.

### Phase 6 — Deployment and infrastructure
- Containerize the Go API and worker.
- Create local, staging, and production configuration separation.
- Add CI quality gates and safe CD to staging first.
- Add managed database, secrets, storage, and queue integrations only after cost/security review.
- Use infrastructure as code when appropriate.
- Add health/readiness checks, graceful shutdown, migrations as a controlled release step, and rollback.
- Document database connection budgets for horizontally scaled instances.

### Phase 7 — Reliability, performance, and release
- Add structured logging with redaction, metrics, tracing, and alerting.
- Establish service-level objectives and error budgets as product operations mature.
- Run realistic load tests and database query analysis.
- Test backup restore and deployment rollback.
- Perform security review and privacy/data lifecycle review.
- Produce a release readiness report; do not claim production readiness if any critical gate is incomplete.

## F. How to work in every phase

Before coding:
1. State objective and scope.
2. List files/modules to inspect.
3. Identify assumptions and decisions requiring user input.
4. Give an implementation plan with acceptance criteria.
5. Identify migration and rollback risks.

During coding:
1. Make small changes.
2. Keep existing behavior unless explicitly changing it.
3. Add tests before or alongside fixes.
4. Keep dependencies justified and pinned by lockfiles.
5. Never fabricate sample secrets or actual patient data.
6. Avoid changing generated files unless the project workflow requires it.

After coding:
1. Run formatter.
2. Run static analysis/linter/type checks.
3. Run targeted tests, then broader tests.
4. Run build checks.
5. Inspect `git diff --check` and full diff.
6. Confirm no secrets or unintended files were added.
7. Update docs and progress tracker.
8. Report exact commands and results. Clearly label anything not run.
9. Stop and wait for user review.

## G. Performance and scale principles

- Stateless API instances; store shared session/rate-limit/cache state outside process memory when multiple instances run.
- Bound DB connections per instance and calculate total connection budget against database capacity.
- Use pagination and query limits for every potentially large list.
- Cache only data with a clear invalidation and privacy policy.
- Avoid caching patient-specific data in shared caches without strict keys, authorization, and TTL.
- Use asynchronous jobs for slow/non-interactive work.
- Use object storage for documents and large binary objects.
- Measure first; do not prematurely add shards, microservices, or complex event sourcing.
- Provide graceful degradation for external AI/email provider outages.

## H. First task — execute now

Perform **Phase 0 only**.

Do not change application source code, dependencies, database schema, CI/CD, deployment configuration, or secrets during Phase 0.

Inspect the current checkout and produce the audit artifacts listed above. Clearly separate:
- confirmed defects observed in source,
- plausible risks requiring runtime verification,
- recommendations that depend on future product decisions.

At the end, provide:
- architecture summary,
- highest-risk findings,
- prioritized phase backlog,
- dependencies and migration order,
- tests/commands available,
- decisions requiring the product owner's input.

Then stop. Wait for explicit instruction before Phase 1.
