package middleware

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/models"
)

// jwtSecret mirrors middleware/auth.js: process.env.JWT_SECRET || 'default-jwt-secret-change-me'
func jwtSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "default-jwt-secret-change-me"
}

// --- Supabase (GoTrue) JWKS support ---
//
// Supabase Auth issues access tokens signed with the project's auth JWT key.
// Historically that was RS256; newer projects default to ES256 (P-256). Both
// key types are served by the project's JWKS endpoint, and either may be
// present, so both are supported. When SUPABASE_JWKS_URL is unset the whole
// path is disabled and tokens fall through to the legacy HS256 verification.

type supabaseJWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
	X   string `json:"x"`
	Y   string `json:"y"`
	Crv string `json:"crv"`
}

type supabaseJWKS struct {
	keysMu      sync.Mutex
	keys        map[string]any // kid -> *rsa.PublicKey or *ecdsa.PublicKey
	jwksURL     string
	issuer      string
	lastFetch   time.Time
	fetchErr    error
	httpTimeout time.Duration
}

var supabaseKeys = &supabaseJWKS{
	keys:        map[string]any{},
	httpTimeout: 15 * time.Second,
}

// initSupabaseJWKS is called once (idempotent) with the configured URL +
// issuer. When the URL is empty, supabaseVerify always reports "not
// configured" and the caller falls back to the legacy HS256 path.
func initSupabaseJWKS(cfg *config.Config) {
	supabaseKeys.keysMu.Lock()
	defer supabaseKeys.keysMu.Unlock()
	if supabaseKeys.jwksURL != "" && supabaseKeys.jwksURL == cfg.SupabaseJWKSURL {
		return
	}
	supabaseKeys.jwksURL = cfg.SupabaseJWKSURL
	supabaseKeys.issuer = strings.TrimRight(cfg.SupabaseURL, "/") + "/auth/v1"
	supabaseKeys.keys = map[string]any{}
	supabaseKeys.lastFetch = time.Time{}
}

// padLeft left-pads a big-endian byte slice with zeros to length n.
func padLeft(b []byte, n int) []byte {
	if len(b) >= n {
		return b[len(b)-n:]
	}
	out := make([]byte, n)
	copy(out[n-len(b):], b)
	return out
}

// baseURL() exposes the configured base URL for tests.
func (j *supabaseJWKS) baseURL() string { return j.jwksURL }

// configured reports whether the Supabase token path is enabled.
func (j *supabaseJWKS) configured() bool {
	j.keysMu.Lock()
	defer j.keysMu.Unlock()
	return j.jwksURL != ""
}

