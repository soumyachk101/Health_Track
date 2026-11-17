# Health_Track — Phase 0 Database Inventory

Source of truth: `backend/internal/database/schema.sql` (13 tables), `backend/internal/models/models.go`, `backend/test/integration_test.go`, `backend/scripts/`. Verified against checked-out `main`, 17 Nov 2026.

## 1. Existing tables (13)

All 13 tables: `users`, `service_providers`, `otps`, `health_records`, `medicines`, `prescriptions`, `mental_health_logs`, `insurance_policies`, `lifestyle_logs`, `activity_logs`, `system_settings`, `appointments`, `services`, `service_requests`. (14 counting `system_settings`.) Full column detail in `schema.sql`.

### Foreign keys
- **No `FOREIGN KEY` constraints are declared anywhere.** Every `user_id`/`patient_id`/`doctor_id`/`provider_id` is a bare `INTEGER` with no FK to `users.id`. Referential integrity is not enforced by the DB.

### Date/time handling
| Table.column | Type | Issue |
|---|---|---|
| `users.date_of_birth` | `TEXT` | no date type |
| `medicines.start_date` / `end_date` | `TEXT` | no date type |
| `prescriptions.prescription_date` / `follow_up_date` | `TEXT` | no date type |
| `insurance_policies.start_date` / `end_date` | `TEXT` | **drives "Expiring Soon"** — no date type, no index |
| `appointments.date` / `time` | `TEXT` | no date type; no slot constraint |
| `service_requests.scheduled_date` | `TEXT` | no date type |
| `otps.expires_at` | `TEXT` | **compared lexically** (`expires_at > now` in `auth.go`); must be `timestamptz` |
| all `*_created_at` / `updated_at` | `TIMESTAMPTZ` | ✔ good |
| `lifestyle_logs.recorded_at` | `DATE` | partial |

Timezone semantics are undefined for the `TEXT` date fields (UTC vs user-local). Must settle before typing them.

### Indexes
- Only **one** explicit index: `idx_users_auth_uuid ON users(auth_uuid)` (`schema.sql:33`). No FK indexes (because no FKs), no index on `insurance_policies.end_date`, `appointments.(doctor_id,date)`, or `otps.(email,otp_type,expires_at)`.
- `users.username` / `users.email` have `UNIQUE NOT NULL` (✔ implicit unique index), `service_providers.user_id` is `UNIQUE` (✔).

### Constraints
- No `CHECK` constraints on `otps.is_used`, `appointments.status`, `service_requests.status`, `medicines.is_active`, `insurance_policies.is_active`, or `services.is_active` — status enums are free-form `TEXT`.
- `prescriptions.document` and `insurance_policies.document` are `TEXT` (holds a file path/URL today) — to be replaced by a `medical_files` reference (ADR-0002).

## 2. Schema lifecycle (D1 — implicit mutation on startup)

`backend/internal/database/postgres.go:15` embeds `schema.sql`; `Connect()` (`postgres.go:29-57`) calls `splitStatements(schemaSQL)` and `pool.Exec`s **every statement on each startup** (`postgres.go:50-55`). Because every statement is `CREATE TABLE IF NOT EXISTS`, it is idempotent and harmless today, but:
- The schema is not versioned or reviewable per-change.
- `ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_uuid` (`schema.sql:32`) runs as a migration-like DDL on every boot — the exact "implicit schema mutation on startup" the plan must remove.
- **Fix:** introduce versioned SQL migrations under `backend/migrations/` (ADR-0001 step 2); keep `schema.sql` as a fresh-DB bootstrap only until the migration system is proven, then remove the startup DDL in a controlled release.

## 3. Tables the planned work needs

| New table | Required by | Status |
|---|---|---|
| `users.role`, `users.status`, `users.password_hash` (argon2id) | ADR-0001 | **Missing** — today `users.password` is the bcrypt column, role is derived (`DeriveRole`), no `status` enum (uses `is_active`/`is_approved` ints) |
| `oauth_accounts` (provider, provider_sub, user_id) | ADR-0001 | **Missing** — today `users.auth_uuid` (Supabase) + Google `sub` are ad-hoc |
| `patient_consents` (patient_id, grantee_id, scope, expiry, status) | ADR-0001 / Phase 4 | **Missing** |
| `audit_logs` (actor, action, resource, ip, at) | ADR-0001 / S11 | **Missing** — today `activity_logs` is a coarse event log, not an audit trail |
| `medical_files` (id, owner_id, patient_id, object_key, mime_type, size_bytes, status, created_at) | ADR-0002 | **Missing** — today `prescriptions.document` / `insurance_policies.document` `TEXT` + local `backend/uploads/` |
| `background_jobs` (id, type, payload, status, idempotency_key, attempts, last_error, enqueued_at, ...) | ADR-0002 | **Missing** |

All six are greenfield; no partial version exists. Migrations must create them with the FKs and checks noted above (expand, not drop, the legacy `document` TEXT columns).

## 4. Legacy adoption notes

- `users.password` for Supabase-provisioned rows is the sentinel string `"supabase:<auth_uuid>"` (`auth.go:550`) — not a real hash. When moving to `password_hash` (argon2id), these rows must be flagged password-less (OTP/OAuth login only).
- Google-login rows get a random 32-byte hex password (`auth.go:905-911`) — also effectively password-less.
- `scripts/supabase_migrate.sql` and `scripts/setup_admin`, `scripts/check_users` are one-off DDL/seed scripts, not versioned migrations. Preserve them as history; replace with `backend/migrations/`.
- `system_settings` is a key/value table (e.g. feature flags / config) — keep as-is.

## 5. Migration strategy (to be executed in Phase 2, not Phase 0)

1. Pick one tool (Goose or golang-migrate recommended; Atlas also viable). **Product-decision-needed.**
2. Baselining: `schema.sql` → migration `0001_baseline` (as-is, for fresh DBs). For the **existing** live DB: snapshot → diff → `0002_adopt` that records current state without data loss.
3. Add FKs/constraints in separate safe migrations: expand → backfill → validate → contract (scan for null/orphan data first).
4. Type the `TEXT` date fields only after settling timezone semantics; do not silently parse legacy strings.
5. Keep the startup-embedded DDL until the migration path is proven on a disposable copy, then remove it in a controlled release.
6. Make the pool `MaxConns` (`postgres.go:38`, hardcoded `5`) configurable and document the total Postgres connection budget across Vercel instances + worker.

## 6. Test posture
- `backend/test/integration_test.go` is build-tagged `//go:build integration` and needs a fresh Postgres DSN.
- `backend/internal/*_test.go` are unit tests.
- **Not run this session** (no live DSN, and Phase 0 forbids source changes); record command + result when executed in Phase 1+.
