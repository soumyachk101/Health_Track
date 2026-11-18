# Health_Track — Research-Aligned Execution Plan

**Repository:** https://github.com/soumyachk101/Health_Track  
**Reviewed:** 17 November 2026  
**Branch reviewed:** `main`  
**Purpose:** Supersede the generic implementation order in the first Claude Code kit with a plan reconciled against the current repository and `research.md`.

---

## 1. Executive decision

Position HealthTrack+ as a **user-owned household health timeline**: one place for a person or household to keep manually entered vitals, medicines, prescriptions, mood and lifestyle logs, insurance documents, and appointment history.

Do not try to become Practo, Tata 1mg, a wearable vendor, a hospital EMR, or an ABDM-certified PHR in this implementation cycle.

The near-term differentiator is that the user can keep a useful health history without buying a watch/CGM, using one pharmacy, using one hospital, or buying a subscription. Keep export free, do not build a pharmacy checkout, and do not monetize prescriptions through advertising.

### Product principles

1. Manual entry is a first-class path. Sensors are optional.
2. The user owns and controls their records.
3. Vitals, mood logs, insurance records, and personal notes must be labelled as user-entered records unless independently verified.
4. Medicines support schedules and adherence logging; the app does not prescribe or alter a clinician's treatment.
5. AI is a general informational/wellness assistant, not a diagnostic or prescribing clinician.
6. No ABHA is required to use core HealthTrack features.
7. Do not claim ABDM compliance, PHR certification, health-locker certification, or national-record connectivity before the required integration, review, security assessment, and directory listing.
8. No paywall for basic data export.

The research file says 28 research lanes were completed and four lanes were not fully researched. That is adequate to prioritize the current product gaps; the incomplete wearable/fitness lanes are not a reason to delay core improvements.

---

## 2. Current repository baseline

The current public `main` branch contains a Next.js/React/TypeScript frontend and a Go/Gin API backed by PostgreSQL. The backend API is configured through `backend/vercel.json` as a Vercel Go function. The frontend also has Vercel configuration.

There are existing Go unit tests, a PostgreSQL-tagged integration suite, and Vitest frontend tests. The integration suite needs a fresh database DSN. The repository currently has no committed `.github` CI workflow, Docker/Compose setup, versioned database migrations, worker/queue implementation, Redis integration, or object-storage package.

### Important repository/documentation issue

The implementation kit is committed under:

`health_track_claude_implementation_kit/`

The root contains `research.md` and an `AGENTS.md`, but no root `CLAUDE.md`. Claude Code should receive a root-level `CLAUDE.md`; do not assume it will automatically load the nested kit instructions.

The root `AGENTS.md` contains an older agent-memory/context dump from May 2026, including references to the previous Express/Node and split-repository setup. Preserve it as historical context if needed, but do not treat it as the current architecture source of truth. Current source code and the current `research.md` take precedence.

### Verified code-level observations

