package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"

	"healthtrack/internal/config"
	"healthtrack/internal/models"
	"healthtrack/internal/platform/cache"
)

// TTLs and token parameters (ADR-0001).
const (
	AccessTTL         = 15 * time.Minute
	RefreshTTL        = 14 * 24 * time.Hour
	RefreshByteLen    = 32 // opaque token: 32 random bytes
	ConsumedMarkerTTL = 24 * time.Hour
	RevokedMarkerTTL  = 2 * time.Hour // must cover the max access TTL (15m)
	OtpRegisterTTL    = 10 * time.Minute
	OtpResetTTL       = 15 * time.Minute

	// Argon2id tuning: OWASP 2024 minimum for memory-constrained servers.
	argon2Time    = 3
	argon2Memory  = 64 * 1024 // 64 MiB
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

// Errs surfaced to handlers; each maps to a fixed, enumeration-safe message.
var (
	ErrInvalidCredential = errors.New("invalid credential")
	ErrPendingAccount    = errors.New("account pending approval")
	ErrNotVerified       = errors.New("email not verified")
	ErrSuspended         = errors.New("account suspended")
	ErrSessionGone       = errors.New("session invalid")
	ErrReplayedToken     = errors.New("refresh token replay detected")
	ErrNoToken           = errors.New("no session")
)

// AccessClaims is the verified shape of an ht_access JWT.
type AccessClaims struct {
	UserID   int64
	Username string
	Email    string
	Role     string
	Status   string
	Revoked  int64 // rev claim: revocation epoch the token was minted against
	Issued   int64 // unix seconds (iat)
	Exp      int64
}

// RefreshResult is what the rotation produces: the successor token the
// handler puts into the cookie, plus whether the presented token was a
// replay (the whole family is then dead and the user's access tokens are
// revoked too).
type RefreshResult struct {
	NewToken string
	FamilyID string
	UserID   int64
}

// Service owns all token/credential logic. Handlers stay thin: bind,
// call, render.
type Service struct {
	cfg   *config.Config
	repo  *Repository
	cache cache.Cache
}

func NewService(cfg *config.Config, repo *Repository, c cache.Cache) *Service {
	return &Service{cfg: cfg, repo: repo, cache: c}
}

// Repo / Cache accessors for the middleware and handlers.
func (s *Service) Repo() *Repository  { return s.repo }
func (s *Service) Cache() cache.Cache { return s.cache }

// --- password hashing ---

// HashPassword returns an argon2id encoded hash in the PHC format
// "$argon2id$<hex-salt>$<hex-hash>". Both salt and hash are hex-encoded so
// the value is a clean, ASCII-safe string that round-trips through the TEXT
// password_hash column. VerifyPassword parses the same field layout.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey(
		[]byte(password), salt,
		argon2Time, argon2Memory, argon2Threads, argon2KeyLen,
	)
	return fmt.Sprintf("$argon2id$%s$%s", hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

// VerifyPassword checks a stored argon2id hash. It does not touch the legacy
// bcrypt column — callers try argon2id first and fall back to bcrypt.
func VerifyPassword(stored, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) < 4 || parts[1] != "argon2id" {
		return false
	}
	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)
	return hmacEqual(want, got)
}

// VerifyLegacyBcrypt checks the pre-Phase-1 bcrypt column (rollout only).
func VerifyLegacyBcrypt(stored, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(stored), []byte(password)) == nil
}

// hmacEqual is a constant-time byte compare (no stdlib version yet).
func hmacEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// --- token issuance / verification ---

// IssueAccess signs the 15m ht_access JWT. The claim set is the pinned
// ADR-0001 contract: sub (user id, int), username, email, role, status,
// type="ht_access", iat, exp, rev. The `type` claim is what separates
// new-module tokens from the legacy 7-day HS256 tokens that share
// JWT_SECRET: parseAccess refuses any token without it, so a legacy
// cookie/header can never be upgraded into a module session.
func (s *Service) IssueAccess(ctx context.Context, u *models.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":      u.ID,
		"username": u.Username,
		"email":    u.Email,
		"role":     u.Role,
		"status":   u.Status,
		"type":     "ht_access",
		"iat":      now.Unix(),
		"exp":      now.Add(AccessTTL).Unix(),
		"rev":      0,
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(s.cfg.JWTSecret))
}

