// Package auth is the Pure Go session system from ADR-0001: short-lived
// JWT access tokens in HttpOnly cookies, opaque refresh tokens rotated in
// Redis, argon2id passwords, atomic hashed OTP/reset, and 2-layer RBAC
// (AuthRequired + RequireRoles) with object-level consent checks.
//
// L3: [INPUT] depends on *database.DB, *config.Config, the platform cache
// (Redis or in-memory) and the models package; [OUTPUT] provides the auth
// handlers, service, repository predicates and the AuthRequired/RequireRoles
// middleware; [POS] the identity module of the backend — handlers stay thin,
// the service owns token/credential logic, the repository owns every
// authorization predicate, so IDOR is impossible by construction;
// [PROTOCOL] Update this header on change, then check CLAUDE.md
package auth

import (
	"context"
	"errors"
	"strings"

	"healthtrack/internal/database"
	"healthtrack/internal/models"
)

// Repository owns the DB-side authorization predicates of the auth module.
// Handlers never call db directly; every cross-user read goes through one
// of these, which is where an IDOR would have to live.
type Repository struct {
	db *database.DB
}

// NewRepository wraps the shared pool.
func NewRepository(db *database.DB) *Repository { return &Repository{db: db} }

// UserDB is the *database.DB behind the repository (exposed so the service
// can run multi-statement transactions for registration, S6).
func (r *Repository) UserDB() *database.DB { return r.db }

// GetByEmail / GetByID are thin passes; the auth service resolves users by
// email first (the login identifier is email-or-username).
func (r *Repository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return models.GetUserByEmail(ctx, r.db, strings.ToLower(strings.TrimSpace(email)))
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	return models.GetUser(ctx, r.db, id)
}

