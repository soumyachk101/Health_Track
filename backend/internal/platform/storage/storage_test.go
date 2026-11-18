package storage

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// TestStorageValidateKey pins the key safety rules: flat, relative, no
// traversal.
func TestStorageValidateKey(t *testing.T) {
	ok := []string{"42/abc.pdf", "files/1/scan.png"}
	bad := []string{"", "/abs.txt", "a/../b", "..", "a/../b.pdf"}
	for _, k := range ok {
		if err := validateKey(k); err != nil {
			t.Errorf("validateKey(%q) = %v, want nil", k, err)
		}
	}
	for _, k := range bad {
		if err := validateKey(k); err == nil {
			t.Errorf("validateKey(%q) = nil, want ErrObjectKey", k)
		}
	}
}

// TestStorageMIMEValidation pins the allowlist and the size cap.
func TestStorageMIMEValidation(t *testing.T) {
	allowed := []string{"application/pdf", "image/png", "image/jpeg", "image/webp"}
	for _, m := range allowed {
		if !ValidMIME[m] {
			t.Errorf("ValidMIME missing %q", m)
		}
		if MIMEExt[m] == "" {
			t.Errorf("MIMEExt missing %q", m)
		}
	}
	forbidden := []string{"image/gif", "text/plain", "application/zip", "image/svg+xml"}
	for _, m := range forbidden {
		if ValidMIME[m] {
			t.Errorf("ValidMIME accepts %q", m)
		}
	}
	if MaxFileSize != 25<<20 {
		t.Errorf("MaxFileSize = %d, want 25MiB", MaxFileSize)
	}
}

// TestFakeStorageCRUD drives the full ObjectStorage lifecycle through the
// in-memory fake: put → head (size/type) → delete → head 404.
func TestFakeStorageCRUD(t *testing.T) {
	ctx := context.Background()
	fake := NewFakeObjectStorage()

	if err := fake.PutObject(ctx, "42/doc.pdf", []byte("%PDF-1.4..."), "application/pdf"); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	info, err := fake.HeadObject(ctx, "42/doc.pdf")
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if info.Size != 11 || info.ContentType != "application/pdf" {
		t.Errorf("head = %+v, want size 11, pdf", info)
	}

	if err := fake.DeleteObject(ctx, "42/doc.pdf"); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if _, err := fake.HeadObject(ctx, "42/doc.pdf"); !errors.Is(err, ErrNotFound) {
		t.Errorf("post-delete head err = %v, want ErrNotFound", err)
	}

	put, err := fake.PresignPut(ctx, "42/scan.png", "image/png", 300_000_000_000)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}
	if !strings.Contains(put, "r2.test") {
		t.Errorf("put url = %q", put)
	}
	get, err := fake.PresignGet(ctx, "42/scan.png", 120_000_000_000)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	if !strings.Contains(get, "r2.test") {
		t.Errorf("get url = %q", get)
	}
}

// TestS3StoreHeadNotFound proves the mocked-S3-API path maps both NotFound
// and NoSuchKey onto ErrNotFound.
func TestS3StoreHeadNotFound(t *testing.T) {
	ctx := context.Background()
	api := NewFakeS3API()
	store := NewS3StoreFromAPI(api, api, "bucket")

	if _, err := store.HeadObject(ctx, "42/missing.pdf"); !errors.Is(err, ErrNotFound) {
		t.Errorf("HeadObject missing = %v, want ErrNotFound", err)
	}

	if err := store.PutObject(ctx, "42/x.pdf", []byte("%PDF"), "application/pdf"); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	info, err := store.HeadObject(ctx, "42/x.pdf")
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if info.ContentType != "application/pdf" {
		t.Errorf("content type = %q", info.ContentType)
	}
}

// TestS3StorePresign pins that presigned URLs flow through the presigner
// seam (the fake returns its canned URL).
func TestS3StorePresign(t *testing.T) {
	ctx := context.Background()
	api := NewFakeS3API()
	api.PresignGet = func(key string, ttl int64) string { return "https://canned/" + key }
	store := NewS3StoreFromAPI(api, api, "b")

	get, err := store.PresignGet(ctx, "7/k.png", 120_000_000_000)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	if get != "https://canned/7/k.png" {
		t.Errorf("get = %q", get)
	}

	put, err := store.PresignPut(ctx, "7/k.png", "image/png", 300_000_000_000)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}
	if put == "" {
		t.Error("empty put url")
	}
}

// TestR2ConfigFromEnv pins the env contract: all four R2_* vars required.
func TestR2ConfigFromEnv(t *testing.T) {
	t.Setenv("R2_ACCOUNT_ID", "acct")
	t.Setenv("R2_BUCKET", "bkt")
	t.Setenv("R2_ACCESS_KEY_ID", "id")
	t.Setenv("R2_SECRET_ACCESS_KEY", "sec")
	cfg, ok := R2ConfigFromEnv()
	if !ok {
		t.Fatal("want ok")
	}
	if cfg.EndpointURL() != "https://acct.r2.cloudflarestorage.com" {
		t.Errorf("endpoint = %q", cfg.EndpointURL())
	}
	os.Setenv("R2_ENDPOINT", "https://gw.test/")
	defer os.Unsetenv("R2_ENDPOINT")
	cfg, _ = R2ConfigFromEnv()
	if cfg.EndpointURL() != "https://gw.test" {
		t.Errorf("override endpoint = %q", cfg.EndpointURL())
	}
}