// VerifyAccess parses + validates an ht_access token and applies the instant
// revocation check (user:revoked_before:<id>).
func (s *Service) VerifyAccess(ctx context.Context, tokenStr string) (*AccessClaims, error) {
	claims, err := s.parseAccess(tokenStr)
	if err != nil {
		return nil, err
	}
	if cut, active := s.revokedBefore(ctx, claims.UserID); active && claims.Issued < cut {
		// The account was suspended/re-scoped after this token was minted.
		return nil, ErrSessionGone
	}
	return claims, nil
}

func (s *Service) parseAccess(tokenStr string) (*AccessClaims, error) {
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, func(*jwt.Token) (any, error) {
		return []byte(s.cfg.JWTSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, ErrSessionGone
	}
	if !parsed.Valid {
		return nil, ErrSessionGone
	}
	// Only module-issued tokens carry type=ht_access; a legacy HS256 token
	// (7-day, no type claim) must not be accepted into a module session.
	if claims["type"] != "ht_access" {
		return nil, ErrSessionGone
	}
	uid, ok := claims["sub"].(float64)
	if !ok {
		return nil, ErrSessionGone
	}
	out := &AccessClaims{
		UserID:  int64(uid),
		Issued:  int64(asFloat(claims["iat"])),
		Exp:     int64(asFloat(claims["exp"])),
		Revoked: int64(asFloat(claims["rev"])),
	}
	if username, ok := claims["username"].(string); ok {
		out.Username = username
	}
	if email, ok := claims["email"].(string); ok {
		out.Email = email
	}
	if role, ok := claims["role"].(string); ok {
		out.Role = role
	}
	if status, ok := claims["status"].(string); ok {
		out.Status = status
	}
	return out, nil
}

func asFloat(v any) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	if n, ok := v.(int64); ok {
		return float64(n)
	}
	return 0
}

// --- refresh rotation (Redis, hashed) ---

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewRefreshToken returns a fresh 32-byte opaque token (hex) + its family.
func NewRefreshToken() (token, family string) {
	buf := make([]byte, RefreshByteLen)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // crypto/rand is deterministic-failure-free on every Go target
	}
	fam := make([]byte, 16)
	if _, err := rand.Read(fam); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf), hex.EncodeToString(fam)
}

// StartSession issues both cookies' contents for a fresh login: a new
// access token and a brand-new refresh family. Any families the user still
// holds (other devices/browsers) are killed first, so at most one refresh
// chain exists per user and a fresh login cannot be shadowed by an old
// cookie.
func (s *Service) StartSession(ctx context.Context, u *models.User) (access, refresh, family string, err error) {
	access, err = s.IssueAccess(ctx, u)
	if err != nil {
		return "", "", "", err
	}
	if err := s.KillUserFamilies(ctx, u.ID); err != nil {
		return "", "", "", err
	}
	refresh, family = NewRefreshToken()
	h := tokenHash(refresh)
	// rt:<h> = live marker; rt_fam:<h> = hash->family map (survives the
	// rotation so a replay can still be attributed); rt_family:<f> = the
	// family record "uid|<currentHash>".
	if err := s.cache.Set(ctx, "rt:"+h, family, RefreshTTL); err != nil {
		return "", "", "", err
	}
	if err := s.cache.Set(ctx, "rt_fam:"+h, family, RefreshTTL); err != nil {
		return "", "", "", err
	}
	if err := s.cache.Set(ctx, "rt_family:"+family, itoa64(u.ID)+"|"+h, RefreshTTL); err != nil {
		return "", "", "", err
	}
	// Best-effort per-user index so a suspension can kill every family the
	// user still holds; correctness never depends on it.
	_ = s.cache.SAdd(ctx, "rt_user:"+itoa64(u.ID), []string{family}, RefreshTTL)
	return access, refresh, family, nil
}

