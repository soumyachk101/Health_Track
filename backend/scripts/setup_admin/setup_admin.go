package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"healthtrack/internal/database"
)

func main() {
	// Load .env if present; ignore not-found so CI/prod env vars still work.
	_ = godotenv.Load()

	ctx := context.Background()
	db, err := database.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Legacy credentials carried over from setup_admin.js — change the
	// password after first login.
	username := "soumya933"
	email := "soumya.chk101@gmail.com"
	password := "Soumya@2842y"

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to hash password: %v\n", err)
		os.Exit(1)
	}

	existing, err := db.QueryOne(ctx, "SELECT id FROM users WHERE username = $1 OR email = $2", username, email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to look up admin user: %v\n", err)
		os.Exit(1)
	}

	if existing != nil {
		if _, err := db.Exec(ctx, `UPDATE users
			SET password = $1, user_type = 'admin', is_superuser = 1, is_approved = 1, is_email_verified = 1
			WHERE id = $2`, string(hashed), existing["id"]); err != nil {
			fmt.Fprintf(os.Stderr, "failed to update admin user: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("User already exists, updated to Admin.")
	} else {
		if _, err := db.Exec(ctx, `INSERT INTO users
			(username, email, password, user_type, is_approved, is_superuser, is_email_verified)
			VALUES ($1, $2, $3, 'admin', 1, 1, 1)`, username, email, string(hashed)); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create admin user: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Admin user created.")
	}

	fmt.Println("Admin user setup complete.")
	os.Exit(0)
}
