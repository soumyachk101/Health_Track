// L3: [INPUT] depends on the queue task builders, *database.DB for the
// background_jobs ledger, and the asynq client (behind the redisPusher
// seam); [OUTPUT] exports Producer (NewProducer, Enqueue, EnqueueScheduled,
// Configured, Close), the JobRecord type and the job-status lookups used by
// the jobs handler; [POS] the single enqueue path for the whole app: when
// REDIS_URL is unset every Enqueue returns ErrNotConfigured and callers
// fall back to their synchronous path (ADR-0002 feature-flag rollout);
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"healthtrack/internal/database"
)

// JobStatus values mirror the pinned status endpoint contract
// (queued | processing | completed | failed), plus dead (DLQ archived).
const (
	JobQueued     = "queued"
	JobProcessing = "processing"
	JobCompleted  = "completed"
	JobFailed     = "failed"
	JobDead       = "dead"
)

// ErrNotConfigured is returned by a Producer with no Redis: the caller
// takes its sync fallback. It is the safe-degradation contract — the app
// must deploy with REDIS_URL unset.
var ErrNotConfigured = errors.New("queue not configured (REDIS_URL empty)")

// redisPusher is the seam between Producer and a live asynq client. The
// production impl wraps *asynq.Client; tests inject a fake that can return
// ErrTaskIDConflict so the idempotency + DLQ paths run without Redis.
type redisPusher interface {
	Push(ctx context.Context, task *asynq.Task, opts ...asynq.Option) error
}

// clientPusher adapts the asynq client to the seam.
type clientPusher struct{ c *asynq.Client }

func (p clientPusher) Push(ctx context.Context, task *asynq.Task, opts ...asynq.Option) error {
	_, err := p.c.EnqueueContext(ctx, task, opts...)
	return err
}

// Producer enqueues tasks and owns the background_jobs ledger. Idempotency
// is two-layered: the asynq TaskID dedupes in-flight duplicates in Redis
// (same key → ErrTaskIDConflict), and the ledger's idempotency_key column
// answers "has this logical job run before?".
type Producer struct {
	db     *database.DB
	client *asynq.Client
	pusher redisPusher
}

// NewProducer wires the asynq client from the Redis URL. An empty or
// malformed URL yields a Producer that never reaches Redis
// (Configured=false, Enqueue → ErrNotConfigured) — the app stays
// deployable without a queue.
func NewProducer(db *database.DB, redisURL string) *Producer {
	if redisURL == "" {
		return &Producer{db: db}
	}
	rconn, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		// A malformed REDIS_URL must not crash the API process: the
		// producer degrades to unconfigured and the sync path carries
		// traffic. The operator sees the misconfiguration in logs.
		logWarn("asynq: bad REDIS_URL (%v); queue disabled, sync fallback active", err)
		return &Producer{db: db}
	}
	client := asynq.NewClient(rconn)
	return &Producer{db: db, client: client, pusher: clientPusher{client}}
}

// newProducerForTest injects a pusher seam with no live client. It is the
// only way to exercise Enqueue's conflict/DLQ logic without Redis.
func newProducerForTest(db *database.DB, pusher redisPusher) *Producer {
	return &Producer{db: db, pusher: pusher}
}

// Configured reports whether Enqueue will actually reach Redis.
func (p *Producer) Configured() bool { return p.pusher != nil }

// Close releases the Redis client. Safe when unconfigured.
func (p *Producer) Close() error {
	if p.client == nil {
		return nil
	}
	return p.client.Close()
}

