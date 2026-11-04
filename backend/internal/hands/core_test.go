package hands

import (
	"testing"
	"time"
)

// TestBpStatus mirrors the legacy getBpStatus tiers, including the boundary
// where 140/90 already classifies as Stage 2 (the Go port uses strict <).
func TestBpStatus(t *testing.T) {
	cases := []struct {
		sys, dia int32
		want     string
	}{
		{0, 0, "Unknown"},
		{110, 0, "Unknown"},
		{0, 70, "Unknown"},
		{110, 70, "Normal"},
		{119, 79, "Normal"},
		{125, 75, "Elevated"},
		{135, 85, "High (Stage 1)"},
		{140, 89, "High (Stage 1)"},
		{140, 90, "High (Stage 2)"},
		{160, 100, "High (Stage 2)"},
	}
	for _, c := range cases {
		if got := bpStatus(c.sys, c.dia); got != c.want {
			t.Errorf("bpStatus(%d, %d) = %q, want %q", c.sys, c.dia, got, c.want)
		}
	}
}

// TestMoodDisplay pins the 1..5 label scale and the Unknown fallback.
func TestMoodDisplay(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{1, "Very Low"},
		{2, "Low"},
		{3, "Neutral"},
		{4, "Good"},
		{5, "Excellent"},
		{0, "Unknown"},
		{6, "Unknown"},
		{-1, "Unknown"},
	}
	for _, c := range cases {
		if got := moodDisplay(c.score); got != c.want {
			t.Errorf("moodDisplay(%d) = %q, want %q", c.score, got, c.want)
		}
	}
}

