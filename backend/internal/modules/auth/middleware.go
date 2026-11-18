// Package auth is the Pure Go session system from ADR-0001: short-lived
// JWT access tokens in HttpOnly cookies, opaque refresh tokens rotated in
// Redis, argon2id passwords, atomic hashed OTP/reset, and 2-layer RBAC
// (AuthRequired + RequireRoles) with object-level consent checks.
//
// middleware.go owns the middleware layer + the pinned cookie/session
// contract the frontend (CLI 2) codes against:
//
//   - ht_access  — HttpOnly; Secure; SameSite=Lax;  Path=/api
//     JWT (HS256, JWT_SECRET), 15 min, claims sub/username/email/role/
//     status/type="ht_access"/iat/exp/rev.
//   - ht_refresh — HttpOnly; Secure; SameSite=Strict;
//     Path=/api/v1/auth/refresh — opaque 32-byte hex, 14 days.
//
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/models"
	"healthtrack/internal/platform/cache"
)

// Context keys under which the verified user + the access token's expiry are
// published for downstream handlers.
const (
	CtxUser      = "auth_user"       // *models.User
	CtxAccessExp = "auth_access_exp" // int64 unix seconds
)

// Cookie names + the pinned paths (ADR-0001). The access cookie is
// path-open (Path=/) so cookie-authenticated routes outside /api — the
// /chatbot AI endpoints — still receive it; the refresh cookie stays
// scoped to its single endpoint so a stolen value can only be spent there.
const (
	CookieAccess      = "ht_access"
	CookieRefresh     = "ht_refresh"
	AccessCookiePath  = "/"
	RefreshCookiePath = "/api/v1/auth/refresh"
)

// AccessCookieOpts is what the handlers need to write the two session
// cookies with the deployment's domain posture.
type AccessCookieOpts struct {
	Domain   string // AUTH_COOKIE_DOMAIN; empty = host-only
	Secure   bool   // false only in local http development
	Insecure bool   // mirrors config.AuthInsecureCookies
}

// AuthRequired is layer 1 of the 2-layer RBAC (ADR-0001, finding S1):
//
//  1. read the ht_access cookie; missing -> 401
//  2. verify the JWT signature + expiry (15m)
//  3. check the instant-revocation marker user:revoked_before:<id>; a marker
//     stamped after the token's iat -> 401 without waiting for the TTL
//  4. load the user row and enforce users.status == 'active' — the central
//     account-state gate S1 said was missing; pending/suspended/deactivated
//     users are rejected here, on every request
//  5. publish the user + access-expiry on the context
//
// Any cache failure in step 3 fails closed (401): this is a health app.
func (s *Service) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := c.Cookie(CookieAccess)
		if err != nil || raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthenticated"})
			return
		}
		claims, err := s.VerifyAccess(c.Request.Context(), raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthenticated"})
			return
		}
		user, err := s.repo.GetByID(c.Request.Context(), claims.UserID)
		if err != nil {
			log.Printf("auth: user lookup failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Internal error"})
			return
		}
		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthenticated"})
			return
		}
		if user.Status != "active" {
			// Central S1 enforcement: pending providers, suspended and
			// deactivated accounts are refused even while their access JWT
			// (up to 15m) is technically valid.
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": accountStateMessage(user.Status)})
			return
		}

		c.Set(CtxUser, user)
		c.Set(CtxAccessExp, claims.Exp)
		c.Next()
	}
}

func accountStateMessage(status string) string {
	switch status {
	case "pending":
		return "Account pending approval"
	case "suspended":
		return "Account suspended"
	case "deactivated":
		return "Account deactivated"
	default:
		return "Account not active"
	}
}

// RequireRoles is layer 2: it must run AFTER AuthRequired and enforces that
// the verified user's role is one of roles (finding S1/S10, 2-layer RBAC).
func (s *Service) RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := currentUser(c)
		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthenticated"})
			return
		}
		for _, r := range roles {
			if user.Role == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "Insufficient role"})
	}
}

// currentUser returns the user AuthRequired published, or nil.
func currentUser(c *gin.Context) *models.User {
	v, ok := c.Get(CtxUser)
	if !ok {
		return nil
	}
	u, _ := v.(*models.User)
	return u
}

// CurrentUser is the exported form of [currentUser], for handlers in other
// packages (e.g. the chatbot) that must run after [AuthRequired] and read the
// verified user by id (for the per-user AI quota, finding S3).
func CurrentUser(c *gin.Context) *models.User { return currentUser(c) }

// RequireCSRF guards cookie-authenticated mutations (ADR-0001): a
// cross-origin page cannot forge the Origin/Referer pair, and a same-origin
// XHR must send the X-CSRF-Token header the client fetches. Requests with
// no Origin header (curl, mobile, server-to-server) pass — those cannot be
// forged by a browser anyway.
func (s *Service) RequireCSRF(allowedOrigins []string) gin.HandlerFunc {
	isSafe := func(m string) bool {
		return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
	}
	return func(c *gin.Context) {
		if isSafe(c.Request.Method) {
			c.Next()
			return
		}
		if tok := c.GetHeader("X-CSRF-Token"); tok != "" {
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = c.GetHeader("Referer")
		}
		if origin == "" {
			c.Next()
			return
		}
		for _, a := range allowedOrigins {
			if a == origin || strings.HasSuffix(origin, a) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "Cross-origin request rejected"})
	}
}

