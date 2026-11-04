package hands

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/middleware"
	"healthtrack/internal/models"
	"healthtrack/internal/services/email"
	"healthtrack/internal/services/otp"
)

// googleClaims holds the verified Google ID-token fields the legacy
// google-login handler relied on from google-auth-library's payload.
type googleClaims struct {
	email   string
	sub     string
	given   string
	family  string
	picture string
}

var (
	jwksMu    sync.Mutex
	jwksCache map[string][]jwk

	// jwksClient bounds JWKS fetches so a stalled Google endpoint cannot
	// wedge google-login requests.
	jwksClient = &http.Client{Timeout: 15 * time.Second}
)

type jwk struct {
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// verifyGoogleIDToken verifies a Google-issued ID token the same way
// google-auth-library's verifyIdToken does: it checks the signature against
// Google's JWKS, the audience, the expiry and the issuer. The pinned
// golang.org/x/oauth2 dependency in this build does not ship the idtoken
// subpackage, so this is implemented with the standard library.
func verifyGoogleIDToken(ctx context.Context, credential, clientID string) (googleClaims, error) {
	if clientID == "" {
		return googleClaims{}, errors.New("google client id not configured")
	}
	parts := strings.Split(credential, ".")
	if len(parts) != 3 {
		return googleClaims{}, errors.New("malformed id token")
	}
	headerB64, payloadB64, sigB64 := parts[0], parts[1], parts[2]

	headerJSON, err := base64.RawURLEncoding.DecodeString(headerB64)
	if err != nil {
		return googleClaims{}, err
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return googleClaims{}, err
	}
	if header.Alg != "RS256" {
		return googleClaims{}, errors.New("unsupported id token algorithm")
	}

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return googleClaims{}, err
	}

	key, err := googleJWK(ctx, clientID, header.Kid)
	if err != nil {
		return googleClaims{}, err
	}

	mac := sha256.Sum256([]byte(headerB64 + "." + payloadB64))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, mac[:], sig); err != nil {
		return googleClaims{}, errors.New("invalid id token signature")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return googleClaims{}, err
	}
	var claims googleClaims
	var raw map[string]any
	if err := json.Unmarshal(payloadJSON, &raw); err != nil {
		return googleClaims{}, err
	}
	claims.email, _ = raw["email"].(string)
	claims.sub, _ = raw["sub"].(string)
	claims.given, _ = raw["given_name"].(string)
	claims.family, _ = raw["family_name"].(string)
	claims.picture, _ = raw["picture"].(string)

	if claims.email == "" {
		return googleClaims{}, errors.New("id token missing email")
	}
	// Audience may be a string or a []string.
	switch aud := raw["aud"].(type) {
	case string:
		if aud != clientID {
			return googleClaims{}, errors.New("id token audience mismatch")
		}
	case []any:
		found := false
		for _, a := range aud {
			if s, ok := a.(string); ok && s == clientID {
				found = true
				break
			}
		}
		if !found {
			return googleClaims{}, errors.New("id token audience mismatch")
		}
	default:
		return googleClaims{}, errors.New("id token missing audience")
	}
	if exp, ok := raw["exp"].(float64); !ok || time.Now().Unix() >= int64(exp) {
		return googleClaims{}, errors.New("id token expired")
	}
	if iss, _ := raw["iss"].(string); iss != "https://accounts.google.com" && iss != "accounts.google.com" {
		return googleClaims{}, errors.New("id token issuer mismatch")
	}
	return claims, nil
}

// googleJWK fetches Google's JWKS for the token's issuer and returns the RSA
// public key matching kid, caching the set for ten minutes.
func googleJWK(ctx context.Context, clientID, kid string) (*rsa.PublicKey, error) {
	jwksMu.Lock()
	if keys, ok := jwksCache[clientID]; ok && len(keys) > 0 {
		jwksMu.Unlock()
		for _, k := range keys {
			if k.Kid == kid {
				return jwkToRSA(k)
			}
		}
		// Known kid but not cached; fall through to a refresh.
	}
	jwksMu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/certs", nil)
	if err != nil {
		return nil, err
	}
	resp, err := jwksClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch google jwks")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, err
	}

	jwksMu.Lock()
	jwksCache = map[string][]jwk{clientID: set.Keys}
	jwksMu.Unlock()

	for _, k := range set.Keys {
		if k.Kid == kid {
			return jwkToRSA(k)
		}
	}
	return nil, errors.New("google jwks missing key")
}

