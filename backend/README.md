# HealthTrack+ Go Backend

This Go/Gin backend replaced the previous Express/JS implementation.

## Requirements

- Go 1.21+
- A Postgres (or Neon) connection string set as `DATABASE_URL`

## Setup

1. Copy `.env.example` to `.env` and fill in your values:
```bash
cp .env.example .env
```

2. Download dependencies:
```bash
go mod download
```

3. Run the server:
```bash
go run .
```

Or build a binary:
```bash
go build -o main .
./main
```

In production (`NODE_ENV=production`) the server runs Gin in release mode.

## Scripts

One-off admin utilities:

- Create/update the admin user:
```bash
go run ./scripts/setup_admin
```

- List all users:
```bash
go run ./scripts/check_users
```

## Configuration

All variables read by `internal/config/config.go` are documented in `.env.example`:
`PORT`, `NODE_ENV`, `JWT_SECRET`, `DATABASE_URL`, `EMAIL_HOST`, `EMAIL_PORT`,
`EMAIL_HOST_USER`, `EMAIL_HOST_PASSWORD`, `EMAIL_USE_TLS`, `DEFAULT_FROM_EMAIL`,
`FRONTEND_URL`, `GOOGLE_CLIENT_ID`, `GROQ_API_KEY`, `OPENAI_API_KEY`,
`GOOGLE_API_KEY`, `SARVAM_API_KEY`, `CORS_ALLOWED_ORIGINS`.

## API Reference

All authenticated endpoints expect `Authorization: Bearer <token>`.

### Auth (`/accounts/api/`)

| Method | Path | Request body | Response |
|---|---|---|---|
| POST | /accounts/api/login | `{"username", "password"}` | 200 `{"success":true,"token","user":{"id","username","email","role"}}` · 401 `{"success":false,"error":"Invalid credentials"}` · 403 `{"success":false,"error":"Email not verified","email","requires_verification":true}` |
| POST | /accounts/api/register | `{"username","email","password","first_name","last_name","role","provider_type","business_name","license_number","specialization","state"}` | 200 `{"success":true,"email","username","message"}` · 400 `{"success":false,"error"}` |
| POST | /accounts/api/verify-otp | `{"otp","email","otp_type","username"}` | 200 `{"success":true,"token","user"}` · 400 `{"success":false,"error":"Invalid or expired OTP"\|"User not found"}` |
| POST | /accounts/api/resend-otp | `{"otp_type","email","first_name"}` | 200 `{"success":true,"message"}` · 400 `{"success":false,"error"}` |
| POST | /accounts/api/forgot-password | `{"email"}` | 200 `{"success":true}` (always, whether or not the email exists) · 400 `{"success":false,"error":"Email is required"}` · 500 `{"success":false,"error":"Failed to send reset email..."}` |
| POST | /accounts/api/reset-password | `{"email","otp","new_password"}` | 200 `{"success":true,"message"}` · 400/404 `{"success":false,"error"}` |
| POST | /accounts/api/google-login | `{"credential","role"}` | 200 `{"success":true,"token","user"}` · 400 `{"success":false,"error":"Google credential is required"\|"Google authentication failed"}` |

Auth endpoints are rate-limited in memory: 30 requests / 15 min and
5 failures / 5 min per client (locked out 15 min when exceeded) →
429 `{"success":false,"error":"Too many requests. Please try again later."}`.

### Core APIs (`/api/`, JWT required)

