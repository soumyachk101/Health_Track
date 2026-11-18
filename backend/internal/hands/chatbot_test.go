// Package hands — chatbot_test.go covers the chatbot off-the-HTTP-path
// contract (ADR-0002 step 4): the async enqueue (202 + job_id), the
// degrade-to-inline path, the "emergency is never enqueued" guarantee, and
// the jobs result-field rendering.
package hands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"healthtrack/internal/config"
	"healthtrack/internal/models"
	"healthtrack/internal/modules/auth"
	"healthtrack/internal/platform/cache"
	"healthtrack/internal/platform/queue"
	"healthtrack/internal/services/ai"
)

func newAuthSvcForTest() *auth.Service {
	c, err := cache.New("") // degrade: in-memory
	if err != nil {
		panic(err)
	}
	return auth.NewService(&config.Config{JWTSecret: "unit-secret"}, auth.NewRepository(nil), c)
}

// chatEngine mounts the real chatbotAPI handler behind a stub that
// publishes the verified user the way AuthRequired would, so the quota +
// owner gate run without a DB.
func chatEngine(cfg *config.Config, prod *queue.Producer, user *models.User) *gin.Engine {
	e := gin.New()
	e.POST("/chatbot/api", func(c *gin.Context) {
		if user != nil {
			c.Set(auth.CtxUser, user)
		}
		chatbotAPI(cfg, newAuthSvcForTest(), prod)(c)
	})
	return e
}

func postChat(t *testing.T, e *gin.Engine, body map[string]any) (int, map[string]any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/chatbot/api", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	out := map[string]any{}
	_ = json.NewDecoder(w.Body).Decode(&out)
	return w.Code, out
}

// enqueueProd builds an unconfigured producer with a fake pusher:
// Configured()==true, no Redis, no DB (db nil => ledger insert skipped).
func enqueueProd(pushes *int, enqueued *[]*asynq.Task) *queue.Producer {
	prod := queue.NewProducer(nil, "")
	queue.SetPusherForTest(prod, func(ctx context.Context, task *asynq.Task, opts ...asynq.Option) error {
		*pushes++
		*enqueued = append(*enqueued, task)
		return nil
	})
	return prod
}

// TestChatbotAPIAsyncEnqueue: with a configured producer a normal chat is
// enqueued as ai:chat and answered 202 {success, job_id} — no inline call.
func TestChatbotAPIAsyncEnqueue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var pushes int
	var enqueued []*asynq.Task
	prod := enqueueProd(&pushes, &enqueued)

	code, out := postChat(t, chatEngine(&config.Config{}, prod, &models.User{ID: 7}), map[string]any{"message": "hello"})
	if code != http.StatusAccepted {
		t.Fatalf("enqueued chat: got %d, want 202: %v", code, out)
	}
	if out["success"] != true {
		t.Errorf("202 success = %v, want true", out["success"])
	}
	jobID, _ := out["job_id"].(string)
	if !strings.HasPrefix(jobID, "ai:chat:") {
		t.Errorf("job_id = %q, want ai:chat:<uuid>", jobID)
	}
	if pushes != 1 || len(enqueued) != 1 {
		t.Fatalf("producer pushes = %d / tasks = %d, want 1/1", pushes, len(enqueued))
	}
	if enqueued[0].Type() != queue.TypeAiChat {
		t.Errorf("enqueued task type = %q, want ai:chat", enqueued[0].Type())
	}
	// The payload is the pinned shape and carries the general-assistant
	// instruction — the worker never invents a prompt.
	var p queue.AiChatPayload
	if err := json.Unmarshal(enqueued[0].Payload(), &p); err != nil {
		t.Fatalf("payload: %v", err)
	}
	if p.Message != "hello" || p.Instruction != ai.AssistantInstruction {
		t.Errorf("payload = %+v, want message + AssistantInstruction", p)
	}
}

// TestChatbotAPIDegradeInline: with an unconfigured producer (REDIS_URL
// unset) the request is NOT 202 — it takes the inline path, which with no
// API keys answers 503 (proving the degrade branch ran, and that the app
// stays deployable without Redis).
func TestChatbotAPIDegradeInline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prod := queue.NewProducer(nil, "") // Configured() == false
	if prod.Configured() {
		t.Fatal("test premise: producer should be unconfigured")
	}
	code, _ := postChat(t, chatEngine(&config.Config{}, prod, &models.User{ID: 7}), map[string]any{"message": "hello"})
	if code == http.StatusAccepted {
		t.Fatalf("unconfigured producer answered 202; degrade path not taken")
	}
	if code != http.StatusServiceUnavailable {
		t.Fatalf("inline degrade: got %d, want 503 (no API key)", code)
	}
}

// TestChatbotAPIEmergencyNotEnqueued: emergency intent returns the safety
// reply synchronously and never enqueues — even with a configured producer.
func TestChatbotAPIEmergencyNotEnqueued(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var pushes int
	var enqueued []*asynq.Task
	prod := enqueueProd(&pushes, &enqueued)

	code, out := postChat(t, chatEngine(&config.Config{}, prod, &models.User{ID: 7}),
		map[string]any{"message": "I want to kill myself"})
	if code != http.StatusOK {
		t.Fatalf("emergency: got %d, want 200", code)
	}
	if out["emergency"] != true {
		t.Errorf("emergency flag not set: %v", out)
	}
	if pushes != 0 {
		t.Errorf("emergency message enqueued %d times, want 0", pushes)
	}
}

// TestChatbotAPINoUserOwnerZero: a request with no verified user (system
// call, e.g. tests without the auth gate) still enqueues, with owner 0.
func TestChatbotAPINoUserOwnerZero(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var pushes int
	var enqueued []*asynq.Task
	prod := enqueueProd(&pushes, &enqueued)

	code, out := postChat(t, chatEngine(&config.Config{}, prod, nil), map[string]any{"message": "hi"})
	if code != http.StatusAccepted {
		t.Fatalf("no-user chat: got %d, want 202: %v", code, out)
	}
	if jobID, _ := out["job_id"].(string); !strings.HasPrefix(jobID, "ai:chat:") {
		t.Errorf("job_id = %q, want ai:chat:<uuid>", jobID)
	}
	if pushes != 1 {
		t.Errorf("pushes = %d, want 1", pushes)
	}
}

// TestWriteJobResponseResult: the jobs payload carries a parsed "result"
// only when the ledger row's result column is non-null (additive — queued
// jobs are unchanged).
func TestWriteJobResponseResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &models.User{ID: 7}

	do := func(row *queue.JobRecord) map[string]any {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/jobs/x", nil)
		writeJobResponse(c, user, row)
		out := map[string]any{}
		_ = json.NewDecoder(w.Body).Decode(&out)
		if w.Code != http.StatusOK {
			t.Fatalf("writeJobResponse: got %d, want 200", w.Code)
		}
		return out
	}

	completed := do(&queue.JobRecord{TaskID: "xyz", Type: queue.TypeAiChat, OwnerID: 7, Status: queue.JobCompleted, Result: `{"response":"ok"}`})
	resp, _ := completed["result"].(map[string]any)
	if resp == nil || resp["response"] != "ok" {
		t.Errorf("completed job result = %v, want {response:ok}", completed["result"])
	}

	queued := do(&queue.JobRecord{TaskID: "abc", Type: queue.TypeAiChat, OwnerID: 7, Status: queue.JobQueued})
	if _, present := queued["result"]; present {
		t.Errorf("queued job carries a result field; must be absent: %v", queued)
	}
}
