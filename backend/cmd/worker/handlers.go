// L3: [INPUT] depends on asynq (task dispatch), the queue task builders +
// ledger stamps, *database.DB, storage.ObjectStorage, and the email/ai
// service packages; [OUTPUT] exports registerTasks (the worker's task
// table) and the handlers type consumed by main.go; [POS] the worker
// process's only task logic: every handler stamps the background_jobs
// ledger around the real work, so GET /api/v1/jobs/:id tracks truth even
// while Redis holds the queue state; [PROTOCOL] Update this header on
// change, then check CLAUDE.md
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/platform/queue"
	"healthtrack/internal/platform/storage"
	"healthtrack/internal/services/ai"
	"healthtrack/internal/services/email"
)

// CleanupStaleThreshold: pending uploads older than this are purged.
const CleanupStaleThreshold = time.Hour

// aiDocByteCap bounds the document bytes handed to the model (PDFs are
// raw-bytes-heuristic until a real extractor lands; 32 KiB keeps the
// prompt bounded).
const aiDocByteCap = 32 << 10

// handlers owns the worker's per-task state: Postgres ledger + R2 store.
type handlers struct {
	db       *database.DB
	cfg      *config.Config
	store    storage.ObjectStorage
	storeErr error
}

// registerTasks wires the pinned task table onto the asynq mux, wrapped
// with the failure-stamp middleware that keeps the ledger in lockstep
// with the retry/DLQ outcomes.
func registerTasks(mux *asynq.ServeMux, h *handlers) {
	mux.HandleFunc(queue.TypeEmailSend, h.handleEmailSend)
	mux.HandleFunc(queue.TypeAnalyzeDocument, h.handleAnalyzeDocument)
	mux.HandleFunc(queue.TypeAiChat, h.handleAiChat)
	mux.HandleFunc(queue.TypeAiTTS, h.handleAiTTS)
	mux.HandleFunc(queue.TypeCleanupStaleFile, h.handleCleanupStale)
	mux.Use(failureStamp(h.db))
}

// failureStamp is the worker-side DLQ pin: when a task handler returns an
// error, asynq decides retry-vs-archive from the task's max-retries count;
// this middleware mirrors that decision onto the ledger so the status
// endpoint reports "failed" (still retrying) vs "failed + attempts=5"
// (dead-lettered in the asynq retry-archive).
func failureStamp(db *database.DB) asynq.MiddlewareFunc {
	return func(next asynq.Handler) asynq.Handler {
		return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
			err := next.ProcessTask(ctx, t)
			key := taskKey(ctx)
			if err != nil {
				if db == nil {
					return err
				}
				retries := retryCount(ctx)
				maxRetries := 0
				if n, ok := asynq.GetMaxRetry(ctx); ok {
					maxRetries = n
				}
				status, _ := queue.RetryOutcome(retries, maxRetries)
				queue.MarkJobStatus(ctx, db, key, status, err.Error())
			}
			return err
		})
	}
}

// handleEmailSend delivers pre-structured mail via the SMTP service. The
// payload carries structured args (kind + recipient + one secret), not
// rendered HTML, so a producer/worker version skew degrades loudly (kind
// unknown → error → DLQ) instead of sending the wrong mail.
func (h *handlers) handleEmailSend(ctx context.Context, t *asynq.Task) error {
	var p queue.EmailSendPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("email:send: bad payload: %w", err)
	}
	key := taskKey(ctx)
	if err := markProcessing(ctx, h.db, key, retryCount(ctx)); err != nil {
		return err // infra blip: let asynq retry the stamp
	}

	mail := email.New(h.cfg)
	switch p.Kind {
	case "otp":
		sent := mail.SendOtpEmail(ctx, p.To, p.Secret, p.FirstName)
		if !sent {
			return fmt.Errorf("email:send: otp delivery failed to %s", redactEmail(p.To))
		}
	case "verification":
		sent := mail.SendVerificationLinkEmail(ctx, p.To, p.Secret, p.FirstName)
		if !sent {
			return fmt.Errorf("email:send: verification delivery failed to %s", redactEmail(p.To))
		}
	case "reset":
		sent := mail.SendResetEmail(ctx, p.To, p.Secret, p.FirstName)
		if !sent {
			return fmt.Errorf("email:send: reset delivery failed to %s", redactEmail(p.To))
		}
	default:
		// Unknown kinds are a producer bug: fail loudly → DLQ, never
		// drop silently.
		return fmt.Errorf("email:send: unknown kind %q", p.Kind)
	}
	return markCompleted(ctx, h.db, key)
}