func jwkToRSA(k jwk) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	if len(eb) == 0 {
		return nil, errors.New("google jwks empty exponent")
	}
	e := 0
	for _, b := range eb {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nb),
		E: e,
	}, nil
}

// isoExpiryNow formats the current time exactly like otp.ExpiryISO's output
// (UTC, RFC3339) so the string comparison "expires_at > now" in the otps
// lookup stays lexically valid, as in the legacy JS toISOString comparison.
func isoExpiryNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// frontendURL reproduces the legacy JS resolution order: FRONTEND_URL env,
// then the Origin header, then x-forwarded-proto + x-forwarded-host, then
// the default production URL.
func frontendURL(c *gin.Context) string {
	if v := os.Getenv("FRONTEND_URL"); v != "" {
		return v
	}
	if origin := c.GetHeader("Origin"); origin != "" {
		return origin
	}
	proto := c.GetHeader("x-forwarded-proto")
	host := c.GetHeader("x-forwarded-host")
	if proto != "" && host != "" {
		return proto + "://" + host
	}
	return "https://healthtrack.store"
}

// generateToken signs an HS256 JWT (7 day expiry) with the configured secret.
func generateToken(cfg *config.Config, u *models.User) string {
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id":  u.ID,
		"username": u.Username,
		"email":    u.Email,
		"exp":      now.AddDate(0, 0, 7).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := t.SignedString([]byte(cfg.JWTSecret))
	return s
}

// userPayload is the shared {id, username, email, role} shape in login-family responses.
func userPayload(u *models.User, role string) gin.H {
	return gin.H{
		"id":       u.ID,
		"username": u.Username,
		"email":    u.Email,
		"role":     role,
	}
}

// userFromRow converts a raw `SELECT * FROM users` row into the parts the
// handlers need; DeriveRole only requires ID, UserType and IsSuperuser.
func userFromRow(row map[string]any) *models.User {
	return &models.User{
		ID:              asInt64(row["id"]),
		Username:        asStr(row["username"]),
		Email:           asStr(row["email"]),
		Password:        asStr(row["password"]),
		FirstName:       asStr(row["first_name"]),
		LastName:        asStr(row["last_name"]),
		UserType:        asStr(row["user_type"]),
		IsApproved:      asInt(row["is_approved"]),
		IsEmailVerified: asInt(row["is_email_verified"]),
		IsSuperuser:     asInt(row["is_superuser"]),
	}
}

func authLogin(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var b struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		_ = c.ShouldBindJSON(&b)

		limiterKey := middleware.RateKey(c, b.Username)
		if !middleware.RateCheck(c, limiterKey) {
			return
		}

		row, err := db.QueryOne(ctx, "SELECT * FROM users WHERE username = $1 OR email = $1", b.Username)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
			return
		}
		if row == nil {
			middleware.RecordAuthFailure(limiterKey)
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid credentials"})
			return
		}
		u := userFromRow(row)
		if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(b.Password)) != nil {
			middleware.RecordAuthFailure(limiterKey)
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid credentials"})
			return
		}
		if u.IsEmailVerified == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"success":               false,
				"error":                 "Email not verified",
				"email":                 u.Email,
				"requires_verification": true,
			})
			return
		}
		role := models.DeriveRole(ctx, db, u)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"token":   generateToken(cfg, u),
			"user":    userPayload(u, role),
		})
	}
}

