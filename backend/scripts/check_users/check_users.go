package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"

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

	users, err := db.QueryMany(ctx, `SELECT id, username, email, user_type, is_superuser
		FROM users ORDER BY id`)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to query users: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("ID\tUSERNAME\tEMAIL\tUSER_TYPE\tIS_SUPERUSER")
	fmt.Println("----------------------------------------------------------------")
	for _, u := range users {
		id, _ := u["id"].(int64)
		username, _ := u["username"].(string)
		email, _ := u["email"].(string)
		userType, _ := u["user_type"].(string)
		isSuperuser, _ := u["is_superuser"].(int)
		fmt.Printf("%d\t%s\t%s\t%s\t%d\n", id, username, email, userType, isSuperuser)
	}

	os.Exit(0)
}
