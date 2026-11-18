// Package auth — handler.go owns the /api/v1/auth endpoints (the pinned
// contract the frontend codes against): register, login, otp, otp/verify,
// password/forgot, password/reset, google, refresh, session, logout.
//
// [INPUT] depends on *Service (token/credential logic), *Repository (DB
// predicates), the platform cache, and the Mailer adapter for OTP delivery;
// [OUTPUT] provides Router + the request/response handlers + SetSessionCookies;
// [POS] the HTTP surface of the auth module — handlers stay thin: bind,
// validate bounds, call the service, render the pinned response shape;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package auth

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/models"
)

// Router builds the /api/v1/auth group (ADR-0001 + PHASE0_API_INVENTORY).
// It is mounted by hands when AUTH_COOKIE_ENABLED, alongside the legacy
// /accounts/api routes which stay read-only until cutover.
type Router struct {
	db     *database.DB
	cfg    *config.Config
	svc    *Service
	mailer OtpMailer // nil = codes are not emailed (dev/tests)
	Opts   AccessCookieOpts
}

// NewRouter wires the auth module: one Service over the shared DB + cache.
// mailer may be nil (the OTP row is still created; the code just isn't sent).
func NewRouter(db *database.DB, cfg *config.Config, svc *Service, mailer OtpMailer) *Router {
	secure := cfg.NodeEnv != "development" && !cfg.AuthInsecureCookies
	return &Router{
		db: db, cfg: cfg, svc: svc, mailer: mailer,
		Opts: AccessCookieOpts{Domain: cfg.AuthCookieDomain, Secure: secure, Insecure: cfg.AuthInsecureCookies},
	}
}

// Register routes the group g (already prefixed /api/v1/auth). The session
// and logout routes carry AuthRequired; every other route is public with a
// per-identifier Redis limiter and the CSRF guard on mutations.
func (r *Router) Register(g gin.IRouter) {
	csrf := r.svc.RequireCSRF(r.cfg.CORSOrigins)
	limit := func(kind string) gin.HandlerFunc {
		return RateLimit(r.svc.Cache(), kind, 15*time.Minute, 60)
	}

	g.POST("/register", limit("auth:register"), csrf, r.register())
	g.POST("/login", limit("auth:login"), csrf, r.login())
	g.POST("/otp", limit("auth:otp"), csrf, r.requestOtp())
	g.POST("/otp/verify", limit("auth:otp"), csrf, r.verifyOtp())
	g.POST("/password/forgot", limit("auth:forgot"), csrf, r.forgotPassword())
	g.POST("/password/reset", limit("auth:reset"), csrf, r.resetPassword())
	g.POST("/google", r.googleLogin())
	g.POST("/refresh", r.refresh())

	g.GET("/session", r.svc.AuthRequired(), r.session())
	g.POST("/logout", r.svc.AuthRequired(), r.logout())
}

// OtpMailer is the email surface the module needs to deliver one-time codes.
// The hands layer adapts the shared email service to it; implementations
// must be best-effort (a down relay never fails the request).
type OtpMailer interface {
	SendOtp(ctx context.Context, to, code, firstName string, purpose string) bool
}

// --- request bodies (field names mirror the legacy /accounts/api shapes so
// the frontend swap is a URL change, not a payload rewrite) ---

type loginBody struct {
	Username string `json:"username"` // email OR username
	Password string `json:"password"`
}

