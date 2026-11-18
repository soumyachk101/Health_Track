// L3: [INPUT] depends on nothing (payload shapes only); [OUTPUT] exports the
// task-type constants, the typed payloads, the task builders with
// idempotency keys + retry policy, and ExponentialBackoff (the
// worker-side retry-delay function); [POS] the single source of truth for
// worker task contracts — cmd/worker and the producer call sites build
// tasks through these builders so a payload-shape change is one file;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// Task type names (the pinned worker contract, ADR-0002).
const (
	TypeEmailSend        = "email:send"
	TypeAnalyzeDocument  = "ai:analyze_document"
	TypeAiChat           = "ai:chat"
	TypeAiTTS            = "ai:tts"
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

// AiChatPayload is the ai:chat task body (pinned contract: message,
// share_chart, instruction). It carries the raw inputs, not a pre-built
// prompt, so both the inline degrade path and the worker rebuild the prompt
// through [BuildChatPrompt] and produce byte-identical model input.
//
// Chart holds the already-bounded chart JSON block ("" when the caller did
// not opt in, or when there is none): the handler truncates it to the cap
// before enqueuing, so the worker never re-implements the bound. The user
// message + chart are personal data: they stay in Redis for the worker, and
// the audit ledger row stores `{}` for ai tasks (same PII guard as
// email:send — see producer.insertJobRow). The result (model text) is stored
// on the row's result JSONB for /jobs/:id polling.
type AiChatPayload struct {
	Message     string `json:"message"`
	ShareChart  bool   `json:"share_chart"`
	Chart       string `json:"chart,omitempty"` // bounded chart block, "" unless opted in
	Instruction string `json:"instruction"`
}

// BuildChatPrompt assembles the final model prompt from an [AiChatPayload]:
// the user message, plus a bounded chart block ONLY when the user opted in
// (share_chart && chart != ""). This is the single source of truth shared by
// the synchronous degrade path (chatbot handler) and the async worker, so a
// queued reply and an inline reply reach the model with identical input.
func BuildChatPrompt(p AiChatPayload) string {
	if p.ShareChart && p.Chart != "" {
		return "Health data the user chose to share (do not diagnose; give general information only):\n" +
			p.Chart + "\n\nQuestion:\n" + p.Message
	}
	return p.Message
}

// AiTTSPayload is the ai:tts task body: the text to synthesize + the
// enqueuing user. Same PII treatment as AiChatPayload.
type AiTTSPayload struct {
	UserID int64  `json:"user_id"`
	Text   string `json:"text"`
}

// taskOptions is the pinned per-type policy: dedicated queue + retry
// ceiling + timeouts.
func taskOptions(taskType string) []asynq.Option {
	queue := QueueFiles
	switch taskType {
	case TypeEmailSend:
		queue = QueueEmail
	case TypeAnalyzeDocument, TypeAiChat, TypeAiTTS:
		queue = QueueAI
	}
	opts := []asynq.Option{asynq.Queue(queue), asynq.MaxRetry(MaxRetries)}
	switch taskType {
	case TypeEmailSend:
		opts = append(opts, asynq.Timeout(2*time.Minute))
	case TypeAnalyzeDocument, TypeAiChat, TypeAiTTS:
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

// NewAiChatTask builds an ai:chat task. Each request is its own logical
// job (no dedupe on the prompt: repeated questions must re-run), so the
// caller supplies a fresh-uuid key: it is the asynq TaskID AND the
// /jobs/:id the caller polls.
func NewAiChatTask(key string, p AiChatPayload) (*asynq.Task, []asynq.Option) {
	payload, _ := json.Marshal(p)
	return asynq.NewTask(TypeAiChat, payload), append([]asynq.Option{asynq.TaskID(key)}, taskOptions(TypeAiChat)...)
}

// NewAiTTSTask builds an ai:tts task (fresh-uuid key, same contract).
func NewAiTTSTask(key string, p AiTTSPayload) (*asynq.Task, []asynq.Option) {
	payload, _ := json.Marshal(p)
	return asynq.NewTask(TypeAiTTS, payload), append([]asynq.Option{asynq.TaskID(key)}, taskOptions(TypeAiTTS)...)
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

// TestPusher is an exported push function type (the redisPusher seam is
// unexported, so external test packages cannot name it). SetPusherForTest
// swaps a Producer's pusher for one recorded in memory, letting the HTTP
// 202/enqueue contract be tested without a live Redis.
type TestPusher func(ctx context.Context, task *asynq.Task, opts ...asynq.Option) error

// SetPusherForTest replaces p's pusher (and marks it configured) with a
// TestPusher-backed one. Test-only: production code uses NewProducer.
func SetPusherForTest(p *Producer, f TestPusher) { p.pusher = testPusherFunc(f) }

type testPusherFunc TestPusher

func (t testPusherFunc) Push(ctx context.Context, task *asynq.Task, opts ...asynq.Option) error {
	return t(ctx, task, opts...)
}

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
