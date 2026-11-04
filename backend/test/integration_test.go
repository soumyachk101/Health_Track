//go:build integration

// Package test holds the Postgres-backed integration suite for the
// healthtrack backend. Run with `go test -tags integration ./test/...`
// and INTEGRATION_DSN (or DATABASE_URL) pointing at a fresh database;
// database.Connect bootstraps the embedded schema on connect.
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/hands"
	"healthtrack/internal/services/email"
)

var (
	testDB *database.DB
	// Identifies this run: identifiers are unique per run, and the old
	// it_* users get wiped by newServerIsolated on re-runs.
	stamp = time.Now().UnixNano()
)

// TestMain connects to INTEGRATION_DSN (falling back to DATABASE_URL) when a
// DSN is available; otherwise testDB stays nil and every test self-skips via
// skipIfNoDB.
func TestMain(m *testing.M) {
	// The JWT middleware verifies tokens with the JWT_SECRET env var (or its
	// built-in default), NOT with cfg.JWTSecret that generateToken signs
	// with. Align the two so the tokens the tests assert on are accepted.
	if os.Getenv("JWT_SECRET") == "" {
		os.Setenv("JWT_SECRET", "it-secret")
	}
	dsn := os.Getenv("INTEGRATION_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn != "" {
		db, err := database.Connect(context.Background(), dsn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "integration: connect to DSN failed: %v\n", err)
			os.Exit(1)
		}
		testDB = db
		code := m.Run()
		db.Close()
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// newServer builds the production router over the shared DB with a
// deterministic config (no AI keys, no SMTP user => dev-fallback paths).
func newServer(db *database.DB, t *testing.T) *srv {
	t.Helper()
	cfg := &config.Config{
		JWTSecret:      "it-secret",
		FrontendURL:    "http://fe.local",
		EmailUser:      "",
		NodeEnv:        "test",
		GroqKey:        "",
		Key:            "",
		SarvamKey:      "",
		GoogleClientID: "",
		EmailHost:      "",
		EmailPort:      587,
		Port:           "",
	}
	engine := hands.BuildRouter(db, cfg, email.New(cfg))
	return &srv{httptest.NewServer(engine)}
}

// newServerIsolated is newServer plus the suite-level cleanup, so tests that
// create users can re-run against the same database.
func newServerIsolated(t *testing.T) *srv {
	t.Helper()
	cleanupTestUsers(t)
	return newServer(testDB, t)
}

// srv wraps httptest.Server so the suite can attach HTTP helper methods to a
// local type (Go forbids defining methods on the non-local *httptest.Server).
type srv struct {
	*httptest.Server
}

func cleanupTestUsers(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if _, err := testDB.Exec(ctx, "DELETE FROM service_providers WHERE user_id IN (SELECT id FROM users WHERE username LIKE 'it_%')"); err != nil {
		t.Fatalf("cleanup service_providers: %v", err)
	}
	if _, err := testDB.Exec(ctx, "DELETE FROM users WHERE username LIKE 'it_%'"); err != nil {
		t.Fatalf("cleanup users: %v", err)
	}
}

func (s *srv) postJSON(t *testing.T, path, token string, body map[string]any) (int, map[string]any) {
	t.Helper()
	return s.doJSON(t, http.MethodPost, path, token, body)
}

func (s *srv) getJSON(t *testing.T, path, token string) (int, map[string]any) {
	t.Helper()
	return s.doJSON(t, http.MethodGet, path, token, nil)
}

func (s *srv) doJSON(t *testing.T, method, path, token string, body map[string]any) (int, map[string]any) {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, s.URL+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("%s %s: expected JSON response, got %d: %v", method, path, resp.StatusCode, err)
	}
	return resp.StatusCode, out
}