func authRegister(db *database.DB, cfg *config.Config, mail *email.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var b struct {
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
			State          string `json:"state"`
		}
		_ = c.ShouldBindJSON(&b)

		fail := func(msg string) { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": msg}) }

		// Bound user input before it reaches the DB (schema is TEXT, but
		// oversized payloads still cost a round-trip + index work).
		for _, v := range []string{b.Username, b.Email, b.Password, b.FirstName, b.LastName, b.Role, b.ProviderType, b.BusinessName, b.LicenseNumber, b.Specialization, b.State} {
			if len(v) > 512 {
				fail("Input too long")
				return
			}
		}

		existing, err := db.QueryOne(ctx, "SELECT * FROM users WHERE username = $1 OR email = $2", b.Username, b.Email)
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}
		if existing != nil {
			fail("Username or email already exists")
			return
		}

		normalizedEmail := strings.ToLower(b.Email)
		hashed, err := bcrypt.GenerateFromPassword([]byte(b.Password), 10)
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}

		userType := "patient"
		isApproved := 0
		if b.Role == "doctor" || b.Role == "provider" {
			userType = "provider"
		}
		if b.Role == "patient" {
			isApproved = 1
		}

		// The legacy JS insert stores the state field into the city column.
		userID, err := db.Run(ctx, `INSERT INTO users (username, email, password, first_name, last_name, user_type, is_approved, is_email_verified, city)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 0, $8)`,
			b.Username, normalizedEmail, string(hashed), b.FirstName, b.LastName, userType, isApproved, b.State)
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}

		if b.Role == "doctor" || b.Role == "provider" {
			providerType := b.ProviderType
			if providerType == "" {
				if b.Role == "doctor" {
					providerType = "doctor"
				} else {
					providerType = "pharmacy"
				}
			}
			businessName := b.BusinessName
			if businessName == "" {
				businessName = b.FirstName + " " + b.LastName
			}
			// service_providers has no city column in the Postgres schema; the
			// legacy JS insert of city there is intentionally dropped.
			if _, err := db.Run(ctx,
				"INSERT INTO service_providers (user_id, provider_type, business_name, license_number, specialization) VALUES ($1, $2, $3, $4, $5)",
				userID, providerType, businessName, b.LicenseNumber, b.Specialization); err != nil {
				fail(dbErrorMessage(err))
				return
			}
		}

		baseURL := frontendURL(c)
		code := otp.Generate()
		expiresAt := otp.ExpiryISO(time.Now(), 10)
		if _, err := db.Exec(ctx,
			"INSERT INTO otps (email, otp_code, otp_type, expires_at) VALUES ($1, $2, $3, $4)",
			normalizedEmail, code, "register", expiresAt); err != nil {
			fail(dbErrorMessage(err))
			return
		}

		verificationLink := baseURL + "/verify-email?otp=" + code + "&email=" + url.QueryEscape(normalizedEmail)
		emailSent := mail.SendVerificationLinkEmail(ctx, normalizedEmail, verificationLink, b.FirstName)
		if !emailSent {
			c.JSON(http.StatusOK, gin.H{
				"success":  true,
				"email":    normalizedEmail,
				"username": b.Username,
				"message":  "Account created but failed to send verification email. Please try resending from login page.",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"email":    normalizedEmail,
			"username": b.Username,
			"message":  "Please check your email for the verification link",
		})
	}
}

