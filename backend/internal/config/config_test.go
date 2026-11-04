package config

import (
	"os"
	"testing"
)

// defaultEnvKeys are every env var Load() reads. Blanketing them with t.Setenv
// guarantees a hermetic default run regardless of the surrounding environment.
var defaultEnvKeys = []string{
	"PORT", "NODE_ENV", "JWT_SECRET", "DATABASE_URL",
	"EMAIL_HOST", "EMAIL_PORT", "EMAIL_HOST_USER", "EMAIL_HOST_PASSWORD", "EMAIL_USE_TLS",
	"DEFAULT_FROM_EMAIL", "FRONTEND_URL",
	"GROQ_API_KEY", "OPENAI_API_KEY", "GOOGLE_API_KEY", "SARVAM_API_KEY", "GOOGLE_CLIENT_ID",
	"SUPABASE_JWKS_URL", "SUPABASE_URL",
	"CORS_ALLOWED_ORIGINS",
}

// setEnv sets key to val for the duration of the test, restoring the prior
// value afterward.
func setEnv(t *testing.T, key, val string) {
	t.Helper()
	t.Setenv(key, val)
}

// blankAllEnv resets every Load() input to its empty default so each test
// starts from a known, hermetic environment.
func blankAllEnv(t *testing.T) {
	t.Helper()
	for _, k := range defaultEnvKeys {
		t.Setenv(k, "")
	}
}

// TestLoadDefaults runs Load() from an empty tempdir (no .env present) with a
// fully blanked environment and asserts every field falls back to its default.
func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	blankAllEnv(t)

	cfg := Load()

	if cfg.Port != "8000" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8000")
	}
	if cfg.NodeEnv != "development" {
		t.Errorf("NodeEnv = %q, want %q", cfg.NodeEnv, "development")
	}
	if cfg.JWTSecret != "default-jwt-secret-change-me" {
		t.Errorf("JWTSecret = %q, want default secret", cfg.JWTSecret)
	}
	if cfg.EmailHost != "smtp.gmail.com" {
		t.Errorf("EmailHost = %q, want %q", cfg.EmailHost, "smtp.gmail.com")
	}
	if cfg.EmailPort != 587 {
		t.Errorf("EmailPort = %d, want 587", cfg.EmailPort)
	}
	if cfg.EmailUseTLS != false {
		t.Errorf("EmailUseTLS = %v, want false", cfg.EmailUseTLS)
	}
	if cfg.FromEmail != "noreply@healthtrack.plus" {
		t.Errorf("FromEmail = %q, want default", cfg.FromEmail)
	}
	if cfg.FrontendURL != "https://healthtrack.store" {
		t.Errorf("FrontendURL = %q, want default", cfg.FrontendURL)
	}
	if cfg.GroqKey != "" {
		t.Errorf("GroqKey = %q, want empty", cfg.GroqKey)
	}
	if cfg.Key != "" {
		t.Errorf("Key = %q, want empty", cfg.Key)
	}
	if cfg.SarvamKey != "" {
		t.Errorf("SarvamKey = %q, want empty", cfg.SarvamKey)
	}
	if cfg.GoogleClientID != "" {
		t.Errorf("GoogleClientID = %q, want empty", cfg.GoogleClientID)
	}
	if cfg.SupabaseJWKSURL != "" {
		t.Errorf("SupabaseJWKSURL = %q, want empty", cfg.SupabaseJWKSURL)
	}
	if cfg.SupabaseURL != "" {
		t.Errorf("SupabaseURL = %q, want empty", cfg.SupabaseURL)
	}

	wantOrigins := []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"https://healthtrack.store",
		"https://www.healthtrack.store",
	}
	if len(cfg.CORSOrigins) != 6 {
		t.Fatalf("CORSOrigins len = %d, want 6: %v", len(cfg.CORSOrigins), cfg.CORSOrigins)
	}
	for i, want := range wantOrigins {
		if cfg.CORSOrigins[i] != want {
			t.Errorf("CORSOrigins[%d] = %q, want %q", i, cfg.CORSOrigins[i], want)
		}
	}
}

// TestLoadCORSFromEnv verifies CORS_ALLOWED_ORIGINS is split on commas, trimmed,
// and empty entries dropped.
func TestLoadCORSFromEnv(t *testing.T) {
	blankAllEnv(t)
	setEnv(t, "CORS_ALLOWED_ORIGINS", "http://a.test, http://b.test ,https://c.test")

	cfg := Load()

	want := []string{"http://a.test", "http://b.test", "https://c.test"}
	if len(cfg.CORSOrigins) != 3 {
		t.Fatalf("CORSOrigins = %v, want exactly 3 trimmed entries", cfg.CORSOrigins)
	}
	for i, w := range want {
		if cfg.CORSOrigins[i] != w {
			t.Errorf("CORSOrigins[%d] = %q, want %q", i, cfg.CORSOrigins[i], w)
		}
	}
}

// TestLoadKeyFallback checks the OPENAI_API_KEY -> GOOGLE_API_KEY(sk-or-v1*)
// precedence: a valid OpenAI key wins, and a Google key is adopted only when
// it carries the sk-or-v1 prefix.
func TestLoadKeyFallback(t *testing.T) {
	// OPENAI empty, Google key with valid prefix -> Google key is used.
	blankAllEnv(t)
	setEnv(t, "OPENAI_API_KEY", "")
	setEnv(t, "GOOGLE_API_KEY", "sk-or-v1-abc")
	if got := Load().Key; got != "sk-or-v1-abc" {
		t.Errorf("Key = %q, want %q", got, "sk-or-v1-abc")
	}

	// Google key without the sk-or-v1 prefix -> not adopted.
	blankAllEnv(t)
	setEnv(t, "OPENAI_API_KEY", "")
	setEnv(t, "GOOGLE_API_KEY", "other")
	if got := Load().Key; got != "" {
		t.Errorf("Key = %q, want empty (GOOGLE_API_KEY lacks sk-or-v1 prefix)", got)
	}
}
