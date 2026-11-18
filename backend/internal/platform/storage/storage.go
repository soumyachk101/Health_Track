// L3: [INPUT] depends on nothing outside stdlib; the S3/R2 backend
// (S3Store) additionally depends on aws-sdk-go-v2 (service/s3 + presigned-url
// + static-credential credentials) resolved for Cloudflare R2's custom
// endpoint; [OUTPUT] exports the ObjectStorage interface, ObjectInfo, the
// sentinel errors, R2Config/R2ConfigFromEnv, the MIME/size validation
// (ValidMIME, MIMEExt, MaxFileSize) used by the files module, and
// NewS3Store/NewS3StoreFromFuncs; [POS] the private-bucket primitive of
// ADR-0002 — the files module owns keys and validation, this package owns
// the object API; the browser never talks to R2 directly except through
// short-TTL pre-signed URLs minted here; [PROTOCOL] Update this header on
// change, then check CLAUDE.md
package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// ErrNotFound reports a missing object (S3 NotFound / NoSuchKey).
var ErrNotFound = errors.New("object not found")

// ErrMisconfigured reports an R2Config that cannot produce a client.
var ErrMisconfigured = errors.New("r2 storage misconfigured")

// ErrObjectKey reports an invalid object key.
var ErrObjectKey = errors.New("invalid object key")

// MaxFileSize caps one patient document at 25 MiB (ADR-0002).
const MaxFileSize = 25 << 20

// ValidMIME is the upload allowlist: PDF and the three document-scanner
// image formats. Anything else is refused at upload-url and at confirm.
var ValidMIME = map[string]bool{
	"application/pdf": true,
	"image/png":       true,
	"image/jpeg":      true,
	"image/webp":      true,
}

// MIMEExt maps an allowed MIME type to its canonical key extension.
var MIMEExt = map[string]string{
	"application/pdf": ".pdf",
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/webp":      ".webp",
}

// ObjectInfo is the HeadObject summary the confirm flow verifies against
// the medical_files row.
type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	LastModified time.Time
}

// ObjectStorage is the S3-compatible bucket the files module drives. The
// bucket is private: every read is a pre-signed GET minted per request, and
// every write goes through the browser's pre-signed PUT or a server
// PutObject (tests).
type ObjectStorage interface {
	// PutObject uploads an object under key with contentType. The caller
	// validates MIME + size before calling this.
	PutObject(ctx context.Context, key string, body []byte, contentType string) error
	// HeadObject returns the stored object's size + content type, or
	// ErrNotFound. confirm uses it to prove the upload landed and matches
	// the declared size.
	HeadObject(ctx context.Context, key string) (*ObjectInfo, error)
	// GetObject downloads the object body (ai:analyze_document reads the
	// document through this).
	GetObject(ctx context.Context, key string) ([]byte, error)
	// DeleteObject removes the object; missing objects are a no-op.
	DeleteObject(ctx context.Context, key string) error
	// PresignPut mints a browser upload URL (5m TTL in the files module).
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	// PresignGet mints a download URL (2m TTL in the files module).
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// R2Config is the Cloudflare R2 wiring: the S3 endpoint is the account's
// R2 edge, not the AWS global endpoint, so the client must use a custom
// endpoint resolver (see NewS3Store).
type R2Config struct {
	AccountID         string
	Bucket            string
	AccessKeyID       string
	SecretAccessKey   string
	// Endpoint overrides the derived https://<account>.r2.cloudflarestorage.com
	// URL (tests, R2-compatible gateways).
	Endpoint string
}

// R2ConfigFromEnv reads R2_ACCOUNT_ID / R2_BUCKET / R2_ACCESS_KEY_ID /
// R2_SECRET_ACCESS_KEY (+ optional R2_ENDPOINT). ok=false when any
// required value is empty: the storage stays misconfigured and the files
// module degrades to 503s instead of writing to local disk.
func R2ConfigFromEnv() (R2Config, bool) {
	cfg := R2Config{
		AccountID:       strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID")),
		Bucket:          strings.TrimSpace(os.Getenv("R2_BUCKET")),
		AccessKeyID:     strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		SecretAccessKey: strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		Endpoint:        strings.TrimRight(strings.TrimSpace(os.Getenv("R2_ENDPOINT")), "/"),
	}
	if cfg.AccountID == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return cfg, false
	}
	return cfg, true
}

// EndpointURL is the S3 endpoint for this account: the explicit override,
// else the standard R2 account endpoint.
func (c R2Config) EndpointURL() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return "https://" + c.AccountID + ".r2.cloudflarestorage.com"
}

// Validate checks the fields NewS3Store needs; callers get ErrMisconfigured
// rather than a half-built client.
func (c R2Config) Validate() error {
	if c.AccountID == "" || c.Bucket == "" {
		return fmt.Errorf("%w: account id and bucket are required", ErrMisconfigured)
	}
	return nil
}
