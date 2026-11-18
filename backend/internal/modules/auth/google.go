package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

// GoogleClaims is the verified subset of a Google ID token the new module
// needs (email + sub + names + picture).
type GoogleClaims struct {
	Email   string
	Sub     string
	Given   string
	Family  string
	Picture string
}

// ErrNoUser is returned when provisioning could not find the created row.
var ErrNoUser = errors.New("auth: user not found")

var (
	googleJwksMu    sync.Mutex
	googleJwksCache map[string][]jwk
	googleJwksHtt   = &http.Client{Timeout: 15 * time.Second}
)

type jwk struct {
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// VerifyGoogleIDToken verifies a Google-issued ID token against Google's
// JWKS (signature + audience + expiry + issuer), the same checks the legacy
// google-login handler did. Pinned to stdlib so no new dep is introduced.
func (s *Service) VerifyGoogleIDToken(ctx context.Context, credential, clientID string) (GoogleClaims, error) {
	if clientID == "" {
		return GoogleClaims{}, errors.New("google client id not configured")
	}
	parts := strings.Split(credential, ".")
	if len(parts) != 3 {
		return GoogleClaims{}, errors.New("malformed id token")
	}
	headerB64, payloadB64, sigB64 := parts[0], parts[1], parts[2]

	headerJSON, err := base64.RawURLEncoding.DecodeString(headerB64)
	if err != nil {
		return GoogleClaims{}, err
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return GoogleClaims{}, err
	}
	if header.Alg != "RS256" {
		return GoogleClaims{}, errors.New("unsupported id token algorithm")
	}

	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return GoogleClaims{}, err
	}
	key, err := googleJWK(ctx, clientID, header.Kid)
	if err != nil {
		return GoogleClaims{}, err
	}
	mac := sha256.Sum256([]byte(headerB64 + "." + payloadB64))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, mac[:], sig); err != nil {
		return GoogleClaims{}, errors.New("invalid id token signature")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return GoogleClaims{}, err
	}
	var raw map[string]any
	if err := json.Unmarshal(payloadJSON, &raw); err != nil {
		return GoogleClaims{}, err
	}
	var out GoogleClaims
	out.Email, _ = raw["email"].(string)
	out.Sub, _ = raw["sub"].(string)
	out.Given, _ = raw["given_name"].(string)
	out.Family, _ = raw["family_name"].(string)
	out.Picture, _ = raw["picture"].(string)

	if out.Email == "" {
		return GoogleClaims{}, errors.New("id token missing email")
	}
	switch aud := raw["aud"].(type) {
	case string:
		if aud != clientID {
			return GoogleClaims{}, errors.New("id token audience mismatch")
		}
	case []any:
		found := false
		for _, a := range aud {
			if s, ok := a.(string); ok && s == clientID {
				found = true
				break
			}
		}
		if !found {
			return GoogleClaims{}, errors.New("id token audience mismatch")
		}
	default:
		return GoogleClaims{}, errors.New("id token missing audience")
	}
	if exp, ok := raw["exp"].(float64); !ok || time.Now().Unix() >= int64(exp) {
		return GoogleClaims{}, errors.New("id token expired")
	}
	if iss, _ := raw["iss"].(string); iss != "https://accounts.google.com" && iss != "accounts.google.com" {
		return GoogleClaims{}, errors.New("id token issuer mismatch")
	}
	return out, nil
}

func googleJWK(ctx context.Context, clientID, kid string) (*rsa.PublicKey, error) {
	googleJwksMu.Lock()
	if keys, ok := googleJwksCache[clientID]; ok && len(keys) > 0 {
		googleJwksMu.Unlock()
		for _, k := range keys {
			if k.Kid == kid {
				return jwkToRSA(k)
			}
		}
	}
	googleJwksMu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/certs", nil)
	if err != nil {
		return nil, err
	}
	resp, err := googleJwksHtt.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch google jwks")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, err
	}
	googleJwksMu.Lock()
	googleJwksCache = map[string][]jwk{clientID: set.Keys}
	googleJwksMu.Unlock()

	for _, k := range set.Keys {
		if k.Kid == kid {
			return jwkToRSA(k)
		}
	}
	return nil, errors.New("google jwks missing key")
}

func jwkToRSA(k jwk) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}
	if len(eb) == 0 {
		return nil, errors.New("google jwks empty exponent")
	}
	e := 0
	for _, b := range eb {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}, nil
}