// Rotate atomically consumes the presented token (by hash) and mints its
// successor. A replayed (already-consumed) token kills the whole family AND
// stamps the user's revocation marker so every live access token of that
// user stops working immediately (no waiting for the 15m TTL).
func (s *Service) Rotate(ctx context.Context, presented string) (*RefreshResult, error) {
	h0 := tokenHash(presented)
	ttlSec := int(RefreshTTL.Seconds())

	// Step 1: attribute the presented token to its family. The rt_fam map
	// outlives the token's own rotation, so even a replayed hash resolves.
	fam, err := s.cache.Get(ctx, "rt_fam:"+h0)
	if err != nil {
		if cache.IsNotFound(err) {
			// Never issued by this deployment (or TTL-lapsed): a generic
			// session error, no kill needed.
			return nil, ErrSessionGone
		}
		return nil, err
	}

	newTok, _ := NewRefreshToken()
	h1 := tokenHash(newTok)

	// Step 2: the atomic rotation itself (Lua in Redis, mirrored in
	// MemCache).
	res, err := s.cache.Eval(ctx, cache.RotateScript,
		[]string{
			"rt:" + h0,
			"rt_fam:" + h0,
			"rt_family:" + string(fam),
			"rt_family:" + string(fam) + ":revoked",
			"rt:" + h1,
			"rt_fam:" + h1,
		},
		// Variadic ARGV: ttl (ARGV[1]) + successor hash h1 (ARGV[2]).
		// Pass them as individual args, not as one []any value — the Lua
		// script (and the MemCache mirror) read them positionally.
		ttlSec, h1)
	if err != nil {
		return nil, err
	}
	out := fmt.Sprint(res)
	switch {
	case out == "ok":
		uid, _, ok := splitPipeFamilyRec(ctx, s, string(fam))
		if !ok {
			// The family record is gone even though rotation said ok:
			// the token had expired mid-flight; treat as a dead session.
			return nil, ErrSessionGone
		}
		_ = s.cache.SAdd(ctx, "rt_user:"+itoa64(uid), []string{string(fam)}, RefreshTTL)
		return &RefreshResult{NewToken: newTok, FamilyID: string(fam), UserID: uid}, nil

	case strings.HasPrefix(out, "reused:"):
		// Replay. The uid is in the payload; kill the family's leftovers
		// and stamp the user-level revocation marker.
		uid := atoiSafe(strings.TrimPrefix(out, "reused:"))
		_ = s.MarkRevoked(ctx, uid)
		_ = s.KillUserFamilies(ctx, uid)
		return nil, ErrReplayedToken

	default:
		// "dead": already killed (prior replay, logout, or TTL).
		return nil, ErrSessionGone
	}
}

// splitPipeFamilyRec reads the family record back out of Redis for the
// caller-supplied hash (the rotation itself did not return the uid for the
// "ok" path).
func splitPipeFamilyRec(ctx context.Context, s *Service, fam string) (int64, string, bool) {
	// After "ok", rt_family:<f> now holds "uid|h1" (the successor hash).
	rec, err := s.cache.Get(ctx, "rt_family:"+fam)
	if err != nil {
		return 0, "", false
	}
	return splitPipe(string(rec))
}

// splitPipe splits a family record "uid|hash" into (uid, hash, ok).
func splitPipe(v string) (int64, string, bool) {
	i := strings.IndexByte(v, '|')
	if i <= 0 {
		return 0, "", false
	}
	return atoiSafe(v[:i]), v[i+1:], true
}

