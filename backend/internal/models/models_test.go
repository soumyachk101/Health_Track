package models

import (
	"context"
	"testing"

	"healthtrack/internal/database"
)

// TestDeriveRoleNoDBPath covers the pure fast paths of DeriveRole, which never
// touch the DB (db may be nil) and are therefore safe to unit-test:
// IsSuperuser==1 or UserType=="admin" -> "admin", UserType=="doctor" -> "doctor".
// The patient/provider branch runs the service_providers lookup against the DB,
// so it is intentionally not exercised here - integration tests cover it.
func TestDeriveRoleNoDBPath(t *testing.T) {
	var nilDB *database.DB // the fast paths never dereference db
	cases := []struct {
		name string
		u    User
		want string
	}{
		{"superuser wins over patient",
			User{IsSuperuser: 1, UserType: "patient"}, "admin"},
		{"admin user type",
			User{UserType: "admin", IsSuperuser: 0}, "admin"},
		{"doctor user type",
			User{UserType: "doctor"}, "doctor"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DeriveRole(context.Background(), nilDB, &c.u)
			if got != c.want {
				t.Errorf("DeriveRole(%+v) = %q, want %q", c.u, got, c.want)
			}
		})
	}
}
