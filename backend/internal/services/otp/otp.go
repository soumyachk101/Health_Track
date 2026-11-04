// Package otp ports generateOtp from routes/auth.js and the ISO expiry
// computation used when writing otps.expires_at.
package otp

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"time"
)

// Generate returns a 6-digit code in [100000, 999999], mirroring
// Math.floor(100000 + Math.random()*900000).
func Generate() string {
	n, err := rand.Int(rand.Reader, big.NewInt(900000)) // 0..899999
	if err != nil {
		return "100000"
	}
	return strconv.Itoa(100000 + int(n.Int64()))
}

// ExpiryISO returns now.Add(minutes) in UTC as RFC3339, matching
// new Date(Date.now()+m*60*1000).toISOString().
func ExpiryISO(now time.Time, minutes int) string {
	return now.Add(time.Duration(minutes) * time.Minute).UTC().Format(time.RFC3339)
}
