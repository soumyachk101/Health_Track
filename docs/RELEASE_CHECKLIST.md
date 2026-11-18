# Production Release Checklist

## Security and privacy
- [ ] All sensitive endpoints enforce authorization server-side.
- [ ] Patient ownership, provider relationship, consent, and tenant checks are tested.
- [ ] Account active/approval policy is enforced consistently.
- [ ] Session lifecycle, token validation, refresh, logout, and revocation are tested.
- [ ] OTP/reset links are hashed, expiring, and atomically one-time.
- [ ] Rate limiting works across multiple instances.
- [ ] Secrets scan is clean; no credentials in Git or logs.
- [ ] Logs/traces redact sensitive health and identity data.
- [ ] File access is private and authorization checked.
- [ ] AI privacy, quota, disclaimer, and failure handling reviewed.

## Data
- [ ] Migrations are versioned and reviewed.
- [ ] Upgrade tested from previous supported schema.
- [ ] Rollback/forward-fix plan documented.
- [ ] Foreign keys and constraints validated.
- [ ] Indexes verified against query plans.
- [ ] Pagination and query limits applied.
- [ ] Backup and restore tested.
- [ ] Retention/deletion/export policy documented.

## Quality
- [ ] Go format, vet, and tests pass.
- [ ] Frontend lint, typecheck, tests, and build pass.
- [ ] Integration tests pass against PostgreSQL.
- [ ] E2E critical user journeys pass.
- [ ] Authorization negative tests pass.
- [ ] Performance/load test report reviewed.
- [ ] Dependency and container scans reviewed.

## Operations
- [ ] Staging deployment verified.
- [ ] Production config isolated and secrets managed.
- [ ] DB connection budget documented.
- [ ] Liveness/readiness endpoints verified.
- [ ] Dashboards and alerts configured.
- [ ] Graceful shutdown verified.
- [ ] Rollback process documented and rehearsed.
- [ ] On-call/incident ownership established.

## Release decision
- [ ] Go / no-go decision recorded by accountable owner.
- [ ] Remaining risks and any accepted exceptions documented.
