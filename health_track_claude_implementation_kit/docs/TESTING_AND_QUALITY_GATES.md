# Testing Strategy and Quality Gates

## Backend — Go
Minimum checks:
```bash
gofmt -w ./...
go vet ./...
go test ./...
```

Run race checks where practical:
```bash
go test -race ./...
```

Also test:
- Handler validation and status codes.
- Service business rules.
- Repository SQL against PostgreSQL, not only mocks.
- Authorization and account states.
- Two-user IDOR/BOLA cases.
- OTP expiry, replay, and concurrent consumption.
- Transaction rollback.
- Pagination limits and ordering.
- Database outage and context cancellation.
- AI/email provider timeout and error behavior.

Do not claim test coverage or success unless commands were actually run.

## Frontend
Inspect existing scripts before choosing commands. Typical checks:
```bash
npm ci
npm run lint
npm test
npm run build
```

If scripts are outdated or broken, correct them as a separately explained change. Do not hide lint/test failures.

Test:
- Auth hydration, refresh, logout, expired session.
- Role and permission UX.
- API error/loading/empty states.
- Form validation.
- Sensitive UI state/cache isolation between accounts.
- Accessibility and keyboard interaction.

## End-to-end
Cover primary workflows:
1. Signup and verification.
2. Login/logout/session expiry.
3. Patient adds and views their own health record.
4. Different patient cannot access that record.
5. Provider approval and booking workflow.
6. Doctor sees only authorized appointments/patients.
7. Admin actions are restricted and audited.
8. File upload/download authorization.
9. AI chatbot quota and provider failure handling.

Use synthetic records only.

## Performance
Before load test:
- Define target RPS/concurrency and realistic user flows.
- Define p95/p99 latency and error thresholds.
- Prepare isolated test data and environment.
- Verify database connection limits and pool sizing.

Measure:
- API throughput and latency.
- DB query latency and pool saturation.
- Slow queries and lock contention.
- Queue lag and worker throughput.
- Memory/CPU and cold starts.
- Error rates during external provider degradation.

## CI quality gates
A pull request should run:
- Formatting check.
- Lint/static analysis.
- Typecheck.
- Unit tests.
- Integration tests where provisionable.
- Build.
- Dependency and secret scanning.
- Migration validation.
- Diff/format checks.

## Completion report format
For each phase, report:
- Commands run.
- Result for each command.
- Tests not run and why.
- Defects fixed.
- Files changed.
- Migration details.
- Compatibility risks.
- Remaining known issues.
