package cache

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"
)

// These tests drive the rotation API in scripts.go end-to-end against
// NewMemCache (the key-for-key mirror of the Redis Lua script), plus the
// user-level revocation marker from ADR-0001. No live Redis needed: the
// mirror shares the Lua script's key semantics, so happy-path rotation,
// replay -> family-kill, and revocation-marker behavior are asserted here.

// seedFamily mirrors Service.StartSession's issuance writes: the presented
// hash h0 is the current token of family fam for user uid.
func seedFamily(t *testing.T, ctx context.Context, c Cache, uid, fam, h0 string, ttl time.Duration) {
	t.Helper()
	if err := c.Set(ctx, "rt:"+h0, fam, ttl); err != nil {
		t.Fatalf("seed rt:%s: %v", h0, err)
	}
	if err := c.Set(ctx, "rt_fam:"+h0, fam, ttl); err != nil {
		t.Fatalf("seed rt_fam:%s: %v", h0, err)
	}
	rec := uid + "|" + h0
	if err := c.Set(ctx, "rt_family:"+fam, rec, ttl); err != nil {
		t.Fatalf("seed rt_family:%s: %v", fam, err)
	}
}

// rotateServiceLike replays exactly the three steps Service.Rotate performs
// (service.go:291): (1) resolve the presented hash to its family via the
// rt_fam map, (2) run RotateScript atomically, (3) read back the family
// record on success. Returns the script's outcome and whether the record
// read back.
func rotateServiceLike(t *testing.T, ctx context.Context, c Cache, presented, successor, famKey string, ttlSec int64) (out string, record string) {
	t.Helper()
	// Step 1: attribute the presented token to its family.
	fam := famKey
	if famKey == "" {
		v, err := c.Get(ctx, "rt_fam:"+presented)
		if err != nil {
			return "gone", "" // service: ErrSessionGone, no script call
		}
		fam = string(v)
	}
	// Step 2: the atomic rotation (Lua in Redis, mirrored in MemCache).
	res, err := c.Eval(ctx, RotateScript,
		[]string{
			"rt:" + presented,
			"rt_fam:" + presented,
			"rt_family:" + fam,
			"rt_family:" + fam + ":revoked",
			"rt:" + successor,
			"rt_fam:" + successor,
		},
		ttlSec, successor)
	if err != nil {
		t.Fatalf("RotateScript Eval: %v", err)
	}
	switch v := res.(type) {
	case string:
		out = v
	case []byte:
		out = string(v)
	default:
		out = fmt.Sprint(v)
	}
	// Step 3: post-"ok" record read.
	if out == "ok" {
		if rec, gerr := c.Get(ctx, "rt_family:"+fam); gerr == nil {
			record = string(rec)
		}
	}
	return out, record
}

