package hands

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/middleware"
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

	r.Static("/uploads", "./uploads")

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "HealthTrack+ API Server is Running"})
	})

	authAPI := r.Group("/accounts/api")
	AuthRouter(authAPI, db, cfg, mail)

	coreAPI := r.Group("/api")
	CoreRouter(coreAPI, db, cfg)

	chatAPI := r.Group("/chatbot")
	ChatbotRouter(chatAPI, cfg)

	adminAPI := r.Group("/admin-panel/api")
	AdminRouter(adminAPI, db, cfg)

	r.NoRoute(func(c *gin.Context) {
		// Unknown paths are 404, not 200: the legacy server mirrored this, but
		// a JSON 404 is the correct contract and keeps crawlers/scanners honest.
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Not found"})
	})

	return r
}

// AuthRouter registers the /accounts/api auth routes.
func AuthRouter(g gin.IRouter, db *database.DB, cfg *config.Config, mail *email.Service) {
	g.POST("/login", authLogin(db, cfg))
	g.POST("/register", authRegister(db, cfg, mail))
	g.POST("/verify-otp", authVerifyOtp(db, cfg))
	g.POST("/resend-otp", authResendOtp(db, cfg, mail))
	g.POST("/forgot-password", authForgotPassword(db, cfg, mail))
	g.POST("/reset-password", authResetPassword(db, cfg))
	g.POST("/google-login", authGoogleLogin(db, cfg))
	// Supabase onboarding: capture role/provider fields server-side after a
	// Supabase sign-up. Idempotent; requires a verified auth token (via the
	// same Supabase/JWKS or legacy HS256 path) but not email confirmation.
	g.POST("/register-profile", middleware.JWTRequired(db, cfg), authRegisterProfile(db, cfg))
}

// CoreRouter registers the /api routes.
func CoreRouter(g gin.IRouter, db *database.DB, cfg *config.Config) {
	jwt := middleware.JWTRequired(db, cfg)

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

// AdminRouter registers the /admin-panel/api routes.
func AdminRouter(g gin.IRouter, db *database.DB, cfg *config.Config) {
	admin := middleware.AdminRequired(db, cfg)

	g.GET("/stats", admin, adminStats(db))
	g.GET("/users", admin, adminUsers(db))
	g.POST("/users/:user_id/action", admin, adminUserAction(db))
}

// ChatbotRouter registers the /chatbot routes.
func ChatbotRouter(g gin.IRouter, cfg *config.Config) {
	g.POST("/api", chatbotAPI(cfg))
	g.POST("/api/tts", chatbotTTS(cfg))
}
