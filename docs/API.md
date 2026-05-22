# ADLTS Core Engine — API Documentation (Frontend)

Base paths:

- Health: `GET /health`
- Uploads (public static): `GET /uploads/*`
- API root: `/api/v1`

## Response envelope

All JSON endpoints respond using a common envelope:

```json
{
  "success": true,
  "data": {},
  "meta": {"page": 1, "limit": 20, "total": 123}
}
```

On errors:

```json
{
  "success": false,
  "error": {
    "code": "SOME_CODE",
    "message": "human readable message",
    "detail": {"any": "optional"}
  }
}
```

## Authentication

The API uses Bearer JWTs.

- Request header: `Authorization: Bearer <access_token>`
- Tokens contain `entity_type` and `sub_id`.

Entity types:

- `candidate`
- `expert`
- `admin`
- `super_admin`
- `institute`
- `transport_authority`

Some routes are additionally restricted by entity type.

## CORS

CORS is permissive for development:

- `Access-Control-Allow-Origin: *`
- Allowed headers include: `Content-Type`, `Authorization`, `X-Internal-Token`, `X-Device-Secret`

## Identity & Auth

Mounted under `/api/v1`.

### Auth endpoints

- `POST /auth/candidates/register`
  - Body (`RegisterCandidateRequest`):
    - `first_name`, `middle_name`, `last_name`, `email`, `password`, `phone`, `fayida_id`, `birth_date` (RFC3339), `gender` (`male|female`)
  - 201: `{ "message": "OTP sent to your email address" }`

- `POST /auth/candidates/verify-otp`
  - Body (`VerifyOTPRequest`): `email`, `code`
  - 200 (`LoginResponse`): `{ access_token, refresh_token, entity_type }`

- `POST /auth/candidates/resend-otp`
  - Body: `{ "email": "..." }`
  - 200: always succeeds (prevents email enumeration)

- `POST /auth/invitations/accept`
  - Body (`AcceptInvitationRequest`):
    - `token`, `password`, plus identity fields depending on the invitation type
  - 201: returns an auth response (token + entity metadata)

- `POST /auth/login`
  - Body (`LoginRequest`): `email`, `password`
  - 200 (`LoginResponse`): `{ access_token, refresh_token, entity_type }`

- `POST /auth/logout` (auth)
- `POST /auth/token/refresh` (auth)
  - Body (`RefreshRequest`): `refresh_token`
- `POST /auth/password/forgot`
  - Body (`ForgotPasswordRequest`): `email`
- `POST /auth/password/reset`
  - Body (`ResetPasswordRequest`): `token`, `password`
- `PATCH /auth/password/change` (auth)
  - Body (`ChangePasswordRequest`): `current_password`, `new_password`

### Users & orgs

All routes below require auth; additional restrictions apply per entity:

- Candidates
  - `GET /candidates` (admin/super-admin)
  - `GET /candidates/me` (candidate)
  - `PATCH /candidates/me` (candidate)
  - `PATCH /candidates/me/photo` (candidate; multipart upload)
  - `DELETE /candidates/me` (candidate; soft delete)
  - `GET /candidates/{id}` (admin/super-admin)
  - `PATCH /candidates/{id}` (super-admin)
  - `PATCH /candidates/{id}/status` (admin/super-admin)
  - `PATCH /candidates/{id}/photo` (admin/super-admin; multipart upload)
  - `DELETE /candidates/{id}` (super-admin; hard delete)

- Experts
  - `GET /experts` (super-admin)
  - `GET /experts/me`, `PATCH /experts/me`, `PATCH /experts/me/photo` (expert)
  - `GET/PATCH/DELETE /experts/{id}` + status/photo (super-admin)

- Institutes
  - `GET /institutes` (admin/super-admin)
  - `GET/PATCH/DELETE /institutes/me`, `PATCH /institutes/me/logo` (institute)
  - `GET /institutes/{id}` (admin/super-admin)
  - `PATCH /institutes/{id}` (super-admin)
  - `PATCH /institutes/{id}/status` (admin/super-admin)
  - `PATCH /institutes/{id}/logo` (super-admin; multipart upload)
  - `DELETE /institutes/{id}` (super-admin)