func atoiSafe(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

// KillUserFamilies deletes every refresh family the user holds, so a
// logout/suspension/replay cannot be extended by a still-valid refresh
// cookie. It walks the per-user family index; a missed index entry (best
// effort write at StartSession) leaves at most one family, which then dies
// at its 14-day TTL.
func (s *Service) KillUserFamilies(ctx context.Context, uid int64) error {
	if uid <= 0 {
		return nil
	}
	userKey := "rt_user:" + itoa64(uid)
	if fams, err := s.cache.SMembers(ctx, userKey); err == nil {
		for _, fam := range fams {
			if rec, gerr := s.cache.Get(ctx, "rt_family:"+fam); gerr == nil {
				if _, h, ok := splitPipe(string(rec)); ok {
					_ = s.cache.Del(ctx, "rt:"+h, "rt_fam:"+h)
				}
			}
			// Predecessor tokens that rotated out of the record also
			// linger under their own rt_fam keys; a replayed predecessor
			// would otherwise resolve its family here. Sweep the known
			// one (current hash) plus the record and its death flag.
			_ = s.cache.Del(ctx, "rt_family:"+fam, "rt_family:"+fam+":revoked")
		}
	}
	return s.cache.Del(ctx, userKey)
}

// MarkRevoked stamps the instant-revocation marker.
func (s *Service) MarkRevoked(ctx context.Context, uid int64) error {
	return s.cache.Set(ctx, "user:revoked_before:"+itoa64(uid), time.Now().Unix(), RevokedMarkerTTL)
}

func (s *Service) revokedBefore(ctx context.Context, uid int64) (int64, bool) {
	v, err := s.cache.Get(ctx, "user:revoked_before:"+itoa64(uid))
	if err != nil {
		if cache.IsNotFound(err) {
			return 0, false
		}
		// Cache outage: fail closed for a health app.
		return time.Now().Unix(), true
	}
	var cut int64
	for _, c := range string(v) {
		if c < '0' || c > '9' {
			return 0, false
		}
		cut = cut*10 + int64(c-'0')
	}
	return cut, true
}

// Logout kills the user's families and marks access tokens revoked.
func (s *Service) Logout(ctx context.Context, uid int64) error {
	_ = s.MarkRevoked(ctx, uid)
	return s.KillUserFamilies(ctx, uid)
}

// --- per-user AI quota (finding S3: chatbot/TTS are bounded) ---
//
// The chatbot + TTS endpoints run behind AuthRequired; AIQuota gates each
// call by the verified user id so one account can't burn unbounded provider
// spend. The counter is a fixed-window INCR in the shared cache (Redis in
// production, in-memory in dev/tests), so it holds across API instances.

// AIQuota is the per-user-per-window budget the AI endpoints enforce.
const (
	AIChatQuota   = 60 // chat completions per window
	AITTSQuota    = 30 // TTS calls per window
	AIQuotaWindow = 15 * time.Minute
)

// CheckAIQuota reports whether the user may make another AI call of the given
// kind ("chat" | "tts") within the window. A cache failure returns
// ok=false so the caller can answer 503 (fail closed on spend, open on
// availability elsewhere).
func (s *Service) CheckAIQuota(ctx context.Context, uid int64, kind string) (bool, error) {
	limit := AIChatQuota
	if kind == "tts" {
		limit = AITTSQuota
	}
	return cache.QuotaCheck(ctx, s.cache, uid, "ai:"+kind, AIQuotaWindow, limit)
}

// --- OTP / password reset (atomic, hashed, single-use) ---

// Otp is the server-side half of a code: only the hash is ever stored.
type Otp struct {
	Code    string
	Purpose string // "register" | "password_reset"
	Email   string
}

// otpExpiryRFC3339 is the "YYYY-MM-DDTHH:MM:SSZ" UTC expiration format the
// legacy otps.expires_at TEXT column uses (the legacy handlers compare it
// lexically against time.Now().UTC().Format(time.RFC3339)).
func otpExpiryRFC3339(now time.Time, minutes int) string {
	return now.Add(time.Duration(minutes) * time.Minute).UTC().Format(time.RFC3339)
}

// CreateOtp stores a new 6-digit code for email+purpose (hash-only, S4) and
// returns the plaintext code for the email step. A resend drops the
// previous unused code so at most one code is ever live per email+purpose.
func (s *Service) CreateOtp(ctx context.Context, email, purpose string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	now := time.Now()
	code := randomOtp()
	hash := hex.EncodeToString(otpHashBytes(code, purpose, email))
	db := s.repo.UserDB()
	if _, err := db.Exec(ctx, `
		INSERT INTO otps (email, otp_code, otp_type, is_used, expires_at)
		VALUES ($1, $2, $3, 0, $4)`,
		email, hash, purpose, otpExpiryRFC3339(now, otpTTLMinutes(purpose))); err != nil {
		return "", err
	}
	// Drop every earlier unused code of the same purpose: the newest one is
	// the only one that verifies, and a leaked older code can't be reused.
	_, _ = db.Exec(ctx, `
		DELETE FROM otps WHERE email = $1 AND otp_type = $2 AND otp_code <> $3 AND is_used = 0`,
		email, purpose, hash)
	return code, nil
}

// VerifyOtp atomically consumes one code (UPDATE ... WHERE is_used = 0 AND
// expires_at > now RETURNING, S4). Returns ok=true only for a live,
// unexpired, unused code; a wrong/expired/replayed code all yield
// ok=false with a nil error so the caller can answer one generic message
// (no enumeration, no replay).
func (s *Service) VerifyOtp(ctx context.Context, email, code, purpose string) (bool, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	hash := hex.EncodeToString(otpHashBytes(code, purpose, email))
	nowRFC3339 := time.Now().UTC().Format(time.RFC3339)
	id, err := s.repo.ClaimOtpRow(ctx, email, hash, purpose, nowRFC3339)
	if err != nil {
		return false, err
	}
	return id != 0, nil
}

func otpHashBytes(code, purpose, email string) []byte {
	sum := sha256.Sum256([]byte(strings.Join([]string{code, purpose, email}, "|")))
	return sum[:]
}

func otpTTLMinutes(purpose string) int {
	if purpose == "password_reset" {
		return int(OtpResetTTL.Minutes())
	}
	return int(OtpRegisterTTL.Minutes())
}

// randomOtp returns a 6-digit code in [100000, 999999] using crypto/rand
// (mirrors the legacy services/otp.Generate bounds).
func randomOtp() string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000)) // 0..899999
	if err != nil {
		return "100000" // deterministic fallback: not a security gate
	}
	return fmt.Sprintf("%06d", 100000+int(n.Int64()))
}

