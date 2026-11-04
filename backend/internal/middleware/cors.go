package middleware

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
)

// NewCORS ports the CORS block of server.js: a fixed allowlist plus dynamic
// hostname rules (*.vercel.app, *.netlify.app, healthtrack.store and its
// subdomains). Disallowed origins get 403, mirroring the JS cors callback.
func NewCORS(cfg *config.Config) gin.HandlerFunc {
	allowAll := false
	for _, o := range cfg.CORSOrigins {
		if o == "*" {
			allowAll = true
		}
	}

	isAllowed := func(origin string) bool {
		if origin == "" {
			return true
		}
		if allowAll {
			return true
		}
		for _, o := range cfg.CORSOrigins {
			if o == origin {
				return true
			}
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		h := u.Hostname()
		return strings.HasSuffix(h, ".vercel.app") ||
			strings.HasSuffix(h, ".netlify.app") ||
			h == "healthtrack.store" ||
			strings.HasSuffix(h, ".healthtrack.store")
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if !isAllowed(origin) {
			c.AbortWithStatus(403)
			return
		}

		if origin == "" {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Accept,Accept-Encoding,Authorization,Content-Type,DNT,Origin,User-Agent,X-Requested-With,X-CSRFToken")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
