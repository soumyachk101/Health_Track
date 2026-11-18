# PostgreSQL Data and Migration Plan

## Principles
- Do not mutate schema at application startup.
- Every schema change must be versioned, reviewed, and repeatable.
- Prefer expand → migrate/backfill → validate → contract.
- Keep old and new application versions compatible during rollout when required.
- Never run destructive migration commands against production without explicit owner approval and a tested backup/restore path.

## Step 1 — Inventory
Record for every table:
- Primary key.
- Foreign-key candidates.
- Unique constraints.
- Nullable/default semantics.
- Relationships and delete behavior.
- Data volume and growth.
- Queries and sort/filter patterns.
- Sensitive fields and retention requirements.
- Existing invalid/orphaned records.

## Step 2 — Establish migrations
Choose one migration tool after checking current project conventions: Goose, Atlas, or golang-migrate. Do not introduce multiple migration systems.

Example layout:

```text
backend/migrations/
  000001_baseline.up.sql
  000001_baseline.down.sql
  000002_add_relationship_constraints.up.sql
  000002_add_relationship_constraints.down.sql
```

If the current schema is embedded and executed during startup, replace this only after migration execution is wired into controlled deployment/release operations.

## Step 3 — Integrity rollout
1. Detect duplicates, nulls, orphan rows, and invalid enum/status values.
2. Produce a remediation report without modifying data.
3. Agree on remediation behavior.
4. Backfill or quarantine invalid records.
5. Add constraints in a production-safe order.
6. Verify query plans and app compatibility.
7. Remove obsolete columns only in a later contract migration.

## Step 4 — Relationships and access scope
Model relationships explicitly:
- User identity.
- Patient profile.
- Provider/doctor profile.
- Provider organization membership if required.
- Patient-provider relationship and consent.
- Health records and prescriptions.
- Appointment participants and status lifecycle.
- Audit events.

Do not assume that a doctor can see every patient or that an admin can read every medical record.

## Step 5 — Data types
Evaluate converting date/time strings to DATE, TIME, TIMESTAMPTZ, or appropriate numeric/enum types. Define timezone semantics first. Avoid silent conversion of ambiguous legacy values.

## Step 6 — Index strategy
Build an API/query inventory. Use EXPLAIN ANALYZE on representative safe datasets. Consider composite indexes for:
- Ownership + chronological ordering.
- Appointment participant + date/time.
- Provider + status + creation time.
- OTP lookup by normalized user/purpose/expiry.

Avoid redundant indexes and account for write amplification.

## Step 7 — Transactions and concurrency
Use transactions for:
- Account/profile creation involving multiple tables.
- OTP consumption and password reset.
- Appointment reservation and collision prevention.
- Status transition plus audit event.
- Upload metadata and ownership records.

Use uniqueness/exclusion constraints or explicit locking for race-sensitive booking slots. Add idempotency keys for repeatable client actions where duplicate requests could create duplicate records.

## Step 8 — Verification and recovery
For each migration:
- Apply to a disposable database.
- Test upgrade from previous supported schema.
- Test rollback only if rollback is truly safe and data loss is understood.
- Run schema and integration tests.
- Verify restore from backup.
- Record duration, locks, and expected impact.
