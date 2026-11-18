package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	NodeEnv        string
	JWTSecret      string
	DatabaseURL    string
	EmailHost      string
	EmailPort      int
	EmailUser      string
	EmailPass      string
	EmailUseTLS    bool
	FromEmail      string
	FrontendURL    string
	GoogleClientID string
	GroqKey        string
	Key            string
	SarvamKey      string
	CORSOrigins    []string
	// Supabase auth (GoTrue): SUPABASE_JWKS_URL points at the JWKS endpoint
	// used to verify Supabase-issued RS256 access tokens. Empty disables the
	// RS256 path (legacy HS256 only).
	SupabaseJWKSURL string
	// SUPABASE_URL is the Supabase project URL; the expected token issuer is
	// SUPABASE_URL + "/auth/v1".
	SupabaseURL string

	// --- Pure Go Auth (ADR-0001) ---
	// REDIS_URL backs the auth module: opaque refresh-token rotation
	// (hashed, Lua-rotated) and the user:revoked_before:<id> instant
	// revocation marker. Empty means the module falls back to an in-memory
	// store (single instance, dev only).
	RedisURL string
	// AUTH_COOKIE_DOMAIN sets the Domain attribute of the ht_access /
	// ht_refresh cookies. Set it to the shared parent domain in production
	// (e.g. "healthtrack.plus") so cookies work across app.* and api.*
	// subdomains. Empty means the cookie is host-only.
	AuthCookieDomain string
	// AuthInsecureCookies disables the Secure attribute so the cookie
	// session works over plain http://localhost in development. Production
	// deployments leave this unset (Secure is always on).
	AuthInsecureCookies bool
	// AuthCookieEnabled is the ADR-0001 feature flag: when false, the new
	// /api/v1/auth cookie routes are not mounted and legacy paths serve
	// everything (rollout step 1 of the ADR; rollback = set this to false).
	AuthCookieEnabled bool

	// --- Schema bootstrap (finding P2.3) ---
	// DB_MIGRATE_ON_BOOT. When false (the default), database.Connect is
	// read-only: the versioned migrations are applied out-of-band by
	// `go run ./cmd/migrate` before the server starts, so a fresh instance can
	// never silently mutate production DDL. When true (the integration test
	// harness, and any deployment that wants boot-time bootstrap), Connect
	// runs database.Migrate right after connecting.
	MigrateOnBoot bool
}

func Load() *Config {
	_ = godotenv.Load()

	cfg := &Config{}

	cfg.Port = envDefault("PORT", "8000")
	cfg.NodeEnv = envDefault("NODE_ENV", "development")
	cfg.JWTSecret = envDefault("JWT_SECRET", "default-jwt-secret-change-me")
	cfg.DatabaseURL = os.Getenv("DATABASE_URL")

	cfg.EmailHost = envDefault("EMAIL_HOST", "smtp.gmail.com")

	emailPort := 587
	if p, err := strconv.Atoi(os.Getenv("EMAIL_PORT")); err == nil && p > 0 {
		emailPort = p
	}
	cfg.EmailPort = emailPort

	cfg.EmailUser = os.Getenv("EMAIL_HOST_USER")
	cfg.EmailPass = os.Getenv("EMAIL_HOST_PASSWORD")
	cfg.EmailUseTLS = strings.EqualFold(os.Getenv("EMAIL_USE_TLS"), "true")

	cfg.FromEmail = envDefault("DEFAULT_FROM_EMAIL", "noreply@healthtrack.plus")
	cfg.FrontendURL = envDefault("FRONTEND_URL", "https://healthtrack.store")

	cfg.GroqKey = os.Getenv("GROQ_API_KEY")

	cfg.Key = os.Getenv("OPENAI_API_KEY")
	if cfg.Key == "" {
		if g := os.Getenv("GOOGLE_API_KEY"); strings.HasPrefix(g, "sk-or-v1") {
			cfg.Key = g
		}
	}
	cfg.SarvamKey = os.Getenv("SARVAM_API_KEY")
	cfg.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	// Supabase Auth (GoTrue): when set, the API accepts Supabase-issued
	// access tokens verified against the JWKS at SUPABASE_JWKS_URL. Empty
	// disables the RS256/ES256 path (legacy HS256 only).
	cfg.SupabaseJWKSURL = strings.TrimSpace(os.Getenv("SUPABASE_JWKS_URL"))
	cfg.SupabaseURL = strings.TrimSpace(os.Getenv("SUPABASE_URL"))

	// Pure Go Auth (ADR-0001): REDIS_URL + cookie posture. The feature flag
	// defaults on, so a fresh deployment gets the cookie path; rolling back
	// to legacy-only is AUTH_COOKIE_ENABLED=false.
	cfg.RedisURL = strings.TrimSpace(os.Getenv("REDIS_URL"))
	cfg.AuthCookieDomain = strings.TrimSpace(os.Getenv("AUTH_COOKIE_DOMAIN"))
	cfg.AuthInsecureCookies = strings.EqualFold(os.Getenv("AUTH_INSECURE_COOKIES"), "true")
	cfg.AuthCookieEnabled = envBool("AUTH_COOKIE_ENABLED", true)

	// Schema bootstrap (finding P2.3): default read-only boot. The versioned
	// runner is out-of-band (`go run ./cmd/migrate`); set DB_MIGRATE_ON_BOOT
	// only where boot-time bootstrap is wanted (integration test harness).
	cfg.MigrateOnBoot = envBool("DB_MIGRATE_ON_BOOT", false)

	if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); strings.TrimSpace(origins) != "" {
		var parts []string
		for _, o := range strings.Split(origins, ",") {
			if o = strings.TrimSpace(o); o != "" {
				parts = append(parts, o)
			}
		}
		cfg.CORSOrigins = parts
	} else {
		cfg.CORSOrigins = defaultCORSSources()
	}

	return cfg
}

func defaultCORSSources() []string {
	return []string{
		"http://localhost:5173",
		"http://127.0.0.1:5173",
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"https://healthtrack.store",
		"https://www.healthtrack.store",
	}
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

// envBool reads a boolean env var; an unset value returns def.
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.EqualFold(v, "true") || v == "1"
}
