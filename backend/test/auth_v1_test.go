//go:build integration

// Package test — auth_v1_test.go holds the Postgres-backed regressions for
// the Pure Go auth module (ADR-0001) that the DB-free unit suite cannot
// cover: central account-state blocking (S1), the object-level consent
// predicate for cross-user reads (S10), and atomic single-use OTP claim
// under replay + concurrency (S4). All tests skip when no DSN is set.
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"healthtrack/internal/config"
	"healthtrack/internal/modules/auth"
	"healthtrack/internal/platform/cache"
)

// testAuthService builds the module service over the shared test pool with a
// degrade-safe in-memory cache (single-process test: no cross-instance
// replay needed; REDIS_URL unset -> cache.New returns a MemCache).
func testAuthService(t *testing.T) *auth.Service {
	t.Helper()
	c, err := cache.New("")
	if err != nil {
		t.Fatalf("cache: %v", err)
	}
	return auth.NewService(&config.Config{JWTSecret: "it-secret"}, auth.NewRepository(testDB), c)
}

// moduleLogin runs POST /api/v1/auth/login and returns the Set-Cookie
// values (ht_access + ht_refresh) to attach to subsequent requests.
// Requires the server's config to have AuthCookieEnabled (newServer sets it).
func moduleLogin(t *testing.T, s *srv, username, password string) []string {
	t.Helper()
	body := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	req, _ := http.NewRequest(http.MethodPost, s.URL+"/api/v1/auth/login", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("module login: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		t.Fatalf("module login: got %d %v", resp.StatusCode, out)
	}
	var cookies []string
	for _, sc := range resp.Header.Values("Set-Cookie") {
		cookies = append(cookies, firstCookieNameValue(sc))
	}
	if len(cookies) < 2 {
		t.Fatalf("module login: expected 2 Set-Cookie (ht_access + ht_refresh), got %d", len(cookies))
	}
	return cookies
}

// getAuthed does a GET with the module cookies and returns (status, body).
func getAuthed(t *testing.T, s *srv, path string, cookies []string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, s.URL+path, nil)
	for _, ck := range cookies {
		req.Header.Add("Cookie", ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// TestAuthV1AccountStateGating (S1): the central AuthRequired gate rejects a
// suspended or pending account even while its 15m access JWT is still
// technically valid — the account state is enforced per request, not at
// login time.
func TestAuthV1AccountStateGating(t *testing.T) {
	skipIfNoDB(t)
	s := newServerIsolated(t)
	defer s.Close()
	ctx := context.Background()

	username := "it_authgate_" + fmt.Sprint(stamp)
	emailAddr := username + "@x.test"
	registerPatient(s, t, username, emailAddr)
	cookies := moduleLogin(t, s, username, "Passw0rd!")

	// Active: the session gate passes.
	code, body := getAuthed(t, s, "/api/v1/auth/session", cookies)
	if code != http.StatusOK {
		t.Fatalf("active session: got %d %v, want 200", code, body)
	}

	// Suspended: the same (still-valid) token is refused centrally.
	if _, err := testDB.Exec(ctx, "UPDATE users SET status = 'suspended' WHERE username = $1", username); err != nil {
		t.Fatalf("suspend user: %v", err)
	}
	code, body = getAuthed(t, s, "/api/v1/auth/session", cookies)
	if code != http.StatusForbidden {
		t.Fatalf("suspended session: got %d %v, want 403", code, body)
	}
	if body["error"] != "Account suspended" {
		t.Fatalf("suspended session error = %v, want 'Account suspended'", body["error"])
	}

	// Pending: same central gate, different state.
	if _, err := testDB.Exec(ctx, "UPDATE users SET status = 'pending' WHERE username = $1", username); err != nil {
		t.Fatalf("pending user: %v", err)
	}
	code, body = getAuthed(t, s, "/api/v1/auth/session", cookies)
	if code != http.StatusForbidden {
		t.Fatalf("pending session: got %d %v, want 403", code, body)
	}
	if body["error"] != "Account pending approval" {
		t.Fatalf("pending session error = %v", body["error"])
	}
}

// TestAuthV1TwoUserIDOR (S10): two unrelated users — the object-level
// CanReadRecords predicate must deny cross-user reads absent a live consent
// grant, allow self-reads, and track grant/revoke transitions.
func TestAuthV1TwoUserIDOR(t *testing.T) {
	skipIfNoDB(t)
	s := newServerIsolated(t)
	defer s.Close()
	ctx := context.Background()

	registerPatient(s, t, "it_idor_a_"+fmt.Sprint(stamp), "it_idor_a_"+fmt.Sprint(stamp)+"@x.test")
	registerPatient(s, t, "it_idor_b_"+fmt.Sprint(stamp), "it_idor_b_"+fmt.Sprint(stamp)+"@x.test")

	var idA, idB int64
	if err := testDB.QueryRow(ctx, "SELECT id FROM users WHERE username = $1", "it_idor_a_"+fmt.Sprint(stamp)).Scan(&idA); err != nil {
		t.Fatalf("id A: %v", err)
	}
	if err := testDB.QueryRow(ctx, "SELECT id FROM users WHERE username = $1", "it_idor_b_"+fmt.Sprint(stamp)).Scan(&idB); err != nil {
		t.Fatalf("id B: %v", err)
	}

	repo := auth.NewRepository(testDB)

	// Self-read is always allowed.
	if ok, _ := repo.CanReadRecords(ctx, idA, idA, "records"); !ok {
		t.Fatal("self-read denied; want allowed")
	}
	// Cross-user read without consent is denied (the IDOR guard).
	if ok, _ := repo.CanReadRecords(ctx, idB, idA, "records"); ok {
		t.Fatal("cross-user read allowed without consent; the IDOR guard failed")
	}
	// A granted consent opens the read...
	if err := repo.GrantConsent(ctx, idA, idB, "records"); err != nil {
		t.Fatalf("grant consent: %v", err)
	}
	if ok, _ := repo.CanReadRecords(ctx, idB, idA, "records"); !ok {
		t.Fatal("cross-user read denied with live consent; want allowed")
	}
	// ...and a revocation closes it again.
	if err := repo.RevokeConsent(ctx, idA, idB, "records"); err != nil {
		t.Fatalf("revoke consent: %v", err)
	}
	if ok, _ := repo.CanReadRecords(ctx, idB, idA, "records"); ok {
		t.Fatal("cross-user read allowed after consent revocation")
	}
}

// TestAuthV1OtpReplayConcurrency (S4): a password-reset code is single-use
// — a direct replay is refused, and a fan-out of concurrent claims yields
// exactly one winner (the atomic claim row).
func TestAuthV1OtpReplayConcurrency(t *testing.T) {
	skipIfNoDB(t)
	s := newServerIsolated(t)
	defer s.Close()
	ctx := context.Background()

	username := "it_otp_" + fmt.Sprint(stamp)
	emailAddr := username + "@x.test"
	registerPatient(s, t, username, emailAddr)

	svc := testAuthService(t)
	code, err := svc.CreateOtp(ctx, emailAddr, "password_reset")
	if err != nil {
		t.Fatalf("CreateOtp: %v", err)
	}

	// First claim succeeds...
	ok, err := svc.VerifyOtp(ctx, emailAddr, code, "password_reset")
	if err != nil || !ok {
		t.Fatalf("first claim: ok=%v err=%v, want true", ok, err)
	}
	// ...the replay is refused.
	if ok, _ := svc.VerifyOtp(ctx, emailAddr, code, "password_reset"); ok {
		t.Fatal("replayed OTP claimed a second time; single-use violated")
	}

	// Concurrency: a fresh code claimed by 8 goroutines yields exactly one
	// winner (the UPDATE ... WHERE is_used = 0 ... RETURNING claim).
	code2, err := svc.CreateOtp(ctx, emailAddr, "register")
	if err != nil {
		t.Fatalf("CreateOtp #2: %v", err)
	}
	const workers = 8
	results := make(chan bool, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _ := svc.VerifyOtp(ctx, emailAddr, code2, "register")
			results <- ok
		}()
	}
	wg.Wait()
	close(results)
	winners := 0
	for ok := range results {
		if ok {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent claims: %d winners, want exactly 1", winners)
	}
}
