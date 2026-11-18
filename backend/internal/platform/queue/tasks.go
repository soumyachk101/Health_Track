// L3: [INPUT] depends on nothing (payload shapes only); [OUTPUT] exports the
// task-type constants, the typed payloads, the task builders with
// idempotency keys + retry policy, and ExponentialBackoff (the
// worker-side retry-delay function); [POS] the single source of truth for
// worker task contracts — cmd/worker and the producer call sites build
// tasks through these builders so a payload-shape change is one file;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// Task type names (the pinned worker contract, ADR-0002).
const (
	TypeEmailSend        = "email:send"
	TypeAnalyzeDocument  = "ai:analyze_document"
	TypeCleanupStaleFile = "files:cleanup_stale"
)

// Queues route each type by blast radius: mail is user-visible, AI is
// expensive and slow, cleanup is cheap and idle-time. The worker processes
// them with distinct priorities (see cmd/worker).
const (
	QueueEmail = "emails"
	QueueAI    = "ai"
	QueueFiles = "files"
)

// MaxRetries is the pinned retry ceiling; after it the task lands in the
// asynq dead-letter (retry-archive) and the ledger row is stamped dead.
const MaxRetries = 5

// ExponentialBackoff is the worker retry-delay function: attempt n waits
// 1m*2^n, capped at 4h. asynq's default is similar but its cap is
// 4h/2^15; the pinned policy is explicit so mail retries stay predictable.
func ExponentialBackoff(n int, _ error, _ *asynq.Task) time.Duration {
	return exponentialBackoff(n)
}

func exponentialBackoff(n int) time.Duration {
	d := time.Minute << uint(n) // n=0 → 1m, n=1 → 2m, n=2 → 4m ...
	if d > 4*time.Hour {
		d = 4 * time.Hour
	}
	return d
}

// EmailSendPayload is the email:send task body. It carries structured args
// (kind + recipient + one secret), not rendered HTML: the worker routes on
// kind to the matching email.Service method, so a producer/worker version
// skew degrades loudly (unknown kind → DLQ) rather than sending the wrong
// mail. To + Secret are the personal-data fields; the worker logs To
// redacted and never logs Secret.
type EmailSendPayload struct {
	Kind      string `json:"kind"` // otp | verification | reset | appointment
	To        string `json:"to"`
	FirstName string `json:"first_name,omitempty"`
	// Secret is the OTP code or the one-time link (verification/reset).
	Secret string `json:"secret"`
}

// AnalyzeDocumentPayload is the ai:analyze_document task body. The worker
// fetches the document through storage.GetObject and records the result on
// the ledger row.
type AnalyzeDocumentPayload struct {
	FileID   int64  `json:"file_id"`
	OwnerID  int64  `json:"owner_id"`
	MimeType string `json:"mime_type"`
}

// CleanupStalePayload is the files:cleanup_stale task body (periodic purge
// of pending uploads older than the threshold).
type CleanupStalePayload struct{}

// taskOptions is the pinned per-type policy: dedicated queue + retry
// ceiling + timeouts.
func taskOptions(taskType string) []asynq.Option {
	queue := QueueFiles
	switch taskType {
	case TypeEmailSend:
		queue = QueueEmail
	case TypeAnalyzeDocument:
		queue = QueueAI
	}
	opts := []asynq.Option{asynq.Queue(queue), asynq.MaxRetry(MaxRetries)}
	switch taskType {
	case TypeEmailSend:
		opts = append(opts, asynq.Timeout(2*time.Minute))
	case TypeAnalyzeDocument:
		opts = append(opts, asynq.Timeout(5*time.Minute))
	}
	return opts
}

// NewEmailTask builds an email:send task with an explicit task id (the
// mail-transaction uuid: one per logical send, not per delivery attempt).
func NewEmailTask(idempotencyID string, p EmailSendPayload) (*asynq.Task, []asynq.Option) {
	payload, _ := json.Marshal(p)
	return asynq.NewTask(TypeEmailSend, payload), append([]asynq.Option{asynq.TaskID(idempotencyID)}, taskOptions(TypeEmailSend)...)
}

// NewAnalyzeDocumentTask builds an ai:analyze_document task keyed on the
// file id: re-confirming the same file is one task, not a storm.
func NewAnalyzeDocumentTask(p AnalyzeDocumentPayload) (*asynq.Task, []asynq.Option) {
	payload, _ := json.Marshal(p)
	id := asynq.TaskID(IdempotencyKey(TypeAnalyzeDocument, fmt.Sprintf("%d", p.FileID)))
	return asynq.NewTask(TypeAnalyzeDocument, payload), append([]asynq.Option{id}, taskOptions(TypeAnalyzeDocument)...)
}

// NewCleanupStaleTask builds one files:cleanup_stale task plus its options
// (the periodic scheduler adds a run-timestamp TaskID so overlapping runs
// don't dedupe against each other).
func NewCleanupStaleTask() (*asynq.Task, []asynq.Option) {
	payload, _ := json.Marshal(CleanupStalePayload{})
	return asynq.NewTask(TypeCleanupStaleFile, payload), taskOptions(TypeCleanupStaleFile)
}

// IdempotencyKey is the pinned <type>:<uuid-or-file-id> string: it is both
// the asynq task id and the background_jobs.idempotency_key.
func IdempotencyKey(taskType, id string) string { return taskType + ":" + id }

// RetryOutcome is the pure dead-letter decision: a failed task that has
// already consumed maxRetries retries goes to the asynq retry-archive
// (dead-letter) and its ledger row is stamped JobDead; fewer retries →
// JobFailed, asynq reschedules with the exponential backoff. asynq makes
// this same call internally; the worker mirrors it to keep the ledger in
// lockstep.
func RetryOutcome(retries, maxRetries int) (status string, dlq bool) {
	if retries >= maxRetries {
		return JobDead, true
	}
	return JobFailed, false
}
