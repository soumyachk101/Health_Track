package middleware

import (
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Limiter is a dependency-light in-memory rate guard for the auth endpoints.
// Two independent sliding windows apply per composite key (client IP +
// lowercased email/username):
//
//   - flood cap: at most 30 requests of any kind over 15 minutes
//   - brute cap: at most 5 recorded credential failures over 5 minutes;
//     exceeding it locks the key out for 15 minutes
type Limiter struct {
	mu        sync.Mutex
	attempts  map[string][]time.Time
	failures  map[string][]time.Time
	blocked   map[string]time.Time
	lastSweep time.Time
}

// DefaultAuthLimiter is the process-wide instance shared by all auth
// handlers, so the caps count across every protected endpoint.
var DefaultAuthLimiter = &Limiter{
	attempts: map[string][]time.Time{},
	failures: map[string][]time.Time{},
	blocked:  map[string]time.Time{},
}

func (m *Limiter) sweepLocked() {
	now := time.Now()
	if now.Sub(m.lastSweep) < 10*time.Minute && len(m.attempts) < 50000 {
		return
	}
	for k, ts := range m.attempts {
		kept := ts[:0]
		for _, t := range ts {
			if t.After(now.Add(-15 * time.Minute)) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(m.attempts, k)
		} else {
			m.attempts[k] = kept
		}
	}
	for k, ts := range m.failures {
		kept := ts[:0]
		for _, t := range ts {
			if t.After(now.Add(-5 * time.Minute)) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(m.failures, k)
		} else {
			m.failures[k] = kept
		}
	}
	for k, until := range m.blocked {
		if now.After(until) {
			delete(m.blocked, k)
		}
	}
	m.lastSweep = now
}

// blockedLocked reports (and clears if expired) the lockout state for key.
// Callers hold m.mu.
func (m *Limiter) blockedLocked(key string, now time.Time) bool {
	until, ok := m.blocked[key]
	if !ok {
		return false
	}
	if now.Before(until) {
		return true
	}
	delete(m.blocked, key)
	return false
}

// recentCountLocked counts timestamps in ts newer than cutoff. Callers
// hold m.mu.
func recentCountLocked(ts []time.Time, cutoff time.Time) int {
	n := 0
	for _, t := range ts {
		if t.After(cutoff) {
			n++
		}
	}
	return n
}

// Allow enforces the flood cap and any active brute-force lockout for key.
// It records the attempt in the flood window.
func (m *Limiter) Allow(key string) bool {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sweepLocked()
	if m.blockedLocked(key, now) {
		return false
	}
	ts := m.attempts[key]
	if recentCountLocked(ts, now.Add(-15*time.Minute)) >= 30 {
		return false
	}
	m.attempts[key] = append(ts, now)
	return true
}

// RecordFailure records a failed credential attempt; 5 failures within 5
// minutes lock the key out for 15 minutes.
func (m *Limiter) RecordFailure(key string) {
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	ts := m.failures[key]
	if recentCountLocked(ts, now.Add(-5*time.Minute)) >= 5 {
		return
	}
	m.failures[key] = append(ts, now)
	if recentCountLocked(m.failures[key], now.Add(-5*time.Minute)) == 5 {
		m.blocked[key] = now.Add(15 * time.Minute)
		log.Printf("auth rate limit: lockout for %q for 15m after 5 failed attempts", key)
	}
}

// RateKey builds the composite limiter key for a request: client IP plus the
// first non-empty identifying value (email or username), lowercased.
func RateKey(c *gin.Context, parts ...string) string {
	key := c.ClientIP()
	for _, p := range parts {
		if p != "" {
			key += ":" + lowerASCII(p)
			break
		}
	}
	return key
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// RateCheck enforces the limiter for key; on rejection it aborts the request
// with 429 and returns false. Handlers call it after binding the body so the
// identifying field is available.
func RateCheck(c *gin.Context, key string) bool {
	if DefaultAuthLimiter.Allow(key) {
		return true
	}
	c.AbortWithStatusJSON(429, gin.H{"success": false, "error": "Too many requests. Please try again later."})
	return false
}

// RecordAuthFailure records a failed credential attempt (bad OTP, bad
// password, missing user) for key.
func RecordAuthFailure(key string) {
	DefaultAuthLimiter.RecordFailure(key)
}
