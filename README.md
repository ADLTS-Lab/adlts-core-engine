# ADLTS Backend

Automated Driving License Testing System backend implemented as a Go modular monolith.

## What is included

- Auth, user, booking, device, exam, scoring, appeals, and analytics endpoints.
- JWT-based RBAC.
- In-memory persistence so the project runs without external services.
- WebSocket ingestion for ESP32-CAM frames.

## Run locally

1. Set`.env` and adjust secrets if needed.
2. Install Go 1.22 or later.
3. From the project root, run:

```bash
go mod tidy
go run ./cmd/api
```

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
