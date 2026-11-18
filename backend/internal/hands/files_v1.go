// L3: [INPUT] depends on the files endpoints of ADR-0002:
// *database.DB (medical_files), the pinned contract (MIME allowlist + 25 MiB
// cap + 5m PUT / 2m GET presigns), platform/storage.ObjectStorage (R2),
// platform/queue.Producer (optional ai:analyze_document), and the CLI-1
// auth module (AuthRequired + CanReadRecords consent predicate); [OUTPUT]
// exports FilesRouter (POST /files/upload-url, POST /files/:id/confirm,
// GET /files/:id/download-url) + the pure gate helpers the IDOR tests
// drive; [POS] the HTTP boundary of the files module: handlers stay thin,
// ownership/consent decisions live in gateDownload/gateConfirm so an IDOR
// can only exist in one place per verb;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package hands

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/models"
	"healthtrack/internal/modules/auth"
	"healthtrack/internal/platform/cache"
	"healthtrack/internal/platform/queue"
	"healthtrack/internal/platform/storage"
)

// FilesRouter mounts the ADR-0002 files endpoints on /api/v1. Auth is CLI
// 1's cookie path (AuthRequired); storage + producer are built here from
// config so BuildRouter keeps its CLI-1 signature.
func FilesRouter(g gin.IRouter, db *database.DB, cfg *config.Config) {
	svc := newAuthService(db, cfg)
	var store storage.ObjectStorage
	if r2, ok := storage.R2ConfigFromEnv(); ok {
		if s, err := storage.NewS3Store(r2); err == nil {
			store = s
		}
	} // R2_* unset → store stays nil → the routes 503, the app degrades.
	prod := queue.NewProducer(db, cfg.RedisURL)
	repo := svc.Repo()

	g.POST("/files/upload-url", svc.AuthRequired(), filesUploadURL(db, store, repo, prod))
	g.POST("/files/:id/confirm", svc.AuthRequired(), filesConfirm(db, store, repo, prod))
	g.GET("/files/:id/download-url", svc.AuthRequired(), filesDownloadURL(db, store, repo))
}

// newAuthService assembles the CLI-1 auth service the v1 routes gate on:
// JWT verify + revocation markers + consent predicates. Redis-backed when
// REDIS_URL is set, in-memory otherwise (single-instance dev fallback).
func newAuthService(db *database.DB, cfg *config.Config) *auth.Service {
	c, err := cache.New(cfg.RedisURL)
	if err != nil {
		c = cache.NewMemCache()
	}
	return auth.NewService(cfg, auth.NewRepository(db), c)
}

// consentChecker is the one predicate the files handlers consult for
// cross-user access. *auth.Repository satisfies it; tests drive a fake.
type consentChecker interface {
	CanReadRecords(ctx context.Context, reader, patient int64, scope string) (bool, error)
}

// recordsScope is the consent scope medical files ride on (0002 default).
const recordsScope = "records"

// uploadTTL / downloadTTL are the pinned presign windows (ADR-0002).
const (
	uploadTTL   = 5 * time.Minute
	downloadTTL = 2 * time.Minute
)

// fileRow is a medical_files row in map-scan form.
type fileRow struct {
	ID        int64
	OwnerID   int64
	PatientID int64
	ObjectKey string
	MIME      string
	SizeBytes int64
	Status    string
	CreatedAt time.Time
}

// loadFileRow reads one medical_files row; (nil, nil) when absent.
func loadFileRow(ctx context.Context, db *database.DB, id int64) (*fileRow, error) {
	m, err := db.QueryOne(ctx, `
		SELECT id, owner_id, COALESCE(patient_id, 0), object_key, mime_type,
		       size_bytes, status, created_at
		FROM medical_files WHERE id = $1`, id)
	if err != nil || m == nil {
		return nil, err
	}
	row := &fileRow{}
	row.ID = asInt64(m["id"])
	row.OwnerID = asInt64(m["owner_id"])
	row.PatientID = asInt64(m["patient_id"])
	row.ObjectKey = asStr(m["object_key"])
	row.MIME = asStr(m["mime_type"])
	row.SizeBytes = asInt64(m["size_bytes"])
	row.Status = asStr(m["status"])
	if t, ok := m["created_at"].(time.Time); ok {
		row.CreatedAt = t
	}
	return row, nil
}

