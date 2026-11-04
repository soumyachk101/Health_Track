# Backend integration tests

Runs the full `hands.BuildRouter` stack against a real Postgres database
(`database.Connect` self-bootstraps the embedded `schema.sql` on connect, so a
fresh empty database is all that is needed).

## Running

The DSN comes from `INTEGRATION_DSN` (falling back to `DATABASE_URL`). If
neither is set, the suite skips cleanly (exits 0) instead of failing — so a
plain `go test` without the tag or DSN does nothing.

```sh
cd backend
INTEGRATION_DSN="postgres://user:pass@localhost:5433/healthtrack?sslmode=disable" \
  go test -tags integration ./test/...
```

### Option A: Docker (throwaway)

```sh
docker run -d --name it-pg -e POSTGRES_PASSWORD=pg -p 5433:5432 postgres:16
export INTEGRATION_DSN="postgres://postgres:pg@localhost:5433/postgres?sslmode=disable"
cd backend && go test -tags integration ./test/...
docker rm -f it-pg
```

### Option B: local throwaway Postgres (no Docker)

initdb + pg_ctlstart on port 5433, then point `INTEGRATION_DSN` at it:

```sh
initdb -D /tmp/it-pg-data
pg_ctl -D /tmp/it-pg-data -o "-p 5433 -k /tmp" -l /tmp/it-pg.log start
createdb -p 5433 -h /tmp healthtrack   # or use the "postgres" database
export INTEGRATION_DSN="postgres://localhost:5433/healthtrack?sslmode=disable"
cd backend && go test -tags integration ./test/...
pg_ctl -D /tmp/it-pg-data stop
```

Use one DSN for the whole run. Tests that create users wipe their `it_*`
users/`service_providers` rows on setup (`newServerIsolated`), so the same
database can be re-run without conflicts; identifiers are uniquified with a
timestamp so parallel runs on shared DBs don't collide.

## Coverage notes

- Every router group is exercised through the real HTTP server: health/NoRoute,
  CORS, trailing slash, register→login→protected core (profile, dashboard,
  health-track, medicines, past-records, mental-health, lifestyle, insurance,
  doctor-gated appointments), admin stats/users/approve-reject + patient
  gating, the forgot/reset-password OTP state transition, chatbot error paths,
  and google-login guards.
- Negative auth attempts each use a distinct username/email (the limiter's
  `RateKey` is clientIP:identifier, with a 5-failures-per-5-min brute cap and
  a 30-requests-per-15-min flood cap), so no identifier accumulates failures
  and nothing trips the 429 path.
- NoRoute: the `NoRoute` handler is registered for GET only (`r.NoRoute`), so
  the test asserts 200 JSON for unknown GETs and 404 (gin's default NoRoute)
  for unknown POSTs — that is the real router behavior, verified against
  `router.go`.
- The live AI happy path is intentionally not tested here; only the
  config-error contracts (503 key-missing, 400 empty input) are.
