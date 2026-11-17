# Security and Privacy Requirements

This document is an engineering baseline, not a certification or legal opinion.

## 1. Identity and sessions
- Use one authoritative identity provider.
- Validate token signature, issuer, audience, expiry, algorithm, and required claims.
- Resolve application role and account status server-side.
- Rotate/revoke sessions according to a documented policy.
- Prefer an HttpOnly, Secure, SameSite cookie-backed session/BFF model where compatible.
- If cookies are used, design CSRF protection and origin validation.
- Do not treat localStorage token obfuscation as a security control.
- Do not remove legacy auth before session expiry/migration and rollback are understood.

## 2. Authorization
Every sensitive route must verify:
- Authenticated principal.
- Active account.
- Required role/permission.
- Resource ownership.
- Patient consent and provider relationship where applicable.
- Organization/tenant membership where applicable.
- Allowed operation on the resource's current state.

Test both positive and negative authorization paths. Include IDOR/BOLA tests with two unrelated users.

## 3. Request handling
- Strict JSON decoding; reject malformed or unknown fields where appropriate.
- Validate required fields, enum membership, string lengths, numeric bounds, dates, and cross-field invariants.
- Set request body, upload, and response limits.
- Use parameterized queries.
- Avoid leaking SQL, stack traces, internal paths, or upstream secrets in API errors.

## 4. Password and OTP flows
- Use a suitable password hashing algorithm and reviewed cost parameters.
- Store OTP/reset tokens as hashes.
- Bind tokens to user, purpose, and expiry.
- Limit attempts and resend frequency.
- Consume atomically exactly once.
- Make password update and token consumption transactional.
- Revoke or invalidate existing sessions after password reset according to policy.
- Keep account enumeration-resistant responses for recovery flows.

## 5. Rate limiting and abuse prevention
Apply shared/distributed limits across instances to:
- Login and password verification.
- Signup and OTP generation/verification.
- Password reset.
- AI/chat and TTS.
- Uploads and expensive reports.
- Public endpoints susceptible to scraping.

Use per-IP plus account/identifier dimensions carefully; do not create easy account-lockout denial-of-service.

## 6. Health data privacy
- Minimize collected data.
- Define purpose, retention, deletion, export, and consent semantics.
- Restrict staff/admin access to the minimum necessary.
- Audit sensitive record reads and writes.
- Do not include medical payloads in logs, analytics, traces, or error messages.
- Separate production data from development/test fixtures.
- Use synthetic data in tests.
- Encrypt transport and managed storage; document encryption-at-rest configuration and key management.

## 7. File security
- Private buckets by default.
- Random, non-guessable object keys.
- Validate actual file type, not only extension or client MIME.
- Enforce byte-size limits.
- Scan files where feasible.
- Authorize ownership before signed URL generation.
- Use short-lived signed URLs.
- Define lifecycle and deletion behavior.

## 8. AI safety
- Treat external model providers as data processors requiring privacy review.
- Send minimum necessary information.
- Redact identifiers where possible.
- Apply quotas, length limits, timeout, retry budget, and cost controls.
- Do not present generated text as diagnosis or prescription.
- Provide clear non-clinical assistance disclaimer.
- Handle model/provider outages safely.
- Do not allow prompt content to override system safety and platform policy.

## 9. Operational security
- Store secrets in environment-specific secret management.
- Rotate exposed credentials; do not include actual values in docs.
- Redact authorization headers, cookies, OTPs, passwords, health fields, and provider keys.
- Log a request/correlation ID, route, status, and latency.
- Keep admin actions auditable.
- Review dependencies and container images.
- Maintain incident response and backup restore procedures.

## 10. Release security gate
No release with:
- Unresolved critical/high authorization defect.
- Exposed secret.
- Unprotected patient record endpoint.
- Unbounded upload or expensive public AI endpoint.
- Unknown migration rollback path.
- Unverified backup/recovery for production data.