// handleAnalyzeDocument runs the AI pass over an uploaded file and stores
// the result on the ledger row (background_jobs.result).
func (h *handlers) handleAnalyzeDocument(ctx context.Context, t *asynq.Task) error {
	var p queue.AnalyzeDocumentPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("ai:analyze_document: bad payload: %w", err)
	}
	key := taskKey(ctx)
	if err := markProcessing(ctx, h.db, key, retryCount(ctx)); err != nil {
		return err
	}
	if h.storeErr != nil {
		return fmt.Errorf("ai:analyze_document: storage unavailable: %v", h.storeErr)
	}

	row, err := h.db.QueryOne(ctx, `SELECT object_key, mime_type, status FROM medical_files WHERE id = $1`, p.FileID)
	if err != nil {
		return fmt.Errorf("ai:analyze_document: file lookup: %w", err)
	}
	if row == nil {
		return fmt.Errorf("ai:analyze_document: file %d missing", p.FileID)
	}
	status, _ := row["status"].(string)
	if status != "active" {
		return fmt.Errorf("ai:analyze_document: file %d not active (status=%s)", p.FileID, status)
	}
	keyName, _ := row["object_key"].(string)
	body, err := h.store.GetObject(ctx, keyName)
	if err != nil {
		return fmt.Errorf("ai:analyze_document: fetch object: %w", err)
	}

	mime, _ := row["mime_type"].(string)
	var summary string
	if strings.HasPrefix(mime, "image/") {
		summary = "image document; OCR not implemented, analysis is metadata-only"
	} else {
		doc := strings.ToValidUTF8(string(body[:min(len(body), aiDocByteCap)]), "…")
		summary, err = ai.Chat(ctx, h.cfg, "Summarize this clinical document in under 200 words: "+doc)
		if err != nil {
			return fmt.Errorf("ai:analyze_document: model call: %w", err)
		}
	}
	result, _ := json.Marshal(map[string]string{"file_id": fmt.Sprintf("%d", p.FileID), "summary": summary})
	if err := markCompleted(ctx, h.db, key); err != nil {
		return err
	}
	return queue.MarkJobResult(ctx, h.db, key, result)
}

// handleAiChat runs the chat completion off the HTTP path and stores the
// reply on the ledger row ({"response": ...}) for GET /jobs/:id. The payload
// carries the pinned {message, share_chart, chart, instruction}; the prompt
// is rebuilt through queue.BuildChatPrompt so a queued reply is byte-identical
// to what the inline degrade path would have sent. On success the row is
// stamped completed + result; on failure the return error lets the
// failureStamp middleware mark it failed/dead with the last_error.
func (h *handlers) handleAiChat(ctx context.Context, t *asynq.Task) error {
	var p queue.AiChatPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("ai:chat: bad payload: %w", err)
	}
	key := taskKey(ctx)
	if err := markProcessing(ctx, h.db, key, retryCount(ctx)); err != nil {
		return err
	}
	instruction := p.Instruction
	if instruction == "" {
		// The pinned general-assistant instruction is the fallback: an older
		// producer that omitted it still gets the safe policy, never a
		// medical-assistant prompt.
		instruction = ai.AssistantInstruction
	}
	text, err := ai.ChatWithInstruction(ctx, h.cfg, queue.BuildChatPrompt(p), instruction)
	if err != nil {
		return fmt.Errorf("ai:chat: model call: %w", err)
	}
	if err := markCompleted(ctx, h.db, key); err != nil {
		return err
	}
	result, _ := json.Marshal(map[string]string{"response": text})
	return queue.MarkJobResult(ctx, h.db, key, result)
}