// registerPatient registers a patient via the API, verifies the users row
// exists unverified, marks it email-verified directly in the DB and returns
// a login token.
func registerPatient(s *srv, t *testing.T, username, emailAddr string) string {
	t.Helper()
	ctx := context.Background()
	code, body := s.postJSON(t, "/accounts/api/register", "", map[string]any{
		"username":   username,
		"email":      emailAddr,
		"password":   "Passw0rd!",
		"first_name": "It",
		"last_name":  "User",
		"role":       "patient",
	})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("register: got %d %v", code, body)
	}
	for _, k := range []string{"success", "email", "username", "message"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("register response missing key %q: %v", k, body)
		}
	}
	row, err := testDB.QueryOne(ctx, "SELECT * FROM users WHERE email = $1", emailAddr)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}
	if row == nil {
		t.Fatalf("user %s not created", emailAddr)
	}
	if got := asAnyInt(row["is_email_verified"]); got != 0 {
		t.Fatalf("new user is_email_verified = %v, want 0", got)
	}
	if _, err := testDB.Exec(ctx, "UPDATE users SET is_email_verified = 1 WHERE email = $1", emailAddr); err != nil {
		t.Fatalf("mark verified: %v", err)
	}
	lc, lb := s.postJSON(t, "/accounts/api/login", "", map[string]any{"username": username, "password": "Passw0rd!"})
	if lc != http.StatusOK || lb["success"] != true {
		t.Fatalf("login after verify: got %d %v", lc, lb)
	}
	token, _ := lb["token"].(string)
	if token == "" {
		t.Fatalf("login: no token in %v", lb)
	}
	return token
}

func asAnyInt(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case float64: // JSON-decoded numbers
		return int64(n)
	}
	return -1
}

// skipIfNoDB skips a test when no DSN was available (testDB is nil), so a
// run without INTEGRATION_DSN reports skips rather than panics on nil.
func skipIfNoDB(t *testing.T) {
	t.Helper()
	if testDB == nil {
		t.Skip("INTEGRATION_DSN not set; start a throwaway Postgres (see test/README.md) and re-run")
	}
}

func TestHealthAndNoRoute(t *testing.T) {
	skipIfNoDB(t)
	s := newServer(testDB, t)
	defer s.Close()

	code, body := s.getJSON(t, "/api/health", "")
	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("GET /api/health: got %d %v", code, body)
	}
	code, body = s.getJSON(t, "/", "")
	if code != http.StatusOK || body["message"] != "HealthTrack+ API Server is Running" {
		t.Fatalf("GET /: got %d %v", code, body)
	}
	// NoRoute answers GET with the JSON message for any unknown path.
	code, body = s.getJSON(t, "/no/such/route", "")
	if code != http.StatusOK || body["message"] != "HealthTrack+ API Server" {
		t.Fatalf("GET /no/such/route: got %d %v", code, body)
	}
	// NoRoute is registered for all methods, so an unknown POST also lands
	// on the JSON NoRoute handler (200), not gin's default 404.
	code, body = s.postJSON(t, "/no/such/route", "", map[string]any{})
	if code != http.StatusOK || body["message"] != "HealthTrack+ API Server" {
		t.Fatalf("POST /no/such/route: got %d %v, want 200 NoRoute JSON", code, body)
	}
}

