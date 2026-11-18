// Package auth — auth_test.go holds the DB-free unit tests for the Pure Go
// session system (ADR-0001): refresh-token rotation, replay -> family
// invalidation, RequireRoles enforcement, the per-user AI quota, argon2id
// round-trip, and malformed-JSON -> 400 on the /api/v1/auth handlers.
//
// These run in a plain `go test ./...` (no Postgres DSN). The DB-dependent
// regressions (pending/suspended blocking, two-user IDOR, OTP replay/
// concurrency) live in the integration suite under backend/test.
package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
	"healthtrack/internal/models"
	"healthtrack/internal/platform/cache"
)

func newTestService(t *testing.T) (*Service, *cache.MemCache) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c := cache.NewMemCache()
	cfg := &config.Config{JWTSecret: "unit-secret"}
	svc := NewService(cfg, NewRepository(nil), c)
	return svc, c
}

// TestRefreshRotation: a fresh session's refresh token rotates exactly once
// and yields a successor; the presented token is consumed.
func TestRefreshRotation(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	u := &models.User{ID: 7, Role: "patient", Status: "active", Username: "u", Email: "u@x.test"}

	_, refresh, _, err := svc.StartSession(ctx, u)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	res, err := svc.Rotate(ctx, refresh)
	if err != nil {
		t.Fatalf("first Rotate: %v", err)
	}
	if res.UserID != 7 || res.NewToken == "" || res.FamilyID == "" {
		t.Fatalf("Rotate result wrong: %+v", res)
	}
}

// TestReplayKillsFamily: presenting an already-consumed refresh token is a
// replay; it must invalidate the whole family so even the (never-spent)
// successor can no longer rotate.
func TestReplayKillsFamily(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	u := &models.User{ID: 9, Role: "patient", Status: "active"}

	_, refresh, _, err := svc.StartSession(ctx, u)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	first, err := svc.Rotate(ctx, refresh)
	if err != nil {
		t.Fatalf("first Rotate: %v", err)
	}
	// Replay the original (already consumed) token.
	if _, err := svc.Rotate(ctx, refresh); err != ErrReplayedToken {
		t.Fatalf("replay: want ErrReplayedToken, got %v", err)
	}
	// The family is now dead: the successor must no longer rotate.
	if _, err := svc.Rotate(ctx, first.NewToken); err == nil {
		t.Fatalf("post-replay successor still rotated; family not invalidated")
	}
}

// TestRequireRoles: layer-2 RBAC — the verified user's role must be in the
// allowed set. Runs off the context AuthRequired published (no DB needed).
func TestRequireRoles(t *testing.T) {
	svc, _ := newTestService(t)
	run := func(role string, allowed ...string) int {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Set(CtxUser, &models.User{ID: 1, Role: role})
		svc.RequireRoles(allowed...)(c)
		return w.Code
	}
	if code := run("patient", "admin"); code != http.StatusForbidden {
		t.Errorf("patient vs admin: got %d, want 403", code)
	}
	if code := run("admin", "admin", "doctor"); code != http.StatusOK {
		t.Errorf("admin allowed: got %d, want 200", code)
	}
	// No user on the context (AuthRequired didn't run) -> 401.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	svc.RequireRoles("admin")(c)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("no user: got %d, want 401", w.Code)
	}
}

// TestQuotaExhaustion: the per-user AI counter admits exactly `limit` calls
// in the window, then denies.
func TestQuotaExhaustion(t *testing.T) {
	svc, c := newTestService(t)
	ctx := context.Background()
	// Use the service's chat quota accessor with a small limit by calling
	// cache.QuotaCheck directly (the same primitive CheckAIQuota wraps).
	for i := 1; i <= 3; i++ {
		ok, err := cache.QuotaCheck(ctx, c, 42, "ai:chat", 15*time.Minute, 3)
		if err != nil {
			t.Fatalf("QuotaCheck %d: %v", i, err)
		}
		if !ok {
			t.Fatalf("call %d: expected admitted, got denied", i)
		}
	}
	if ok, _ := cache.QuotaCheck(ctx, c, 42, "ai:chat", 15*time.Minute, 3); ok {
		t.Fatalf("4th call in-window: expected denied, got admitted")
	}
	// A different user has its own counter.
	if ok, _ := cache.QuotaCheck(ctx, c, 43, "ai:chat", 15*time.Minute, 3); !ok {
		t.Fatalf("other user should have a fresh counter")
	}
	_ = svc // quota is shared via svc.Cache(); exercised through cache.QuotaCheck
}

// TestArgon2RoundTrip: HashPassword -> VerifyPassword round-trips, and a
// wrong password is rejected.
func TestArgon2RoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cret!")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !VerifyPassword(hash, "s3cret!") {
		t.Error("VerifyPassword accepted a mismatch (true password rejected)")
	}
	if VerifyPassword(hash, "wrong") {
		t.Error("VerifyPassword accepted a wrong password")
	}
}

// TestMalformedJSON400: a malformed /api/v1/auth/login body is a clean 400,
// not a zero-value fall-through (finding S9). Uses a nil repo/db because the
// binding fails before any DB access.
func TestMalformedJSON400(t *testing.T) {
	svc, c := newTestService(t)
	r := NewRouter(nil, &config.Config{JWTSecret: "x"}, svc, nil)
	engine := gin.New()
	r.Register(engine.Group("/api/v1/auth"))

	do := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w.Code
	}
	if code := do(`{"username": "a"}`); code != http.StatusBadRequest {
		t.Errorf("truncated JSON: got %d, want 400", code)
	}
	if code := do(`{not json`); code != http.StatusBadRequest {
		t.Errorf("garbage JSON: got %d, want 400", code)
	}
	_ = c
}