// JobRecord is the background_jobs row the status endpoint returns.
type JobRecord struct {
	TaskID     string    `json:"task_id"`
	Type       string    `json:"type"`
	OwnerID    int64     `json:"owner_id"`
	Status     string    `json:"status"`
	Attempts   int       `json:"attempts"`
	LastError  string    `json:"last_error,omitempty"`
	EnqueuedAt time.Time `json:"enqueued_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Enqueue pushes task and records the ledger row. key is the idempotency
// string (the asynq TaskID); pass "" for a task without a stable key and a
// fresh uuid is generated. owner is the enqueuer's user id (the jobs status
// endpoint gates on it). On ErrTaskIDConflict the existing job is returned,
// not an error — that is the idempotent re-enqueue contract.
func (p *Producer) Enqueue(ctx context.Context, task *asynq.Task, key string, owner int64, opts ...asynq.Option) (*JobRecord, error) {
	if p.pusher == nil {
		return nil, ErrNotConfigured
	}
	if key == "" {
		key = uuid.NewString()
		opts = append(opts, asynq.TaskID(key))
	}
	if err := p.pusher.Push(ctx, task, opts...); err != nil {
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			// Same task id is in flight (or in the archive): return
			// what the ledger knows; an unknown key means the
			// original process crashed between push and insert.
			if row, ierr := p.JobByIdempotencyKey(ctx, key); ierr == nil && row != nil {
				return row, nil
			}
			return &JobRecord{TaskID: key, Type: task.Type(), OwnerID: owner, Status: JobQueued}, nil
		}
		return nil, fmt.Errorf("enqueue %s: %w", task.Type(), err)
	}
	row := &JobRecord{
		TaskID:     key,
		Type:       task.Type(),
		OwnerID:    owner,
		Status:     JobQueued,
		EnqueuedAt: time.Now().UTC(),
	}
	if p.db != nil {
		_ = insertJobRow(ctx, p.db, row, key, task)
	}
	return row, nil
}

// EnqueueScheduled is Enqueue with a delayed start.
func (p *Producer) EnqueueScheduled(ctx context.Context, task *asynq.Task, key string, owner int64, delay time.Duration, opts ...asynq.Option) (*JobRecord, error) {
	return p.Enqueue(ctx, task, key, owner, append(opts, asynq.ProcessIn(delay))...)
}

// Dispatch is the ADR-0002 feature-flag pattern for moving a blocking
// call off the HTTP path: when the queue is configured the task is
// enqueued (the job is returned for the /jobs/:id poll); otherwise the
// sync fallback runs in the caller's goroutine — exactly today's legacy
// behavior — so the app stays deployable with REDIS_URL unset. A failed
// sync fallback is returned to the caller; an unconfigured queue with no
// fallback is a no-op (the work is intentionally dropped only when the
// caller says so).
func Dispatch(ctx context.Context, p *Producer, task *asynq.Task, key string, owner int64, syncFallback func(context.Context) error) (*JobRecord, error) {
	if p != nil && p.Configured() {
		return p.Enqueue(ctx, task, key, owner)
	}
	if syncFallback != nil {
		if err := syncFallback(ctx); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

// mailKind constants: the email:send task's structured kind (the worker
// routes on these to the matching email.Service method — see
// cmd/worker/handlers.go).
const (
	MailKindOtp          = "otp"
	MailKindVerification = "verification"
	MailKindReset        = "reset"
)

// DispatchMail is the legacy-call-site form of Dispatch: one email:send
// task when the queue is configured, otherwise today's synchronous SMTP
// call. A Redis blip (configured but enqueue erroring) ALSO falls through
// to the sync path: auth-critical mail (OTP, reset) must not be blocked by
// a queue outage. Returns true when the mail was accepted (enqueued) or
// delivered (sync fallback).
func DispatchMail(ctx context.Context, p *Producer, kind, to, firstName, secret string, sync func(ctx context.Context) bool) bool {
	if p != nil && p.Configured() {
		key := IdempotencyKey(TypeEmailSend, uuid.NewString())
		task, opts := NewEmailTask(key, EmailSendPayload{Kind: kind, To: to, FirstName: firstName, Secret: secret})
		if _, err := p.Enqueue(ctx, task, key, 0, opts...); err == nil {
			return true
		}
		// enqueue failed: fall through to the sync path below.
	}
	return sync(ctx)
}

// JobByIdempotencyKey loads the ledger row for a key (nil when absent).
func (p *Producer) JobByIdempotencyKey(ctx context.Context, key string) (*JobRecord, error) {
	if p.db == nil || key == "" {
		return nil, nil
	}
	row, err := p.db.QueryOne(ctx, `
		SELECT task_id, type, COALESCE(owner_id, 0), status, attempts,
		       COALESCE(last_error, ''), enqueued_at, updated_at
		FROM background_jobs WHERE idempotency_key = $1`, key)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return scanJobRow(row)
}

// JobByTaskID loads the ledger row for an asynq task id (the :id in
// GET /jobs/:id).
func (p *Producer) JobByTaskID(ctx context.Context, taskID string) (*JobRecord, error) {
	if p.db == nil || taskID == "" {
		return nil, nil
	}
	row, err := p.db.QueryOne(ctx, `
		SELECT task_id, type, COALESCE(owner_id, 0), status, attempts,
		       COALESCE(last_error, ''), enqueued_at, updated_at
		FROM background_jobs WHERE task_id = $1`, taskID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return scanJobRow(row)
}

// insertJobRow upserts the ledger row on the idempotency key. The owner is
// stamped so the status endpoint can reject a foreign poller; system
// enqueues (owner 0) store NULL (owner_id is an FK to users). Mail task
// payloads carry a recipient address + one-time secret, so they are NOT
// persisted to Postgres (Redis keeps them for the worker; the ledger
// stores an empty object instead) — no PII in the audit store.
func insertJobRow(ctx context.Context, db *database.DB, row *JobRecord, key string, task *asynq.Task) error {
	var owner any
	if row.OwnerID != 0 {
		owner = row.OwnerID
	}
	var payload string
	if task != nil && task.Type() == TypeEmailSend {
		payload = "{}"
	} else {
		payload = string(task.Payload())
	}
	_, err := db.Exec(ctx, `
		INSERT INTO background_jobs (task_id, type, owner_id, payload, status, idempotency_key, enqueued_at, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7, NOW())
		ON CONFLICT (idempotency_key) DO UPDATE SET updated_at = NOW()`,
		row.TaskID, row.Type, owner, payload, row.Status, key, row.EnqueuedAt)
	return err
}