| Area | Current observation | Why it matters |
|---|---|---|
| Database startup | `database.Connect` executes the embedded `schema.sql` statements at startup | Schema change lifecycle is implicit; introduce controlled versioned migrations before adding feature tables/columns |
| Database model | Tables use text date fields in several places and many `user_id` relationships have no declared FK | Referential integrity and date validation are weaker than they should be |
| Auth | Backend accepts Supabase RS256/ES256 tokens and legacy self-issued HS256 tokens | Two token issuance paths remain active; transition must be deliberate |
| Auth state | Middleware resolves the user but does not centrally reject inactive or unapproved accounts | Account policy must be enforced server-side and consistently |
| OTP | OTP values are stored as raw text; reset flow selects without `is_used` and deletes after password update in a separate statement | Replay/race and partial-update risk; hash and consume atomically |
| Registration | User, provider profile, and OTP are created in separate database operations | Partial registrations can be left behind if a later step fails |
| Input handling | Multiple handlers discard `ShouldBindJSON` errors | Malformed JSON can become zero-value requests instead of a clean 400 response |
| Rate limiting | Current limiter is in process memory and applies to auth flows | Limits do not coordinate across serverless instances; chatbot/TTS are not covered |
| CORS | Any `.vercel.app` and `.netlify.app` subdomain is accepted in addition to configured origins | Production origin policy is wider than necessary |
| Uploads | Router mounts `./uploads` at public `/uploads` | Any sensitive files placed there are publicly addressable; insurance/prescription documents must not use this as private storage |
| Chatbot | `/chatbot/api` and TTS have no authentication; only basic empty-input validation | Public AI endpoints can be abused and do not have user-level quotas |
| AI policy | Current system instruction calls the model a “medical assistant” | This conflicts with research: keep the assistant general and prohibit diagnosis/prescription claims |
| Missing write APIs | Mental-health, lifestyle, and insurance have GET endpoints but no matching create routes in `CoreRouter` | Wiring the frontend buttons alone is insufficient; add backend handlers and tests |
| Medicines | Schema has `frequency`, start/end dates, and active status, but no dose-time schedule or taken/skipped log | Add a schedule and adherence model without removing the existing frequency field |
| Insurance | Schema has a `document` column, but API response does not include the document; UI “Expiring Soon” is a placeholder | Build secure document handling and calculate expiry from the stored end date |
| Exports | No CSV/PDF export API is present in the current route inventory | Add user-authorized exports with bounded queries and safe file generation |
| Frontend sessions | Supabase and legacy tokens are stored in localStorage; some dashboard pages call `fetch` directly and read the legacy `token` key | Centralize session/API use first; only migrate to cookies/BFF after production origin and compatibility decisions |
| Voice locale | Browser speech recognition is configured for `en-US`; Sarvam TTS is hard-coded to `hi-IN` | Research's Indian-language opportunity is not yet a complete Bengali/Hindi/localization feature; add explicit locale selection and test supported locales later |
| Admin | Health Data, Reports, and Settings pages route to a common AdminView placeholder | Treat these as deferred product work, not functioning admin capabilities |
| Deployment | Backend is configured as a Vercel Go function, not Cloud Run | Keep current deployment while stabilizing; do not migrate platforms without a measured reason |

No production environment, live database, secrets, deployed runtime, or load test was inspected. These are static-source findings, not a penetration test or production certification.

---

## 3. Updated implementation sequence

The work is reordered around two constraints:

- Fix the dangerous platform/data handling before expanding the amount of sensitive information stored.
- Deliver the research-backed product gaps before investing in expensive integrations or broad architectural rewrites.

### Phase 0 — Reconcile instructions and establish a verified baseline

**Scope:** documentation and audit only; no application source changes.

Tasks:
1. Add a root `CLAUDE.md` based on `health_track_claude_implementation_kit/CLAUDE.md`.
2. Place the kit's documents under root `docs/` (merge carefully; do not create duplicate nested documentation).
3. Keep `research.md` at the root and explicitly tell Claude Code to read it as the product strategy and feature-priority source.
4. Update the master prompt to name `research.md`, this plan, and the actual current deployment model.
5. Label `AGENTS.md` as historical agent context or archive it without deleting useful history.
6. Inventory all frontend routes, backend endpoints, tables, auth paths, external providers, upload paths, tests, and deployment config.
7. Run only safe local checks; record commands and outcomes. Do not use production data.
8. Produce a current API inventory, data model inventory, security findings, and feature backlog.

**Acceptance criteria:**
- Claude can identify the current Go/Gin + PostgreSQL backend and Next.js frontend correctly.
- Claude has read both `research.md` and the implementation requirements.
- Every finding is marked confirmed, runtime-verification-needed, or product-decision-needed.
- No application code or production resources are changed.

### Phase 1 — Security and correctness baseline

**P0/P1: complete before adding more sensitive data workflows.**

#### 1A. Authentication and account state
- Map every login path: email/password, Google, OTP, Supabase sign-in, and legacy token issuance.
- Document the identity source of truth and choose one supported long-term flow.
- Keep a backwards-compatible transition; do not revoke all legacy sessions in one unplanned step.
- Validate token signature, allowed algorithm, issuer, audience, expiry, and subject.
- Enforce account active/verification/approval policy in backend middleware or policy services. Pending providers may have a restricted onboarding experience, but must not get approved-provider privileges.
- Do not trust frontend role state or Supabase `user_metadata` as backend authorization.
- Decide whether a same-site BFF/HttpOnly cookie session is feasible for the actual frontend and API domains. Do not implement a half-migration that breaks refresh, logout, OAuth, or cross-origin requests.

