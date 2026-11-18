// Command migrate applies the versioned schema migrations embedded in
// internal/database/migrations to the database named by DATABASE_URL.
//
// It is the ONLY supported way to change the schema: the server itself no
// longer mutates DDL on boot (Phase 1, finding P2.3). Run it once per
// environment before starting the API, and again on every deploy that
// ships a new migration file.
//
//	$ export DATABASE_URL=postgresql://...
//	$ go run ./cmd/migrate
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"healthtrack/internal/database"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		log.Fatalf("parse DATABASE_URL: %v", err)
	}
	defer pool.Close()

	if err := database.MigratePool(ctx, pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	fmt.Println("migrations applied")
}
