// Package worker is the long-lived background-task process (ADR-0002):
// the asynq consumer for email:send / ai:analyze_document /
// files:cleanup_stale, with graceful shutdown on SIGINT/SIGTERM,
// structured PII-redacted logs and per-queue concurrency limits.
//
// It is a SEPARATE process from the API (the ADR's rollout decision: both
// run on Cloud Run with managed Redis). It needs DATABASE_URL + REDIS_URL;
// without REDIS_URL it exits with a clear error (the API degrades to its
// sync fallback — the queue is what the worker drains).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"healthtrack/internal/config"
	"healthtrack/internal/database"
	"healthtrack/internal/platform/queue"
	"healthtrack/internal/platform/storage"
)

func main() {
	cfg := config.Load()

	// The worker is a consumer: no Redis, nothing to drain.
	if cfg.RedisURL == "" {
		log.Fatal("worker: REDIS_URL is not set; refusing to start (the API degrades to its sync path, but the worker has no queue)")
	}
	if cfg.DatabaseURL == "" {
		log.Fatal("worker: DATABASE_URL is not set; refusing to start (task state is ledgered in Postgres)")
	}
	r2cfg, r2ok := storage.R2ConfigFromEnv()
	if !r2ok {
		// R2 unset: cleanup + analyze tasks fail-fast per task and DLQ;
		// mail still runs. A mail-only deployment is a valid partial
		// rollout, so warn instead of refusing.
		log.Printf("worker: R2_* not configured; files:cleanup_stale + ai:analyze_document will fail until R2 is set")
	}

	db, err := database.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("worker: connect database: %v", err)
	}
	defer db.Close()

	var store storage.ObjectStorage
	var storeErr error
	if r2ok {
		store, storeErr = storage.NewS3Store(r2cfg)
	}

	// --- asynq server: one process, bounded concurrency, pinned retry curve
	rconn, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		log.Fatalf("worker: parse REDIS_URL: %v", err)
	}
	queues := map[string]int{
		queue.QueueEmail: 5, // user-visible mail first
		queue.QueueAI:    3, // expensive + slow
		queue.QueueFiles: 1, // cheap, idle-time
		"default":        1,
	}
	srv := asynq.NewServer(rconn, asynq.Config{
		Concurrency: 8,
		Queues:      queues,
		// The pinned exponential backoff (1m*2^n, cap 4h; max retries
		// 5 per task) — explicit so the curve is the ADR's, not asynq's
		// default.
		RetryDelayFunc: queue.ExponentialBackoff,
	})

	mux := asynq.NewServeMux()
	h := &handlers{db: db, cfg: cfg, store: store, storeErr: storeErr}
	registerTasks(mux, h)

	// Periodic stale-purge enqueues files:cleanup_stale every 30m.
	producer := queue.NewProducer(db, cfg.RedisURL)
	go runCleanupScheduler(producer)

	// Graceful shutdown: SIGINT/SIGTERM drain in-flight tasks.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if s := <-sig; s != nil {
			log.Printf("worker: signal %s; shutting down gracefully", s)
			srv.Shutdown()
		}
	}()

	log.Printf("worker: started (concurrency=8, queues %v)", queues)
	if err := srv.Run(mux); err != nil {
		log.Fatalf("worker: run: %v", err)
	}
	_ = producer.Close()
	log.Printf("worker: stopped")
}

// runCleanupScheduler enqueues the periodic stale-file purge. It is a
// best-effort scheduler: a lost tick just means one late cleanup pass.
func runCleanupScheduler(producer *queue.Producer) {
	if !producer.Configured() {
		return
	}
	tick := time.NewTicker(30 * time.Minute)
	defer tick.Stop()
	for range tick.C {
		task, opts := queue.NewCleanupStaleTask()
		// Run-timestamp key: overlapping runs don't dedupe against
		// each other, but a same-second manual + scheduled run collapses.
		key := queue.IdempotencyKey(queue.TypeCleanupStaleFile, time.Now().Format("20060102150405"))
		// System task: no enqueuing user, so owner 0.
		if _, err := producer.Enqueue(context.Background(), task, key, 0, opts...); err != nil {
			log.Printf("worker: cleanup enqueue: %v", err)
		}
	}
}
