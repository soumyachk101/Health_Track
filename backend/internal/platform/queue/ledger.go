// L3: [INPUT] depends on *database.DB (background_jobs) + the JobStatus
// constants; [OUTPUT] exports MarkJob / MarkJobStatus / MarkJobResult /
// EnsureJobRow (the worker's ledger stamps) and the logWarn hook;
// [POS] the Postgres half of the queue: Redis holds the queue, this holds
// the user-visible truth; [PROTOCOL] Update this header on change, then
// check CLAUDE.md
package queue

import (
	"context"
	"log"
	"time"

	"healthtrack/internal/database"
)

// logWarn is the one logging hook the queue package exposes: misconfigured
// REDIS_URL, lost ledger writes. Kept as a function so a test can assert
// the degradation path without capturing log output.
var logWarn = log.Printf

// scanJobRow lifts a background_jobs row (map form from *database.DB)
// into the JobRecord the status endpoint returns.
func scanJobRow(row map[string]any) (*JobRecord, error) {
	rec := &JobRecord{}
	if v, ok := row["task_id"].(string); ok {
		rec.TaskID = v
	}
	if v, ok := row["type"].(string); ok {
		rec.Type = v
	}
	switch v := row["owner_id"].(type) {
	case int64:
		rec.OwnerID = v
	case int:
		rec.OwnerID = int64(v)
	}
	if v, ok := row["status"].(string); ok {
		rec.Status = v
	}
	switch v := row["attempts"].(type) {
	case int64:
		rec.Attempts = int(v)
	case int:
		rec.Attempts = v
	}
	if v, ok := row["last_error"].(string); ok {
		rec.LastError = v
	}
	// result is JSONB: pgx returns it as []byte (nil when the column is
	// NULL, which stays the zero value "").
	if v, ok := row["result"].([]byte); ok {
		rec.Result = string(v)
	}
	if v, ok := row["enqueued_at"].(time.Time); ok {
		rec.EnqueuedAt = v
	}
	if v, ok := row["updated_at"].(time.Time); ok {
		rec.UpdatedAt = v
	}
	return rec, nil
}

// MarkJob stamps the ledger row's status/attempts/last_error. Called from
// the worker before and after each task; failures are logged, not returned
// — a ledger blip must not kill a task that is itself succeeding.
func MarkJob(ctx context.Context, db *database.DB, key, status, lastErr string, attempts int) {
	_, err := db.Exec(ctx, `
		UPDATE background_jobs
		SET status = $2, attempts = $3, last_error = $4, updated_at = NOW()
		WHERE idempotency_key = $1`, key, status, attempts, lastErr)
	if err != nil {
		logWarn("queue: ledger update failed for %s: %v", key, err)
	}
}

// MarkJobStatus stamps status + last_error WITHOUT touching attempts
// (completion must not erase the retry count the processing stamps).
func MarkJobStatus(ctx context.Context, db *database.DB, key, status, lastErr string) {
	_, err := db.Exec(ctx, `
		UPDATE background_jobs SET status = $2, last_error = $3, updated_at = NOW()
		WHERE idempotency_key = $1`, key, status, lastErr)
	if err != nil {
		logWarn("queue: ledger status update failed for %s: %v", key, err)
	}
}

// MarkJobResult stores the task's result payload (JSON) on the row.
func MarkJobResult(ctx context.Context, db *database.DB, key string, result []byte) error {
	_, err := db.Exec(ctx, `
		UPDATE background_jobs SET result = $2, updated_at = NOW()
		WHERE idempotency_key = $1`, key, result)
	if err != nil {
		logWarn("queue: ledger result update failed for %s: %v", key, err)
	}
	return err
}

// EnsureJobRow creates the ledger row for a key that the original process
// lost between push and insert (crash recovery): the worker's first
// attempt finds no row and creates one here so the status endpoint can
// track it. typeOf may be "" (the row is stamped minimal).
func EnsureJobRow(ctx context.Context, db *database.DB, key, typeOf string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO background_jobs (task_id, type, status, idempotency_key, enqueued_at, updated_at)
		VALUES ($1, $2, 'queued', $3, NOW(), NOW())
		ON CONFLICT (idempotency_key) DO NOTHING`, key, typeOf, key)
	return err
}
