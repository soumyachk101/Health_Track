// L3: [INPUT] depends on the CLI-1 auth module (auth.Service: VerifyAccess +
// Repository.GetByID + the CtxUser/CookieAccess constants) and the legacy
// Bearer middleware (middleware.JWTRequired / AdminRequired); [OUTPUT]
// exports sessionGate and adminSessionGate — the "cookie OR Bearer" gate the
// core + admin routes use so the cookie-only frontend authenticates;
// [POS] the ADR-0001 expand step for residual risk #8: it composes the two
// credential systems already in the tree without a new verifier, and keeps
// the Bearer path live for the cutover window;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package hands

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/middleware"
	"healthtrack/internal/models"
	"healthtrack/internal/modules/auth"
)

// sessionGate authenticates a core-route caller via the new cookie session
// when the ht_access cookie is present, and otherwise falls back to the
// legacy Bearer path. Both surfaces publish the verified user under the
// module key (auth.CtxUser) AND the legacy "user" key, so the downstream
// handlers — which read c.MustGet("user") — work whichever credential
// authenticated the request.
//
// A present-but-invalid cookie does NOT silently fall back to Bearer: a
// client carrying an expired / revoked / suspended session must see the
// 401/403, not have it swapped for a header token — that would mask
// revocation and defeat the instant-invalidation guarantee (finding S1).
func sessionGate(db *database.DB, cfg *config.Config, authSvc *auth.Service) gin.HandlerFunc {
	bearerGate := middleware.JWTRequired(db, cfg)
	return func(c *gin.Context) {
		if raw, err := c.Cookie(auth.CookieAccess); err == nil {
			if cookieSessionUser(c, authSvc, raw) {
				c.Next()
			}
			return // cookie path already aborted on failure
		}
		// No cookie: the legacy Bearer path (kept for the cutover window).
		bearerGate(c)
	}
}

// adminSessionGate is sessionGate plus the admin-role check, so the
// /admin-panel routes accept either credential and then require admin.
func adminSessionGate(db *database.DB, cfg *config.Config, authSvc *auth.Service) gin.HandlerFunc {
	bearerAdminGate := middleware.AdminRequired(db, cfg)
	return func(c *gin.Context) {
		if raw, err := c.Cookie(auth.CookieAccess); err == nil {
			if !cookieSessionUser(c, authSvc, raw) {
				return
			}
			u := c.MustGet("user").(*models.User)
			if u.IsSuperuser != 1 && u.UserType != "admin" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "Admin access required"})
				return
			}
			c.Next()
			return
		}
		bearerAdminGate(c)
	}
}

// cookieSessionUser verifies the ht_access cookie through the auth module
// (signature + expiry + the user:revoked_before instant-invalidation
// marker) and the central account-state gate, then publishes the user
// under both context keys. It aborts the request on any failure and
// returns false; true means c.Next() should be called.
func cookieSessionUser(c *gin.Context, authSvc *auth.Service, raw string) bool {
	ctx := c.Request.Context()
	claims, err := authSvc.VerifyAccess(ctx, raw)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthenticated"})
		return false
	}
	user, err := authSvc.Repo().GetByID(ctx, claims.UserID)
	if err != nil {
		log.Printf("auth: user lookup failed: %v", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Internal error"})
		return false
	}
	if user == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthenticated"})
		return false
	}
	// Central S1 gate: a non-active account is refused even while its 15m
	// access token is technically valid (pending / suspended / deactivated).
	if user.Status != "active" {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": "account not active"})
		return false
	}
	c.Set(auth.CtxUser, user)
	c.Set("user", user)
	return true
}