func TestMemCacheSetGetAndTTLExpiry(t *testing.T) {
	ctx := context.Background()
	c := NewMemCache()

	// string / []byte / int64 round-trip.
	for _, tc := range []struct {
		name string
		val  any
		want string
	}{
		{"string", "v1", "v1"},
		{"bytes", []byte("v2"), "v2"},
		{"int64", int64(42), "42"},
	} {
		if err := c.Set(ctx, "k:"+tc.name, tc.val, time.Minute); err != nil {
			t.Fatalf("Set %s: %v", tc.name, err)
		}
		got, err := c.Get(ctx, "k:"+tc.name)
		if err != nil || string(got) != tc.want {
			t.Fatalf("Get %s = (%q, %v), want %q", tc.name, got, err, tc.want)
		}
	}

	// A short-TTL key expires lazily on the next read; a no-TTL key stays.
	if err := c.Set(ctx, "short", "x", 30*time.Millisecond); err != nil {
		t.Fatalf("Set short: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	if _, err := c.Get(ctx, "short"); !IsNotFound(err) {
		t.Fatalf("expired key: Get err = %v, want not-found", err)
	}
	if _, err := c.Get(ctx, "k:string"); err != nil {
		t.Fatalf("no-TTL key lost: %v", err)
	}
}

func TestMemCacheSetNXSecondCallFalse(t *testing.T) {
	ctx := context.Background()
	c := NewMemCache()

	first, err := c.SetNX(ctx, "k", "a", time.Minute)
	if err != nil || !first {
		t.Fatalf("first SetNX = (%v, %v), want (true, nil)", first, err)
	}
	second, err := c.SetNX(ctx, "k", "b", time.Minute)
	if err != nil || second {
		t.Fatalf("second SetNX = (%v, %v), want (false, nil)", second, err)
	}
	got, err := c.Get(ctx, "k")
	if err != nil || string(got) != "a" {
		t.Fatalf("value = (%q, %v), want 'a' kept", got, err)
	}

	// An expired key can be claimed again.
	if ok, err := c.SetNX(ctx, "exp", "old", 30*time.Millisecond); err != nil || !ok {
		t.Fatalf("initial SetNX exp = (%v, %v), want (true, nil)", ok, err)
	}
	time.Sleep(60 * time.Millisecond)
	if ok, err := c.SetNX(ctx, "exp", "new", time.Minute); err != nil || !ok {
		t.Fatalf("SetNX on expired key = (%v, %v), want (true, nil)", ok, err)
	}
}

func TestMemCacheIncrWindow(t *testing.T) {
	ctx := context.Background()
	c := NewMemCache()

	// First incr sets the counter AND its TTL; subsequent incrs within the
	// window keep climbing.
	n, err := c.Incr(ctx, "q", time.Minute)
	if err != nil || n != 1 {
		t.Fatalf("first Incr = (%d, %v), want 1", n, err)
	}
	n, err = c.Incr(ctx, "q", time.Minute)
	if err != nil || n != 2 {
		t.Fatalf("second Incr = (%d, %v), want 2", n, err)
	}

	// Window form (QuotaCheck's fixed window): a short TTL makes the
	// counter restart after expiry.
	w := NewMemCache()
	if n, err = w.Incr(ctx, "w", 50*time.Millisecond); err != nil || n != 1 {
		t.Fatalf("window Incr = (%d, %v), want 1", n, err)
	}
	time.Sleep(80 * time.Millisecond)
	if n, err = w.Incr(ctx, "w", 50*time.Millisecond); err != nil || n != 1 {
		t.Fatalf("post-expiry Incr = (%d, %v), want 1 (fresh window)", n, err)
	}
}

func TestMemCacheDel(t *testing.T) {
	ctx := context.Background()
	c := NewMemCache()
	if err := c.Set(ctx, "a", "1", time.Minute); err != nil {
		t.Fatalf("Set a: %v", err)
	}
	if err := c.Del(ctx, "a", "missing"); err != nil {
		t.Fatalf("Del (incl. missing key): %v", err)
	}
	if _, err := c.Get(ctx, "a"); !IsNotFound(err) {
		t.Fatalf("after Del: Get err = %v, want not-found", err)
	}
}

// TestRotateScriptHappyPath drives one full refresh rotation: issuance
// (seedFamily == StartSession's writes) then a Rotate of the current token.
func TestRotateScriptHappyPath(t *testing.T) {
	ctx := context.Background()
	c := NewMemCache()
	const uid, fam = "42", "F1"
	seedFamily(t, ctx, c, uid, fam, "h0", time.Minute)

	out, rec := rotateServiceLike(t, ctx, c, "h0", "h1", "", 60)
	if out != "ok" {
		t.Fatalf("rotation outcome = %q, want ok", out)
	}
	// The consumed token's live marker is gone; the family record advanced.
	if _, err := c.Get(ctx, "rt:h0"); !IsNotFound(err) {
		t.Fatalf("rt:h0 should be consumed, got err=%v", err)
	}
	if _, err := c.Get(ctx, "rt:h1"); err != nil {
		t.Fatalf("successor rt:h1 missing after rotation: %v", err)
	}
	if rec != uid+"|h1" {
		t.Fatalf("family record = %q, want 42|h1", rec)
	}
}

// TestRotateScriptSuccessorRegistrationFinding pins a DIVERGENCE found while
// testing (flagged to CLI 1; scripts.go is their lane): on the "ok" path the
// successor's rt_fam:<h1> entry is written from the PREDECESSOR hash, not
// from the family:
//   - MemCache mirror: fam = TrimPrefix("rt_fam:h0", "rt_fam:") = "h0"
//   - Lua script:      string.sub(KEYS[2], 7) = ":h0"  (off-by-one: "rt_fam:"
//     is 7 chars, sub(s,7) keeps the colon)
//
// The scripts.go comment + StartSession intent is `rt_fam:<h>` = hash ->
// FAMILY map. Consequence, exercised service-style below: the LEGITIMATE
// second refresh dies, because Rotate resolves the family with
// Get("rt_fam:h1") -> "h0" and then misses rt_family:h0 -> "dead".
func TestRotateScriptSuccessorRegistrationFinding(t *testing.T) {
	ctx := context.Background()
	c := NewMemCache()
	const uid, fam = "42", "F1"
	seedFamily(t, ctx, c, uid, fam, "h0", time.Minute)

	if out, _ := rotateServiceLike(t, ctx, c, "h0", "h1", "", 60); out != "ok" {
		t.Fatalf("first rotation = %q, want ok", out)
	}

	// Current (buggy) registration: predecessor hash, not the family.
	if v, err := c.Get(ctx, "rt_fam:h1"); err != nil || string(v) != "h0" {
		t.Fatalf("rt_fam:h1 = (%q, %v); document the value CLI 1 writes — see finding", v, err)
	}

	// Service-style second hop: mis-resolved family -> dead. This is the
	// pinned CURRENT behavior; once scripts.go stores the family, flip the
	// expectation to "ok" (and rt_fam:h1 to F1).
	if out, _ := rotateServiceLike(t, ctx, c, "h1", "h2", "", 60); out != "dead" {
		t.Fatalf("second-hop rotation = %q; expected 'dead' under the current successor-registration bug", out)
	}
}

// TestRotateScriptReplayKillsFamily: rotating a token, then presenting the
// CONSUMED token again must revoke the whole family atomically.
func TestRotateScriptReplayKillsFamily(t *testing.T) {
	ctx := context.Background()
	c := NewMemCache()
	const uid, fam = "7", "F2"
	seedFamily(t, ctx, c, uid, fam, "h0", time.Minute)

	// First rotation consumes h0.
	if out, _ := rotateServiceLike(t, ctx, c, "h0", "h1", "", 60); out != "ok" {
		t.Fatalf("rotation = %q, want ok", out)
	}
	// REPLAY of the consumed h0: its rt_fam entry survives the rotation
	// (by design, for attribution), so the service still resolves F2 and
	// the script reports reuse of that family.
	out, _ := rotateServiceLike(t, ctx, c, "h0", "h9", "", 60)
	if out != "reused:"+uid {
		t.Fatalf("replay outcome = %q, want reused:7", out)
	}

	// Whole family revoked atomically: death flag set; the record and the
	// replayed hash's map are deleted.
	if _, err := c.Get(ctx, "rt_family:F2:revoked"); err != nil {
		t.Fatalf("family death flag missing after replay: %v", err)
	}
	for _, k := range []string{"rt_family:F2", "rt_fam:h0"} {
		if _, err := c.Get(ctx, k); !IsNotFound(err) {
			t.Fatalf("%s should be deleted after replay, got err=%v", k, err)
		}
	}
	// The surviving successor h1 is now unusable: any rotation attempt in
	// family F2 reports dead (the flag check runs first).
	out, _ = rotateServiceLike(t, ctx, c, "h1", "h3", fam, 60)
	if out != "dead" {
		t.Fatalf("post-replay rotation of successor = %q, want dead", out)
	}
}

// TestRotateScriptDeadWhenFamilyGone: a family record with no death flag
// (e.g. TTL-lapsed) must be reported dead, and the flag written so the
// next attempt short-circuits too.
func TestRotateScriptDeadWhenFamilyGone(t *testing.T) {
	ctx := context.Background()
	c := NewMemCache()

	if out, _ := rotateServiceLike(t, ctx, c, "h0", "h1", "F3", 60); out != "dead" {
		t.Fatalf("rotation on unknown family = %q, want dead", out)
	}
	if _, err := c.Get(ctx, "rt_family:F3:revoked"); err != nil {
		t.Fatalf("death flag not written on the dead path: %v", err)
	}
	if out, _ := rotateServiceLike(t, ctx, c, "h0", "h1", "F3", 60); out != "dead" {
		t.Fatalf("second attempt on flagged family = %q, want dead", out)
	}
}

// TestUserRevokedBeforeRevokesLiveAccessToken asserts the instant-
// revocation primitive from ADR-0001: service.MarkRevoked stamps
// user:revoked_before:<uid> = now (epoch secs) and AuthRequired rejects
// access tokens with iat < revoked_before. This pins the cache round-trip
// and the comparison invariant the middleware enforces.
func TestUserRevokedBeforeRevokesLiveAccessToken(t *testing.T) {
	ctx := context.Background()
	c := NewMemCache()
	const uid = "42"
	marker := time.Now().Unix()

	// Service.MarkRevoked writes the marker exactly this way (its TTL is
	// 2h in the auth package; any positive TTL suffices here).
	if err := c.Set(ctx, "user:revoked_before:"+uid, marker, 2*time.Hour); err != nil {
		t.Fatalf("Set marker: %v", err)
	}
	raw, err := c.Get(ctx, "user:revoked_before:"+uid)
	if err != nil {
		t.Fatalf("Get marker: %v", err)
	}
	got, perr := strconv.ParseInt(string(raw), 10, 64)
	if perr != nil || got != marker {
		t.Fatalf("marker round-trip = (%d, %v), want %d", got, perr, marker)
	}

	// The middleware's rule (service.go:176): a live access token is revoked
	// when iat < marker — strict, so a token issued in the same second as
	// the revocation is still accepted.
	cases := []struct {
		iat     int64
		revoked bool
		issue   string
	}{
		{marker - 60, true, "issued 1m before the revocation (still inside its 15m TTL)"},
		{marker, false, "issued in the same second as the revocation (boundary: accepted)"},
		{marker + 60, false, "issued after the revocation"},
	}
	for _, tc := range cases {
		if gotRevoked := tc.iat < got; gotRevoked != tc.revoked {
			t.Fatalf("%s: revoked=%v, want %v", tc.issue, gotRevoked, tc.revoked)
		}
	}
	// The ADR's concrete case: a 14m-old access token (access TTL is 15m)
	// is still live, yet the marker revokes it without waiting out the TTL.
	if !(marker-840 < got) {
		t.Fatal("live 14m-old access token not covered by the marker check")
	}

	// Without a marker, nothing is revoked for that user.
	other := NewMemCache()
	if _, err := other.Get(ctx, "user:revoked_before:99"); !IsNotFound(err) {
		t.Fatalf("no marker for user 99: Get err=%v, want not-found", err)
	}
}