// fetchJWKS downloads the key set, replacing the cache when the fetch
// succeeds. A 15 minute TTL bounds how often the remote endpoint is hit.
func (j *supabaseJWKS) fetchJWKS(ctx context.Context) error {
	j.keysMu.Lock()
	now := time.Now()
	if j.jwksURL == "" {
		j.keysMu.Unlock()
		return errors.New("SUPABASE_JWKS_URL not configured")
	}
	if !j.lastFetch.IsZero() && now.Sub(j.lastFetch) < 15*time.Minute && j.fetchErr == nil {
		j.keysMu.Unlock()
		return nil
	}
	j.keysMu.Unlock()

	client := &http.Client{Timeout: j.httpTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned %d", resp.StatusCode)
	}
	var set struct {
		Keys []supabaseJWK `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return err
	}

	parsed := map[string]any{}
	for _, k := range set.Keys {
		var key any
		switch k.Kty {
		case "RSA":
			n, err := base64.RawURLEncoding.DecodeString(k.N)
			if err != nil {
				continue
			}
			e, err := base64.RawURLEncoding.DecodeString(k.E)
			if err != nil {
				continue
			}
			modulus := new(big.Int).SetBytes(n)
			exponent := int(new(big.Int).SetBytes(e).Int64())
			if exponent == 0 {
				continue
			}
			key = &rsa.PublicKey{N: modulus, E: exponent}
		case "EC":
			x, err := base64.RawURLEncoding.DecodeString(k.X)
			if err != nil {
				continue
			}
			y, err := base64.RawURLEncoding.DecodeString(k.Y)
			if err != nil {
				continue
			}
			curve := elliptic.P256()
			if strings.EqualFold(k.Crv, "P-384") {
				curve = elliptic.P384()
			}
			// Uncompressed point: 0x04 || X || Y (each padded to the curve byte size).
			b := curve.Params().BitSize / 8
			buf := make([]byte, 0, 1+2*b)
			buf = append(buf, 0x04)
			buf = append(buf, padLeft(x, b)...)
			buf = append(buf, padLeft(y, b)...)
			pubX, pubY := elliptic.Unmarshal(curve, buf)
			if pubX == nil {
				continue
			}
			key = &ecdsa.PublicKey{Curve: curve, X: pubX, Y: pubY}
		default:
			continue
		}
		parsed[k.Kid] = key
	}

	j.keysMu.Lock()
	j.keys = parsed
	j.lastFetch = now
	j.keysMu.Unlock()
	return nil
}

// supabaseVerify validates a Supabase-issued access token: RS256 or ES256
// signature against the project JWKS, plus exp / iss / aud claims. On
// success it returns the `sub` claim (auth.users.id, i.e. auth_uuid).
// It returns a "not configured" error when the JWKS URL was never set.
func (j *supabaseJWKS) supabaseVerify(ctx context.Context, tokenStr string) (string, error) {
	if !j.configured() {
		return "", errors.New("supabase not configured")
	}

	// Parse the header to learn the kid + alg before full verification.
	segments := strings.Split(tokenStr, ".")
	if len(segments) != 3 {
		return "", errors.New("malformed token")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	decoded, err := base64.RawURLEncoding.DecodeString(segments[0])
	if err != nil {
		return "", errors.New("malformed token header")
	}
	if err := json.Unmarshal(decoded, &header); err != nil {
		return "", errors.New("malformed token header")
	}

	if err := j.fetchJWKS(ctx); err != nil {
		log.Printf("supabase JWKS fetch failed: %v", err)
		return "", err
	}

	j.keysMu.Lock()
	pub, ok := j.keys[header.Kid]
	j.keysMu.Unlock()
	if !ok {
		// The kid may be new since the last fetch; force one more try.
		j.keysMu.Lock()
		j.lastFetch = time.Time{}
		j.keysMu.Unlock()
		if err := j.fetchJWKS(ctx); err == nil {
			j.keysMu.Lock()
			pub, ok = j.keys[header.Kid]
			j.keysMu.Unlock()
		}
		if !ok {
			return "", fmt.Errorf("no JWKS key for kid %q", header.Kid)
		}
	}

	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		return pub, nil
	},
		jwt.WithValidMethods([]string{"RS256", "ES256"}),
		jwt.WithIssuer(j.issuer),
		jwt.WithAudience("authenticated"),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", err
		}
		return "", fmt.Errorf("supabase token invalid: %w", err)
	}
	if !parsed.Valid {
		return "", errors.New("supabase token invalid")
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", errors.New("supabase token has no sub")
	}
	return sub, nil
}

// authenticate runs the JWTRequired logic and, on success, loads the user and
// stores it in the gin context under "user". It aborts the request with the
// appropriate 401 on any failure and returns nil.
//
// Two token formats are accepted:
//  1. Supabase Auth (GoTrue) access tokens (RS256/ES256, verified against the
//     configured JWKS); the user is resolved by the token's `sub`
//     (auth.users.id == users.auth_uuid).
//  2. Legacy HS256 self-issued tokens (7 day, claims user_id/username/email)
//     signed with JWT_SECRET, resolved by user_id. Kept so sessions issued
//     before the migration keep working.
func authenticate(ctx context.Context, c *gin.Context, db *database.DB, cfg *config.Config) *models.User {
	authHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		c.AbortWithStatusJSON(401, gin.H{"error": "Authorization header missing or invalid"})
		return nil
	}
	tokenStr := strings.SplitN(authHeader, " ", 2)[1]

	// Supabase path first: only meaningful when configured; a token that
	// fails Supabase verification (or that is not a Supabase token at all)
	// falls through to the legacy HS256 path below.
	if supabaseKeys.configured() {
		if sub, err := supabaseKeys.supabaseVerify(ctx, tokenStr); err == nil {
			user, lerr := models.GetUserByAuthUUID(ctx, db, sub)
			if lerr != nil {
				log.Printf("Supabase user lookup error: %v", lerr)
				c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
				return nil
			}
			if user == nil {
				// Auth user exists in GoTrue but no app row was provisioned
				// yet (trigger lag / legacy account). Retry once after a
				// short wait, then give up.
				time.Sleep(250 * time.Millisecond)
				user, lerr = models.GetUserByAuthUUID(ctx, db, sub)
				if lerr == nil && user != nil {
					c.Set("user", user)
					return user
				}
				c.AbortWithStatusJSON(401, gin.H{"error": "User not found"})
				return nil
			}
			c.Set("user", user)
			return user
		}
	}

	// Legacy self-issued HS256 path.
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(*jwt.Token) (interface{}, error) {
		return []byte(jwtSecret()), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			c.AbortWithStatusJSON(401, gin.H{"error": "Token has expired"})
		} else {
			c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
		}
		return nil
	}

	rawID, ok := claims["user_id"].(float64)
	if !ok {
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
		return nil
	}

	user, err := models.GetUser(ctx, db, int64(rawID))
	if err != nil {
		log.Printf("JWT user lookup error: %v", err)
		c.AbortWithStatusJSON(401, gin.H{"error": "Invalid token"})
		return nil
	}
	if user == nil {
		c.AbortWithStatusJSON(401, gin.H{"error": "User not found"})
		return nil
	}

	c.Set("user", user)
	return user
}

// InitFromConfig wires the JWKS cache for the running process. Called from
// BuildRouter so the middleware and the register-profile handler share one
// cache. Safe to call more than once.
func InitFromConfig(cfg *config.Config) {
	initSupabaseJWKS(cfg)
}

// JWTRequired verifies the Bearer token, loads the user and puts it in the
// context under "user". Consumers retrieve it via c.MustGet("user").(*models.User).
func JWTRequired(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authenticate(c.Request.Context(), c, db, cfg) == nil {
			return
		}
		c.Next()
	}
}

// AdminRequired runs JWTRequired first, then enforces admin access.
func AdminRequired(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := authenticate(c.Request.Context(), c, db, cfg)
		if user == nil {
			return
		}
		if user.IsSuperuser != 1 && user.UserType != "admin" {
			c.AbortWithStatusJSON(403, gin.H{"success": false, "error": "Admin access required"})
			return
		}
		c.Next()
	}
}
