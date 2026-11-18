// L3: [INPUT] depends on *database.DB (background_jobs ledger), the
// queue package's job lookups, and the CLI-1 auth module (AuthRequired);
// [OUTPUT] exports JobsRouter (GET /api/v1/jobs/:id) — the pinned status
// endpoint the frontend polls after enqueued AI/analysis jobs;
// [POS] the read-only status boundary: it never enqueues, and the owner
// gate lives here so a foreign poller gets 403, not 404-with-data;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package hands

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/models"
	"healthtrack/internal/platform/queue"
)

// JobsRouter mounts GET /jobs/:id on the /api/v1 group. `:id` is the
// job id the confirm endpoint returned (the asynq task id, which equals
// the ledger idempotency key). Status values follow the pinned contract:
// queued | processing | completed | failed; a dead-lettered task (attempts
// exhausted) is reported as failed with its attempt count.
func JobsRouter(g gin.IRouter, db *database.DB, cfg *config.Config) {
	svc := newAuthService(db, cfg)
	prod := queue.NewProducer(db, cfg.RedisURL)

	g.GET("/jobs/:id", svc.AuthRequired(), jobStatus(prod))
}

// jobStatus: load the ledger row by task id, gate on owner (or admin),
// render the pinned status shape. Unknown id → 404; known row the caller
// doesn't own → 403 (a 404 would let a foreign caller probe which ids
// exist).
func jobStatus(prod *queue.Producer) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authUserFromCtx(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Not authenticated"})
			return
		}
		row, err := prod.JobByTaskID(c.Request.Context(), c.Param("id"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load job"})
			return
		}
		if row == nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "job not found"})
			return
		}
		writeJobResponse(c, user, row)
	}
}

// writeJobResponse applies the owner gate + renders the pinned shape.
func writeJobResponse(c *gin.Context, user *models.User, row *queue.JobRecord) {
	// Owner gate: the enqueuer or an admin may read; everyone else → 403.
	isAdmin := user.Role == "admin" || user.IsSuperuser == 1
	if row.OwnerID != 0 && row.OwnerID != user.ID && !isAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "access denied"})
		return
	}
	// dead collapses to the pinned contract value "failed" (attempts
	// carries the 5); the dead-letter fact lives in asynq's archive.
	status := row.Status
	if status == queue.JobDead {
		status = queue.JobFailed
	}
	out := gin.H{
		"success":    true,
		"id":         row.TaskID,
		"type":       row.Type,
		"status":     status,
		"attempts":   row.Attempts,
		"last_error": row.LastError,
		"updated_at": row.UpdatedAt,
	}
	// result: the parsed JSON the worker stored (e.g. {"response": "..."}
	// for ai:chat, {"audio": "..."} for ai:tts). Absent when the task has
	// not completed — that is what "result?" in the pinned contract means.
	if row.Result != "" {
		var v any
		if err := json.Unmarshal([]byte(row.Result), &v); err == nil {
			out["result"] = v
		}
	}
	c.JSON(http.StatusOK, out)
}
