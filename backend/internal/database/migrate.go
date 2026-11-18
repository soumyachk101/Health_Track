// Package database exposes the shared Postgres pool and the versioned
// migration runner. The runner is the ONLY place schema changes happen:
// the API boot is read-only unless the DB_MIGRATE_ON_BOOT flag says otherwise
// (see main.go), so a fresh deployment can never silently run ad-hoc DDL.
package database

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// schemaVersionsDDL records which migration files each environment has
// applied. It is created here (rather than in a migration) because the
// runner needs it to exist before it can record anything.
const schemaVersionsDDL = `CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`

// Migrate applies every pending embedded migration, in version order, and
// records each in schema_migrations. Each file runs inside one transaction;
// a failure aborts that file and stops the run.
//
// A database that was bootstrapped by the legacy DDL-on-startup has no
// schema_migrations rows, so on a first run 0001 (the idempotent baseline
// copy of that DDL) and every later file are both applied. The baseline is
// idempotent, so re-running it against an already-populated database is a
// no-op.
func Migrate(ctx context.Context, db *DB) error {
	return MigratePool(ctx, db.pool)
}

// MigratePool is Migrate over a raw pool (used by cmd/migrate, which owns
// its own connection lifecycle).
func MigratePool(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schemaVersionsDDL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		version := versionOf(name)
		if applied[version] {
			continue
		}
		data, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, pool, version, data); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

// versionOf maps a filename like "0002_pure_go_auth.sql" to "0002".
func versionOf(name string) string {
	base := strings.TrimSuffix(name, ".sql")
	if i := strings.IndexByte(base, '_'); i > 0 {
		return base[:i]
	}
	return base
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, version string, sql []byte) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, stmt := range splitStatements(string(sql)) {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	// The table was just created, so the query always succeeds.
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		seen[v] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return seen, nil
}
