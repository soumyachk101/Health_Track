// auth_v1.go mounts the Pure Go auth module (ADR-0001) at /api/v1/auth, next
// to the legacy /accounts/api routes which stay alive unchanged for the
// cutover window.
//
// [INPUT] depends on modules/auth (Service + Router + Repository), the
// platform cache (Redis when REDIS_URL is valid, in-memory otherwise), and
// the shared email service for OTP delivery;
// [OUTPUT] provides BuildAuthModule (wires the degrade-safe service + router)
// and the emailOtpMailer adapter;
// [POS] the glue between the gin router and the auth module: builds the
// cache, wires the service, and returns both for the router to mount + the
// chatbot to gate on — no request logic lives here;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package hands

import (
	"context"
	"log"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/modules/auth"
	"healthtrack/internal/platform/cache"
	"healthtrack/internal/services/email"
)

// BuildAuthModule wires the auth module over the shared pool. The cache
// degrades safely: an empty REDIS_URL -> in-memory store (single instance,
// dev/tests); a malformed REDIS_URL logs + falls back to in-memory rather
// than taking the API down (the legacy auth paths keep serving). Returns the
// service (for the chatbot's AuthRequired + quota gate) and the router (to
// mount the /api/v1/auth group when AUTH_COOKIE_ENABLED).
func BuildAuthModule(db *database.DB, cfg *config.Config) (*auth.Service, *auth.Router) {
	c, err := cache.New(cfg.RedisURL)
	if err != nil {
		log.Printf("auth: REDIS_URL misconfigured (%v); degrading to in-memory cache", err)
		c = cache.NewMemCache()
	}
	svc := auth.NewService(cfg, auth.NewRepository(db), c)
	router := auth.NewRouter(db, cfg, svc, newEmailOtpMailer(cfg))
	return svc, router
}

// emailOtpMailer adapts the shared email service to the module's OtpMailer
// interface. Delivery is best-effort: a down relay degrades the code email to
// a no-op and never fails the request.
type emailOtpMailer struct {
	mail *email.Service
}

func newEmailOtpMailer(cfg *config.Config) auth.OtpMailer {
	return &emailOtpMailer{mail: email.New(cfg)}
}

func (m *emailOtpMailer) SendOtp(ctx context.Context, to, code, firstName, purpose string) bool {
	if m == nil || m.mail == nil {
		return false
	}
	// Code-based delivery for both purposes: registration verification and
	// the new code-based password reset (purpose only labels the code).
	return m.mail.SendOtpEmail(ctx, to, code, firstName)
}
