//go:build integration

// Package test — chat_queue_test.go covers the ai:chat queue contract that
// the in-package handler tests can't reach without Redis or a DB: that an
// enqueue writes a real background_jobs ledger row (owner + status), and
// that a completed row's result JSONB is read back through the producer's
// job lookup (the exact call jobStatus makes before rendering "result").
//
// The producer is driven through the exported SetPusherForTest seam: an
// in-memory pusher stands in for Redis so no live queue is needed, while the
// ledger writes hit the real Postgres (skipped when no DSN is set).
package test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hibiken/asynq"

	"healthtrack/internal/platform/queue"
)

// TestChatEnqueueWritesLedgerRow: enqueuing an ai:chat task records a
// background_jobs row (idempotency key, owner, status=queued). This is the
// "enqueue -> 202 + ledger row" half of the pinned contract — the 202
// itself is asserted in the hands test; here we assert the durable row.
func TestChatEnqueueWritesLedgerRow(t *testing.T) {
	skipIfNoDB(t)
	ctx := context.Background()
	key := queue.IdempotencyKey(queue.TypeAiChat, "chat-queue-"+stampStr())

	prod := queue.NewProducer(testDB, "") // no Redis: degraded, DB-backed
	queue.SetPusherForTest(prod, func(ctx context.Context, task *asynq.Task, opts ...asynq.Option) error {
		return nil
	})

	task, opts := queue.NewAiChatTask(key, queue.AiChatPayload{
		Message: "hello", Instruction: "assistant",
	})
	if task.Type() != queue.TypeAiChat {
		t.Fatalf("task type = %q, want ai:chat", task.Type())
	}
	_ = key
	rec, err := prod.Enqueue(ctx, task, key, 0, opts...)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if rec.Status != queue.JobQueued {
		t.Fatalf("enqueued record status = %q, want queued", rec.Status)
	}

	// The durable row is what a fresh /jobs/:id poll can find later.
	row, err := prod.JobByIdempotencyKey(ctx, key)
	if err != nil {
		t.Fatalf("JobByIdempotencyKey: %v", err)
	}
	if row == nil {
		t.Fatalf("no ledger row written for key %q", key)
	}
	if row.Type != queue.TypeAiChat {
		t.Errorf("ledger row type = %q, want ai:chat", row.Type)
	}
	if row.Result != "" {
		t.Errorf("freshly-queued row has result = %q, want empty", row.Result)
	}

	// Clean up so the suite is re-runnable.
	_, _ = testDB.Exec(ctx, "DELETE FROM background_jobs WHERE idempotency_key = $1", key)
}

// TestChatJobResultRoundTrip: a completed ai:chat row's result JSONB is read
// back through the producer lookup — the path jobStatus uses to render the
// "result" field. (The rendering itself is asserted in the hands test.)
func TestChatJobResultRoundTrip(t *testing.T) {
	skipIfNoDB(t)
	ctx := context.Background()
	key := queue.IdempotencyKey(queue.TypeAiChat, "chat-result-"+stampStr())

	prod := queue.NewProducer(testDB, "")
	queue.SetPusherForTest(prod, func(ctx context.Context, task *asynq.Task, opts ...asynq.Option) error {
		return nil
	})
	task, opts := queue.NewAiChatTask(key, queue.AiChatPayload{Message: "hi", Instruction: "assistant"})
	if _, err := prod.Enqueue(ctx, task, key, 0, opts...); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Simulate the worker finishing: stamp completed + the result JSON the
	// handler writes (handleAiChat stamps exactly this shape).
	queue.MarkJobStatus(ctx, testDB, key, queue.JobCompleted, "")
	if err := queue.MarkJobResult(ctx, testDB, key, []byte(`{"response":"all good"}`)); err != nil {
		t.Fatalf("MarkJobResult: %v", err)
	}

	row, err := prod.JobByTaskID(ctx, key)
	if err != nil {
		t.Fatalf("JobByTaskID: %v", err)
	}
	if row == nil {
		t.Fatalf("no row for completed key %q", key)
	}
	if row.Status != queue.JobCompleted {
		t.Errorf("status = %q, want completed", row.Status)
	}
	if !strings.Contains(row.Result, "response") {
		t.Errorf("result = %q, want the {\"response\":...} object", row.Result)
	}

	_, _ = testDB.Exec(ctx, "DELETE FROM background_jobs WHERE idempotency_key = $1", key)
}

// stampStr is a per-run unique suffix (the shared test stamp, stringified).
func stampStr() string { return fmt.Sprintf("%d", stamp) }