// TestFrequencyDisplay pins the frequency label map and passthrough of
// unknown values.
func TestFrequencyDisplay(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"once", "Once Daily"},
		{"twice", "Twice Daily"},
		{"thrice", "Three Times Daily"},
		{"asneeded", "As Needed"},
		{"weird", "weird"},
		{"", ""},
	}
	for _, c := range cases {
		if got := frequencyDisplay(c.in); got != c.want {
			t.Errorf("frequencyDisplay(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestPolicyTypeDisplay pins the policy label map and passthrough.
func TestPolicyTypeDisplay(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"health", "Health Insurance"},
		{"life", "Life Insurance"},
		{"term", "Term Life Insurance"},
		{"other", "other"},
		{"", ""},
	}
	for _, c := range cases {
		if got := policyTypeDisplay(c.in); got != c.want {
			t.Errorf("policyTypeDisplay(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestTrunc50 pins the truncate-to-50 + ellipsis behavior.
func TestTrunc50(t *testing.T) {
	short := "abc"
	if got := trunc50(short); got != short {
		t.Errorf("trunc50(%q) = %q, want unchanged", short, got)
	}
	exact := "01234567890123456789012345678901234567890123456789" // exactly 50 chars
	if len(exact) != 50 {
		t.Fatalf("test fixture broken: got len %d, want 50", len(exact))
	}
	if got := trunc50(exact); got != exact {
		t.Errorf("trunc50(50 chars) changed its input: %q", got)
	}
	// 51 chars -> first 50 + "..."
	long := exact + "X"
	want := exact + "..."
	if got := trunc50(long); got != want {
		t.Errorf("trunc50(51 chars) = %q, want %q", got, want)
	}
}

// TestDateOnly pins date extraction from strings, time.Time and NULL.
func TestDateOnly(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want string
	}{
		{"timestamp string", "2026-01-02T03:04:05Z", "2026-01-02"},
		{"plain date string", "2026-01-02", "2026-01-02"},
		{"short string", "2026", "2026"},
		{"time.Time", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), "2026-01-02"},
		{"nil", nil, ""},
	}
	for _, c := range cases {
		if got := dateOnly(c.v); got != c.want {
			t.Errorf("dateOnly(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestAsInt: integers in any pgx shape, 0 otherwise.
func TestAsInt(t *testing.T) {
	cases := []struct {
		v    any
		want int
	}{
		{int32(42), 42},
		{int64(42), 42},
		{int(42), 42},
		{nil, 0},
		{"7", 0},
		{float64(7), 0},
	}
	for _, c := range cases {
		if got := asInt(c.v); got != c.want {
			t.Errorf("asInt(%#v) = %d, want %d", c.v, got, c.want)
		}
	}
}

// TestAsInt64: integers in any pgx shape, 0 otherwise.
func TestAsInt64(t *testing.T) {
	cases := []struct {
		v    any
		want int64
	}{
		{int32(42), 42},
		{int64(42), 42},
		{int(42), 42},
		{nil, 0},
		{float64(7), 0},
	}
	for _, c := range cases {
		if got := asInt64(c.v); got != c.want {
			t.Errorf("asInt64(%#v) = %d, want %d", c.v, got, c.want)
		}
	}
}

// TestAsStr and stringOrEmpty: strings pass through, everything else is "".
func TestAsStr(t *testing.T) {
	if got := asStr("x"); got != "x" {
		t.Errorf("asStr(%q) = %q, want %q", "x", got, "x")
	}
	if got := asStr(nil); got != "" {
		t.Errorf("asStr(nil) = %q, want empty", got)
	}
	if got := asStr(int32(1)); got != "" {
		t.Errorf("asStr(int32) = %q, want empty", got)
	}
	if got := stringOrEmpty("y"); got != "y" {
		t.Errorf("stringOrEmpty(%q) = %q", "y", got)
	}
	if got := stringOrEmpty(nil); got != "" {
		t.Errorf("stringOrEmpty(nil) = %q, want empty", got)
	}
}

// TestStrPtr: string -> pointer, everything else nil.
func TestStrPtr(t *testing.T) {
	if got := strPtr("v"); got == nil || *got != "v" {
		t.Errorf("strPtr(%q) = %v, want pointer to value", "v", got)
	}
	if got := strPtr(nil); got != nil {
		t.Errorf("strPtr(nil) = %v, want nil", got)
	}
	if got := strPtr(float64(1)); got != nil {
		t.Errorf("strPtr(float64) = %v, want nil", got)
	}
}

// TestInt32Ptr: integer shapes -> pointer, others nil.
func TestInt32Ptr(t *testing.T) {
	p := func(v int32) *int32 { return &v }
	cases := []struct {
		v    any
		want *int32
	}{
		{int32(5), p(5)},
		{int64(6), p(6)},
		{int(7), p(7)},
		{nil, nil},
		{float64(8), nil},
	}
	for _, c := range cases {
		if got := int32Ptr(c.v); c.want == nil {
			if got != nil {
				t.Errorf("int32Ptr(%#v) = %v, want nil", c.v, *got)
			}
		} else if got == nil || *got != *c.want {
			t.Errorf("int32Ptr(%#v) = %v, want %v", c.v, got, *c.want)
		}
	}
}

// TestNumPtr: numeric shapes -> *float64, everything else nil.
func TestNumPtr(t *testing.T) {
	p := func(v float64) *float64 { return &v }
	cases := []struct {
		v    any
		want *float64
	}{
		{float64(1.5), p(1.5)},
		{float32(2), p(2)},
		{int32(3), p(3)},
		{int64(4), p(4)},
		{nil, nil},
		{"5", nil},
	}
	for _, c := range cases {
		if got := numPtr(c.v); c.want == nil {
			if got != nil {
				t.Errorf("numPtr(%#v) = %v, want nil", c.v, *got)
			}
		} else if got == nil || *got != *c.want {
			t.Errorf("numPtr(%#v) = %v, want %v", c.v, got, *c.want)
		}
	}
}

// TestBoolPtr: integer/boolean flags coerce to *bool, NULL stays nil.
func TestBoolPtr(t *testing.T) {
	tr, fa := true, false
	cases := []struct {
		v    any
		want *bool
	}{
		{true, &tr},
		{int32(1), &tr},
		{int32(0), &fa},
		{int64(2), &tr},
		{nil, nil},
		{"1", nil},
	}
	for _, c := range cases {
		if got := boolPtr(c.v); c.want == nil {
			if got != nil {
				t.Errorf("boolPtr(%#v) = %v, want nil", c.v, *got)
			}
		} else if got == nil || *got != *c.want {
			t.Errorf("boolPtr(%#v) = %v, want %v", c.v, got, *c.want)
		}
	}
}

// TestJsStringNum: JS-style null-or-string serialization of numeric columns.
// Zero values must be nil (JSON null); others are formatted without a
// trailing ".0".
func TestJsStringNum(t *testing.T) {
	cases := []struct {
		v    any
		want any
	}{
		{float64(0), nil},
		{int32(0), nil},
		{float64(1.5), "1.5"},
		{float32(3), "3"},
		{int64(120), "120"},
		{nil, nil},
		{"5", nil},
	}
	for _, c := range cases {
		if got := jsStringNum(c.v); got != c.want {
			t.Errorf("jsStringNum(%#v) = %#v, want %#v", c.v, got, c.want)
		}
	}
}

// TestDisplayName pins the "first last" trim behavior.
func TestDisplayName(t *testing.T) {
	cases := []struct {
		first, last, want string
	}{
		{"Ada", "Lovelace", "Ada Lovelace"},
		{"", "", ""},
		{"  ", "  ", ""},
		{"Ada", "", "Ada"},
	}
	for _, c := range cases {
		if got := displayName(c.first, c.last); got != c.want {
			t.Errorf("displayName(%q, %q) = %q, want %q", c.first, c.last, got, c.want)
		}
	}
}