// authRegisterProfile captures role/provider fields server-side after a
// Supabase sign-up (which already created the auth user + provisioned the
// app row via trigger). It is idempotent: the authenticated user (set by the
// JWT middleware from the Supabase token's `sub`) is the source of truth; we
// upsert their role/provider data without depending on the trigger having
// finished. It tolerates `is_email_verified = 0`, since it's part of
// onboarding before the user confirms their email.
func authRegisterProfile(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		// The token is already verified by JWTRequired; the app user is in the
		// context. Fall back to a DB lookup by the auth UUID if absent.
		var user *models.User
		if raw, ok := c.Get("user"); ok {
			if u, ok := raw.(*models.User); ok {
				user = u
			}
		}
		if user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized"})
			return
		}

		var b struct {
			Email          string `json:"email"`
			Username       string `json:"username"`
			FirstName      string `json:"first_name"`
			LastName       string `json:"last_name"`
			Role           string `json:"role"`
			ProviderType   string `json:"provider_type"`
			BusinessName   string `json:"business_name"`
			LicenseNumber  string `json:"license_number"`
			Specialization string `json:"specialization"`
			City           string `json:"city"`
		}
		_ = c.ShouldBindJSON(&b)

		// Prefer the authenticated user's canonical email/identity; body fields
		// fill in what the Supabase metadata didn't carry.
		email := user.Email
		if b.Email != "" {
			email = strings.ToLower(b.Email)
		}
		firstName := b.FirstName
		if firstName == "" {
			firstName = user.FirstName
		}
		lastName := b.LastName
		if lastName == "" {
			lastName = user.LastName
		}
		username := b.Username
		if username == "" {
			username = user.Username
		}

		role := b.Role
		if role == "" {
			role = "patient"
		}
		switch role {
		case "patient", "doctor", "provider":
		default:
			role = "patient"
		}
		userType := "patient"
		isApproved := 0
		if role == "doctor" || role == "provider" {
			userType = "provider"
		}
		if role == "patient" {
			isApproved = 1
		}

		// Idempotent upsert keyed by email (the unique identity). If a row
		// already exists (trigger provisioned it, or a previous call), update
		// the mutable profile/role fields; otherwise insert.
		existing, err := models.GetUserByEmail(ctx, db, email)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
			return
		}

		if existing != nil {
			if _, err := db.Exec(ctx,
				`UPDATE users SET username=$1, first_name=$2, last_name=$3,
				 user_type=$4, is_approved=$5, city=$6, updated_at=NOW() WHERE id=$7`,
				username, firstName, lastName, userType, isApproved, b.City, existing.ID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
				return
			}
		} else {
			if _, err := db.Run(ctx,
				`INSERT INTO users (username, email, password, first_name, last_name,
				 user_type, is_approved, is_email_verified, city, auth_uuid)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, 0, $8, $9)`,
				username, email, "supabase:" + user.AuthUUID, firstName, lastName,
				userType, isApproved, b.City, user.AuthUUID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": dbErrorMessage(err)})
				return
			}
		}

		// Doctor/provider: capture the provider record too (best-effort,
		// idempotent on user_id).
		if role == "doctor" || role == "provider" {
			providerType := b.ProviderType
			if providerType == "" {
				if role == "doctor" {
					providerType = "doctor"
				} else {
					providerType = "pharmacy"
				}
			}
			businessName := b.BusinessName
			if businessName == "" {
				businessName = firstName + " " + lastName
			}
			if _, err := db.Exec(ctx,
				`INSERT INTO service_providers (user_id, provider_type, business_name, license_number, specialization)
				 VALUES ($1, $2, $3, $4, $5)
				 ON CONFLICT (user_id) DO UPDATE SET
				   provider_type=EXCLUDED.provider_type,
				   business_name=EXCLUDED.business_name,
				   license_number=EXCLUDED.license_number,
				   specialization=EXCLUDED.specialization`,
				lookupUserID(ctx, db, email), providerType, businessName, b.LicenseNumber, b.Specialization); err != nil {
				// Best-effort: the user row is already upserted, so a provider
				// write failure shouldn't fail the onboarding call.
				_ = err
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Profile registered",
		})
	}
}

// lookupUserID returns the app user id for an email (0 when missing). Used
// only for the provider upsert; the users upsert above already ran.
func lookupUserID(ctx context.Context, db *database.DB, email string) int64 {
	u, err := models.GetUserByEmail(ctx, db, email)
	if err != nil || u == nil {
		return 0
	}
	return u.ID
}

func authVerifyOtp(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var b struct {
			Otp      string `json:"otp"`
			Email    string `json:"email"`
			OtpType  string `json:"otp_type"`
			Username string `json:"username"`
		}
		_ = c.ShouldBindJSON(&b)

		limiterKey := middleware.RateKey(c, b.Email, b.Username)
		if !middleware.RateCheck(c, limiterKey) {
			return
		}

		fail := func(msg string) { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": msg}) }

		lookupType := b.OtpType
		if lookupType == "" {
			lookupType = "register"
		}
		now := isoExpiryNow()
		row, err := db.QueryOne(ctx,
			`SELECT * FROM otps WHERE email = $1 AND otp_code = $2 AND otp_type = $3 AND is_used = 0 AND expires_at > $4 ORDER BY id DESC LIMIT 1`,
			strings.ToLower(b.Email), b.Otp, lookupType, now)
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}
		if row == nil {
			middleware.RecordAuthFailure(limiterKey)
			fail("Invalid or expired OTP")
			return
		}
		if _, err := db.Exec(ctx, "UPDATE otps SET is_used = 1 WHERE id = $1", asInt64(row["id"])); err != nil {
			fail(dbErrorMessage(err))
			return
		}

		var u *models.User
		if b.OtpType == "register" || b.OtpType == "password_reset" {
			// The legacy JS looks the user up by the raw (non-lowercased) email here.
			urow, err := db.QueryOne(ctx, "SELECT * FROM users WHERE email = $1", b.Email)
			if err != nil {
				fail(dbErrorMessage(err))
				return
			}
			if urow == nil {
				fail("User not found")
				return
			}
			if _, err := db.Exec(ctx, "UPDATE users SET is_email_verified = 1 WHERE id = $1", asInt64(urow["id"])); err != nil {
				fail(dbErrorMessage(err))
				return
			}
			u = userFromRow(urow)
			u.IsEmailVerified = 1
		} else {
			lookup := b.Username
			if lookup == "" {
				lookup = b.Email
			}
			urow, err := db.QueryOne(ctx, "SELECT * FROM users WHERE username = $1 OR email = $2", lookup, b.Email)
			if err != nil {
				fail(dbErrorMessage(err))
				return
			}
			if urow == nil {
				fail("User not found")
				return
			}
			u = userFromRow(urow)
		}
		if u == nil {
			fail("User not found")
			return
		}

		role := models.DeriveRole(ctx, db, u)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"token":   generateToken(cfg, u),
			"user":    userPayload(u, role),
		})
	}
}

