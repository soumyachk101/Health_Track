// Package main — handlers_test.go covers the deterministic paths of the
// ai:chat worker handler: bad-payload rejection and the no-provider-key
// model failure (the error the failureStamp middleware stamps as
// failed/dead + last_error). Both run without a DB or a live provider.
//
// The success path (stamping background_jobs.result on a completed job) is
// not unit-tested here: it needs a live AI provider + Postgres DSN. The
// jobs endpoint's result rendering IS covered (hands writeJobResponse test),
// and the full enqueue -> worker -> /jobs/:id loop is a DSN-gated
// integration check.
package main

import (
	"context"
	"strings"
	"testing"

	"github.com/hibiken/asynq"

	"healthtrack/internal/config"
)

// TestHandleAiChatBadPayload: a malformed task body is rejected before any
// model call (no DB, no provider needed).
func TestHandleAiChatBadPayload(t *testing.T) {
	h := &handlers{cfg: &config.Config{}}
	task := asynq.NewTask("ai:chat", []byte("{not json"))
	err := h.handleAiChat(context.Background(), task)
	if err == nil {
		t.Fatal("expected an error for a bad payload, got nil")
	}
	if !strings.Contains(err.Error(), "bad payload") {
		t.Errorf("error = %q, want a 'bad payload' error", err.Error())
	}
}

// TestHandleAiChatNoProviderFails: with no API key configured the model
// call fails and handleAiChat returns the wrapped error. No DB needed — the
// failure happens before any ledger result stamp; the failureStamp
// middleware is what turns this error into a failed/dead + last_error row.
func TestHandleAiChatNoProviderFails(t *testing.T) {
	h := &handlers{cfg: &config.Config{GroqKey: "", Key: "", SarvamKey: ""}}
	payload := `{"message":"hi","share_chart":false,"instruction":""}`
	task := asynq.NewTask("ai:chat", []byte(payload))
	err := h.handleAiChat(context.Background(), task)
	if err == nil {
		t.Fatal("expected a model-call failure with no provider key, got nil")
	}
	if !strings.Contains(err.Error(), "ai:chat") || !strings.Contains(err.Error(), "model call") {
		t.Errorf("error = %q, want a wrapped ai:chat model-call error", err.Error())
	}
}