- Transport authorities
  - `GET /transport-authorities` (super-admin)
  - `GET/PATCH /transport-authorities/me`, `PATCH /transport-authorities/me/logo` (transport-authority)
  - `GET/PATCH/DELETE /transport-authorities/{id}` + status/logo (super-admin)

- Admins
  - `GET /admins` (super-admin)
  - `GET/PATCH /admins/me` (admin)
  - `GET/PATCH/DELETE /admins/{id}` + status (super-admin)

- Super admins (super-admin only)
  - `GET /super-admins`
  - `GET/PATCH /super-admins/me`
  - `GET/PATCH/DELETE /super-admins/{id}`

- Invitations (admin/super-admin)
  - `POST /invitations`
  - `GET /invitations`
  - `GET /invitations/{id}`
  - `POST /invitations/{id}/resend`
  - `DELETE /invitations/{id}`

## Booking, Scheduling, Payments

Mounted under `/api/v1`.

### Bookings

- `POST /bookings` (candidate)
  - Body (`CreateBookingRequest`): `institute_id`

- `GET /bookings` (auth)
- `GET /bookings/{id}` (auth)
- `DELETE /bookings/{id}` (auth)

- `PATCH /bookings/{id}/verify` (institute/admin/super-admin)
  - Body (`VerifyBookingRequest`):
    - `action`: `approve|reject`
    - `rejection_reason` (required if `action=reject`)

- `PATCH /bookings/{id}/schedule` (admin/super-admin)
  - Body (`ScheduleBookingRequest`): `slot_id`

- `PATCH /bookings/{id}/reschedule` (auth)
  - Body (`RescheduleBookingRequest`): `slot_id`

### Payments

- `POST /bookings/{id}/payments` (candidate)
  - Body (`InitiatePaymentRequest`): `amount_cents`, `currency` (defaults to `ETB`)
  - 201 (`InitiatePaymentResponse`): `{ payment_id, checkout_url, tx_ref }`

- `POST /bookings/{id}/payments/retry` (candidate)
- `GET /bookings/{id}/payments` (auth)

### Chapa webhook callback (public)

- `POST /bookings/{id}/payments/callback`
  - Header: `x-chapa-signature: <signature>`
  - Body: Chapa callback payload (at minimum uses `tx_ref` and `status`)

## Slots

- `POST /slots` (admin/super-admin)
  - Body (`CreateSlotRequest`): `institute_id`, `starts_at`, `ends_at`, `capacity`

- `GET /slots` (auth)
- `GET /slots/{id}` (auth)

## Appeals

Mounted under `/api/v1` and requires auth (JWT).

- `POST /appeals` (auth)
  - Body:
    - `test_id` (uuid)
    - `session_id` (uuid)
    - `reason` (string)
  - 201: `{ id }`

- `GET /appeals/{id}` (auth)
  - 200: returns the appeal record

- `PATCH /appeals/{id}/resolve` (auth)
  - Body:
    - `decision`: `accepted|rejected`
    - `resolution`: string

Notes:

- Appeal creation is rejected with `403 APPEAL_WINDOW_CLOSED` when the appeal window has closed.

## Recording (Playback + Frames)

Mounted under `/api/v1` and requires auth.

- `GET /recordings/{test_id}/frames`
  - 200: array of `{ key, url }` where `url` is a presigned URL.

- `GET /recordings/{test_id}/play`
  - Streams MJPEG: `Content-Type: multipart/x-mixed-replace; boundary=frame`

## Reporting

Mounted under `/api/v1` and requires auth + entity type in: `admin`, `super_admin`, `institute`, `expert`.

- `POST /reports/{testID}/generate`
  - 200: `{ test_id, report_url }`

- `GET /reports/{testID}/pdf`
  - Returns `application/pdf` and will generate the report if not cached.

See [docs/REPORTING_SETUP.md](REPORTING_SETUP.md) for environment variables and runtime dependencies.
