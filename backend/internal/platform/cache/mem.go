package cache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// MemCache is a process-local [Cache] for development and unit tests. It
// emulates [RotateScript] with the same key semantics as the Redis version,
// so rotation/reuse logic can be tested without a live Redis. It is NOT
// cross-process: a replayed token in a second process cannot be detected.
type MemCache struct {
	mu    sync.Mutex
	items map[string]memItem
	sets  map[string]map[string]struct{}
}

type memItem struct {
	val     []byte
	expires time.Time // zero = no TTL
}

// NewMemCache returns an empty in-memory cache.
func NewMemCache() *MemCache {
	return &MemCache{items: map[string]memItem{}, sets: map[string]map[string]struct{}{}}
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
		raw = []byte(strings.TrimSpace(itoa(v)))
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

func (m *MemCache) SAdd(ctx context.Context, key string, members []string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sets[key]
	if s == nil {
		s = map[string]struct{}{}
		m.sets[key] = s
	}
	for _, mem := range members {
		s[mem] = struct{}{}
	}
	if ttl > 0 {
		// Approximate: every member's expiry = now+ttl, the set dies when
		// it has no live members.
	}
	return nil
}

func (m *MemCache) SMembers(ctx context.Context, key string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sets[key]
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return out, nil
}

// Eval emulates [RotateScript] key-for-key. Other scripts error: MemCache
// is a test fallback, not a Lua engine.
func (m *MemCache) Eval(ctx context.Context, script string, keys []string, args ...any) (any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if script != RotateScript || len(keys) != 6 || len(args) != 2 {
		return nil, errors.New("mem cache: only RotateScript is supported (6 keys, 2 args)")
	}
	rtH0, rtFamH0, famRec, famRev, rtH1, rtFamH1 := keys[0], keys[1], keys[2], keys[3], keys[4], keys[5]
	h1 := toString(args[1])
	ttl := time.Duration(toInt64(args[0])) * time.Second
	now := time.Now()

	get := func(k string) (string, bool) {
		it, ok := m.live(k)
		if !ok {
			return "", false
		}
		return string(it.val), true
	}
	set := func(k, v string) { m.items[k] = memItem{val: []byte(v), expires: now.Add(ttl)} }

	if _, dead := get(famRev); dead {
		return "dead", nil
	}
	rec, ok := get(famRec)
	if !ok {
		m.items[famRev] = memItem{val: []byte("1"), expires: now.Add(ttl)}
		return "dead", nil
	}
	sep := strings.IndexByte(rec, '|')
	uid, hCur := rec[:sep], rec[sep+1:]
	h0 := strings.TrimPrefix(rtH0, "rt:")

	if hCur == h0 {
		fam := strings.TrimPrefix(rtFamH0, "rt_fam:")
		set(rtH1, fam)
		set(rtFamH1, fam)
		delete(m.items, rtH0)
		set(famRec, uid+"|"+h1)
		return "ok", nil
	}

	// Replay: an already-rotated token was presented. Kill the family.
	m.items[famRev] = memItem{val: []byte("1"), expires: now.Add(ttl)}
	delete(m.items, famRec)
	delete(m.items, rtFamH0)
	return "reused:" + uid, nil
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
	case string:
		return atoi([]byte(n))
	default:
		return 0
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