func TestCORS(t *testing.T) {
	skipIfNoDB(t)
	s := newServer(testDB, t)
	defer s.Close()

	do := func(origin, method, path string) (*http.Response, error) {
		req, err := http.NewRequest(method, s.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		return http.DefaultClient.Do(req)
	}

	resp, err := do("https://app.vercel.app", http.MethodOptions, "/accounts/api/login")
	if err != nil {
		t.Fatalf("OPTIONS allowed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS allowed origin: got %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://app.vercel.app" {
		t.Fatalf("OPTIONS allowed origin: Allow-Origin = %q, want echo", got)
	}

	resp, err = do("https://evil.example", http.MethodOptions, "/accounts/api/login")
	if err != nil {
		t.Fatalf("OPTIONS disallowed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("OPTIONS disallowed origin: got %d, want 403", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("OPTIONS disallowed origin: Allow-Origin = %q, want no header", got)
	}
}

func TestTrailingSlash(t *testing.T) {
	skipIfNoDB(t)
	s := newServer(testDB, t)
	defer s.Close()
	code, body := s.getJSON(t, "/api/health/", "")
	if code != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("GET /api/health/: got %d %v", code, body)
	}
}

func TestRegisterLoginProtectedFlow(t *testing.T) {
	skipIfNoDB(t)
	s := newServerIsolated(t)
	defer s.Close()
	ctx := context.Background()

	emailAddr := fmt.Sprintf("it_user_%d@x.test", stamp)
	code, body := s.postJSON(t, "/accounts/api/register", "", map[string]any{
		"username":   "it_user_" + fmt.Sprint(stamp),
		"email":      emailAddr,
		"password":   "Passw0rd!",
		"first_name": "It",
		"last_name":  "User",
		"role":       "patient",
	})
	if code != http.StatusOK {
		t.Fatalf("register: got %d %v", code, body)
	}
	for _, k := range []string{"success", "email", "username", "message"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("register response missing key %q: %v", k, body)
		}
	}
	if body["success"] != true || body["email"] != emailAddr {
		t.Fatalf("register: %v", body)
	}
	row, err := testDB.QueryOne(ctx, "SELECT * FROM users WHERE email = $1", emailAddr)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}
	if row == nil {
		t.Fatalf("users row not created for %s", emailAddr)
	}
	if got := asAnyInt(row["is_email_verified"]); got != 0 {
		t.Fatalf("is_email_verified = %v, want 0", got)
	}

	if _, err := testDB.Exec(ctx, "UPDATE users SET is_email_verified = 1 WHERE email = $1", emailAddr); err != nil {
		t.Fatalf("mark verified: %v", err)
	}

	lc, lb := s.postJSON(t, "/accounts/api/login", "", map[string]any{
		"username": "it_user_" + fmt.Sprint(stamp),
		"password": "Passw0rd!",
	})
	if lc != http.StatusOK || lb["success"] != true {
		t.Fatalf("login: got %d %v", lc, lb)
	}
	token, _ := lb["token"].(string)
	if token == "" {
		t.Fatalf("login: no token")
	}
	role := lb["user"].(map[string]any)["role"]
	if role != "patient" {
		t.Fatalf("login role = %v, want patient", role)
	}

	code, body = s.getJSON(t, "/api/profile", token)
	if code != http.StatusOK {
		t.Fatalf("profile: got %d %v", code, body)
	}
	if _, ok := body["user"].(map[string]any); !ok {
		t.Fatalf("profile missing user object: %v", body)
	}
	if _, ok := body["messages"].([]any); !ok {
		t.Fatalf("profile missing messages array: %v", body)
	}

	code, body = s.getJSON(t, "/api/dashboard", token)
	if code != http.StatusOK {
		t.Fatalf("dashboard: got %d %v", code, body)
	}
	for _, k := range []string{"user", "latest_record", "active_medicines", "recent_activities", "latest_mental_health"} {
		if _, ok := body[k]; !ok {
			t.Fatalf("dashboard missing key %q: %v", k, body)
		}
	}

	// Negative: no token / tampered token.
	code, body = s.getJSON(t, "/api/dashboard", "")
	if code != http.StatusUnauthorized {
		t.Fatalf("dashboard without token: got %d %v", code, body)
	}
	if body["error"] != "Authorization header missing or invalid" {
		t.Fatalf("dashboard without token error = %v", body["error"])
	}
	tampered := token[:len(token)-1] + "x"
	code, body = s.getJSON(t, "/api/dashboard", tampered)
	if code != http.StatusUnauthorized {
		t.Fatalf("dashboard tampered token: got %d %v", code, body)
	}

	// Un-verified path: a fresh (distinct-identifier) user that is not marked
	// verified must get 403 requires_verification on login.
	unvEmail := fmt.Sprintf("it_unverified_%d@x.test", stamp)
	code, body = s.postJSON(t, "/accounts/api/register", "", map[string]any{
		"username": "it_unverified_" + fmt.Sprint(stamp),
		"email":    unvEmail,
		"password": "Passw0rd!",
		"role":     "patient",
	})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("register unverified user: got %d %v", code, body)
	}
	code, body = s.postJSON(t, "/accounts/api/login", "", map[string]any{
		"username": "it_unverified_" + fmt.Sprint(stamp),
		"password": "Passw0rd!",
	})
	if code != http.StatusForbidden {
		t.Fatalf("unverified login: got %d %v", code, body)
	}
	if body["requires_verification"] != true || body["error"] != "Email not verified" {
		t.Fatalf("unverified login: %v", body)
	}
}

