package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

// --- pure-policy tests: backoff curve, retry/DLQ outcome, key shapes ---

// TestExponentialBackoffPinned pins the ADR retry curve: 1m*2^n, cap 4h.
func TestExponentialBackoffPinned(t *testing.T) {
	cases := []struct {
		n    int
		want time.Duration
	}{
		{0, time.Minute},
		{1, 2 * time.Minute},
		{2, 4 * time.Minute},
		{3, 8 * time.Minute},
		{5, 32 * time.Minute},
		{11, 4 * time.Hour}, // 1m*2^11 = 1024m → capped
		{40, 4 * time.Hour}, // capped for any large n
	}
	for _, c := range cases {
		if got := ExponentialBackoff(c.n, nil, nil); got != c.want {
			t.Errorf("backoff(n=%d) = %s, want %s", c.n, got, c.want)
		}
	}
}

// TestRetryOutcomePinsDLQ pins when a failed attempt lands in the
// dead-letter (retry-archive) instead of being rescheduled: with
// MaxRetries=5, a task fails dead on the attempt where 5 retries are
// already consumed.
func TestRetryOutcomePinsDLQ(t *testing.T) {
	cases := []struct {
		retries int
		status  string
		dlq     bool
	}{
		{0, JobFailed, false}, // first failure: will retry
		{3, JobFailed, false},
		{MaxRetries - 1, JobFailed, false},
		{MaxRetries, JobDead, true}, // asynq archives this one
		{MaxRetries + 1, JobDead, true},
	}
	for _, c := range cases {
		status, dlq := RetryOutcome(c.retries, MaxRetries)
		if status != c.status || dlq != c.dlq {
			t.Errorf("RetryOutcome(%d) = (%s, %v), want (%s, %v)", c.retries, status, dlq, c.status, c.dlq)
		}
	}
}

// TestIdempotencyKeyShape pins the <type>:<uuid-or-file-id> string form.
func TestIdempotencyKeyShape(t *testing.T) {
	if got := IdempotencyKey(TypeAnalyzeDocument, "42"); got != "ai:analyze_document:42" {
		t.Errorf("key = %q", got)
	}
	if got := IdempotencyKey(TypeEmailSend, "u-1"); got != "email:send:u-1" {
		t.Errorf("key = %q", got)
	}
}

// TestTaskPayloadMarshaled pins the analyze-document payload contract the
// worker decodes (a skew here is a silent no-op task).
func TestTaskPayloadMarshaled(t *testing.T) {
	task, opts := NewAnalyzeDocumentTask(AnalyzeDocumentPayload{FileID: 7, OwnerID: 3, MimeType: "application/pdf"})
	if task.Type() != TypeAnalyzeDocument {
		t.Errorf("type = %s", task.Type())
	}
	var p AnalyzeDocumentPayload
	if err := jsonUnmarshal(task.Payload(), &p); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if p.FileID != 7 || p.OwnerID != 3 || p.MimeType != "application/pdf" {
		t.Errorf("payload = %+v", p)
	}
	if len(opts) == 0 {
		t.Error("no options: retry/queue policy missing")
	}
}

// --- producer tests: not-configured degradation + enqueue idempotency ---

// fakePusher records pushes and can force the task-id conflict error.
type fakePusher struct {
	pushes    int
	failFirst bool
}

func (f *fakePusher) Push(ctx context.Context, task *asynq.Task, opts ...asynq.Option) error {
	f.pushes++
	if f.failFirst && f.pushes == 1 {
		return fmt.Errorf("%w", asynq.ErrTaskIDConflict)
	}
	return nil
}

// TestEnqueueNotConfigured pins the safe-degradation contract: no REDIS_URL
// → Enqueue reports ErrNotConfigured and the caller uses its sync path.
func TestEnqueueNotConfigured(t *testing.T) {
	p := NewProducer(nil, "")
	if p.Configured() {
		t.Error("empty URL must be unconfigured")
	}
	_, err := p.Enqueue(context.Background(), asynq.NewTask(TypeCleanupStaleFile, nil), "k", 0)
	if !errors.Is(err, ErrNotConfigured) {
		t.Errorf("err = %v, want ErrNotConfigured", err)
	}
}

// TestEnqueueBadURLStillDegrades: a malformed URL must not crash the API
// process — the producer degrades to unconfigured.
func TestEnqueueBadURLStillDegrades(t *testing.T) {
	p := NewProducer(nil, "://not-a-redis-url")
	if p.Configured() {
		t.Error("malformed URL must be unconfigured")
	}
}

// TestEnqueueIdempotency: a duplicate push (ErrTaskIDConflict) returns the
// existing ledger-known job, not an error, and pushes exactly once more.
func TestEnqueueIdempotency(t *testing.T) {
	fake := &fakePusher{}
	p := newProducerForTest(nil, fake) // db nil: no ledger, pure push path

	// First push of an "in flight" task collides in Redis:
	fake.failFirst = true
	task, _ := NewEmailTask("email:send:u-9", EmailSendPayload{Kind: "otp", To: "a@b.c", Secret: "x"})
	row, err := p.Enqueue(context.Background(), task, "email:send:u-9", 1)
	if err != nil {
		t.Fatalf("conflict must return the job, not an error: %v", err)
	}
	if row == nil || row.TaskID != "email:send:u-9" || row.OwnerID != 1 {
		t.Errorf("row = %+v", row)
	}

	// Second push of the same key succeeds; no conflict, same row shape.
	fake.failFirst = false
	row2, err := p.Enqueue(context.Background(), task, "email:send:u-9", 1)
	if err != nil {
		t.Fatalf("second enqueue: %v", err)
	}
	if row2.TaskID != "email:send:u-9" || row2.Status != JobQueued {
		t.Errorf("row2 = %+v", row2)
	}
	if fake.pushes != 2 {
		t.Errorf("pushes = %d, want 2", fake.pushes)
	}
}

// TestEnqueueGeneratesKey: an empty key gets a fresh uuid assigned and the
// TaskID option carried on the push.
func TestEnqueueGeneratesKey(t *testing.T) {
	fake := &fakePusher{}
	p := newProducerForTest(nil, fake)
	_, err := p.Enqueue(context.Background(), asynq.NewTask(TypeCleanupStaleFile, []byte("{}")), "", 0)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if fake.pushes != 1 {
		t.Errorf("pushes = %d", fake.pushes)
	}
}

// jsonUnmarshal is a local alias to keep the import list honest in tests.
func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
