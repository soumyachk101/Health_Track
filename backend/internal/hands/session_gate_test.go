// L3: [INPUT] depends on sessionGate/adminSessionGate, the auth module
// service (in-memory cache degrade), and the legacy Bearer middleware;
// [OUTPUT] test-only: the residual-#8 gate dispatch rules that are
// verifiable without a DSN; [POS] pins the one security-critical dispatch
// decision: a PRESENT but invalid cookie aborts (401) instead of falling
// back to the Bearer path — that fallback would mask instant revocation;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package hands

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"healthtrack/internal/config"
	"healthtrack/internal/modules/auth"
	"healthtrack/internal/platform/cache"
)

// gateTestService builds a degrade-safe auth service (no Redis, nil DB —
// only the pre-lookup paths are exercised, so no DSN is needed).
func gateTestService(t *testing.T) *auth.Service {
	t.Helper()
	c, err := cache.New("")
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	return auth.NewService(&config.Config{JWTSecret: "it-secret"}, auth.NewRepository(nil), c)
}

func gateEngine(t *testing.T, gate gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.GET("/x", gate)
	return e
}

// TestSessionGateNoCookieNoBearer pins the anonymous rejection: no cookie,
// no Bearer header → the legacy path's 401.
func TestSessionGateNoCredential(t *testing.T) {
	gate := sessionGate(nil, &config.Config{}, gateTestService(t))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	gateEngine(t, gate).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("no credential: code = %d, want 401", w.Code)
	}
}

// TestSessionGateInvalidCookieDoesNotFallback is the residual-#8 pin: a
// PRESENT but invalid ht_access cookie (a garbage token) gets the module's
// 401 body — NOT the Bearer middleware's 401 body. If the gate fell through
// to the Bearer check on a bad cookie, the response body would be
// {"error":"Authorization header missing or invalid"}; asserting the module
// body proves revocation/expiry can't be masked by a header token.
func TestSessionGateInvalidCookieDoesNotFallback(t *testing.T) {
	gate := sessionGate(nil, &config.Config{}, gateTestService(t))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccess, Value: "not-a-jwt"})
	gateEngine(t, gate).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("invalid cookie: code = %d, want 401", w.Code)
	}
	// The module 401 carries both "success" and "error" keys; the Bearer
	// middleware 401 carries only "error". Asserting on the "success"
	// key proves the cookie path (not a Bearer fallback) produced it.
	// gin.H is a map, so Go marshals the keys sorted: error before success.
	if got := w.Body.String(); got != `{"error":"unauthenticated","success":false}` {
		t.Errorf("invalid cookie body = %s, want the module 401 (not the Bearer-middleware 401)", got)
	}
}

// TestSessionGateLegacyCookieTypeRejected: a legacy HS256 token (no
// type=ht_access claim) must NOT be accepted as a cookie session — the
// module only accepts its own token shape.
func TestSessionGateLegacyCookieTypeRejected(t *testing.T) {
	svc := gateTestService(t)
	// Mint the legacy shape locally: HS256, user_id claim, no type claim —
	// exactly what the Bearer middleware accepts.
	now := time.Now()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": float64(1),
		"iat":     now.Unix(),
		"exp":     now.Add(7 * 24 * time.Hour).Unix(),
	}).SignedString([]byte("it-secret"))
	if err != nil {
		t.Fatalf("mint legacy token: %v", err)
	}
	legacyTok := tok
	gate := sessionGate(nil, &config.Config{}, svc)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	req.AddCookie(&http.Cookie{Name: auth.CookieAccess, Value: legacyTok})
	gateEngine(t, gate).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("legacy-shaped cookie: code = %d, want 401", w.Code)
	}
}

// TestAdminGateNoCredential pins the admin route's anonymous rejection.
func TestAdminGateNoCredential(t *testing.T) {
	gate := adminSessionGate(nil, &config.Config{}, gateTestService(t))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	gateEngine(t, gate).ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("admin gate, no credential: code = %d, want 401", w.Code)
	}
}
