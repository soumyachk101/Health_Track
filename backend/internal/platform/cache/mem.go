package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

// MemCache is a process-local [Cache] for development and unit tests. It
// emulates the Redis key/value + atomic rotation semantics well enough for a
// single process; cross-process reuse-detection does not apply (there is no
// second process to replay from).
type MemCache struct {
	mu    sync.Mutex
	items map[string]memItem
}

type memItem struct {
	val     []byte
	expires time.Time // zero = no TTL
}

// NewMemCache returns an empty in-memory cache.
func NewMemCache() *MemCache {
	return &MemCache{items: map[string]memItem{}}
}

func (m *MemCache) live(key string) (memItem, bool) {
	it, ok := m.items[key]
	if !ok {
		return memItem{}, false
	}
	if !it.expires.IsZero() && time.Now().After(it.expires) {
		delete(m.items, key)
		return memItem{}, false
	}
	return it, true
}

func (m *MemCache) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.live(key)
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]byte, len(it.val))
	copy(out, it.val)
	return out, nil
}

func (m *MemCache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var raw []byte
	switch v := val.(type) {
	case string:
		raw = []byte(v)
	case []byte:
		raw = append([]byte(nil), v...)
	case int64:
		raw = []byte(itoa(v))
	case int:
		raw = []byte(itoa(int64(v)))
	default:
		raw = nil
	}
	expires := time.Time{}
	if ttl > 0 {
		expires = time.Now().Add(ttl)
	}
	m.items[key] = memItem{val: raw, expires: expires}
	return nil
}

func (m *MemCache) SetNX(ctx context.Context, key string, val any, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	_, exists := m.live(key)
	if exists {
		m.mu.Unlock()
		return false, nil
	}
	m.mu.Unlock()
	if err := m.Set(ctx, key, val, ttl); err != nil {
		return false, err
	}
	return true, nil
}

func (m *MemCache) Del(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		delete(m.items, k)
	}
	return nil
}

func (m *MemCache) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := int64(1)
	if it, ok := m.live(key); ok {
		n = atoi(it.val) + 1
	}
	var expires time.Time
	if ttl > 0 {
		expires = time.Now().Add(ttl)
	}
	m.items[key] = memItem{val: []byte(itoa(n)), expires: expires}
	return n, nil
}

// Eval emulates [RotateScript] and [ClaimOtpScript] with the same visible
// semantics as their Redis versions. Any other script errors: MemCache is a
// test fallback, not a Lua engine.
func (m *MemCache) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch script {
	case RotateScript:
		if len(keys) != 2 || len(args) != 2 {
			return nil, errors.New("mem cache: rotate needs 2 keys, 2 args")
		}
		oldKey, famKey := keys[0], keys[1]
		ttlSec := toInt64(args[1])
		now := time.Now()
		if _, ok := m.live(oldKey); !ok {
			// Presented token already consumed: replay. Kill the family.
			delete(m.items, famKey)
			m.items[famKey+":reused"] = memItem{val: []byte("1"), expires: now.Add(time.Duration(ttlSec) * time.Second)}
			return "reused", nil
		}
		// Happy rotation: register the successor, extend the family, drop
		// the presented token.
		newHash := "rt:" + toString(args[0])
		m.items[newHash] = memItem{val: []byte(famKey), expires: now.Add(time.Duration(ttlSec) * time.Second)}
		if it, ok := m.live(famKey); ok {
			it.expires = now.Add(time.Duration(ttlSec) * time.Second)
			m.items[famKey] = it
		} else {
			m.items[famKey] = memItem{val: []byte("1"), expires: now.Add(time.Duration(ttlSec) * time.Second)}
		}
		delete(m.items, oldKey)
		return "ok", nil

	case ClaimOtpScript:
		if len(keys) != 1 || len(args) != 1 {
			return nil, errors.New("mem cache: claim-otp needs 1 key, 1 arg")
		}
		key := keys[0]
		ttl := time.Duration(toInt64(args[0])) * time.Second
		if _, ok := m.live(key); ok {
			return "taken", nil
		}
		m.items[key] = memItem{val: []byte("1"), expires: time.Now().Add(ttl)}
		return "claimed", nil

	default:
		return nil, errors.New("mem cache: unknown script")
	}
}

func (m *MemCache) Ping(ctx context.Context) error { return nil }

// GetRaw exposes a key's stored bytes for tests.
func (m *MemCache) GetRaw(key string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	if it, ok := m.live(key); ok {
		return it.val
	}
	return nil
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return ""
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case []byte:
		return atoi(n)
	default:
		return atoi(toString(v))
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [24]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func atoi(b []byte) int64 {
	var n int64
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
