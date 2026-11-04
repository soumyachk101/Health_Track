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
