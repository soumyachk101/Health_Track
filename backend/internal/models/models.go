package models

import (
	"context"
	"time"

	"healthtrack/internal/database"
)

type User struct {
	ID                int64
	Username          string
	Email             string
	Password          string
	FirstName         string
	LastName          string
	UserType          string
	Phone             string
	Address           string
	City              string
	DateOfBirth       string
	ProfileImage      string
	EmergencyContact  string
	EmergencyPhone    string
	BloodGroup        string
	VerificationToken string
	IsApproved        int
	IsEmailVerified   int
	IsSuperuser       int
	IsStaff           int
	IsActive          int
	CreatedAt         time.Time
	UpdatedAt         time.Time
	// AuthUUID is the Supabase auth.users.id this row is provisioned from
	// (empty for legacy rows that predate Supabase).
	AuthUUID string
}

const userSelect = `SELECT id, username, email, password,
  COALESCE(first_name, ''),
  COALESCE(last_name, ''),
  COALESCE(user_type, 'patient'),
  COALESCE(phone, ''),
  COALESCE(address, ''),
  COALESCE(city, ''),
  COALESCE(date_of_birth, ''),
  COALESCE(profile_image, ''),
  COALESCE(emergency_contact, ''),
  COALESCE(emergency_phone, ''),
  COALESCE(blood_group, ''),
  COALESCE(verification_token, ''),
  COALESCE(is_approved, 0),
  COALESCE(is_email_verified, 0),
  COALESCE(is_superuser, 0),
  COALESCE(is_staff, 0),
  COALESCE(is_active, 1),
  COALESCE(created_at, NOW()),
  COALESCE(updated_at, NOW()),
  COALESCE(auth_uuid::text, '')
  FROM users WHERE `

// GetUser returns the user with the given id, or (nil, nil) when not found.
func GetUser(ctx context.Context, db *database.DB, id int64) (*User, error) {
	var u User
	err := db.QueryRow(ctx, userSelect+"id = $1", id).Scan(
		&u.ID, &u.Username, &u.Email, &u.Password,
		&u.FirstName, &u.LastName, &u.UserType, &u.Phone, &u.Address, &u.City,
		&u.DateOfBirth, &u.ProfileImage, &u.EmergencyContact, &u.EmergencyPhone,
		&u.BloodGroup, &u.VerificationToken, &u.IsApproved, &u.IsEmailVerified,
		&u.IsSuperuser, &u.IsStaff, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
		&u.AuthUUID,
	)
	if err != nil {
		if err == database.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// GetUserByEmail returns the user with the given email, or (nil, nil) when not found.
func GetUserByEmail(ctx context.Context, db *database.DB, email string) (*User, error) {
	var u User
	err := db.QueryRow(ctx, userSelect+"email = $1", email).Scan(
		&u.ID, &u.Username, &u.Email, &u.Password,
		&u.FirstName, &u.LastName, &u.UserType, &u.Phone, &u.Address, &u.City,
		&u.DateOfBirth, &u.ProfileImage, &u.EmergencyContact, &u.EmergencyPhone,
		&u.BloodGroup, &u.VerificationToken, &u.IsApproved, &u.IsEmailVerified,
		&u.IsSuperuser, &u.IsStaff, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
		&u.AuthUUID,
	)
	if err != nil {
		if err == database.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// GetUserByAuthUUID returns the app row provisioned for the given Supabase
// auth.users.id, or (nil, nil) when there is no such row. Supabase-issued
// tokens carry `sub` = this UUID.
func GetUserByAuthUUID(ctx context.Context, db *database.DB, authUUID string) (*User, error) {
	var u User
	err := db.QueryRow(ctx, userSelect+"auth_uuid = $1", authUUID).Scan(
		&u.ID, &u.Username, &u.Email, &u.Password,
		&u.FirstName, &u.LastName, &u.UserType, &u.Phone, &u.Address, &u.City,
		&u.DateOfBirth, &u.ProfileImage, &u.EmergencyContact, &u.EmergencyPhone,
		&u.BloodGroup, &u.VerificationToken, &u.IsApproved, &u.IsEmailVerified,
		&u.IsSuperuser, &u.IsStaff, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
		&u.AuthUUID,
	)
	if err != nil {
		if err == database.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// DeriveRole resolves the effective role of a user: admin, doctor, provider,
// or patient.
func DeriveRole(ctx context.Context, db *database.DB, u *User) string {
	if u.IsSuperuser == 1 || u.UserType == "admin" {
		return "admin"
	}
	if u.UserType == "doctor" {
		return "doctor"
	}
	var n int
	_ = db.QueryRow(ctx, "SELECT COUNT(*) FROM service_providers WHERE user_id = $1 AND provider_type = 'doctor'", u.ID).Scan(&n)
	if n > 0 {
		return "doctor"
	}
	if u.UserType == "provider" || u.UserType == "doctor" {
		return "provider"
	}
	return "patient"
}
