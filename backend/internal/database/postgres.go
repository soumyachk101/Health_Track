package database

import (
	"context"
	_ "embed"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// ErrNoRows is returned by QueryRow when no rows are returned; it mirrors
// pgx.ErrNoRows.
var ErrNoRows = pgx.ErrNoRows

type DB struct {
	pool *pgxpool.Pool
}

// Connect opens a pgxpool connection to the given DSN (sslmode is read from
// the connection string), pings it, and executes every statement in the
// embedded schema sequentially.
func Connect(ctx context.Context, url string) (*DB, error) {
	if strings.TrimSpace(url) == "" {
		return nil, errors.New("DATABASE_URL required")
	}

	pcfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	pcfg.MaxConns = 5
	pcfg.MaxConnIdleTime = 30 * time.Second
	pcfg.ConnConfig.ConnectTimeout = 10 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	for _, stmt := range splitStatements(schemaSQL) {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return &DB{pool: pool}, nil
}

// Exec runs a statement and returns the raw command tag.
func (db *DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return db.pool.Exec(ctx, sql, args...)
}

// Run runs a statement. INSERTs get " RETURNING id" appended (unless one is
// already present) and return the new id; other statements return the row count.
func (db *DB) Run(ctx context.Context, sql string, args ...any) (int64, error) {
	trimmed := strings.TrimSpace(sql)
	if strings.HasPrefix(strings.ToUpper(trimmed), "INSERT") &&
		!strings.Contains(strings.ToUpper(trimmed), "RETURNING") {
		var id int64
		if err := db.pool.QueryRow(ctx, trimmed+" RETURNING id", args...).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}
	tag, err := db.pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (db *DB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return db.pool.QueryRow(ctx, sql, args...)
}

// QueryOne returns the first row as a map keyed by column name, or (nil, nil)
// when no rows match.
func (db *DB) QueryOne(ctx context.Context, sql string, args ...any) (map[string]any, error) {
	rows, err := db.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return mapScanRow(rows)
}

func (db *DB) QueryMany(ctx context.Context, sql string, args ...any) ([]map[string]any, error) {
	rows, err := db.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		m, err := mapScanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// mapScanRow scans the current row of rows into a map keyed by column name.
// It mirrors pgx's internal mapRowScanner: Values() applies the type map for
// proper Go type conversion.
func mapScanRow(rows pgx.Rows) (map[string]any, error) {
	fds := rows.FieldDescriptions()
	values, err := rows.Values()
	if err != nil {
		return nil, err
	}
	m := make(map[string]any, len(fds))
	for i, v := range values {
		m[fds[i].Name] = v
	}
	return m, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func splitStatements(sql string) []string {
	var out []string
	for _, s := range strings.Split(sql, ";") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}
