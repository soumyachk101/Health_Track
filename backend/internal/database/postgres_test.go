package database

import (
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestSplitStatements pins the SQL splitter used when applying the embedded
// schema: semicolons delimit, surrounding whitespace is trimmed, and empty
// fragments are dropped.
func TestSplitStatements(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"CREATE TABLE a(id INT); CREATE TABLE b(id INT);",
			[]string{"CREATE TABLE a(id INT)", "CREATE TABLE b(id INT)"}},
		{"", nil},
		{"SELECT 1", []string{"SELECT 1"}},
		{"  \n  SELECT 1 ;  \n  INSERT INTO t(x) VALUES(1);;  ",
			[]string{"SELECT 1", "INSERT INTO t(x) VALUES(1)"}},
	}
	for _, c := range cases {
		got := splitStatements(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitStatements(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestErrNoRows asserts our exported sentinel is identical to pgx's so the
// error-equality checks across the codebase hold.
func TestErrNoRows(t *testing.T) {
	if ErrNoRows != pgx.ErrNoRows {
		t.Errorf("ErrNoRows = %v, want pgx.ErrNoRows", ErrNoRows)
	}
	if !errors.Is(ErrNoRows, pgx.ErrNoRows) {
		t.Error("errors.Is(ErrNoRows, pgx.ErrNoRows) = false, want true")
	}
}