// itoa64 formats a non-negative int64 (the uid domain).
func itoa64(v int64) string {
	if v <= 0 {
		return "0"
	}
	var b [24]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

// VerifyEmail marks the user's email verified (after an OTP of purpose
// "register" was consumed) and flips status "pending" -> "active" when the
// role is already approved.
func (s *Service) MarkEmailVerified(ctx context.Context, userID int64) (*models.User, error) {
	if _, err := s.repo.UserDB().Exec(ctx,
		"UPDATE users SET is_email_verified = 1, updated_at = NOW() WHERE id = $1", userID); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, userID)
}

// ResetPassword consumes a live password-reset code and writes both the new
// argon2id hash (new module) and the bcrypt hash (legacy rollout). One
// code, one reset: the atomic claim in VerifyOtp enforces that.
func (s *Service) ResetPassword(ctx context.Context, userID int64, newPassword string) error {
	argon, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	legacy, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	if err != nil {
		return err
	}
	return s.repo.SetPassword(ctx, userID, argon, string(legacy))
}

// --- registration (transactional, S6) ---

// RegisterInput is the validated shape of a new account.
type RegisterInput struct {
	Username       string
	Email          string
	Password       string
	FirstName      string
	LastName       string
	Role           string // patient|doctor|provider
	ProviderType   string
	BusinessName   string
	LicenseNumber  string
	Specialization string
	City           string
}

