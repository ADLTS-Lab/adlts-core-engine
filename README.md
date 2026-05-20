# ADLTS Core Engine

ADLTS Core Engine is the Go backend for the Automated Driving License Testing System. It is organized as a modular monolith with a single HTTP process, PostgreSQL persistence, JWT authentication, email delivery, and public media serving.

## Prerequisites

- Go 1.22 or later
- PostgreSQL 14+ with the schema in `migrations/001_schema.sql`
- Optional SMTP credentials for OTP, password reset, and invitation emails

## Local Setup

1. Copy the environment file or create a local `.env` file.
2. Set `DATABASE_URL` and `JWT_SECRET` at minimum.
3. Apply the migration in `migrations/001_schema.sql` to your database.
4. Start the server:

The API listens on `PORT` and defaults to `8080`.

## Environment Variables

| Variable | Required | Description |
| --- | --- | --- |
| `DATABASE_URL` | Yes | PostgreSQL connection string used by the API and tests |
| `JWT_SECRET` | Yes | Secret used to sign and verify access tokens |
| `PORT` | No | HTTP port, default `8080` |
| `INTERNAL_API_KEY` | No | Shared secret for internal endpoints |
| `SUPER_ADMIN_NAME` | No | Seeded root super-admin name, default `Root Admin` |
| `SUPER_ADMIN_EMAIL` | No | Seeded root super-admin email, default `root@adlts.et` |
| `SUPER_ADMIN_PASSWORD` | No | Seeded root super-admin password |
| `UPLOADS_DIR` | No | Base directory for uploaded media, default `../uploads` |
| `MEDIA_MAX_SIZE_MB` | No | Maximum upload size, default `5` |
| `FRONTEND_BASE_URL` | No | Frontend base URL for payment return links |
| `CHAPA_SECRET_KEY` | No | Chapa API secret key |
| `CHAPA_WEBHOOK_SECRET` | No | Chapa webhook signature secret |
| `CHAPA_BASE_URL` | No | Chapa API base URL, default `https://api.chapa.co/v1` |
| `SMTP_HOST` | No | SMTP server host |
| `SMTP_PORT` | No | SMTP server port, default `587` |
| `SMTP_USER` | No | SMTP username |
| `SMTP_PASSWORD` | No | SMTP password |
| `SMTP_FROM` | No | Sender address |
| `SMTP_FROM_NAME` | No | Sender display name, default `ADLTS` |
| `TEST_DATABASE_URL` | No | Optional database URL for identity tests |

If SMTP variables are omitted, mail delivery is skipped in development mode.

## API Surface

Public endpoints:

- `GET /health`
- `GET /uploads/*`

Versioned API root: `/api/v1`

### Auth

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| POST | `/auth/candidates/register` | Public | Candidate self-registration and OTP trigger |
| POST | `/auth/candidates/verify-otp` | Public | Verify OTP and activate account |
| POST | `/auth/candidates/resend-otp` | Public | Resend verification code |
| POST | `/auth/invitations/accept` | Public | Complete invitation-based registration |
| POST | `/auth/login` | Public | Return access and refresh tokens |
| POST | `/auth/logout` | Authenticated | Client-side logout acknowledgement |
| POST | `/auth/token/refresh` | Authenticated | Token refresh placeholder |
| POST | `/auth/password/forgot` | Public | Send password reset link |
| POST | `/auth/password/reset` | Public | Set a new password from reset token |
| PATCH | `/auth/password/change` | Authenticated | Change own password |

### Identity Resources

- `candidates`: list, self profile, admin read/update, status change, photo upload, and deletion flows
- `experts`: list, self profile, super-admin read/update, status change, photo upload, and deletion flows
- `institutes`: list, self profile, admin read/update, status change, logo upload, and deletion flows
- `transport-authorities`: list, self profile, super-admin read/update, status change, logo upload, and deletion flows
- `admins`: list, self profile, super-admin read/update, status change, and deletion flows
- `super-admins`: list, self profile, update, and deletion flows restricted to super-admins
- `invitations`: create, list, retrieve, resend, and cancel

At present, `internal/identity` is the only mounted API module. The remaining domains in `internal/` are scaffolded for later modules.
## Payment Gateway

Provider: Chapa (https://chapa.co) via hosted checkout.

Chapa supports Telebirr, CBE Birr, Amole, Visa/Mastercard, and bank transfers on their hosted page. This backend only uses:

1. `POST /transaction/initialize` to get a checkout URL
2. `GET /transaction/verify/{tx_ref}` to confirm payment status

Known limitations (out of scope for v1): direct integrations with Telebirr, CBE Birr, Awash, Dashen/Amole, or Stripe.

## Architectural Organization

### Current repository layout

```text
adlts-backend/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── app.go
│   ├── domain/
│   ├── identity/
│   └── platform/
│       ├── config/
│       ├── db/
│       ├── httpx/
│       ├── mailer/
│       ├── media/
│       └── security/
├── migrations/
└── tests/
```

### Target modular organization

The codebase is evolving toward a broader backend split:

```text
adlts-backend/
├── cmd/
│   ├── api/
│   ├── worker/
│   └── stream-gateway/
├── internal/
│   ├── app/
│   ├── platform/
│   ├── identity/
│   ├── booking/
│   ├── sessions/
│   ├── streaming/
│   ├── detection/
│   ├── scoring/
│   ├── appeals/
│   ├── reviews/
│   ├── notifications/
│   ├── reporting/
│   └── audit/
├── migrations/
├── deployments/
├── scripts/
├── tests/
├── go.mod
└── README.md
```

## Implementation Notes

- The server seeds a root super-admin from the configured `SUPER_ADMIN_*` values on startup.
- Authentication uses JWT bearer tokens.
- Media uploads are stored under the configured uploads directory and served back through `/uploads/*`.
- The codebase is structured as a modular monolith, with additional modules planned around the same core.

4. The server listens on `:8080` by default.


Already Done are
auth
users
booking
schedule
institute
devices
exams
appeals
analytics
admin
authority
ws
IOT
Scorin


## Environment variables

- `PORT`: HTTP port.
- `JWT_SECRET`: Required for signing auth tokens. we'll  Change this before production use.
- `INTERNAL_API_KEY`: Protects internal scoring endpoints.
- `SEED_DEMO_DATA`: Seeds demo users, institute, bookings, exams, devices, and appeals when `true`.

No external API keys are required for local development. The only secrets the backend expects are `JWT_SECRET` and `INTERNAL_API_KEY`, and both already have local defaults so the server will start even if you do not set them.

## Frontend-Friendly Flows

- `POST /auth/register` and `POST /auth/register/candidate` both create a candidate account.
- `POST /auth/request-otp` issues a short-lived OTP for the currently authenticated user.
- `POST /devices/register` returns the device secret plus the WebSocket stream URL so device onboarding can happen without guessing internal state.
- `POST /scoring/analyze` and `POST /internal/scoring/frame-process` are working backend endpoints for the frontend and integration teams even before the real ML engine is wired in.

## Demo credentials

When `SEED_DEMO_DATA=true`, the following accounts are created with password `Password123!`:

- `authority@adlts.local`
- `admin@adlts.local`
- `institute@adlts.local`
- `examiner@adlts.local`
- `candidate@adlts.local`

## Notes

- The backend currently uses in-memory storage, so data resets when the process restarts.
- The scoring endpoints return deterministic mock analysis from the submitted frame payload until the real ML service is introduced.
- The current structure is a modular monolith: route groups are separated by domain, but the implementation still shares one process and one store.