func TestProtectedCoreState(t *testing.T) {
	skipIfNoDB(t)
	s := newServerIsolated(t)
	defer s.Close()
	ctx := context.Background()

	token := registerPatient(s, t, fmt.Sprintf("it_core_%d", stamp), fmt.Sprintf("it_core_%d@x.test", stamp))

	code, body := s.postJSON(t, "/api/health-track/add", token, map[string]any{
		"blood_pressure_systolic":  120,
		"blood_pressure_diastolic": 80,
		"weight":                   70.5,
		"notes":                    "ok",
	})
	if code != http.StatusOK {
		t.Fatalf("health-track/add: got %d %v", code, body)
	}
	if body["message"] != "Health record added successfully" {
		t.Fatalf("health-track/add message: %v", body["message"])
	}

	code, body = s.getJSON(t, "/api/health-track", token)
	if code != http.StatusOK {
		t.Fatalf("health-track: got %d %v", code, body)
	}
	records, ok := body["records"].([]any)
	if !ok || len(records) == 0 {
		t.Fatalf("health-track records: %v", body)
	}
	first := records[0].(map[string]any)
	if got := first["bp_status"]; got != "High (Stage 1)" {
		t.Fatalf("bp_status = %v, want High (Stage 1) for 120/80", got)
	}

	code, body = s.postJSON(t, "/api/medicines/add", token, map[string]any{
		"name":       "A",
		"dosage":     "1",
		"frequency":  "twice",
		"start_date": "2026-01-01",
	})
	if code != http.StatusOK {
		t.Fatalf("medicines/add: got %d %v", code, body)
	}
	code, body = s.getJSON(t, "/api/medicines", token)
	if code != http.StatusOK {
		t.Fatalf("medicines: got %d %v", code, body)
	}
	if got := asAnyInt(body["active_count"]); got != 1 {
		t.Fatalf("active_count = %v, want 1", body["active_count"])
	}
	meds := body["medicines"].([]any)
	if len(meds) != 1 {
		t.Fatalf("medicines len = %d, want 1: %v", len(meds), meds)
	}
	if got := meds[0].(map[string]any)["frequency_display"]; got != "Twice Daily" {
		t.Fatalf("frequency_display = %v", got)
	}

	for path, keys := range map[string][]string{
		"/api/past-records":   {"health_records", "prescriptions"},
		"/api/mental-health":  {"avg_mood", "logs"},
		"/api/lifestyle":      {"logs"},
		"/api/insurance":      {"policies", "active_policies"},
	} {
		code, body = s.getJSON(t, path, token)
		if code != http.StatusOK {
			t.Fatalf("%s: got %d %v", path, code, body)
		}
		for _, k := range keys {
			if _, ok := body[k]; !ok {
				t.Fatalf("%s missing key %q: %v", path, k, body)
			}
		}
	}

	// Negative: a non-doctor provider (pharmacy) cannot list appointments.
	pharmEmail := fmt.Sprintf("it_pharm_%d@x.test", stamp)
	pharmName := "it_pharm_" + fmt.Sprint(stamp)
	code, body = s.postJSON(t, "/accounts/api/register", "", map[string]any{
		"username":      pharmName,
		"email":         pharmEmail,
		"password":      "Passw0rd!",
		"role":          "provider",
		"provider_type": "pharmacy",
		"business_name": "ItPharm",
	})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("register pharmacy provider: got %d %v", code, body)
	}
	if _, err := testDB.Exec(ctx, "UPDATE users SET is_email_verified = 1 WHERE email = $1", pharmEmail); err != nil {
		t.Fatalf("mark pharm verified: %v", err)
	}
	lc, lb := s.postJSON(t, "/accounts/api/login", "", map[string]any{"username": pharmName, "password": "Passw0rd!"})
	if lc != http.StatusOK || lb["success"] != true {
		t.Fatalf("pharmacy login: got %d %v", lc, lb)
	}
	pharmToken, _ := lb["token"].(string)
	if pharmToken == "" {
		t.Fatalf("pharmacy login: no token")
	}
	code, body = s.getJSON(t, "/api/appointments", pharmToken)
	if code != http.StatusForbidden {
		t.Fatalf("appointments as pharmacy: got %d %v", code, body)
	}
	if body["error"] != "Not a doctor" {
		t.Fatalf("appointments as pharmacy error = %v", body["error"])
	}
}

