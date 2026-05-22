# Feature Coverage (API + Tests)

_Last updated: 2026-05-22_

This document maps **implemented features** to their **HTTP surface area** and the **automated tests** that currently exercise them.

Base URLs:

- Public: `GET /health`, `GET /uploads/*`
- API root: `/api/v1`

## Summary

- Identity + Auth: **covered by integration tests** (`tests/identity/*`)
- Booking + Scheduling + Payments: **covered by integration tests** (`tests/booking/*`) + optional live Chapa smoke test
- Appeals: **covered by integration tests** (`tests/appeal/*`)
- Recording playback/frames: **implemented, no automated tests yet**
- Reporting (generate + PDF): **implemented, no automated tests yet**

## Identity & Auth

Implementation: `internal/identity/*` mounted in `internal/app/app.go`.

### Auth endpoints

- `POST /api/v1/auth/candidates/register` (public)
- `POST /api/v1/auth/candidates/verify-otp` (public)
- `POST /api/v1/auth/candidates/resend-otp` (public)
- `POST /api/v1/auth/invitations/accept` (public)
- `POST /api/v1/auth/login` (public)
- `POST /api/v1/auth/logout` (auth)
- `POST /api/v1/auth/token/refresh` (auth)
- `POST /api/v1/auth/password/forgot` (public)
- `POST /api/v1/auth/password/reset` (public)
- `PATCH /api/v1/auth/password/change` (auth)

### Identity resources

All identity resource routes are under `/api/v1` and require auth; per-route access is enforced by entity type.

- Candidates
  - `GET /candidates` (admin/super-admin)
  - `GET /candidates/me`, `PATCH /candidates/me`, `PATCH /candidates/me/photo`, `DELETE /candidates/me` (candidate)
  - `GET /candidates/{id}` (admin/super-admin)
  - `PATCH /candidates/{id}` (super-admin)
  - `PATCH /candidates/{id}/status` (admin/super-admin)
  - `PATCH /candidates/{id}/photo` (admin/super-admin)
  - `DELETE /candidates/{id}` (super-admin)
- Experts
  - `GET /experts` (super-admin)
  - `GET/PATCH /experts/me`, `PATCH /experts/me/photo` (expert)
  - `GET/PATCH /experts/{id}`, `PATCH /experts/{id}/status`, `PATCH /experts/{id}/photo`, `DELETE /experts/{id}` (super-admin)
- Institutes
  - `GET /institutes` (admin/super-admin)
  - `GET/PATCH /institutes/me`, `PATCH /institutes/me/logo`, `DELETE /institutes/me` (institute)
  - `GET /institutes/{id}` (admin/super-admin)
  - `PATCH /institutes/{id}` (super-admin)
  - `PATCH /institutes/{id}/status` (admin/super-admin)
  - `PATCH /institutes/{id}/logo` (super-admin)
  - `DELETE /institutes/{id}` (super-admin)
- Transport Authorities
  - `GET /transport-authorities` (super-admin)
  - `GET/PATCH /transport-authorities/me`, `PATCH /transport-authorities/me/logo` (transport-authority)
  - `GET/PATCH /transport-authorities/{id}`, `PATCH /transport-authorities/{id}/status`, `PATCH /transport-authorities/{id}/logo`, `DELETE /transport-authorities/{id}` (super-admin)
- Admins
  - `GET /admins` (super-admin)
  - `GET/PATCH /admins/me` (admin)
  - `GET/PATCH /admins/{id}`, `PATCH /admins/{id}/status`, `DELETE /admins/{id}` (super-admin)
- Super Admins
  - `GET /super-admins`, `GET/PATCH /super-admins/me`, `GET/PATCH/DELETE /super-admins/{id}` (super-admin)
- Invitations
  - `POST /invitations`, `GET /invitations`, `GET /invitations/{id}`, `POST /invitations/{id}/resend`, `DELETE /invitations/{id}` (admin/super-admin)

### Tests

- `tests/identity/identity_suite_test.go`
  - Candidate registration + OTP verification
  - Invitation flows (create/list/get/resend/accept/cancel)
  - Login + `/admins/me`

## Booking, Scheduling, Payments

Implementation: `internal/booking/*`.

### Booking endpoints

- `POST /api/v1/bookings/{id}/payments/callback` (public; Chapa webhook)
- `POST /api/v1/bookings` (candidate)
- `GET /api/v1/bookings` (auth)
- `GET /api/v1/bookings/{id}` (auth)
- `DELETE /api/v1/bookings/{id}` (auth)
- `PATCH /api/v1/bookings/{id}/verify` (institute/admin/super-admin)
- `PATCH /api/v1/bookings/{id}/schedule` (admin/super-admin)
- `PATCH /api/v1/bookings/{id}/reschedule` (auth)
- Payments
  - `POST /api/v1/bookings/{id}/payments` (candidate)
  - `POST /api/v1/bookings/{id}/payments/retry` (candidate)
  - `GET /api/v1/bookings/{id}/payments` (auth)

### Slot endpoints

- `POST /api/v1/slots` (admin/super-admin)
- `GET /api/v1/slots` (auth)
- `GET /api/v1/slots/{id}` (auth)

### Tests

- `tests/booking/booking_suite_test.go`
  - Create booking + list
  - Schedule booking + initiate payment + webhook callback handling
- `tests/booking/chapa_live_test.go`
  - Optional live smoke test for Chapa provider (skips unless `CHAPA_SECRET_KEY` is set)

## Appeals

Implementation: `internal/appeal/*`.

Mounted under `/api/v1` and requires JWT auth (via `internal/app/app.go`).

Endpoints:

- `GET /api/v1/appeals/{id}`
- `POST /api/v1/appeals`
- `PATCH /api/v1/appeals/{id}/resolve`

Tests:

- `tests/appeal/appeal_suite_test.go`
  - Create/get/resolve appeal
  - Appeal window enforcement (403 when closed)

## Recording (Playback + Frames)

Implementation: `internal/recording/*`.

Mounted under `/api/v1` and requires JWT auth.

Endpoints:

- `GET /api/v1/recordings/{test_id}/play` (MJPEG stream)
- `GET /api/v1/recordings/{test_id}/frames` (frame list)

Tests:

- None yet.

## Reporting

Implementation: `internal/reporting/*`.

Mounted under `/api/v1` and requires JWT auth **and** one of these entities:
`admin`, `super_admin`, `institute`, `expert`.

Endpoints:

- `POST /api/v1/reports/{testID}/generate`
- `GET /api/v1/reports/{testID}/pdf` (generates on-demand if missing)

Tests:

- None yet.

## Gaps / Next Tests to Add

- Appeals: add authorization coverage (who can resolve) + evidence snapshot coverage.
- Recording: unit test `FrameList` against a fake MinIO client; integration test could be added if MinIO is available in CI.
- Reporting: unit test for renderer/template execution; integration tests need stable chromedp runtime (Chrome/Chromium installed).