| Method | Path | Request body | Response |
|---|---|---|---|
| GET | /api/dashboard | — | 200 `{"user":{"name","email"},"latest_record","active_medicines","active_medicines_count","recent_activities","latest_mental_health"}` |
| GET | /api/health-track | — | 200 `{"records":[{recorded_at, bp fields, blood_sugar, weight, heart_rate, oxygen_level, bp_status}]}` |
| POST | /api/health-track/add | `{"blood_pressure_systolic","blood_pressure_diastolic","blood_sugar","weight","heart_rate","temperature","oxygen_level","notes"}` (all optional) | 200 `{"message":"Health record added successfully"}` |
| GET | /api/medicines | — | 200 `{"medicines":[{name,dosage,frequency_display,start_date,end_date,is_active}],"active_count"}` |
| POST | /api/medicines/add | `{"name","dosage","frequency","start_date","end_date","prescribed_by","notes"}` | 200 `{"message":"Medicine added successfully"}` |
| GET | /api/prescriptions | — | 200 `{"prescriptions":[{prescription_date,doctor_name,hospital_name,diagnosis,follow_up_date}]}` |
| POST | /api/prescriptions/add | `{"doctor_name","hospital_name","diagnosis","prescription_date","follow_up_date","notes"}` | 200 `{"message":"Prescription added successfully"}` |
| GET | /api/profile | — | 200 `{"user":{profile fields},"messages":[{"tags":"success","message"}]}` |
| GET | /api/mental-health | — | 200 `{"avg_mood","logs":[{recorded_at,mood_score,mood_score_display,stress_level_display,sleep_hours,notes}]}` |
| GET | /api/lifestyle | — | 200 `{"logs":[{recorded_at,water_intake,exercise_minutes,steps_count,calories_consumed}]}` |
| GET | /api/insurance | — | 200 `{"policies":[{provider_name,policy_type_display,policy_number,coverage_amount,end_date,is_active}],"active_policies"}` |
| GET | /api/past-records | — | 200 `{"health_records":[...],"prescriptions":[...]}` |
| GET | /api/appointments | — | 200 `{"success":true,"appointments":[{id,patient_name,doctor_name,date,time,reason,status,type,meeting_link}]}` · 403 `{"success":false,"error":"Not a doctor"}` (non-doctor providers) |
| POST | /api/appointments | `{"doctor_id","date","time","reason","type"}` | 200 `{"success":true,"message","id"}` |
| POST | /api/appointments/:appointment_id/action | `{"action":"accept"\|"reject"\|"complete"}` | 200 `{"success":true,"status"}` · 403 `{"success":false,"error":"Permission denied"}` |
| GET | /api/service-requests | — | 200 `{"success":true,"requests":[{id,patient_name,provider_name,service_name,price,status,address,scheduled_date}]}` |
| POST | /api/service-requests | `{"provider_id","service_name","price","address"}` | 200 `{"success":true,"message","id"}` |
| POST | /api/service-requests/:request_id/action | `{"action":"accept"\|"decline"\|"complete"}` | 200 `{"success":true,"status"}` · 403 `{"success":false,"error":"Permission denied"}` |

DB errors on these routes surface as 400 `{"error":"Database request failed. Please try again."}`.

### Chatbot (`/chatbot/`, no auth)

| Method | Path | Request body | Response |
|---|---|---|---|
| POST | /chatbot/api | `{"message"}` | 200 `{"response"}` · 400 `{"error":"No message provided"}` · 503 `{"error":"Server configuration error: API key missing"}` · 500 `{"error":"Chat service error. Please try again later."}` |
| POST | /chatbot/api/tts | `{"text"}` | 200 `{"audio"}` · 400 `{"error":"No text provided"}` · 503 `{"error":"TTS configuration error: API key missing"}` · 500 `{"error":"TTS service encountered an error."}` |

### Admin (`/admin-panel/api/`, JWT + admin role)

| Method | Path | Request | Response |
|---|---|---|---|
| GET | /admin-panel/api/stats | — | 200 `{"success":true,"stats":{"total_users","patients","providers","pending_approvals","total_records"}}` |
| GET | /admin-panel/api/users | query `?type=` & `?search=` | 200 `{"success":true,"users":[{id,username,email,user_type,is_approved,date_joined}]}` |
| POST | /admin-panel/api/users/:user_id/action | `{"action":"approve"\|"reject"\|"delete"}` | 200 `{"success":true}` · 404 `{"success":false,"error":"User not found"}` |

403 `{"success":false,"error":"Admin access required"}` for non-admins.

### Misc

- `GET /api/health` → 200 `{"status":"ok"}` (readiness probe)
- `GET /` → 200 `{"message":"HealthTrack+ API Server is Running"}`
- Unknown routes → 200 `{"message":"HealthTrack+ API Server"}` (GET and all methods,
  matching the legacy no-route handler)
- `GET /uploads/*` → static files from `./uploads`

Auth failure shapes on protected routes: 401 `{"error":"Authorization header missing or invalid"}` /
`{"error":"Token has expired"}` / `{"error":"Invalid token"}` / `{"error":"User not found"}`.

## Deploy

Deployed to Vercel as a Go serverless function via `vercel.json` (`@vercel/go`,
CGO_ENABLED=0 native binary; entrypoint `main.go`); all paths route to the binary.

## Tech Stack

- **Runtime:** Go 1.21+
- **Framework:** Gin
- **Database:** PostgreSQL via pgx v5 (connection pooling)
- **Auth:** JWT HS256 (algorithm pinned) + bcrypt (cost 10)
- **Email:** SMTP via `net/smtp`
- **AI:** Groq / OpenRouter / Sarvam HTTP APIs
