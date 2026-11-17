# Health_Track — Engineering Roadmap

## Delivery strategy
Use incremental modernization. Keep the application deployable after every phase. Prefer a modular monolith and delay microservice extraction until measured scaling or organizational needs justify it.

## Priority definitions
- P0: security, data integrity, or availability risk that must be addressed before broad production use.
- P1: core maintainability, correctness, or scaling gap.
- P2: operational maturity and product completeness.
- P3: later optimization or enhancement.

## Phase 0 — Audit (no application changes)
Deliver:
- Baseline audit with paths/line references.
- API and database inventories.
- Security findings classified by confidence and severity.
- Current build/test/deploy state.
- Target architecture, dependency map, backlog, and ADRs.

Exit gate: product owner has reviewed findings and accepted phase order.

## Phase 1 — Security and correctness
Deliver:
- Strict JSON binding and request validation.
- Server-side active/approved account checks.
- Object-level authorization and consent-aware access checks.
- Authentication/session design and migration plan.
- Atomic OTP/password-reset workflow.
- Distributed rate limiting where instances are replicated.
- Security regression test suite.

Exit gate: sensitive routes have explicit authorization tests; no unresolved known critical IDOR/privilege escalation defects.

## Phase 2 — Data foundation
Deliver:
- Versioned migrations.
- Startup no longer performs implicit schema mutation.
- Foreign keys, constraints, and deletion behavior.
- Typed date/time fields with safe migration.
- Query-backed indexes.
- Pagination contract.
- Backup/restore and migration runbooks.

Exit gate: migration tested on representative copy; backup restore demonstrated; old and new app versions coexist during expand/contract rollout where required.

## Phase 3 — Backend architecture
Deliver:
- Domain module boundaries.
- Handler/service/repository separation.
- Typed DTOs and centralized errors.
- `/api/v1` or explicit API compatibility strategy.
- Pagination, bounded filtering, transactions, idempotency.
- OpenAPI and contract tests.

Exit gate: existing feature workflows pass regression tests and API behavior is documented.

## Phase 4 — Frontend architecture
Deliver:
- Central session integration.
- Central typed API client.
- Consistent async UI states and validation.
- Role-aware routing and cache isolation.
- Fully implemented admin pages or clearly scoped deferred features.
- E2E coverage for primary user journeys.

Exit gate: critical user journeys run in browser automation without auth/data leakage.

## Phase 5 — Files and asynchronous work
Deliver:
- Private object storage.
- Upload/download authorization and limits.
- Background worker and queue for appropriate tasks.
- Retries, idempotency, dead-letter strategy, and job observability.

Exit gate: worker failure/retry behavior tested; file access authorization tested.

## Phase 6 — Delivery infrastructure
Deliver:
- Reproducible container build.
- Local/staging/production separation.
- CI checks and controlled staging deployment.
- Secrets manager integration.
- Database connection budget and scaling configuration.
- Health/readiness checks, graceful shutdown, rollback plan.
- Infrastructure-as-code where justified.

Exit gate: staging release can be built, deployed, tested, and rolled back reproducibly.

## Phase 7 — Reliability and release readiness
Deliver:
- Structured, redacted logs.
- Metrics, tracing, dashboards, alerts.
- Load and stress test report.
- Restore and rollback evidence.
- Privacy and data lifecycle review.
- Security review and final readiness report.

Exit gate: all critical release gates are green or explicitly waived by accountable owners with documented risk acceptance.
