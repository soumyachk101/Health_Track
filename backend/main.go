package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/hands"
	"healthtrack/internal/services/email"
)

func main() {
	cfg := config.Load()

	// Set the gin mode before the engine is created so release mode takes
	// effect across every middleware built afterwards.
	if cfg.NodeEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Refuse to serve production traffic under the well-known development JWT
	// secret: any holder could then mint valid user tokens. Fail fast at
	// startup instead of falling back silently.
	if cfg.NodeEnv == "production" && cfg.JWTSecret == "default-jwt-secret-change-me" {
		log.Fatal("JWT_SECRET is not set; refusing to start in production with the default development secret")
	}

	ctx := context.Background()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	mail := email.New(cfg)
	r := hands.BuildRouter(db, cfg, mail)

	addr := cfg.Port
	if !strings.Contains(addr, ":") {
		addr = ":" + addr
	}
	log.Printf("HealthTrack+ server running on %s", addr)

	// Explicit read/write timeouts so a stuck client can never pin a
	// connection (and a leaked goroutine holding the DB pool) indefinitely.
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