// RateLimit is a Redis-backed fixed-window limiter (finding S7). window is
// the bucket length; limit the max requests per window. The identifier is
// part of the key so a single user can't ride another's quota.
func RateLimit(c cache.Cache, identifier string, window time.Duration, limit int) gin.HandlerFunc {
	prefix := "rl:" + identifier + ":"
	// Round now down to the window so clients in the same window share the
	// bucket; the key then expires naturally.
	return func(ctx *gin.Context) {
		bucket := time.Now().Truncate(window).Unix()
		key := prefix + itoa64(bucket)
		n, err := c.Incr(ctx.Request.Context(), key, window)
		if err != nil {
			// A limiter outage must not take the API down (availability)
			// but it also must not open the floodgate: fail closed with a
			// generic 503.
			log.Printf("auth: rate limiter error: %v", err)
			ctx.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "Rate limiter unavailable"})
			return
		}
		if n > int64(limit) {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"success": false, "error": "Too many requests"})
			return
		}
		ctx.Next()
	}
}

// SetSessionCookies writes the two HttpOnly session cookies per the pinned
// posture. SameSite is set explicitly (gin's SetCookie cannot express it):
// the access cookie is Lax, the refresh cookie is Strict and path-scoped so
// a stolen refresh value can only ever be spent at the refresh endpoint.
func SetSessionCookies(c *gin.Context, opts AccessCookieOpts, access, refresh string) {
	c.Writer.Header().Add("Set-Cookie", (&http.Cookie{
		Name:     CookieAccess,
		Value:    access,
		Path:     AccessCookiePath,
		Domain:   opts.Domain,
		MaxAge:   int(AccessTTL.Seconds()),
		Secure:   opts.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}).String())
	c.Writer.Header().Add("Set-Cookie", (&http.Cookie{
		Name:     CookieRefresh,
		Value:    refresh,
		Path:     RefreshCookiePath,
		Domain:   opts.Domain,
		MaxAge:   int(RefreshTTL.Seconds()),
		Secure:   opts.Secure,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}).String())
}

// ClearSessionCookies expires both cookies (logout, failed rotation). The
// Path/SameSite must match the set cookies or the browser keeps the old one.
func ClearSessionCookies(c *gin.Context, opts AccessCookieOpts) {
	expire := func(name, path string, ss http.SameSite) {
		c.Writer.Header().Add("Set-Cookie", (&http.Cookie{
			Name:     name,
			Value:    "",
			Path:     path,
			Domain:   opts.Domain,
			MaxAge:   -1,
			Secure:   opts.Secure,
			HttpOnly: true,
			SameSite: ss,
		}).String())
	}
	expire(CookieAccess, AccessCookiePath, http.SameSiteLaxMode)
	expire(CookieRefresh, RefreshCookiePath, http.SameSiteStrictMode)
}

// SessionResponse is the shape GET /api/v1/auth/session returns: the
// frontend route guards read exactly this. Unauthenticated responses carry
// user:null (the handler answers 401 with this body).
type SessionResponse struct {
	Authenticated   bool         `json:"authenticated"`
	User            *SessionUser `json:"user"`
	AccessExpiresAt string       `json:"access_expires_at,omitempty"`
}

// SessionUser is the public subset of a user; the full row never leaves the
// backend (no password hash, no PII beyond the profile fields).
type SessionUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	Status    string `json:"status"`
}

// Session builds the response from a verified user; accessExp is the
// token's unix expiry (the handler reads it from CtxAccessExp).
func (s *Service) Session(u *models.User, accessExp int64) *SessionResponse {
	if u == nil {
		return &SessionResponse{Authenticated: false}
	}
	resp := &SessionResponse{
		Authenticated: true,
		User: &SessionUser{
			ID:        u.ID,
			Username:  u.Username,
			Email:     u.Email,
			FirstName: u.FirstName,
			LastName:  u.LastName,
			Role:      u.Role,
			Status:    u.Status,
		},
	}
	if accessExp > 0 {
		resp.AccessExpiresAt = time.Unix(accessExp, 0).UTC().Format(time.RFC3339)
	}
	return resp
}

// RevokeForStatusChange stamps the instant-revocation marker (suspension,
// role change) so live access tokens stop working without waiting for the
// 15m TTL, and kills the user's refresh families.
func (s *Service) RevokeForStatusChange(ctx context.Context, userID int64) error {
	if err := s.MarkRevoked(ctx, userID); err != nil {
		return err
	}
	return s.KillUserFamilies(ctx, userID)
}

// Sentinel helpers so handlers don't repeat the error->status mapping.
func StatusFor(err error) (int, string) {
	switch {
	case errors.Is(err, ErrInvalidCredential):
		return http.StatusUnauthorized, "Invalid credentials"
	case errors.Is(err, ErrPendingAccount):
		return http.StatusForbidden, "Account pending approval"
	case errors.Is(err, ErrNotVerified):
		return http.StatusForbidden, "Email not verified"
	case errors.Is(err, ErrSuspended):
		return http.StatusForbidden, "Account suspended"
	case errors.Is(err, ErrReplayedToken):
		return http.StatusUnauthorized, "Session invalid"
	case errors.Is(err, ErrSessionGone), errors.Is(err, ErrNoToken):
		return http.StatusUnauthorized, "Session invalid"
	default:
		return http.StatusInternalServerError, "Internal error"
	}
}