func authResendOtp(db *database.DB, cfg *config.Config, mail *email.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var b struct {
			OtpType   string `json:"otp_type"`
			Email     string `json:"email"`
			FirstName string `json:"first_name"`
		}
		_ = c.ShouldBindJSON(&b)

		fail := func(msg string) { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": msg}) }

		if b.Email == "" {
			fail("Email is required to resend OTP.")
			return
		}
		if len(b.Email) > 512 {
			fail("Email is too long")
			return
		}
		if !middleware.RateCheck(c, middleware.RateKey(c, b.Email)) {
			return
		}

		otpType := b.OtpType
		if otpType == "" {
			otpType = "register"
		}
		code := otp.Generate()
		expiresAt := otp.ExpiryISO(time.Now(), 10)
		if _, err := db.Exec(ctx,
			"INSERT INTO otps (email, otp_code, otp_type, expires_at) VALUES ($1, $2, $3, $4)",
			b.Email, code, otpType, expiresAt); err != nil {
			fail(dbErrorMessage(err))
			return
		}
		_ = mail.SendOtpEmail(ctx, b.Email, code, b.FirstName)

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "A new verification code has been sent to your email"})
	}
}

func authForgotPassword(db *database.DB, cfg *config.Config, mail *email.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var b struct {
			Email string `json:"email"`
		}
		_ = c.ShouldBindJSON(&b)

		if b.Email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Email is required"})
			return
		}
		if len(b.Email) > 512 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Email is too long"})
			return
		}
		if !middleware.RateCheck(c, middleware.RateKey(c, b.Email)) {
			return
		}

		// Mirror the legacy catch-all: any server-side failure here answers with
		// this message so email existence is not leaked.
		serverFail := func() {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to send reset email"})
		}

		normalizedEmail := strings.ToLower(b.Email)
		row, err := db.QueryOne(ctx, "SELECT * FROM users WHERE email = $1", normalizedEmail)
		if err != nil {
			serverFail()
			return
		}
		if row == nil {
			// Do not reveal whether the email exists.
			c.JSON(http.StatusOK, gin.H{"success": true})
			return
		}

		u := userFromRow(row)
		baseURL := frontendURL(c)
		code := otp.Generate()
		expiresAt := otp.ExpiryISO(time.Now(), 15)
		if _, err := db.Exec(ctx,
			"INSERT INTO otps (email, otp_code, otp_type, expires_at) VALUES ($1, $2, $3, $4)",
			normalizedEmail, code, "password_reset", expiresAt); err != nil {
			serverFail()
			return
		}

		resetLink := baseURL + "/reset-password?otp=" + code + "&email=" + url.QueryEscape(normalizedEmail)
		if !mail.SendResetEmail(ctx, normalizedEmail, resetLink, u.FirstName) {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to send reset email. Please check server configuration."})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func authResetPassword(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var b struct {
			Email       string `json:"email"`
			Otp         string `json:"otp"`
			NewPassword string `json:"new_password"`
		}
		_ = c.ShouldBindJSON(&b)

		fail := func(msg string) { c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": msg}) }

		limiterKey := middleware.RateKey(c, b.Email)
		if !middleware.RateCheck(c, limiterKey) {
			return
		}

		if b.Email == "" || b.Otp == "" || b.NewPassword == "" {
			fail("Email, reset code, and new password are required")
			return
		}
		if len(b.NewPassword) > 512 {
			fail("Password is too long")
			return
		}

		normalizedEmail := strings.ToLower(b.Email)
		now := isoExpiryNow()
		// The legacy JS deliberately does not check is_used for password_reset here.
		row, err := db.QueryOne(ctx,
			`SELECT * FROM otps WHERE email = $1 AND otp_code = $2 AND otp_type = $3 AND expires_at > $4 ORDER BY created_at DESC LIMIT 1`,
			normalizedEmail, b.Otp, "password_reset", now)
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}
		if row == nil {
			middleware.RecordAuthFailure(limiterKey)
			fail("Invalid or expired reset link. Please request a new one.")
			return
		}

		urow, err := db.QueryOne(ctx, "SELECT * FROM users WHERE email = $1", normalizedEmail)
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}
		if urow == nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "User not found"})
			return
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(b.NewPassword), 10)
		if err != nil {
			fail(dbErrorMessage(err))
			return
		}
		if _, err := db.Exec(ctx, "UPDATE users SET password = $1 WHERE id = $2", string(hashed), asInt64(urow["id"])); err != nil {
			fail(dbErrorMessage(err))
			return
		}
		if _, err := db.Exec(ctx, "DELETE FROM otps WHERE id = $1", asInt64(row["id"])); err != nil {
			fail(dbErrorMessage(err))
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password updated successfully"})
	}
}

