-- 0004: background-job ledger (ADR-0002).
--
-- background_jobs mirrors the asynq queue state in Postgres so
-- GET /api/v1/jobs/:id can serve status without reading Redis. task_id is
-- the asynq task ID (a UUID string, set with asynq.TaskID): asynq assigns
-- no numeric ids of its own, so the ledger carries the surrogate key users
-- poll on and the UUID asynq knows the task under.
--
-- The row is inserted after the successful Redis push, so a process that
-- crashes between push and insert may leave an orphaned task in the queue
-- (it still runs, retries and reaches the asynq archive; only its ledger
-- row is missing, which surfaces as 404 on the status endpoint).
--
-- idempotency_key is <type>:<uuid-or-file-id> and unique: re-enqueueing the
-- same logical work returns the existing row instead of a second task.
--
-- status: queued -> processing -> completed | failed; failed after max
-- retries is archived to the asynq dead-letter (retry-archive) and the row
-- is stamped dead.

CREATE TABLE IF NOT EXISTS background_jobs (
  id BIGSERIAL PRIMARY KEY,
  task_id TEXT NOT NULL UNIQUE,
  type TEXT NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued', 'processing', 'completed', 'failed', 'dead')),
  idempotency_key TEXT UNIQUE,
  -- the enqueuer: GET /api/v1/jobs/:id refuses service to any user who is
  -- not the owner (the pinned jobs contract) or admin.
  owner_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  result JSONB,
  enqueued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- worker scans and the status endpoint.
CREATE INDEX IF NOT EXISTS idx_background_jobs_status ON background_jobs(status, enqueued_at);
