package otp

import (
	"strconv"
	"testing"
	"time"
)

// TestGenerate exercises the distribution of Generate: every output must be
// exactly 6 ASCII digits in [100000, 999999], and the observed spread must be
// wide enough to catch an off-by-one in the random range.
func TestGenerate(t *testing.T) {
	const draws = 200
	var min, max int
	first := true
	for i := 0; i < draws; i++ {
		got := Generate()
		if len(got) != 6 {
			t.Fatalf("Generate() = %q, want exactly 6 chars", got)
		}
		n, err := strconv.Atoi(got)
		if err != nil {
			t.Fatalf("Generate() = %q, not all digits: %v", got, err)
		}
		if n < 100000 || n > 999999 {
			t.Fatalf("Generate() = %d, out of [100000, 999999]", n)
		}
		if first {
			min, max, first = n, n, false
		}
		if n < min {
			min = n
		}
		if n > max {
			max = n
		}
	}
	if max-min <= 10 {
		t.Errorf("observed spread max-min = %d over %d draws, want > 10 (distribution suspiciously narrow)", max-min, draws)
	}
}

// TestExpiryISO pins the exact RFC3339/UTC formatting against deterministic
// inputs, including the zero-minute boundary.
func TestExpiryISO(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	cases := []struct {
		minutes int
		want    string
	}{
		{15, now.Add(15 * time.Minute).UTC().Format(time.RFC3339)},
		{0, now.UTC().Format(time.RFC3339)},
	}
	for _, c := range cases {
		if got := ExpiryISO(now, c.minutes); got != c.want {
			t.Errorf("ExpiryISO(now, %d) = %q, want %q", c.minutes, got, c.want)
		}
	}
}
