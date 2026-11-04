package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
)

func ginTestMode(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
}

// TestStripTrailingSlash mirrors server.js middleware: a single trailing '/'
// is removed when the path is longer than one char; the query string is left
// untouched.
func TestStripTrailingSlash(t *testing.T) {
	ginTestMode(t)

	cases := []struct {
		name      string
		orig      string
		wantPath  string
		wantQuery string
	}{
		{"trailing slash removed", "/a/b/", "/a/b", ""},
		{"no trailing slash", "/a/b", "/a/b", ""},
		{"root slash unchanged", "/", "/", ""},
		{"query preserved via slash", "/a/b/?x=1", "/a/b", "x=1"},
		{"query preserved bare path", "/a/b?x=1", "/a/b", "x=1"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := gin.New()
			r.Use(StripTrailingSlash())
			r.Any("/*any", func(ctx *gin.Context) {})

			req := httptest.NewRequest(http.MethodGet, c.orig, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			// The middleware mutates the request URL in place.
			if req.URL.Path != c.wantPath {
				t.Errorf("path after middleware = %q, want %q", req.URL.Path, c.wantPath)
			}
			if got := req.URL.RawQuery; got != c.wantQuery {
				t.Errorf("query = %q, want %q", got, c.wantQuery)
			}
			_ = rec
		})
	}
}

// TestRecoverAndLog confirms a panicking handler is converted into a 500 with
// the canonical JSON body.
func TestRecoverAndLog(t *testing.T) {
	ginTestMode(t)

	r := gin.New()
	r.Use(RecoverAndLog())
	r.GET("/boom", func(ctx *gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	if body["error"] != "Internal server error" {
		t.Errorf("body error = %q, want %q", body["error"], "Internal server error")
	}
}

// newCORSServer wires a router with NewCORS and a catch-all handler.
func newCORSServer(t *testing.T, cfg *config.Config) *gin.Engine {
	t.Helper()
	ginTestMode(t)
	r := gin.New()
	r.Use(NewCORS(cfg))
	r.Any("/*any", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})
	return r
}

func probe(r *gin.Engine, method, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/anything", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// TestCORSAllowedOrigin exercises the allowlist plus the dynamic hostname
// rules (*.vercel.app, *.netlify.app, healthtrack.store and subdomains).
func TestCORSAllowedOrigin(t *testing.T) {
	cfg := &config.Config{
		CORSOrigins: []string{"https://app.test", "https://healthtrack.store"},
	}
	r := newCORSServer(t, cfg)

	t.Run("GET allowed allowlist origin", func(t *testing.T) {
		rec := probe(r, http.MethodGet, "https://app.test")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.test" {
			t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://app.test")
		}
		if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
			t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
		}
	})

	t.Run("GET allowed healthtrack.store", func(t *testing.T) {
		if rec := probe(r, http.MethodGet, "https://healthtrack.store"); rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("GET allowed vercel hostname", func(t *testing.T) {
		rec := probe(r, http.MethodGet, "https://app.vercel.app")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.vercel.app" {
			t.Errorf("Access-Control-Allow-Origin = %q, want the echo", got)
		}
	})

	t.Run("GET allowed healthtrack subdomain", func(t *testing.T) {
		if rec := probe(r, http.MethodGet, "https://a.healthtrack.store"); rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("GET disallowed origin", func(t *testing.T) {
		rec := probe(r, http.MethodGet, "https://evil.example")
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Access-Control-Allow-Origin = %q, want absent on 403", got)
		}
	})

	t.Run("empty origin allowed", func(t *testing.T) {
		rec := probe(r, http.MethodGet, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "*")
		}
	})

	t.Run("OPTIONS allowed -> 204", func(t *testing.T) {
		rec := probe(r, http.MethodOptions, "https://app.test")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.test" {
			t.Errorf("Access-Control-Allow-Origin = %q, want the echo", got)
		}
	})

	t.Run("OPTIONS disallowed -> 403", func(t *testing.T) {
		if rec := probe(r, http.MethodOptions, "https://evil.example"); rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})
}

// TestCORSWildcard confirms "*" in the origins list admits any origin with
// credentials enabled.
func TestCORSWildcard(t *testing.T) {
	r := newCORSServer(t, &config.Config{CORSOrigins: []string{"*"}})

	origin := "https://random.example"
	rec := probe(r, http.MethodGet, origin)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, origin)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
	}
}

// TestJWTRequiredNoHeader covers the non-DB failure paths of JWTRequired:
// a missing Authorization header and an unparseable token. A well-formed
// signed token would proceed to the DB user lookup, which needs a live DB,
// so only these two sentinel cases are asserted here.
func TestJWTRequiredNoHeader(t *testing.T) {
	ginTestMode(t)

	run := func(authHeader string) *httptest.ResponseRecorder {
		var nilDB *database.DB // safe: both cases abort before touching db
		// Empty config -> SUPABASE_JWKS_URL unset -> Supabase path disabled,
		// so these sentinel cases exercise the legacy HS256 fallback.
		r := gin.New()
		r.Use(JWTRequired(nilDB, &config.Config{}))
		r.GET("/me", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"ok": true})
		})
		req := httptest.NewRequest(http.MethodGet, "/me", nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	expectError := func(rec *httptest.ResponseRecorder, want string) {
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body not JSON: %v (%q)", err, rec.Body.String())
		}
		if body["error"] != want {
			t.Errorf("error = %q, want %q", body["error"], want)
		}
	}

	t.Run("no header", func(t *testing.T) {
		expectError(run(""), "Authorization header missing or invalid")
	})
	t.Run("garbage token", func(t *testing.T) {
		expectError(run("Bearer garbage"), "Invalid token")
	})
}
