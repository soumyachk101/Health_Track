-- 0003: R2 object-storage manifest (ADR-0002).
--
-- medical_files is the ownership/consent ledger for patient documents that
-- live in private Cloudflare R2. The object itself is never in Postgres;
-- only the key and the validation facts (mime/size) are, so confirm can
-- verify the upload and download-url can be gated on consent.
--
-- status lifecycle: pending (upload-url issued) -> active (confirmed by
-- HeadObject) -> deleted (purged by files:cleanup_stale or admin action).
--
-- Run via the migration runner: `go run ./cmd/migrate` (DATABASE_URL set).
-- Idempotent: re-running is a no-op.

CREATE TABLE IF NOT EXISTS medical_files (
  id SERIAL PRIMARY KEY,
  owner_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  -- patient_id records whose records the file documents; for a self-upload
  -- it equals owner_id, for a doctor upload it is the patient.
  patient_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
  object_key TEXT NOT NULL UNIQUE,
  mime_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'deleted')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_medical_files_owner ON medical_files(owner_id);
-- cleanup scan: pending rows older than the purge threshold.
CREATE INDEX IF NOT EXISTS idx_medical_files_status ON medical_files(status, created_at);