func TestAdminGatingAndState(t *testing.T) {
	skipIfNoDB(t)
	s := newServerIsolated(t)
	defer s.Close()
	ctx := context.Background()

	hashed, err := bcrypt.GenerateFromPassword([]byte("ItAdmin1!"), 10)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	adminEmail := fmt.Sprintf("it_admin_%d@x.test", stamp)
	if _, err := testDB.Exec(ctx,
		`INSERT INTO users (username, email, password, user_type, is_approved, is_email_verified, is_superuser)
		 VALUES ($1, $2, $3, 'admin', 1, 1, 1)`,
		"it_admin_"+fmt.Sprint(stamp), adminEmail, string(hashed)); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	lc, lb := s.postJSON(t, "/accounts/api/login", "", map[string]any{
		"username": "it_admin_" + fmt.Sprint(stamp),
		"password": "ItAdmin1!",
	})
	if lc != http.StatusOK || lb["success"] != true {
		t.Fatalf("admin login: got %d %v", lc, lb)
	}
	adminToken, _ := lb["token"].(string)
	if adminToken == "" {
		t.Fatalf("admin login: no token")
	}
	if role := lb["user"].(map[string]any)["role"]; role != "admin" {
		t.Fatalf("admin role = %v, want admin", role)
	}

	code, body := s.getJSON(t, "/admin-panel/api/stats", adminToken)
	if code != http.StatusOK {
		t.Fatalf("admin stats: got %d %v", code, body)
	}
	stats, ok := body["stats"].(map[string]any)
	if !ok {
		t.Fatalf("admin stats missing stats object: %v", body)
	}
	for _, k := range []string{"total_users", "patients", "providers", "pending_approvals", "total_records"} {
		if _, ok := stats[k]; !ok {
			t.Fatalf("admin stats missing key %q: %v", k, stats)
		}
	}

	code, body = s.getJSON(t, "/admin-panel/api/users", adminToken)
	if code != http.StatusOK {
		t.Fatalf("admin users: got %d %v", code, body)
	}
	users := body["users"].([]any)
	if len(users) == 0 {
		t.Fatalf("admin users: empty list")
	}
	for _, u := range users {
		m := u.(map[string]any)
		for _, k := range []string{"id", "username", "email", "user_type", "is_approved", "date_joined"} {
			if _, ok := m[k]; !ok {
				t.Fatalf("admin user missing key %q: %v", k, m)
			}
		}
	}

	// Provision a pending provider directly (distinct from any other test's
	// data, since newServerIsolated cleans it up) so the action endpoints
	// have their own target.
	provEmail := fmt.Sprintf("it_provider_%d@x.test", stamp)
	// db.Run appends " RETURNING id" to bare INSERTs; returning it as the
	// new user id.
	provID, err := testDB.Run(ctx,
		`INSERT INTO users (username, email, password, user_type, is_approved, is_email_verified)
		 VALUES ($1, $2, $3, 'provider', 0, 1)`,
		"it_provider_"+fmt.Sprint(stamp), provEmail, string(hashed))
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if _, err := testDB.Exec(ctx,
		"INSERT INTO service_providers (user_id, provider_type, business_name) VALUES ($1, 'pharmacy', 'ItProv')",
		provID); err != nil {
		t.Fatalf("create service_providers: %v", err)
	}

	code, body = s.postJSON(t, fmt.Sprintf("/admin-panel/api/users/%d/action", provID), adminToken, map[string]any{"action": "reject"})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("admin reject: got %d %v", code, body)
	}
	row, err := testDB.QueryOne(ctx, "SELECT * FROM users WHERE id = $1", provID)
	if err != nil || row == nil {
		t.Fatalf("query user %d: %v", provID, err)
	}
	if got := asAnyInt(row["is_approved"]); got != 0 {
		t.Fatalf("after reject is_approved = %v, want 0", got)
	}
	code, body = s.postJSON(t, fmt.Sprintf("/admin-panel/api/users/%d/action", provID), adminToken, map[string]any{"action": "approve"})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("admin approve: got %d %v", code, body)
	}
	row, err = testDB.QueryOne(ctx, "SELECT * FROM users WHERE id = $1", provID)
	if err != nil || row == nil {
		t.Fatalf("query user after approve: %v", err)
	}
	if got := asAnyInt(row["is_approved"]); got != 1 {
		t.Fatalf("after approve is_approved = %v, want 1", got)
	}

	// Gating: a patient token must be rejected with 403 on admin routes.
	patientToken := registerPatient(s, t, fmt.Sprintf("it_gate_%d", stamp), fmt.Sprintf("it_gate_%d@x.test", stamp))
	code, body = s.getJSON(t, "/admin-panel/api/stats", patientToken)
	if code != http.StatusForbidden {
		t.Fatalf("admin stats as patient: got %d %v", code, body)
	}
	if body["success"] != false || body["error"] != "Admin access required" {
		t.Fatalf("admin stats as patient body: %v", body)
	}
}