// handleAiTTS synthesizes speech off the HTTP path and stores the audio
// URI on the ledger row ({"audio": ...}).
func (h *handlers) handleAiTTS(ctx context.Context, t *asynq.Task) error {
	var p queue.AiTTSPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("ai:tts: bad payload: %w", err)
	}
	key := taskKey(ctx)
	if err := markProcessing(ctx, h.db, key, retryCount(ctx)); err != nil {
		return err
	}
	audioURI, err := ai.TTS(ctx, h.cfg, p.Text)
	if err != nil {
		return fmt.Errorf("ai:tts: synthesis: %w", err)
	}
	if err := markCompleted(ctx, h.db, key); err != nil {
		return err
	}
	result, _ := json.Marshal(map[string]string{"audio": audioURI})
	return queue.MarkJobResult(ctx, h.db, key, result)
}

// handleCleanupStale purges pending uploads older than the threshold:
// delete the R2 object, mark the row deleted. Both steps are idempotent.
func (h *handlers) handleCleanupStale(ctx context.Context, t *asynq.Task) error {
	key := taskKey(ctx)
	if err := markProcessing(ctx, h.db, key, retryCount(ctx)); err != nil {
		return err
	}
	rows, err := h.db.QueryMany(ctx, `
		SELECT id, object_key FROM medical_files
		WHERE status = 'pending' AND created_at < NOW() - $1::INTERVAL`,
		CleanupStaleThreshold.String())
	if err != nil {
		return fmt.Errorf("files:cleanup_stale: query: %w", err)
	}
	for _, r := range rows {
		id, _ := r["id"].(int64)
		objectKey, _ := r["object_key"].(string)
		if h.store != nil {
			if err := h.store.DeleteObject(ctx, objectKey); err != nil {
				// Keep going: one bad object must not stall the sweep;
				// the next run retries it (still pending).
				log.Printf("files:cleanup_stale: delete %d: %v", id, err)
				continue
			}
		}
		if _, err := h.db.Run(ctx, `UPDATE medical_files SET status = 'deleted' WHERE id = $1 AND status = 'pending'`, id); err != nil {
			log.Printf("files:cleanup_stale: mark deleted %d: %v", id, err)
		}
	}
	return markCompleted(ctx, h.db, key)
}

// --- ledger stamps + PII helpers (shared by every task) ---

// taskKey is the idempotency key of the running task: the asynq task id IS
// our key (the producers always set TaskID = idempotency key).
func taskKey(ctx context.Context) string {
	if id, ok := asynq.GetTaskID(ctx); ok {
		return id
	}
	return ""
}

// retryCount is the number of retries already consumed (0 on first run).
func retryCount(ctx context.Context) int {
	n, _ := asynq.GetRetryCount(ctx)
	return n
}

// markProcessing stamps the ledger 'processing' (attempts = retries+1).
func markProcessing(ctx context.Context, db *database.DB, key string, retries int) error {
	if key == "" || db == nil {
		return nil
	}
	// First run of an unknown task (crash between push and insert) gets a
	// row created here so the status endpoint can track it from now on.
	if err := queue.EnsureJobRow(ctx, db, key, ""); err != nil {
		return err
	}
	queue.MarkJob(ctx, db, key, queue.JobProcessing, "", retries+1)
	return nil
}

// markCompleted stamps 'completed' on the ledger (attempts stay as-is:
// they document how many tries it took).
func markCompleted(ctx context.Context, db *database.DB, key string) error {
	if key == "" || db == nil {
		return nil
	}
	queue.MarkJobStatus(ctx, db, key, queue.JobCompleted, "")
	return nil
}

// redactEmail masks the local part of an address for logs: "j***@x.com".
func redactEmail(addr string) string {
	at := strings.IndexByte(addr, '@')
	if at <= 1 {
		return "***"
	}
	return addr[:1] + "***" + addr[at:]
}
