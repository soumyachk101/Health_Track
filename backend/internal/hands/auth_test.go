package hands

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"healthtrack/internal/config"
	"healthtrack/internal/models"
)

// TestFrontendURL pins the legacy resolution order: FRONTEND_URL env, then the
// Origin header, then x-forwarded-proto + x-forwarded-host, then the default.
func TestFrontendURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newCtx := func(modify func(*http.Request)) (*gin.Context, *httptest.ResponseRecorder) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		modify(req)
		c.Request = req
		return c, w
	}

	t.Run("FRONTEND_URL env wins", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "https://env.example")
		c, _ := newCtx(func(req *http.Request) {
			req.Header.Set("Origin", "https://origin.example")
		})
		if got := frontendURL(c); got != "https://env.example" {
			t.Errorf("frontendURL = %q, want the env value", got)
		}
	})

	t.Run("Origin header next", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "")
		c, _ := newCtx(func(req *http.Request) {
			req.Header.Set("Origin", "https://origin.example")
		})
		if got := frontendURL(c); got != "https://origin.example" {
			t.Errorf("frontendURL = %q, want the Origin header", got)
		}
	})

	t.Run("x-forwarded proto+host next", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "")
		c, _ := newCtx(func(req *http.Request) {
			req.Header.Set("x-forwarded-proto", "https")
			req.Header.Set("x-forwarded-host", "h.example")
		})
		if got := frontendURL(c); got != "https://h.example" {
			t.Errorf("frontendURL = %q, want %q", got, "https://h.example")
		}
	})

	t.Run("default fallback", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "")
		c, _ := newCtx(func(*http.Request) {})
		if got := frontendURL(c); got != "https://healthtrack.store" {
			t.Errorf("frontendURL = %q, want the default", got)
		}
	})
}

// TestGenerateTokenShape mints a token and re-parses it with the same secret,
// asserting every claim and a 7-day-ish expiry window (clock skew allowed).
func TestGenerateTokenShape(t *testing.T) {
	cfg := &config.Config{JWTSecret: "unit-test-secret"}
	u := &models.User{ID: 7, Username: "u", Email: "e@x"}

	before := time.Now()
	tok := generateToken(cfg, u)

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(tok, claims, func(*jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !parsed.Valid {
		t.Fatal("token is not valid")
	}

	// user_id comes back as a float64 from JSON claims.
	if id, ok := claims["user_id"].(float64); !ok || int64(id) != 7 {
		t.Errorf("user_id = %v (%T), want numeric 7", claims["user_id"], claims["user_id"])
	}
	if got, _ := claims["username"].(string); got != "u" {
		t.Errorf("username = %q, want %q", got, "u")
	}
	if got, _ := claims["email"].(string); got != "e@x" {
		t.Errorf("email = %q, want %q", got, "e@x")
	}

	exp, ok := claims["exp"].(float64)
	if !ok {
		t.Fatalf("exp = %v (%T), want numeric", claims["exp"], claims["exp"])
	}
	// The token was minted after `before` with exp = mint+7d; allow one hour
	// of clock slack on each end.
	lo := before.Add(6*24*time.Hour + 23*time.Hour)
	hi := time.Now().Add(7*24*time.Hour + time.Hour)
	if expT := time.Unix(int64(exp), 0); expT.Before(lo) || expT.After(hi) {
		t.Errorf("exp = %s, want within [%s, %s]", expT.UTC(), lo.UTC(), hi.UTC())
	}
}

// TestGenerateTokenSecretBinding ensures a token signed with one secret does
// not verify under a different one (guards against a hardcoded-secret slip).
func TestGenerateTokenSecretBinding(t *testing.T) {
	cfg := &config.Config{JWTSecret: "secret-one"}
	u := &models.User{ID: 1, Username: "u", Email: "e@x"}
	tok := generateToken(cfg, u)

	_, err := jwt.Parse(tok, func(*jwt.Token) (interface{}, error) {
		return []byte("secret-two"), nil
	})
	if err == nil || strings.Contains(err.Error(), "token signature is invalid") == false {
		t.Errorf("expected signature mismatch against wrong secret, got %v", err)
	}
}
