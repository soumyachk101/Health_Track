-- 0002: Pure Go Auth schema (ADR-0001).
--
-- Expand-only: adds the new identity/consent/audit structures alongside the
-- legacy columns. Nothing here drops or renames an existing column, so the
-- legacy HS256 / Supabase-JWKS paths keep working until cutover (ADR-0001
-- rollout step 3 removes them in a later phase).
--
-- Run via the migration runner: `go run ./cmd/migrate` (DATABASE_URL set).
-- Idempotent: re-running is a no-op.

-- users.role: the effective RBAC role used by RequireRoles.
-- Backfilled from user_type / is_superuser; new sign-ups write it directly.
ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'patient';

-- users.status: central account state enforced by AuthRequired().
--   'active'     - may sign in and call protected routes
--   'pending'    - provider account awaiting admin approval (legacy is_approved = 0)
--   'suspended'  - admin-suspended (legacy is_active = 0)
--   'deactivated'- self-removed account
ALTER TABLE users ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';

-- users.password_hash: argon2id for the new /api/v1/auth flows. NULL until
-- the user next sets or resets a password through the new module; the legacy
-- bcrypt `password` column is kept for the legacy login path.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- Backfill role from the legacy fields (one-off; the UPDATE is cheap even on
-- large tables and is safe to re-run).
UPDATE users SET role = CASE
  WHEN is_superuser = 1 OR user_type = 'admin' THEN 'admin'
  WHEN user_type = 'doctor' THEN 'doctor'
  WHEN user_type = 'provider' THEN 'provider'
  ELSE 'patient'
END;

-- Backfill status from the legacy flags.
UPDATE users SET status = CASE
  WHEN is_active = 0 THEN 'suspended'
  WHEN user_type IN ('doctor', 'provider') AND is_approved = 0 THEN 'pending'
  ELSE 'active'
END;

CREATE INDEX IF NOT EXISTS idx_users_status ON users (status);
CREATE INDEX IF NOT EXISTS idx_users_role ON users (role);

-- oauth_accounts: external identity providers linked to an app user. Google
-- is the first provider; the unique (provider, provider_account_id) pair is
-- the natural key of a login.
CREATE TABLE IF NOT EXISTS oauth_accounts (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  provider_account_id TEXT NOT NULL,
  email TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_oauth_accounts_provider_account UNIQUE (provider, provider_account_id)
);

CREATE INDEX IF NOT EXISTS idx_oauth_accounts_user_id ON oauth_accounts (user_id);

-- patient_consents: a patient authorizes a doctor/provider to read their
-- records. Doctor/provider read access in the service layer MUST match a
-- granted row; without one the only visible data is the caller's own.
CREATE TABLE IF NOT EXISTS patient_consents (
  id SERIAL PRIMARY KEY,
  patient_user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  grantee_user_id INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
  scope TEXT NOT NULL DEFAULT 'records',
  status TEXT NOT NULL DEFAULT 'granted',
  granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at TIMESTAMPTZ,
  CONSTRAINT uq_patient_consents_pair UNIQUE (patient_user_id, grantee_user_id, scope)
);

CREATE INDEX IF NOT EXISTS idx_patient_consents_grantee ON patient_consents (grantee_user_id, patient_user_id);
CREATE INDEX IF NOT EXISTS idx_patient_consents_patient ON patient_consents (patient_user_id);

-- audit_logs: who did what to which record. The user_id FK lets an audit
-- query pull one user's trail; details is free-form (no patient PII).
CREATE TABLE IF NOT EXISTS audit_logs (
  id SERIAL PRIMARY KEY,
  user_id INTEGER REFERENCES users (id) ON DELETE SET NULL,
  action TEXT NOT NULL,
  resource_type TEXT NOT NULL DEFAULT '',
  resource_id TEXT NOT NULL DEFAULT '',
  ip_address TEXT NOT NULL DEFAULT '',
  details TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created ON audit_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs (resource_type, resource_id);
