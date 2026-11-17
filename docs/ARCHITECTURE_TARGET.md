# Target Architecture — Health_Track

## Recommended starting point
A modular monolith with a stateless Go API. This is simpler to operate than microservices and allows domain boundaries to mature before extraction.

## Logical view

```text
Patients / Doctors / Providers / Admins
                 |
           Next.js Web App
                 |
       Session / BFF boundary
                 |
           Go HTTP API
                 |
     Authentication + Authorization
                 |
   -----------------------------------
   | Domain Services / Use Cases      |
   | Auth | Patient | Health | Booking|
   | Provider | Admin | AI | Notify   |
   -----------------------------------
          |            |          |
      PostgreSQL    Object     Queue / Jobs
                    Storage        |
                                Worker
                                  |
                          Email / AI / Reports

Cross-cutting: configuration, secrets, structured logs, metrics, tracing,
rate limiting, audit events, health checks, backup and recovery.
```

## Suggested backend boundaries

```text
backend/
  cmd/
    api/
    worker/
  internal/
    platform/
      config/
      database/
      cache/
      storage/
      queue/
      observability/
    modules/
      auth/
      users/
      patients/
      providers/
      healthrecords/
      medicines/
      prescriptions/
      appointments/
      insurance/
      notifications/
      admin/
      ai/
    shared/
      errors/
      validation/
      pagination/
  migrations/
  tests/
```

Adapt this layout to the existing Go module and deployment adapter. Do not create empty packages or move everything in one commit.

## Suggested module internals

```text
healthrecords/
  handler.go       # HTTP transport only
  service.go       # business rules and orchestration
  repository.go    # SQL persistence
  dto.go           # request/response contracts
  model.go         # internal domain types
  validation.go
  service_test.go
  handler_test.go
```

## Data access
- Keep SQL parameterized.
- Use context-aware database operations.
- Use transactions for multi-step updates.
- Return typed models rather than unstructured maps where practical.
- Enforce ownership and tenant scope in repository query predicates and service authorization.
- Never rely on the UI to filter sensitive records.

## Scaling principles
1. Stateless API instances.
2. Explicit DB pool limits and total connection budget.
3. Pagination and bounded query complexity.
4. Redis only for distributed state or measured cache use.
5. Private object storage for durable files.
6. Queue/worker for work not required to finish the HTTP request.
7. Measure before extracting services or adding sharding.

## Deployment target (candidate, not an assumption)
- Next.js: Vercel or existing supported host.
- Go API: Cloud Run or another container platform.
- PostgreSQL: managed PostgreSQL / Cloud SQL.
- Redis: managed Redis only if distributed rate limits or measured cache needs require it.
- Files: private object storage.
- Jobs: managed queue and separate worker.
- Secrets: managed secrets store.
- Observability: platform logs, metrics, traces, dashboards and alerts.

## Required architecture decisions
Document decisions about:
- Single auth identity source and session boundary.
- Role and permission model.
- Patient-provider consent and data sharing.
- Single clinic versus multi-organization tenancy.
- File storage and retention.
- Async job delivery/retry semantics.
- API compatibility and versioning.
- Deployment platform and cost budget.