// uploadMeta is the upload-url request body.
type uploadMeta struct {
	MimeType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	PatientID int64  `json:"patient_id,omitempty"` // defaults to the owner (self-upload)
}

// validateUploadMeta is the MIME + size gate, pure so tests pin it
// without a gin round-trip.
func validateUploadMeta(mime string, size int64) error {
	if !storage.ValidMIME[mime] {
		return fmt.Errorf("unsupported media type: only PDF, PNG, JPEG and WEBP are accepted")
	}
	if size <= 0 || size > storage.MaxFileSize {
		return fmt.Errorf("file size must be between 1 byte and 25 MiB")
	}
	return nil
}

// gateUpload is the ownership rule for upload-url + confirm: the caller
// may upload for patient P iff P == caller or the caller holds a live
// consent grant. This is where an IDOR would have to live; it does not.
func gateUpload(ctx context.Context, repo consentChecker, caller, patient int64) (bool, error) {
	if caller == patient {
		return true, nil
	}
	return repo.CanReadRecords(ctx, caller, patient, recordsScope)
}

// gateDownload is the ownership rule for download-url: same predicate,
// applied to the row's owner's patient. Unrelated reader → false → 403.
func gateDownload(ctx context.Context, repo consentChecker, caller int64, row *fileRow) bool {
	ok, err := gateUpload(ctx, repo, caller, row.PatientID)
	if err != nil {
		// Consent predicate blew up: fail closed (deny), not open.
		return false
	}
	return ok
}

// authUserFromCtx reads the user AuthRequired published; nil if absent
// (tests may set the key directly).
func authUserFromCtx(c *gin.Context) *models.User {
	v, ok := c.Get(auth.CtxUser)
	if !ok {
		return nil
	}
	u, _ := v.(*models.User)
	return u
}

// --- handlers ---

// filesUploadURL: validate → pending row → presigned PUT (5m). Storage
// unset → 503 (the app degrades; local-disk fallback is gone by design).
func filesUploadURL(db *database.DB, store storage.ObjectStorage, repo consentChecker, prod *queue.Producer) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authUserFromCtx(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Not authenticated"})
			return
		}
		var in uploadMeta
		if err := c.BindJSON(&in); err != nil {
			// Empty body = self-upload with defaults is not valid: a
			// MIME type is required to pick the key extension + allowlist.
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "mime_type and size_bytes are required"})
			return
		}
		if err := validateUploadMeta(in.MimeType, in.SizeBytes); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		ctx := c.Request.Context()
		patient := in.PatientID
		if patient == 0 {
			patient = user.ID // self-upload default
		}
		if ok, _ := gateUpload(ctx, repo, user.ID, patient); !ok {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "consent required to upload for this patient"})
			return
		}
		if store == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "object storage not configured"})
			return
		}

		key := fmt.Sprintf("%d/%s%s", user.ID, uuid.NewString(), storage.MIMEExt[in.MimeType])
		tag, err := db.Exec(ctx, `
			INSERT INTO medical_files (owner_id, patient_id, object_key, mime_type, size_bytes, status)
			VALUES ($1, $2, $3, $4, $5, 'pending')
			RETURNING id`, user.ID, patient, key, in.MimeType, in.SizeBytes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to create file record"})
			return
		}
		fileID := int64(tag.RowsAffected())

		putURL, err := store.PresignPut(ctx, key, in.MimeType, uploadTTL)
		if err != nil {
			// No dangling pending row: the presign failed before any
			// upload could target it.
			_, _ = db.Exec(ctx, `DELETE FROM medical_files WHERE id = $1`, fileID)
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "failed to sign upload URL"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "file_id": fileID, "put_url": putURL, "expires_in": int(uploadTTL.Seconds())})
	}
}