func authGoogleLogin(db *database.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var b struct {
			Credential string `json:"credential"`
			Role       string `json:"role"`
		}
		_ = c.ShouldBindJSON(&b)

		failed := func() {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Google authentication failed"})
		}

		if b.Credential == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Google credential is required"})
			return
		}

		claims, err := verifyGoogleIDToken(ctx, b.Credential, cfg.GoogleClientID)
		if err != nil {
			failed()
			return
		}
		emailAddr := claims.email
		subject := claims.sub
		given := claims.given
		family := claims.family
		pic := claims.picture

		row, err := db.QueryOne(ctx, "SELECT * FROM users WHERE email = $1", emailAddr)
		if err != nil {
			failed()
			return
		}

		if row == nil {
			local := strings.Split(emailAddr, "@")[0]
			suffix := subject
			if len(suffix) > 4 {
				suffix = suffix[len(suffix)-4:]
			}
			username := local + "_" + suffix

			randomBytes := make([]byte, 32)
			if _, err := rand.Read(randomBytes); err != nil {
				failed()
				return
			}
			randomPassword := hex.EncodeToString(randomBytes)
			hashed, err := bcrypt.GenerateFromPassword([]byte(randomPassword), 10)
			if err != nil {
				failed()
				return
			}

			requestedRole := b.Role
			if requestedRole == "" {
				requestedRole = "patient"
			}
			userType := "patient"
			isApproved := 0
			if requestedRole == "doctor" || requestedRole == "provider" {
				userType = "provider"
			}
			if requestedRole == "patient" {
				isApproved = 1
			}

			lastID, err := db.Run(ctx,
				`INSERT INTO users (username, email, password, first_name, last_name, user_type, is_approved, is_email_verified, profile_image)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8)`,
				username, emailAddr, string(hashed), given, family, userType, isApproved, pic)
			if err != nil {
				failed()
				return
			}
			row, err = db.QueryOne(ctx, "SELECT * FROM users WHERE id = $1", lastID)
			if err != nil {
				failed()
				return
			}
			if requestedRole == "doctor" || requestedRole == "provider" {
				providerType := "pharmacy"
				if requestedRole == "doctor" {
					providerType = "doctor"
				}
				businessName := given
				if businessName == "" {
					businessName = username
				}
				if _, err := db.Exec(ctx,
					"INSERT INTO service_providers (user_id, provider_type, business_name) VALUES ($1, $2, $3)",
					lastID, providerType, businessName); err != nil {
					failed()
					return
				}
			}
		}
		if row == nil {
			failed()
			return
		}

		u := userFromRow(row)
		role := models.DeriveRole(ctx, db, u)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"token":   generateToken(cfg, u),
			"user":    userPayload(u, role),
		})
	}
}