#### 1B. OTP, registration, and password reset
- Hash OTPs before storage.
- Bind each OTP to email, purpose, expiry, and attempt limit.
- Use a transaction and conditional one-time consumption (`is_used = 0` / atomic delete-returning / row lock) so parallel requests cannot reuse it.
- Update the password and consume the reset token in the same transaction.
- Create the user, provider profile, and registration OTP atomically where possible.
- Make malformed JSON return 400; never ignore JSON binding errors.
- Keep recovery responses resistant to account enumeration.

#### 1C. Sensitive access and uploads
- Audit every health-record, medicine, prescription, insurance, appointment, admin, and service-request route for ownership and role enforcement.
- Remove public access to sensitive upload paths. Do not store patient documents in a publicly mounted directory.
- Until private object storage is ready, do not expose new sensitive file uploads through a public URL.
- Do not allow blanket admin access to clinical record contents by default.
- Redact access tokens, OTPs, passwords, health payloads, and reset links from logs.

#### 1D. AI endpoints
- Add authentication to chat as recommended by the research, but keep it a general assistant.
- Add per-user and per-IP quotas, strict message length limits, provider timeouts, and safe fallback handling.
- The assistant must not diagnose, rank possible diseases, prescribe, change dose instructions, or claim to be a certified medical device.
- Add emergency-intent handling before normal model generation. For India launch copy, show the emergency information agreed by the product owner (research proposes 112 and 108) and tell users to contact emergency services/qualified clinicians. Verify localized emergency guidance before launch.
- Only send chart data to AI through an explicit, user-visible, limited context-sharing flow. Default to no chart context; list which records are being shared and allow revocation. Authentication alone is not consent to send a complete chart to an external model.
- Update the privacy page to accurately describe the data sent to Groq/OpenRouter/Sarvam and the user's choices.

**Acceptance criteria:**
- Regression tests cover inactive/unapproved accounts, cross-user IDOR, OTP replay/concurrency, malformed payloads, and chatbot quota/auth.
- No unauthenticated user can access private records or private documents.
- Chat assistant behavior and UI do not claim diagnosis or prescribing ability.

### Phase 2 — Database migration foundation

Do this before adding medicine schedule tables or new feature columns.

Tasks:
1. Select one migration system (for example Goose, Atlas, or golang-migrate) after checking project compatibility.
2. Create a baseline strategy for both fresh databases and existing databases already initialized by `schema.sql`.
3. Verify existing schema before marking a legacy database as baselined.
4. Keep the current startup schema path until the migration process is proven; then remove automatic DDL from application startup in a controlled release.
5. Scan for null, duplicate, orphaned, and malformed legacy data before adding FK/unique/check constraints.
6. Add foreign keys and constraints in separate safe migrations, using expand → backfill → validate → contract.
7. Add typed date/time fields gradually. First settle timezone semantics; do not silently parse ambiguous legacy strings.
8. Make pool sizes configurable and document total PostgreSQL connection budget across Vercel instances/workers.
9. Add pagination and maximum page sizes for all potentially growing list endpoints.

**Acceptance criteria:**
- Fresh database can be created from migrations.
- Existing database can be adopted without data loss.
- Migration upgrade and recovery procedures are tested on a disposable database.
- Application startup does not unexpectedly mutate production schema after migration cutover.

### Phase 3 — Research-backed product MVP

Implement in this order. Each sub-feature should be its own reviewable change with backend, frontend, and tests.

#### 3A. Mental-health, lifestyle, and insurance entry
Current schema tables exist; current API routes expose reads but not the required create operations.

Add:
- `POST /api/mental-health` (or an explicitly versioned equivalent).
- `POST /api/lifestyle`.
- `POST /api/insurance`.
- Validated frontend forms and submit/error/success states.
- Ownership derived from authenticated principal, never accepted from the client.
- Input bounds and valid numeric ranges; preserve nullable measurements.
- Tests for valid and invalid data and cross-user access.

Acceptance: the three current “Feature coming soon” buttons create records and refreshed lists show the new record.