func (r *Repository) GetByOauthAccount(ctx context.Context, provider, accountID string) (*models.User, error) {
	row, err := r.db.QueryOne(ctx, `
		SELECT u.* FROM users u
		JOIN oauth_accounts oa ON oa.user_id = u.id
		WHERE oa.provider = $1 AND oa.provider_account_id = $2`, provider, accountID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return userFromRow(row), nil
}

// LinkOauthAccount binds a verified provider account to a user and returns
// the user. When the provider account is already bound to another user the
// second binding is refused (one external identity per person).
func (r *Repository) LinkOauthAccount(ctx context.Context, user *models.User, provider, accountID, email string) error {
	if _, err := r.db.Exec(ctx, `
		INSERT INTO oauth_accounts (user_id, provider, provider_account_id, email)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (provider, provider_account_id) DO NOTHING`,
		user.ID, provider, accountID, email); err != nil {
		return err
	}
	return nil
}

// SetPassword stores the new argon2id hash in password_hash and mirrors it
// into the legacy bcrypt column so the old login path keeps working during
// rollout (the legacy column is dropped in the cutover phase).
func (r *Repository) SetPassword(ctx context.Context, userID int64, argon2Hash, bcryptHash string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET password_hash = $1, password = $2, updated_at = NOW()
		WHERE id = $3`, argon2Hash, bcryptHash, userID)
	return err
}

// SetStatus changes the account state and stamps updated_at. The service is
// responsible for the matching Redis revocation markers (instant invalidation
// of live access tokens).
func (r *Repository) SetStatus(ctx context.Context, userID int64, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, userID)
	return err
}

// ClaimOtpRow atomically consumes one otps row: a single UPDATE ... WHERE
// is_used = 0 AND expires_at > nowRFC3339 RETURNING id means a replayed or
// racing code gets "no rows" instead of a second use. The otpHash is the
// SHA-256 hex digest the service computed; plaintext codes are never in the
// table for new-module rows. nowRFC3339 is the UTC "YYYY-MM-DDTHH:MM:SSZ"
// cutoff — the otps.expires_at column is TEXT in that format, so the
// comparison stays lexical (same convention the legacy handlers use).
func (r *Repository) ClaimOtpRow(ctx context.Context, email, otpHash, otpType, nowRFC3339 string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
		UPDATE otps SET is_used = 1
		WHERE email = $1 AND otp_code = $2 AND otp_type = $3 AND is_used = 0 AND expires_at > $4
		RETURNING id`,
		strings.ToLower(strings.TrimSpace(email)), otpHash, otpType, nowRFC3339).Scan(&id)
	if errors.Is(err, database.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ConsentGranted reports whether patientUser granted granteeUser the given
// scope, i.e. there is a live (granted, not revoked) row. Doctor/provider
// reads of a patient's records MUST pass through this predicate.
func (r *Repository) ConsentGranted(ctx context.Context, patientUserID, granteeUserID int64, scope string) (bool, error) {
	var n int
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM patient_consents
		WHERE patient_user_id = $1 AND grantee_user_id = $2
		  AND scope = $3 AND status = 'granted' AND revoked_at IS NULL`,
		patientUserID, granteeUserID, scope).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// CanReadRecords is the object-level rule every records read applies:
// a user always sees their own rows; anyone else sees them only with a
// live consent grant for that scope. This is the IDOR guard (S10).
func (r *Repository) CanReadRecords(ctx context.Context, reader, patient int64, scope string) (bool, error) {
	if reader == patient {
		return true, nil
	}
	return r.ConsentGranted(ctx, patient, reader, scope)
}

// userFromRow converts a raw `SELECT * FROM users` row into a models.User,
// mirroring the hands helper but reading the new role/status columns.
func userFromRow(row map[string]any) *models.User {
	if row == nil {
		return nil
	}
	toInt := func(v any, def int) int {
		switch n := v.(type) {
		case int32:
			return int(n)
		case int64:
			return int(n)
		case int:
			return n
		default:
			return def
		}
	}
	toString := func(v any) string {
		if s, ok := v.(string); ok {
			return s
		}
		return ""
	}
	u := &models.User{
		ID:              int64(toInt(row["id"], 0)),
		Username:        toString(row["username"]),
		Email:           toString(row["email"]),
		Password:        toString(row["password"]),
		FirstName:       toString(row["first_name"]),
		LastName:        toString(row["last_name"]),
		UserType:        toString(row["user_type"]),
		IsApproved:      toInt(row["is_approved"], 0),
		IsEmailVerified: toInt(row["is_email_verified"], 0),
		IsSuperuser:     toInt(row["is_superuser"], 0),
		IsActive:        toInt(row["is_active"], 1),
		Role:            toString(row["role"]),
		Status:          toString(row["status"]),
		PasswordHash:    toString(row["password_hash"]),
	}
	if u.Role == "" {
		u.Role = "patient"
	}
	if u.Status == "" {
		u.Status = "active"
	}
	return u
}

// GrantConsent writes a live consent row (idempotent per pair+scope).
func (r *Repository) GrantConsent(ctx context.Context, patient, grantee int64, scope string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO patient_consents (patient_user_id, grantee_user_id, scope)
		VALUES ($1, $2, $3)
		ON CONFLICT (patient_user_id, grantee_user_id, scope)
		DO UPDATE SET status = 'granted', revoked_at = NULL`,
		patient, grantee, scope)
	return err
}

// RevokeConsent marks the consent row revoked (granted_at stays for audit).
func (r *Repository) RevokeConsent(ctx context.Context, patient, grantee int64, scope string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE patient_consents SET status = 'revoked', revoked_at = NOW()
		WHERE patient_user_id = $1 AND grantee_user_id = $2 AND scope = $3 AND status = 'granted'`,
		patient, grantee, scope)
	return err
}

// RecordAudit appends an access/decision trail. details carries no PII:
// caller passes only the resource kind/id and a short reason.
func (r *Repository) RecordAudit(ctx context.Context, userID int64, action, resourceType, resourceID, ip, details string) {
	// Audit is best-effort: a lost log line must never break the request.
	_, _ = r.db.Exec(ctx, `
		INSERT INTO audit_logs (user_id, action, resource_type, resource_id, ip_address, details)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		userID, action, resourceType, resourceID, ip, details)
}
