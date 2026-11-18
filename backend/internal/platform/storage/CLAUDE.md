# platform/storage/

> L2 | Parent: backend/ (see CLAUDE.md)

S3-compatible object storage for Cloudflare R2 (ADR-0002). Private bucket:
no public mount; every read/write is either a short-TTL pre-signed URL or a
server-side call. The files module owns keys + MIME/size validation; this
package owns the object API.

## Member list

- `storage.go`: ObjectStorage interface (Put/Head/Get/Delete/PresignPut/PresignGet),
  ObjectInfo, sentinel errors (ErrNotFound/ErrMisconfigured/ErrObjectKey),
  R2Config + R2ConfigFromEnv (env wiring: R2_ACCOUNT_ID / R2_BUCKET /
  R2_ACCESS_KEY_ID / R2_SECRET_ACCESS_KEY, optional R2_ENDPOINT override),
  MIME allowlist (ValidMIME: pdf/png/jpeg/webp) + MIMEExt + MaxFileSize (25 MiB).
- `s3.go`: S3Store — the concrete R2 backend; aws-sdk-go-v2 client with a
  custom BaseEndpoint (the account R2 edge, not the AWS global endpoint) +
  static keypair credentials; builds through the S3API/S3Presigner seams so
  tests inject fakes.
- `apifake.go`: S3API / S3Presigner seams + FakeS3API (in-memory object map,
  canned presign URLs) — the "mocked S3 API" test path.
- `fake.go`: FakeObjectStorage — in-memory ObjectStorage for handler-level
  tests (no aws-sdk types needed).

## Invariants

- Object keys are flat (`<owner-id>/<uuid><ext>`); `validateKey` rejects
  absolute keys and `..` segments.
- A misconfigured R2_* set yields a nil store at the call site, never a
  half-built client: `R2ConfigFromEnv` returns `ok=false` and the files
  routes degrade to 503.

[PROTOCOL]: Update this header on change, then check CLAUDE.md