#### 3B. Insurance expiry and documents
- Calculate “Expiring Soon” from policy end date and active status.
- Choose and document a default window (for example 30 days); make it configurable if product requires it.
- Show expired, active, and expiring states consistently.
- Include document presence and a secure view/download action on each policy row.
- Store file metadata and private object keys, not public permanent URLs.
- Mark whether a policy was user-entered or issued/imported by an insurer.
- Keep claims and settlement with the insurer; HealthTrack is a personal record, not an insurer adjudication service.

Acceptance: expiry card is calculated from data; a user can only access their own policy and file; expired policies remain in history.

#### 3C. Medicine schedules and adherence
Keep the existing frequency field for backwards compatibility.

Add a normalized schedule model that can represent one or more local dose times per medicine, plus timezone and effective dates as needed. Add a dose log for each scheduled occurrence with states such as taken or skipped. Avoid storing all adherence state in one boolean on the medicine row.

MVP:
- Configure clock time(s) when adding/editing a medicine.
- Display today's schedule.
- Allow the user to mark a dose taken or skipped.
- Show adherence history.
- Keep refill countdown separate from dose reminders.
- Do not alter the prescribed dosage or recommend medication changes.
- Browser/PWA push reminders can be a later opt-in feature; the core schedule and log must work without push permission.

Acceptance: schedules survive reload, duplicate taps do not create duplicate dose events, timezone behavior is defined, and a user cannot read another user's medicine schedule.

#### 3D. Free exports
- CSV export for vitals, medicines, and prescriptions.
- Doctor-ready PDF summary with latest blood pressure, latest blood sugar, active medicines, latest prescription, units, and generated timestamp.
- Export only the requesting user's permitted records.
- Add explicit empty states and clear date ranges.
- Prevent CSV formula injection in cells beginning with spreadsheet formula characters.
- Bound query sizes and avoid loading the entire database into memory for large exports.
- Label the document “user-maintained HealthTrack record”; do not imply it is an official hospital chart.

Acceptance: exports open correctly, are complete for the selected scope, contain no other user's data, and remain free.

#### 3E. Dashboard improvements
- Add latest sleep-hours value/summary from the existing mental-health log.
- Show data timestamp and source so users can distinguish a current measurement from an older entry.
- Never infer sleep stages from sleep-hours data.

Acceptance: dashboard shows sleep hours when present and a helpful empty state when absent.

### Phase 4 — Household and care-sharing workflows

Do not implement this before access grants and audit trails are designed.

- Add explicit caregiver sharing with view-only default permissions.
- Share only selected data categories and date ranges.
- Grants must have an expiry and a revoke action.
- Record access events without copying medical payloads into logs.
- Add a user-controlled report-sharing workflow for a specific doctor/visit.
- Do not grant a caregiver or doctor blanket access based only on knowing a patient ID.
- Define whether household members are separate users and how invitation/identity verification works.

Appointments:
- Existing booking and status endpoints exist, but there is no clearly defined price/cancellation policy model in the current schema.
- Do not invent prices. Add a displayed price and cancellation rule only after a provider pricing/product decision.
- Store a short visit note after completion with clear author/source metadata.
- Enforce valid appointment status transitions and concurrency-safe slot booking.

Acceptance: two unrelated users cannot access one another's records; granted users can see only the scope granted; revocation takes effect; access is auditable.

### Phase 5 — Integrations and measured scaling

#### Health Connect
- Defer until there is an Android native client or an appropriate native bridge. A regular Next.js web page cannot simply read the on-device Android Health Connect database.
- Manual user entry wins when imported readings conflict, as specified by research.
- Store source metadata for imported data and allow unlink/delete.
- Do not require a watch or sensor.

#### ABDM / ABHA
- Defer P1–P3 integration until product ownership, consent flows, legal/privacy review, sandbox access, technical review, security assessment, and directory listing requirements are established.
- Keep personal manual logs separate from ABDM-linked care-context records.
- Do not require ABHA for normal HealthTrack use.
- Do not display “ABDM-compliant” or certification badges before approval.

