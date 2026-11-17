// Package cache is the platform-level Redis client (REDIS_URL) shared by
// the auth module (refresh rotation, instant revocation, rate limiting) and,
// later, the ADR-0002 queue and limiter.
//
// [Cache] is the narrow storage surface the auth module needs; [RedisCache]
// implements it over go-redis, and [MemCache] is a process-local fallback
// for single-instance development and unit tests (it has no cross-process
// atomicity, so refresh rotation over MemCache is only safe with one
// process — production always uses Redis).
package cache

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache is the storage interface consumed by the auth module.
//
// Values are marshalled by the caller; [Eval] runs a Lua script atomically
// and returns the script's result (go-redis decodes it to the native Go
// type: int64 for integers, []byte for strings, []any for arrays).
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, val any, ttl time.Duration) error
	SetNX(ctx context.Context, key string, val any, ttl time.Duration) (bool, error)
	Del(ctx context.Context, keys ...string) error
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
	Ping(ctx context.Context) error
}

// ErrNotFound is the MemCache not-found sentinel; IsNotFound below accepts
// it and redis.Nil interchangeably.
var ErrNotFound = errors.New("cache: not found")

// RedisCache is the Redis-backed [Cache].
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache dials REDIS_URL. The URL carries credentials; keep it out of
// logs and version control.
func NewRedisCache(redisURL string) (*RedisCache, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &RedisCache{client: redis.NewClient(opts)}, nil
}

// Client exposes the underlying go-redis client for package-internal tests
// and for ADR-0002's queue (which needs the full API).
func (r *RedisCache) Client() *redis.Client { return r.client }

// Close releases the connection pool.
func (r *RedisCache) Close() error { return r.client.Close() }

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	v, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (r *RedisCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	return r.client.Set(ctx, key, val, ttl).Err()
}

func (r *RedisCache) SetNX(ctx context.Context, key string, val any, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, val, ttl).Result()
}

func (r *RedisCache) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Incr increments key; on the first increment the key gets ttl. The
// increment and the first-set expiry are not one atomic step, but a missed
// EXPIRE only ever makes a limiter window live slightly longer — safe in
// the direction of stricter limiting.
func (r *RedisCache) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_ = r.client.Expire(ctx, key, ttl).Err()
	}
	return n, nil
}

func (r *RedisCache) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	return r.client.Eval(ctx, script, keys, args...).Result()
}

func (r *RedisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// IsNotFound reports whether err means "no value at this key" on either
// backend, so callers never import go-redis for the sentinel.
func IsNotFound(err error) bool {
	return errors.Is(err, redis.Nil) || errors.Is(err, ErrNotFound)
}