// confirmBody lets the client opt into the async AI pass.
type confirmBody struct {
	AnalyzeDocument bool `json:"analyze_document"`
}

// filesConfirm: HeadObject verify → active → optional ai:analyze_document.
func filesConfirm(db *database.DB, store storage.ObjectStorage, repo consentChecker, prod *queue.Producer) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authUserFromCtx(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Not authenticated"})
			return
		}
		id := c.Param("id")
		fileID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid file id"})
			return
		}
		ctx := c.Request.Context()
		row, err := loadFileRow(ctx, db, fileID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load file"})
			return
		}
		if row == nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "file not found"})
			return
		}
		// IDOR gate: owner of a foreign file without consent → 403.
		if !gateDownload(ctx, repo, user.ID, row) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "access denied"})
			return
		}

		var body confirmBody
		_ = c.BindJSON(&body) // optional; a GET-style POST with no body is fine

		if store == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "object storage not configured"})
			return
		}
		info, err := store.HeadObject(ctx, row.ObjectKey)
		if errors.Is(err, storage.ErrNotFound) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": "upload not found in storage; the pre-signed PUT has not landed"})
			return
		}
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "storage check failed"})
			return
		}
		// Size/type mismatch = the browser uploaded something else than
		// declared: keep the row pending and report it.
		if row.SizeBytes > 0 && info.Size != row.SizeBytes {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": "uploaded size does not match the declared size"})
			return
		}
		if info.ContentType != "" && row.MIME != "" && info.ContentType != row.MIME {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": "uploaded content type does not match the declared type"})
			return
		}

		if _, err := db.Exec(ctx, `UPDATE medical_files SET status = 'active' WHERE id = $1 AND status = 'pending'`, fileID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to confirm file"})
			return
		}

		resp := gin.H{"success": true, "file_id": fileID, "status": "active"}
		if body.AnalyzeDocument {
			task, opts := queue.NewAnalyzeDocumentTask(queue.AnalyzeDocumentPayload{
				FileID: fileID, OwnerID: user.ID, MimeType: row.MIME,
			})
			key := queue.IdempotencyKey(queue.TypeAnalyzeDocument, strconv.FormatInt(fileID, 10))
			job, err := prod.Enqueue(ctx, task, key, user.ID, opts...)
			if errors.Is(err, queue.ErrNotConfigured) {
				// Sync fallback: no Redis → the AI pass runs in-request
				// (ADR-0002 rollout flag), so the response degrades to
				// "job not enqueued" without failing the confirm.
				resp["job_id"] = nil
			} else if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "failed to enqueue analysis"})
				return
			} else {
				resp["job_id"] = job.TaskID
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}

// filesDownloadURL: consent gate → active check → presigned GET (2m).
func filesDownloadURL(db *database.DB, store storage.ObjectStorage, repo consentChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authUserFromCtx(c)
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Not authenticated"})
			return
		}
		fileID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid file id"})
			return
		}
		ctx := c.Request.Context()
		row, err := loadFileRow(ctx, db, fileID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to load file"})
			return
		}
		if row == nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "file not found"})
			return
		}
		// IDOR: two unrelated users → no self-match, no consent → 403.
		if !gateDownload(ctx, repo, user.ID, row) {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "access denied"})
			return
		}
		if row.Status != "active" {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": "file not confirmed"})
			return
		}
		if store == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "object storage not configured"})
			return
		}
		url, err := store.PresignGet(ctx, row.ObjectKey, downloadTTL)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "failed to sign download URL"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "get_url": url, "expires_in": int(downloadTTL.Seconds())})
	}
}