#### Infrastructure and scaling
- Keep the modular monolith.
- Keep the current Vercel deployment while measuring real constraints; do not migrate to Cloud Run just because it appeared in the original generic target architecture.
- Add CI early: Go formatting/vet/tests, frontend lint/typecheck/tests/build, dependency and secret scans.
- Configure database pool limits for serverless concurrency.
- Use a shared rate-limit store/provider when multiple instances need common quotas.
- Introduce private object storage for documents.
- Add a queue/worker only for work that genuinely needs retries or must outlive a request (reminders, email, PDF generation at scale).
- Add structured logs, redaction, metrics, error alerts, backup/PITR, and restore tests.
- Add language selection for browser speech recognition and TTS only for verified supported locales; do not advertise Bengali support merely because Sarvam is configured.
- Consider Cloud Run/container hosting only if measured requirements (long-lived processes, workers, connection management, networking, or platform limits) justify it.
- Do not introduce microservices, Kubernetes, sharding, or event sourcing without evidence.

Acceptance: system behavior is observable; connection limits are safe; backup restore and deployment rollback have been tested; performance changes are supported by measurements.

---

## 4. Suggested delivery order

| Order | Deliverable | Priority |
|---|---|---|
| 0 | Root Claude instructions + reconciled audit | Must do first |
| 1 | Auth/account-state/OTP/input hardening | P0 |
| 2 | Secure upload path, AI endpoint controls, exact CORS and shared quotas | P0/P1 |
| 3 | Versioned migration baseline and migration test strategy | P1 |
| 4 | Mental-health, lifestyle, and insurance create flows | Product MVP |
| 5 | Insurance expiry and private document view | Product MVP |
| 6 | Medicine clock schedules and taken/skipped log | Product MVP |
| 7 | CSV exports and doctor-ready PDF | Product MVP |
| 8 | Dashboard sleep summary | Product MVP |
| 9 | Caregiver consent, revocable sharing, appointment improvements | P2 |
| 10 | Health Connect, ABDM, advanced deployment changes | Later |

One developer can treat this as a multi-iteration project, not a single large Claude Code run. Estimate effort only after Phase 0 confirms testability and current feature completeness.

---

## 5. Architecture decisions to record before coding

Create ADRs for:
1. Auth source of truth and migration from dual token paths.
2. Frontend/API production domains and cookie/BFF feasibility.
3. Patient/provider/administrator access policy.
4. Caregiver identity, data scopes, expiry, and revocation.
5. Private document storage and retention.
6. Medicine schedule/timezone/dose-log semantics.
7. Migration tool and legacy-database baselining.
8. AI chart-context consent and provider data-sharing policy.
9. Vercel serverless limits, PostgreSQL connection budget, and whether/when to move the API.
10. ABDM product scope and certification claims.

Do not create empty modules just to match an ideal folder tree. Extract handler/service/repository boundaries gradually as features are touched.

---

## 6. Explicit non-goals for this cycle

- Pharmacy marketplace, medicine checkout, or medicine delivery.
- Diagnosing symptoms, naming likely diseases, prescribing medicines, or changing doses.
- Weight-loss subscription, GLP-1 sales, or advertising based on prescription data.
- US-only medicine interaction checks applied to arbitrary Indian free-text medicine names.
- Mandatory smartwatch, ring, CGM, or ABHA.
- ABDM certification claims before approvals.
- Broad admin access to personal clinical records.
- Microservices/Kubernetes without a measured need.
- Charging users for CSV/PDF exports.

---

## 7. Claude Code execution instruction

Use the repository's **current checked-out source** as the code truth. Read `research.md`, the root `CLAUDE.md`, and this plan before implementation. Use the original kit documents as supporting checklists, but this research-aligned plan supersedes their generic phase ordering where they conflict.

Start with Phase 0 only. Do not change application source code, dependencies, schema, deployment configuration, or production resources during Phase 0. Produce the audit and backlog, identify runtime-only unknowns, then stop for review.

After approval, implement one numbered phase at a time. For every phase:
- Show intended files and acceptance criteria before edits.
- Preserve existing workflows and API compatibility.
- Add tests with the change.
- Run and report exact test/build/security commands.
- Inspect the full diff and check for secrets.
- Update `docs/PROGRESS.md`.
- Stop at the phase boundary for review.

Never claim a test, scan, deployment, certification, or compliance result that was not actually verified.