type registerBody struct {
	Username       string `json:"username"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Role           string `json:"role"`
	ProviderType   string `json:"provider_type"`
	BusinessName   string `json:"business_name"`
	LicenseNumber  string `json:"license_number"`
	Specialization string `json:"specialization"`
	State          string `json:"state"` // legacy: stored into city
}

type otpBody struct {
	Email string `json:"email"`
	Role  string `json:"role"` // "register" (default) | "password_reset"
}

// verifyOtpBody accepts the pinned `code` field and the legacy `otp` field,
// so either client shape verifies.
type verifyOtpBody struct {
	Code     string `json:"code"`
	Otp      string `json:"otp"`
	Email    string `json:"email"`
	OtpType  string `json:"otp_type"`
	Username string `json:"username"`
}

type forgotBody struct {
	Email string `json:"email"`
}

type resetBody struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	Otp         string `json:"otp"`
	NewPassword string `json:"new_password"`
}

type googleBody struct {
	Credential string `json:"credential"`
	Role       string `json:"role"`
}

func (b verifyOtpBody) code() string {
	if b.Code != "" {
		return b.Code
	}
	return b.Otp
}

func (b resetBody) code() string {
	if b.Code != "" {
		return b.Code
	}
	return b.Otp
}

// register -> pinned 201 {success,message} — ALWAYS the same body
// (enumeration-safe: an existing email gets the identical answer). The
// users row + provider row + OTP row land in one DB transaction (S6); the
// OTP email is best-effort.
func (r *Router) register() gin.HandlerFunc {
	return func(c *gin.Context) {
		var b registerBody
		if err := c.ShouldBindJSON(&b); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
			return
		}
		for _, v := range []string{b.Username, b.Email, b.Password, b.FirstName, b.LastName, b.Role, b.ProviderType, b.BusinessName, b.LicenseNumber, b.Specialization, b.State} {
			if len(v) > 512 {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Input too long"})
				return
			}
		}
		if b.Email == "" || b.Username == "" || b.Password == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "username, email, and password are required"})
			return
		}

		in := &RegisterInput{
			Username: b.Username, Email: b.Email, Password: b.Password,
			FirstName: b.FirstName, LastName: b.LastName, Role: b.Role,
			ProviderType: b.ProviderType, BusinessName: b.BusinessName,
			LicenseNumber: b.LicenseNumber, Specialization: b.Specialization,
			City: b.State,
		}
		code, err := r.svc.Register(c.Request.Context(), in)
		if err != nil {
			log.Printf("auth: register failed: %v", err)
			// Pinned: the body is identical whether the account already
			// existed or creation failed — 201 either way.
			c.JSON(http.StatusCreated, gin.H{"success": true, "message": "Please check your email"})
			return
		}
		if r.mailer != nil {
			_ = r.mailer.SendOtp(c.Request.Context(), b.Email, code, b.FirstName, "register")
		}
		c.JSON(http.StatusCreated, gin.H{"success": true, "message": "Please check your email"})
	}
}

// login -> sets both session cookies + returns the user (pinned
// {success,user}); credential failures collapse to 401 "Invalid
// credentials" (no enumeration).
func (r *Router) login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var b loginBody
		if err := c.ShouldBindJSON(&b); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
			return
		}
		if b.Username == "" || b.Password == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "username and password are required"})
			return
		}
		// Login already rotated into a NEW refresh family (StartSession
		// killed the user's prior families), so no extra Logout here.
		u, access, refresh, _, err := r.svc.Login(c.Request.Context(), b.Username, b.Password)
		if err != nil {
			code, msg := StatusFor(err)
			c.AbortWithStatusJSON(code, gin.H{"success": false, "error": msg})
			return
		}
		SetSessionCookies(c, r.Opts, access, refresh)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"user":    gin.H{"id": u.ID, "username": u.Username, "email": u.Email, "role": u.Role},
		})
	}
}

// requestOtp -> stores a fresh 6-digit code (hash only) for the email.
func (r *Router) requestOtp() gin.HandlerFunc {
	return func(c *gin.Context) {
		var b otpBody
		if err := c.ShouldBindJSON(&b); err != nil || b.Email == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "email is required"})
			return
		}
		if len(b.Email) > 512 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Email is too long"})
			return
		}
		purpose := "register"
		if b.Role == "password_reset" {
			purpose = "password_reset"
		}
		code, err := r.svc.CreateOtp(c.Request.Context(), b.Email, purpose)
		if err != nil {
			log.Printf("auth: create otp failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Could not send code"})
			return
		}
		if r.mailer != nil {
			_ = r.mailer.SendOtp(c.Request.Context(), b.Email, code, "", purpose)
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Verification code sent"})
	}
}

// verifyOtp -> atomically consumes the code; on success the email is marked
// verified and the account promoted out of "pending" when already approved.
// Pinned: {success,user} + session cookies on success.
func (r *Router) verifyOtp() gin.HandlerFunc {
	return func(c *gin.Context) {
		var b verifyOtpBody
		if err := c.ShouldBindJSON(&b); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
			return
		}
		purpose := b.OtpType
		if purpose == "" {
			purpose = "register"
		}
		ok, err := r.svc.VerifyOtp(c.Request.Context(), b.Email, b.code(), purpose)
		if err != nil || !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid or expired OTP"})
			return
		}
		u, _ := r.svc.Repo().GetByEmail(c.Request.Context(), b.Email)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid or expired OTP"})
			return
		}
		if _, err := r.svc.MarkEmailVerified(c.Request.Context(), u.ID); err != nil {
			log.Printf("auth: mark verified failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Could not verify email"})
			return
		}
		if u.Status == "pending" && u.IsApproved == 1 {
			_ = r.svc.Repo().SetStatus(c.Request.Context(), u.ID, "active")
			u.Status = "active"
		}
		// Pinned: a verified login issues session cookies.
		access, refresh, _, err := r.svc.StartSession(c.Request.Context(), u)
		if err == nil {
			SetSessionCookies(c, r.Opts, access, refresh)
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"user":    gin.H{"id": u.ID, "username": u.Username, "email": u.Email, "role": u.Role},
		})
	}
}

// forgotPassword -> pinned uniform 202 {success:true} for every email (no
// enumeration), storing a fresh reset code when the account exists.
func (r *Router) forgotPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		var b forgotBody
		if err := c.ShouldBindJSON(&b); err != nil || b.Email == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "email is required"})
			return
		}
		if len(b.Email) > 512 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Email is too long"})
			return
		}
		if u, _ := r.svc.Repo().GetByEmail(c.Request.Context(), b.Email); u != nil {
			if code, err := r.svc.CreateOtp(c.Request.Context(), b.Email, "password_reset"); err == nil && r.mailer != nil {
				_ = r.mailer.SendOtp(c.Request.Context(), b.Email, code, u.FirstName, "password_reset")
			}
		}
		c.JSON(http.StatusAccepted, gin.H{"success": true})
	}
}

// resetPassword -> atomically consumes the code + writes both password
// hashes; every live session of the user is then revoked.
func (r *Router) resetPassword() gin.HandlerFunc {
	return func(c *gin.Context) {
		var b resetBody
		if err := c.ShouldBindJSON(&b); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid request body"})
			return
		}
		if b.Email == "" || b.code() == "" || b.NewPassword == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "email, reset code, and new password are required"})
			return
		}
		if len(b.NewPassword) > 512 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Password is too long"})
			return
		}
		ok, err := r.svc.VerifyOtp(c.Request.Context(), b.Email, b.code(), "password_reset")
		if err != nil || !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid or expired reset link. Please request a new one."})
			return
		}
		u, _ := r.svc.Repo().GetByEmail(c.Request.Context(), b.Email)
		if u == nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid or expired reset link. Please request a new one."})
			return
		}
		if err := r.svc.ResetPassword(c.Request.Context(), u.ID, b.NewPassword); err != nil {
			log.Printf("auth: reset password failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Could not update password"})
			return
		}
		// A credential change revokes every session the user holds.
		_ = r.svc.Logout(c.Request.Context(), u.ID)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password updated successfully"})
	}
}

// googleLogin -> verifies a Google ID token, upserts user + oauth link,
// issues a session. Pinned: {success,user} + cookies.
func (r *Router) googleLogin() gin.HandlerFunc {
	return func(c *gin.Context) {
		var b googleBody
		if err := c.ShouldBindJSON(&b); err != nil || b.Credential == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Google credential is required"})
			return
		}
		claims, err := r.svc.VerifyGoogleIDToken(c.Request.Context(), b.Credential, r.cfg.GoogleClientID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "error": "Google authentication failed"})
			return
		}
		u, _ := r.svc.Repo().GetByOauthAccount(c.Request.Context(), "google", claims.Sub)
		if u == nil {
			u, err = r.provisionGoogle(c, claims, b.Role)
			if err != nil {
				log.Printf("auth: google provisioning failed: %v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Could not create Google account"})
				return
			}
		}
		if u.Status != "active" {
			code, msg := StatusFor(statusError(u.Status))
			c.AbortWithStatusJSON(code, gin.H{"success": false, "error": msg})
			return
		}
		access, refresh, _, err := r.svc.StartSession(c.Request.Context(), u)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Could not start session"})
			return
		}
		SetSessionCookies(c, r.Opts, access, refresh)
		c.JSON(http.StatusOK, gin.H{"success": true, "user": gin.H{"id": u.ID, "username": u.Username, "email": u.Email, "role": u.Role}})
	}
}

// session -> the pinned unified endpoint the frontend route guards read:
// {user, access_expires_at}. AuthRequired already answered 401
// "unauthenticated" for anonymous requests.
func (r *Router) session() gin.HandlerFunc {
	return func(c *gin.Context) {
		u := currentUser(c)
		exp, _ := c.Get(CtxAccessExp)
		var expInt int64
		if n, ok := exp.(int64); ok {
			expInt = n
		}
		c.JSON(http.StatusOK, r.svc.Session(u, expInt))
	}
}

// logout -> revokes families + access marker, clears both cookies.
func (r *Router) logout() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := currentUser(c); u != nil {
			_ = r.svc.Logout(c.Request.Context(), u.ID)
		}
		ClearSessionCookies(c, r.Opts)
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// refresh -> atomically rotates the refresh cookie. A replay revokes the
// whole family + the user's access marker; the client must sign in again.
func (r *Router) refresh() gin.HandlerFunc {
	return func(c *gin.Context) {
		presented, err := c.Cookie(CookieRefresh)
		if err != nil || presented == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "No refresh session"})
			return
		}
		res, err := r.svc.Rotate(c.Request.Context(), presented)
		if err != nil {
			if err == ErrReplayedToken {
				log.Printf("auth: refresh-token replay detected; family revoked")
			}
			ClearSessionCookies(c, r.Opts)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Session invalid"})
			return
		}
		u, _ := r.svc.Repo().GetByID(c.Request.Context(), res.UserID)
		if u == nil || u.Status != "active" {
			_ = r.svc.Logout(c.Request.Context(), res.UserID)
			ClearSessionCookies(c, r.Opts)
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "Account not active"})
			return
		}
		access, err := r.svc.IssueAccess(c.Request.Context(), u)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Could not issue access token"})
			return
		}
		// The rotated successor replaces the presented token in the same
		// cookie; the family record now points at res.NewToken's hash.
		SetSessionCookies(c, r.Opts, access, res.NewToken)
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// provisionGoogle creates the user row (email-verified, google-linked) for a
// first-time Google login. Username is derived like the legacy handler.
func (r *Router) provisionGoogle(c *gin.Context, claims GoogleClaims, requestedRole string) (*models.User, error) {
	local := strings.Split(claims.Email, "@")[0]
	suffix := claims.Sub
	if len(suffix) > 4 {
		suffix = suffix[len(suffix)-4:]
	}
	username := local + "_" + suffix
	role := normalizeRole(requestedRole)
	status := "active"
	if role != "patient" {
		status = "pending"
	}

	var userID int64
	err := r.db.QueryRow(c.Request.Context(), `
		INSERT INTO users (username, email, password, role, status, is_email_verified, is_active, profile_image)
		VALUES ($1, $2, '', $3, $4, 1, 1, $5)
		RETURNING id`,
		username, claims.Email, role, status, claims.Picture).Scan(&userID)
	if err != nil {
		return nil, err
	}
	u, _ := r.svc.Repo().GetByID(c.Request.Context(), userID)
	if u == nil {
		return nil, ErrNoUser
	}
	if err := r.svc.Repo().LinkOauthAccount(c.Request.Context(), u, "google", claims.Sub, claims.Email); err != nil {
		return nil, err
	}
	if role == "doctor" || role == "provider" {
		pType := "pharmacy"
		if role == "doctor" {
			pType = "doctor"
		}
		biz := claims.Given
		if biz == "" {
			biz = username
		}
		_, _ = r.db.Exec(c.Request.Context(),
			"INSERT INTO service_providers (user_id, provider_type, business_name) VALUES ($1,$2,$3)",
			userID, pType, biz)
	}
	return u, nil
}

// statusError maps an account status to the sentinel StatusFor understands.
func statusError(status string) error {
	switch status {
	case "pending":
		return ErrPendingAccount
	case "suspended":
		return ErrSuspended
	default:
		return ErrSuspended
	}
}