func TestOTPFlow(t *testing.T) {
	skipIfNoDB(t)
	s := newServerIsolated(t)
	defer s.Close()
	ctx := context.Background()

	emailAddr := fmt.Sprintf("it_reset_%d@x.test", stamp)
	username := fmt.Sprintf("it_reset_%d", stamp)
	// Pre-reset token is intentionally unused; the post-reset logins below
	// prove the password actually changed.
	_ = registerPatient(s, t, username, emailAddr)

	code, body := s.postJSON(t, "/accounts/api/forgot-password", "", map[string]any{"email": emailAddr})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("forgot-password: got %d %v", code, body)
	}
	otpRow, err := testDB.QueryOne(ctx,
		`SELECT * FROM otps WHERE email = $1 AND otp_type = 'password_reset' ORDER BY id DESC LIMIT 1`, emailAddr)
	if err != nil {
		t.Fatalf("query otps: %v", err)
	}
	if otpRow == nil {
		t.Fatalf("no password_reset otp row for %s", emailAddr)
	}
	otpCode, _ := otpRow["otp_code"].(string)
	if otpCode == "" {
		t.Fatalf("otp row missing otp_code: %v", otpRow)
	}

	// Negative first: a wrong otp is rejected with 400 (distinct-identifier
	// counter: this is the only recorded failure for this email).
	code, body = s.postJSON(t, "/accounts/api/reset-password", "", map[string]any{
		"email":        emailAddr,
		"otp":          "000000",
		"new_password": "NewPass1!",
	})
	if code != http.StatusBadRequest {
		t.Fatalf("reset with wrong otp: got %d %v", code, body)
	}
	if body["error"] != "Invalid or expired reset link. Please request a new one." {
		t.Fatalf("wrong otp error = %v", body["error"])
	}

	code, body = s.postJSON(t, "/accounts/api/reset-password", "", map[string]any{
		"email":        emailAddr,
		"otp":          otpCode,
		"new_password": "NewPass1!",
	})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("reset-password: got %d %v", code, body)
	}
	if body["message"] != "Password updated successfully" {
		t.Fatalf("reset-password message: %v", body["message"])
	}

	code, body = s.postJSON(t, "/accounts/api/login", "", map[string]any{"username": emailAddr, "password": "NewPass1!"})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("login with new password: got %d %v", code, body)
	}
	// One recorded failure for this identifier (below the 5/5min cap).
	code, body = s.postJSON(t, "/accounts/api/login", "", map[string]any{"username": emailAddr, "password": "Passw0rd!"})
	if code != http.StatusUnauthorized || body["success"] != false {
		t.Fatalf("login with old password: got %d %v, want 401", code, body)
	}
}

func TestChatbotConfigErrors(t *testing.T) {
	s := newServer(testDB, t)
	defer s.Close()

	code, body := s.postJSON(t, "/chatbot/api", "", map[string]any{"message": "hi"})
	if code != http.StatusServiceUnavailable {
		t.Fatalf("chatbot no keys: got %d %v", code, body)
	}
	if body["error"] != "Server configuration error: API key missing" {
		t.Fatalf("chatbot no keys error = %v", body["error"])
	}

	code, body = s.postJSON(t, "/chatbot/api", "", map[string]any{})
	if code != http.StatusBadRequest || body["error"] != "No message provided" {
		t.Fatalf("chatbot empty message: got %d %v", code, body)
	}

	code, body = s.postJSON(t, "/chatbot/api/tts", "", map[string]any{"text": "hi"})
	if code != http.StatusServiceUnavailable {
		t.Fatalf("tts no key: got %d %v", code, body)
	}
	if body["error"] != "TTS configuration error: API key missing" {
		t.Fatalf("tts no key error = %v", body["error"])
	}

	code, body = s.postJSON(t, "/chatbot/api/tts", "", map[string]any{"text": ""})
	if code != http.StatusBadRequest || body["error"] != "No text provided" {
		t.Fatalf("tts empty text: got %d %v", code, body)
	}
}

func TestGoogleLoginShape(t *testing.T) {
	s := newServer(testDB, t)
	defer s.Close()

	code, body := s.postJSON(t, "/accounts/api/google-login", "", map[string]any{})
	if code != http.StatusBadRequest || body["error"] != "Google credential is required" {
		t.Fatalf("google-login missing credential: got %d %v", code, body)
	}

	// cfg has an empty GoogleClientID, so any credential fails verification
	// before any network call.
	code, body = s.postJSON(t, "/accounts/api/google-login", "", map[string]any{"credential": "garbage"})
	if code != http.StatusBadRequest || body["error"] != "Google authentication failed" {
		t.Fatalf("google-login garbage credential: got %d %v", code, body)
	}
}