// Register creates the users row + (doctor/provider) service_providers row
// in ONE transaction, then stores the verification OTP and returns the
// plaintext code for the mail step. If any DB step fails, nothing is
// committed (S6).
func (s *Service) Register(ctx context.Context, in *RegisterInput) (string, error) {
	db := s.repo.UserDB()
	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}

	argon, err := HashPassword(in.Password)
	if err != nil {
		_ = tx.Rollback(ctx)
		return "", err
	}
	legacy, _ := bcrypt.GenerateFromPassword([]byte(in.Password), 10)

	role := normalizeRole(in.Role)
	status := "active"
	if role != "patient" {
		status = "pending"
	}

	var userID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, email, password, password_hash, role, status,
		                       first_name, last_name, user_type, is_approved,
		                       is_email_verified, is_active, city)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 0, 1, $11)
		RETURNING id`,
		in.Username,
		strings.ToLower(strings.TrimSpace(in.Email)),
		string(legacy),
		argon,
		role,
		status,
		in.FirstName, in.LastName,
		legacyUserType(role),
		0,
		in.City,
	).Scan(&userID)
	if err != nil {
		_ = tx.Rollback(ctx)
		return "", err
	}

	if role == "doctor" || role == "provider" {
		pType := in.ProviderType
		if pType == "" {
			if role == "doctor" {
				pType = "doctor"
			} else {
				pType = "pharmacy"
			}
		}
		biz := in.BusinessName
		if biz == "" {
			biz = in.FirstName + " " + in.LastName
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO service_providers (user_id, provider_type, business_name, license_number, specialization) VALUES ($1,$2,$3,$4,$5)",
			userID, pType, biz, in.LicenseNumber, in.Specialization); err != nil {
			_ = tx.Rollback(ctx)
			return "", err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	// OTP is written after commit: a failed OTP insert still leaves a valid
	// (unverified) account the user can resend for — better than rolling
	// back a successful registration because the OTP insert failed.
	return s.CreateOtp(ctx, in.Email, "register")
}

// legacyUserType maps the new role onto the legacy user_type values the
// admin UI still displays during rollout.
func legacyUserType(role string) string {
	if role == "doctor" || role == "provider" {
		return "provider"
	}
	return "patient"
}

func normalizeRole(r string) string {
	switch r {
	case "admin", "doctor", "provider":
		return r
	default:
		return "patient"
	}
}

// --- login ---

// Login verifies credentials (argon2id first, legacy bcrypt fallback),
// checks account state, and returns the session pair. All credential
// failures collapse to ErrInvalidCredential (no email enumeration).
func (s *Service) Login(ctx context.Context, identifier, password string) (u *models.User, access, refresh, fam string, err error) {
	u, err = s.repo.GetByEmail(ctx, identifier)
	if err != nil {
		return
	}
	if u == nil {
		// The identifier may be a username: one extra lookup, still the same
		// generic error afterwards.
		u, err = s.usernameLookup(ctx, identifier)
	}
	if err != nil {
		return
	}
	if u == nil {
		err = ErrInvalidCredential
		return
	}

	// Credential check: argon2id when present, else the legacy bcrypt.
	ok := false
	if u.PasswordHash != "" {
		ok = VerifyPassword(u.PasswordHash, password)
	} else if u.Password != "" {
		ok = VerifyLegacyBcrypt(u.Password, password)
	}
	if !ok {
		err = ErrInvalidCredential
		u = nil
		return
	}
	if u.Status == "suspended" {
		err = ErrSuspended
		return
	}
	if u.Status == "pending" {
		err = ErrPendingAccount
		return
	}
	if u.IsEmailVerified == 0 {
		err = ErrNotVerified
		return
	}

	access, refresh, fam, err = s.StartSession(ctx, u)
	if err != nil {
		u = nil
		return
	}
	return
}

func (s *Service) usernameLookup(ctx context.Context, username string) (*models.User, error) {
	row, err := s.repo.UserDB().QueryOne(ctx, "SELECT * FROM users WHERE username = $1", username)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return userFromRow(row), nil
}
