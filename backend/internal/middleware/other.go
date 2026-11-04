package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// StripTrailingSlash mirrors the server.js middleware: when the path ends in
// '/' and is longer than one character, one trailing '/' is removed. The query
// string is preserved untouched.
func StripTrailingSlash() gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if len(p) > 1 && strings.HasSuffix(p, "/") {
			c.Request.URL.Path = strings.TrimSuffix(p, "/")
		}
		c.Next()
	}
}

// RequestLogger writes one structured JSON line per request, carrying a
// request id (honored from an incoming X-Request-ID header, otherwise
// generated) and the response latency.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			b := make([]byte, 8)
			if _, err := rand.Read(b); err != nil {
				rid = "-"
			} else {
				rid = hex.EncodeToString(b)
			}
		}
		c.Header("X-Request-ID", rid)
		start := time.Now()
		c.Next()
		log.Printf(`{"time":"%s","request_id":"%s","method":"%s","path":"%s","status":%d,"client_ip":"%s","latency_ms":%d}`,
			time.Now().UTC().Format(time.RFC3339Nano),
			rid,
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			c.ClientIP(),
			time.Since(start).Milliseconds(),
		)
	}
}

// MaxBodySize caps request bodies at limit bytes. Oversized bodies fail JSON
// binding with an error instead of being accepted, bounding what reaches the
// database layer.
func MaxBodySize(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

// SecurityHeaders sets the browser-facing hardening headers on every API
// response. The pages are served by the frontend (Vercel/Netlify), which sets
// its own CSP; here we only enforce the headers that matter on JSON + file
// responses: no MIME sniffing, never frameable, and no sensitive referrer.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), geolocation=(), payment=(), usb=()")
		c.Next()
	}
}

// RecoverAndLog recovers from panics in downstream handlers, logs them with
// a stack trace and responds with 500, mirroring a JS try/catch that rejects
// with a 500.
func RecoverAndLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("recovered from panic on %s %s: %v\n%s",
					c.Request.Method, c.Request.URL.Path, r, debug.Stack())
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			}
		}()
		c.Next()
	}
}
