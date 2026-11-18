// L3: [INPUT] depends on the files_v1 gate helpers + the storage allowlist;
// [OUTPUT] test-only: MIME/size validation, the download/confirm IDOR
// gate with two unrelated users, and the upload-owner gate; [POS] the
// hands-level pin for the files module's authorization decisions — the
// handlers stay thin, so these are the tests that would catch an IDOR;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package hands

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/models"
	"healthtrack/internal/modules/auth"
)

func ginTestModeFiles(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
}

// fakeConsent is a consentChecker driven by a reader→scope grant set,
// mirroring the auth.Repository semantics (self always passes).
type fakeConsent struct {
	granted map[int64]map[string]bool // reader -> scope -> granted
}

func newFakeConsent() *fakeConsent {
	return &fakeConsent{granted: map[int64]map[string]bool{}}
}

// grant records a live consent grant for the reader on the given scope.
func (f *fakeConsent) grant(reader int64, scope string) {
	if f.granted[reader] == nil {
		f.granted[reader] = map[string]bool{}
	}
	f.granted[reader][scope] = true
}

// revoke drops a grant (a revoked consent must not read).
func (f *fakeConsent) revoke(reader int64, scope string) {
	delete(f.granted[reader], scope)
}

// CanReadRecords mirrors the auth.Repository semantics: self always
// passes; cross-user passes only on a live grant for that scope.
func (f *fakeConsent) CanReadRecords(ctx context.Context, reader, patient int64, scope string) (bool, error) {
	if reader == patient {
		return true, nil
	}
	return f.granted[reader][scope], nil
}

// TestFilesValidateUploadMeta pins the MIME allowlist + 25 MiB cap with
// the boundary on each side.
func TestFilesValidateUploadMeta(t *testing.T) {
	cases := []struct {
		mime string
		size int64
		ok   bool
	}{
		{"application/pdf", 1024, true},
		{"image/png", 25 << 20, true}, // exactly the cap
		{"image/jpeg", 1, true},
		{"image/webp", 1 << 20, true},
		{"image/png", 25<<20 + 1, false}, // one byte over the cap
		{"image/gif", 100, false},        // gif is not an allowlist entry
		{"application/zip", 100, false},  // archives carry binaries
		{"image/svg+xml", 100, false},    // SVG can script the browser
		{"text/plain", 100, false},
		{"application/pdf", 0, false}, // zero bytes is not an upload
		{"application/pdf", -1, false},
	}
	for i, c := range cases {
		err := validateUploadMeta(c.mime, c.size)
		if c.ok && err != nil {
			t.Errorf("case %d (%s, %d): unexpected error %v", i, c.mime, c.size, err)
		}
		if !c.ok && err == nil {
			t.Errorf("case %d (%s, %d): want rejection, got nil", i, c.mime, c.size)
		}
	}
}

// TestFilesDownloadIDOR pins the pinned IDOR test: two UNRELATED users.
// User B's download of user A's file is refused; the doctor with a live
// consent grant is admitted; the revoked grant is not.
func TestFilesDownloadIDOR(t *testing.T) {
	ctx := context.Background()
	fake := newFakeConsent()

	rowA := &fileRow{ID: 1, OwnerID: 100, PatientID: 100, ObjectKey: "100/k1.pdf", MIME: "application/pdf", SizeBytes: 10, Status: "active"}
	rowB := &fileRow{ID: 2, OwnerID: 200, PatientID: 200, ObjectKey: "200/k2.png", MIME: "image/png", SizeBytes: 20, Status: "active"}

	// Unrelated reader: no self-match, no consent → deny (403 in the handler).
	if gateDownload(ctx, fake, 200, rowA) {
		t.Error("unrelated user 200 must be denied user 100's file")
	}
	// The owner reads their own file.
	if !gateDownload(ctx, fake, 100, rowA) {
		t.Error("owner 100 must read their own file")
	}
	// A doctor with a live consent grant on 100's records is admitted.
	fake.grant(300, recordsScope)
	if !gateDownload(ctx, fake, 300, rowA) {
		t.Error("doctor 300 with a live consent grant must read patient 100's file")
	}
	// A revoked grant is not a grant.
	fake.revoke(300, recordsScope)
	if gateDownload(ctx, fake, 300, rowA) {
		t.Error("revoked consent must not read")
	}
	// B's own row is still B's.
	if !gateDownload(ctx, fake, 200, rowB) {
		t.Error("owner 200 must read their own file")
	}
}

// TestFilesGateDownloadFailsClosed pins that a consent-predicate error
// denies (fail-closed), not admits.
type erroringConsent struct{}

func (erroringConsent) CanReadRecords(ctx context.Context, reader, patient int64, scope string) (bool, error) {
	return false, errors.New("db down")
}

func TestFilesGateDownloadFailsClosed(t *testing.T) {
	row := &fileRow{OwnerID: 100, PatientID: 100}
	if gateDownload(context.Background(), erroringConsent{}, 300, row) {
		t.Error("a consent-predicate error must deny, not admit")
	}
	// Self still passes even when the predicate errors: the self-check
	// short-circuits before the DB call.
	if !gateDownload(context.Background(), erroringConsent{}, 100, row) {
		t.Error("the owner's self-read must not depend on the consent DB")
	}
}

// TestFilesDownloadURLHTTP pins the HTTP boundary: no user → 401; an
// authenticated user with a non-numeric :id → 400 (no DB touched).
func TestFilesDownloadURLHTTP(t *testing.T) {
	ginTestModeFiles(t)
	gin.SetMode(gin.TestMode)

	// The handler is built with nil store + a db it never reaches on the
	// 401/400 paths; a fake consent repo keeps it wired.
	h := filesDownloadURL(nil, nil, newFakeConsent())

	var userAuthed *gin.Engine
	{
		e := gin.New()
		// Emulate AuthRequired having published the user, then run the
		// handler with a non-numeric id so the DB is never consulted.
		e.GET("/files/:id/download-url", func(c *gin.Context) {
			c.Set(auth.CtxUser, &models.User{ID: 100})
			c.Next()
		}, h)
		userAuthed = e
	}

	// Unauthenticated → 401.
	e401 := gin.New()
	e401.GET("/files/:id/download-url", h)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/files/1/download-url", nil)
	e401.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated code = %d, want 401", w.Code)
	}

	// Authenticated + malformed id → 400, before any DB call.
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/files/not-a-number/download-url", nil)
	userAuthed.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("bad id code = %d, want 400", w2.Code)
	}
}

// TestFilesUploadURLRequiresMIME pins that a missing/invalid MIME is a
// 400, and a missing user is a 401 — both before any DB/storage work.
func TestFilesUploadURLRequiresMIME(t *testing.T) {
	ginTestModeFiles(t)
	h := filesUploadURL(nil, nil, newFakeConsent(), nil)

	e := gin.New()
	e.POST("/files/upload-url", h)

	// No user → 401 (the context never got CtxUser).
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/files/upload-url", nil)
	e.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated = %d, want 401", w.Code)
	}

	// Authed + bad MIME → 400.
	eAuth := gin.New()
	eAuth.POST("/files/upload-url", func(c *gin.Context) {
		c.Set(auth.CtxUser, &models.User{ID: 100})
		c.Next()
	}, h)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/files/upload-url", strings.NewReader(`{"mime_type":"image/gif","size_bytes":10}`))
	eAuth.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("bad mime = %d, want 400", w2.Code)
	}
}
