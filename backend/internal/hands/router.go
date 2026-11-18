package hands

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/middleware"
	"healthtrack/internal/modules/auth"
	"healthtrack/internal/platform/queue"
	"healthtrack/internal/services/email"
)

// BuildRouter assembles the full application router, mirroring the mount
// layout of the legacy Express server.
func BuildRouter(db *database.DB, cfg *config.Config, mail *email.Service) *gin.Engine {
	r := gin.New()
	// Wire the shared Supabase JWKS cache so the middleware and the
	// register-profile handler verify tokens with one consistent key set.
	middleware.InitFromConfig(cfg)
	r.Use(middleware.RecoverAndLog(), middleware.StripTrailingSlash(), middleware.NewCORS(cfg), middleware.MaxBodySize(1<<20), middleware.SecurityHeaders(), middleware.RequestLogger())

	// ADR-0002 queue producer: configured when REDIS_URL is set, a no-op
	// degrade otherwise (legacy SMTP call sites keep their sync fallback).
	prod := queue.NewProducer(db, cfg.RedisURL)

	// The public /uploads static mount is REMOVED (ADR-0002, contract step):
	// patient documents now live in private R2 and are served only through
	// ownership/consent-gated pre-signed GET URLs (GET /api/v1/files/:id/
	// download-url). No filesystem write path remains for uploads.

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "HealthTrack+ API Server is Running"})
	})

	// Pure Go auth module (ADR-0001): one shared service backs the
	// /api/v1/auth cookie routes, the chatbot's AuthRequired + AI-quota
	// gate, and (residual #8) the core + admin session gates — so
	// revocation and spend limits hold across every surface.
	authSvc, authV1Router := BuildAuthModule(db, cfg)

	authAPI := r.Group("/accounts/api")
	AuthRouter(authAPI, db, cfg, mail, prod, authSvc)

	coreAPI := r.Group("/api")
	CoreRouter(coreAPI, db, cfg, authSvc)

	chatAPI := r.Group("/chatbot")
	ChatbotRouter(chatAPI, cfg, authSvc, prod)

	adminAPI := r.Group("/admin-panel/api")
	AdminRouter(adminAPI, db, cfg, authSvc)

	// ADR-0002: private R2 files + background-job status. CLI 1 mounts
	// /api/v1/auth/* on the same group prefix; the two mounts never clash
	// (disjoint sub-paths, different files).
	v1 := r.Group("/api/v1")
	FilesRouter(v1, db, cfg)
	JobsRouter(v1, db, cfg)
	// ADR-0001: the new cookie session system, additive + flag-gated. The
	// legacy /accounts/api + Bearer HS256 paths above stay alive unchanged
	// for the cutover window (rollout step 1; rollback = AUTH_COOKIE_ENABLED=false).
	if cfg.AuthCookieEnabled {
		authV1Router.Register(v1.Group("/auth"))
	}

	r.NoRoute(func(c *gin.Context) {
		// Unknown paths are 404, not 200: the legacy server mirrored this, but
		// a JSON 404 is the correct contract and keeps crawlers/scanners honest.
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Not found"})
	})

	return r
}

// AuthRouter registers the /accounts/api auth routes.
func AuthRouter(g gin.IRouter, db *database.DB, cfg *config.Config, mail *email.Service, prod *queue.Producer, authSvc *auth.Service) {
	g.POST("/login", authLogin(db, cfg))
	g.POST("/register", authRegister(db, cfg, mail, prod))
	g.POST("/verify-otp", authVerifyOtp(db, cfg))
	g.POST("/resend-otp", authResendOtp(db, cfg, mail, prod))
	g.POST("/forgot-password", authForgotPassword(db, cfg, mail, prod))
	g.POST("/reset-password", authResetPassword(db, cfg))
	g.POST("/google-login", authGoogleLogin(db, cfg))
	// Supabase onboarding: capture role/provider fields server-side after a
	// Supabase sign-up. Idempotent; requires a verified session — the
	// cookie session OR the legacy Bearer token (residual #8 gate).
	g.POST("/register-profile", sessionGate(db, cfg, authSvc), authRegisterProfile(db, cfg))
}

// CoreRouter registers the /api routes. The gate is cookie-OR-Bearer: the
// cookie-only frontend authenticates with ht_access; the Bearer path stays
// live for the cutover window (residual #8).
func CoreRouter(g gin.IRouter, db *database.DB, cfg *config.Config, authSvc *auth.Service) {
	jwt := sessionGate(db, cfg, authSvc)

	g.GET("/dashboard", jwt, coreDashboard(db, cfg))
	g.POST("/health-track/add", jwt, coreAddHealthRecord(db, cfg))
	g.POST("/medicines/add", jwt, coreAddMedicine(db, cfg))
	g.POST("/prescriptions/add", jwt, coreAddPrescription(db, cfg))
	g.GET("/medicines", jwt, coreListMedicines(db, cfg))
	g.GET("/health-track", jwt, coreHealthTrack(db, cfg))
	g.GET("/prescriptions", jwt, corePrescriptions(db, cfg))
	g.GET("/profile", jwt, coreProfile(db, cfg))
	g.GET("/mental-health", jwt, coreMentalHealth(db, cfg))
	g.GET("/lifestyle", jwt, coreLifestyle(db, cfg))
	g.GET("/insurance", jwt, coreInsurance(db, cfg))
	g.GET("/past-records", jwt, corePastRecords(db, cfg))
	g.GET("/appointments", jwt, coreListAppointments(db, cfg))
	g.POST("/appointments", jwt, coreBookAppointment(db, cfg))
	g.POST("/appointments/:appointment_id/action", jwt, coreAppointmentAction(db, cfg))
	g.GET("/service-requests", jwt, coreListServiceRequests(db, cfg))
	g.POST("/service-requests", jwt, coreServiceRequest(db, cfg))
	g.POST("/service-requests/:request_id/action", jwt, coreServiceRequestAction(db, cfg))
}

// AdminRouter registers the /admin-panel/api routes. The gate is
// cookie-OR-Bearer, plus the admin check on top (residual #8).
func AdminRouter(g gin.IRouter, db *database.DB, cfg *config.Config, authSvc *auth.Service) {
	admin := adminSessionGate(db, cfg, authSvc)

	g.GET("/stats", admin, adminStats(db))
	g.GET("/users", admin, adminUsers(db))
	g.POST("/users/:user_id/action", admin, adminUserAction(db))
}

// ChatbotRouter registers the /chatbot routes. Both run behind the auth
// module's AuthRequired + a per-user AI quota (finding S3), so AI spend is
// bounded per account and anonymous callers can't reach the provider. The
// producer offloads the chat model call to the ai:chat task when Redis is
// configured; TTS stays synchronous for now (recorded limitation).
func ChatbotRouter(g gin.IRouter, cfg *config.Config, authSvc *auth.Service, prod *queue.Producer) {
	gate := authSvc.AuthRequired()
	g.POST("/api", gate, chatbotAPI(cfg, authSvc, prod))
	g.POST("/api/tts", gate, chatbotTTS(cfg, authSvc))
}
