# Deployment and Operations Runbook

This is a target runbook template. Confirm the actual hosting provider and configuration before changing deployments.

## Environments
Maintain separate:
- Local development.
- Staging/testing.
- Production.

Keep credentials, databases, storage buckets, Supabase projects, and AI provider keys separate per environment.

## Candidate deployment layout
- Next.js frontend: retain current host (for example Vercel) unless there is a reason to migrate.
- Go API: container platform such as Cloud Run.
- PostgreSQL: managed PostgreSQL.
- Redis: managed Redis only when shared rate limiting/cache is required.
- Documents: private object storage.
- Jobs: managed queue and worker.
- Secrets: managed secret store.

Do not change provider or region without confirming budget, data residency, latency, backup, and operator access.

## Release sequence
1. Review and approve PR.
2. Build immutable versioned artifacts.
3. Run tests, scans, and migration validation.
4. Back up production database before risky migrations.
5. Apply backward-compatible migrations through a controlled job.
6. Deploy API and worker.
7. Deploy frontend.
8. Run smoke and health checks.
9. Observe logs, latency, errors, DB pool, and job queue.
10. Keep rollback artifacts and rollback instructions ready.

## Database connection budget
Calculate:

`maximum application instances × maximum pool connections per instance + workers + migration/admin connections`

This total must stay below the database connection budget with operational headroom. Tune per-instance pool size, max instances, and concurrency together.

## Health checks
Distinguish:
- Liveness: process is alive.
- Readiness: instance can safely accept traffic and required dependencies are usable.
- Dependency diagnostics: DB, queue, storage, and external provider status.

Do not expose internal diagnostics publicly with credentials or sensitive configuration.

## Rollback
Document rollback for:
- Frontend artifact.
- API/worker artifact.
- Feature flags/config.
- Database migration.

A database rollback is not always safe after writes occur. Prefer forward fixes or expand/contract migrations when data has already been written in the new format.

## Backups and recovery
- Define backup frequency and retention.
- Enable point-in-time recovery if supported and appropriate.
- Restrict backup access.
- Test restore periodically in an isolated environment.
- Record RPO/RTO targets with the product owner.
- Do not claim recovery readiness until a restore has been demonstrated.

## Incident basics
- Preserve request/correlation IDs.
- Redact sensitive data.
- Identify impacted service and time window.
- Check deployment changes, error rates, latency, database saturation, queue lag, and provider outages.
- Roll back or disable a feature only under the documented incident procedure.
- Record root cause and preventive action.
