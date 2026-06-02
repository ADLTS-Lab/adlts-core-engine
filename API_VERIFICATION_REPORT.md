# API Verification Report

- Run time: 2026-06-02T02:37:47Z
- Backend: `http://localhost:8080`
- Run directory: `tmp/api-verification/20260602_023747`
- Total cases executed: 102
- PASS: 92
- WARN: 6
- FAIL: 4
- Critical failures: 2
- Ready for frontend verification: No

## Tokens acquired by role

| role | token sample |
| --- | --- |
| candidate | eyJhbGciOi...lo9BiQ |
| admin | eyJhbGciOi...SHxyx4 |
| super_admin | eyJhbGciOi...JywHn8 |
| institute | eyJhbGciOi...72Ybs0 |
| expert | eyJhbGciOi...tVX8aM |
| transport_authority | eyJhbGciOi...wmdLP4 |

## Endpoint pass/fail/warn table

| Group | Name | Method | Path | Role | Status | Expected | Result | Severity | Reason |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| A | GET /health | GET | /health | public | 200 | 200 | PASS | critical | expected |
| B | POST /api/v1/auth/login (candidate) | POST | /api/v1/auth/login | public | 200 | 200 | PASS | critical | expected |
| B | POST /api/v1/auth/login (admin) | POST | /api/v1/auth/login | public | 200 | 200 | PASS | critical | expected |
| B | POST /api/v1/auth/login (super_admin) | POST | /api/v1/auth/login | public | 200 | 200 | PASS | critical | expected |
| B | POST /api/v1/auth/login (institute) | POST | /api/v1/auth/login | public | 200 | 200 | PASS | critical | expected |
| B | POST /api/v1/auth/login (expert) | POST | /api/v1/auth/login | public | 200 | 200 | PASS | critical | expected |
| B | POST /api/v1/auth/login (transport_authority) | POST | /api/v1/auth/login | public | 200 | 200 | PASS | critical | expected |
| B | POST /api/v1/auth/logout (candidate) | POST | /api/v1/auth/logout | candidate | 200 | 200 | PASS | critical | expected |
| B | POST /api/v1/auth/token/refresh (candidate) | POST | /api/v1/auth/token/refresh | candidate | 200 | 200 | PASS | critical | expected |
| B | POST /api/v1/auth/password/forgot | POST | /api/v1/auth/password/forgot | public | 200 | 200 | PASS | critical | expected |
| B | POST /api/v1/auth/password/reset (invalid token) | POST | /api/v1/auth/password/reset | public | 400 | 400,422 | PASS | critical | expected |
| B | PATCH /api/v1/auth/password/change (candidate, invalid current password) | PATCH | /api/v1/auth/password/change | candidate | 401 | 400 | FAIL | critical | unexpected_status |
| B | POST /api/v1/auth/candidates/register | POST | /api/v1/auth/candidates/register | public | 422 | 201,400 | FAIL | critical | unexpected_status |
| B | POST /api/v1/auth/candidates/verify-otp (invalid) | POST | /api/v1/auth/candidates/verify-otp | public | 400 | 400 | PASS | critical | expected |
| B | POST /api/v1/auth/candidates/resend-otp | POST | /api/v1/auth/candidates/resend-otp | public | 200 | 200,404 | PASS | warn | expected |
| B | POST /api/v1/auth/invitations/accept (invalid token) | POST | /api/v1/auth/invitations/accept | public | 400 | 400 | PASS | critical | expected |
| B | Unauthenticated protected endpoint must require auth | GET | /api/v1/candidates | public | 401 | 401 | PASS | critical | expected |
| B | Wrong-role protected endpoint returns denial | GET | /api/v1/admins | candidate | 403 | 403,401 | PASS | critical | expected |
| C | GET /api/v1/candidates/me | GET | /api/v1/candidates/me | candidate | 200 | 200 | PASS | critical | expected |
| C | PATCH /api/v1/candidates/me | PATCH | /api/v1/candidates/me | candidate | 200 | 200,400 | PASS | critical | expected |
| C | GET /api/v1/admins/me | GET | /api/v1/admins/me | admin | 200 | 200 | PASS | critical | expected |
| C | GET /api/v1/super-admins/me | GET | /api/v1/super-admins/me | super_admin | 200 | 200 | PASS | critical | expected |
| C | GET /api/v1/experts/me | GET | /api/v1/experts/me | expert | 200 | 200 | PASS | critical | expected |
| C | PATCH /api/v1/experts/me (if supported) | PATCH | /api/v1/experts/me | expert | 200 | 200,400 | PASS | warn | expected |
| C | GET /api/v1/institutes/me | GET | /api/v1/institutes/me | institute | 200 | 200 | PASS | critical | expected |
| C | PATCH /api/v1/institutes/me | PATCH | /api/v1/institutes/me | institute | 200 | 200,400 | PASS | warn | expected |
| C | GET /api/v1/transport-authorities/me | GET | /api/v1/transport-authorities/me | transport_authority | 200 | 200 | PASS | critical | expected |
| D | GET /api/v1/candidates (admin) | GET | /api/v1/candidates | admin | 200 | 200,404 | PASS | critical | expected |
| D | GET /api/v1/candidates?page=1&search=candidate | GET | /api/v1/candidates?page=1&search=candidate | admin | 200 | 200,404 | PASS | critical | expected |
| D | GET /api/v1/candidates/{seeded} | GET | /api/v1/candidates/10000000-0000-4000-8000-000000000141 | admin | 404 | 200,404 | PASS | warn | expected |
| D | PATCH /api/v1/candidates/{seeded}/status | PATCH | /api/v1/candidates/10000000-0000-4000-8000-000000000141/status | admin | 200 | 200,400,409,422,403 | PASS | warn | expected |
| D | GET /api/v1/admins | GET | /api/v1/admins | super_admin | 200 | 200,404 | PASS | critical | expected |
| D | GET /api/v1/experts | GET | /api/v1/experts | super_admin | 200 | 200,404 | PASS | critical | expected |
| D | GET /api/v1/institutes | GET | /api/v1/institutes | admin | 200 | 200,404 | PASS | critical | expected |
| D | GET /api/v1/institutes/active | GET | /api/v1/institutes/active | candidate | 200 | 200 | PASS | critical | expected |
| D | GET /api/v1/transport-authorities | GET | /api/v1/transport-authorities | super_admin | 200 | 200,404 | PASS | warn | expected |
| E | GET /api/v1/bookings (candidate) | GET | /api/v1/bookings | candidate | 200 | 200 | PASS | critical | expected |
| E | POST /api/v1/bookings | POST | /api/v1/bookings | candidate | 201 | 201,200 | PASS | warn | expected |
| E | GET /api/v1/bookings (institute) | GET | /api/v1/bookings | institute | 200 | 200 | PASS | critical | expected |
| E | PATCH /api/v1/bookings/{bookingID}/verify | PATCH | /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/verify | institute | 400 | 200,400,409 | PASS | warn | expected |
| G | POST /api/v1/slots (admin) | POST | /api/v1/slots | admin | 201 | 201,400 | PASS | warn | expected |
| G | POST /api/v1/slots (admin second slot) | POST | /api/v1/slots | admin | 201 | 201,400 | PASS | warn | expected |
| E | PATCH /api/v1/bookings/{bookingID}/schedule | PATCH | /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/schedule | admin | 200 | 200,400,409,422 | PASS | warn | expected |
| E | PATCH /api/v1/bookings/{bookingID}/reschedule | PATCH | /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/reschedule | candidate | 200 | 200,400,409,422 | PASS | warn | expected |
| E | DELETE /api/v1/bookings/{bookingID} (throwaway only) | DELETE | /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345 | admin | 200 | 200,409,400,422 | PASS | warn | expected |
| F | POST /api/v1/bookings/{id}/payments | POST | /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/payments | candidate | 404 | 200,201,400,409,422 | WARN | warn | missing_endpoint |
| F | POST /api/v1/bookings/{id}/payments/retry | POST | /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/payments/retry | candidate | 404 | 200,201,400,409,422 | WARN | warn | missing_endpoint |
| F | GET /api/v1/bookings/{id}/payments | GET | /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/payments | candidate | 404 | 200,404 | PASS | warn | expected |
| F | POST /api/v1/bookings/{id}/payments/callback invalid signature | POST | /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/payments/callback | public | 401 | 400,401,422 | PASS | critical | expected |
| G | GET /api/v1/slots?institute_id={id} | GET | /api/v1/slots?institute_id=10000000-0000-4000-8000-000000000011 | candidate | 200 | 200,400,404 | PASS | critical | expected |
| G | GET /api/v1/slots/{id} | GET | /api/v1/slots/6bfd148f-0815-46ba-962e-354a4f09e844 | candidate | 200 | 200,404 | PASS | warn | expected |
| H | GET /api/v1/devices?page=1 | GET | /api/v1/devices?page=1 | admin | 200 | 200,404 | PASS | critical | expected |
| H | POST /api/v1/devices | POST | /api/v1/devices | admin | 409 | 201,200,400 | FAIL | warn | unexpected_status |
| H | GET /api/v1/devices/{id} | GET | /api/v1/devices/00000000-0000-0000-0000-000000000000 | admin | 404 | 200,404 | PASS | warn | expected |
| H | PATCH /api/v1/devices/{id}/status | PATCH | /api/v1/devices/00000000-0000-0000-0000-000000000000/status | admin | 200 | 200,400,404 | PASS | warn | expected |
| H | GET /api/v1/devices/{id}/qr-code?password=Device123! | GET | /api/v1/devices/00000000-0000-0000-0000-000000000000/qr-code?password=Device123! | admin | 404 | 200,400,403,404 | PASS | warn | expected |
| H | PATCH /api/v1/devices/{id} | PATCH | /api/v1/devices/00000000-0000-0000-0000-000000000000 | admin | 200 | 200,400,404 | PASS | warn | expected |
| I | GET /api/v1/test-level-types | GET | /api/v1/test-level-types | admin | 200 | 200 | PASS | critical | expected |
| I | GET /api/v1/guidelines | GET | /api/v1/guidelines | candidate | 200 | 200 | PASS | critical | expected |
| I | GET /api/v1/guidelines/faq | GET | /api/v1/guidelines/faq | candidate | 200 | 200 | PASS | critical | expected |
| I | GET /api/v1/maneuver-types | GET | /api/v1/maneuver-types | admin | 200 | 200 | PASS | critical | expected |
| I | GET /api/v1/test-level-mappings | GET | /api/v1/test-level-mappings | admin | 200 | 200 | PASS | critical | expected |
| I | PUT /api/v1/test-level-mappings | PUT | /api/v1/test-level-mappings | admin | 200 | 200,400,409,422 | PASS | warn | expected |
| J | GET /api/v1/test-plans | GET | /api/v1/test-plans | admin | 200 | 200 | PASS | critical | expected |
| J | GET /api/v1/test-plans/{planID} | GET | /api/v1/test-plans/849e5682-1930-463a-8dbc-d791319494fc | admin | 200 | 200,404 | PASS | warn | expected |
| J | GET /api/v1/test-plans/{planID}/maneuvers | GET | /api/v1/test-plans/849e5682-1930-463a-8dbc-d791319494fc/maneuvers | admin | 200 | 200,404 | PASS | warn | expected |
| J | POST /api/v1/test-plans | POST | /api/v1/test-plans | admin | 201 | 200,201 | PASS | warn | expected |
| J | PATCH /api/v1/test-plans/{planID} | PATCH | /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f | admin | 200 | 200,400,409 | PASS | warn | expected |
| J | POST /api/v1/test-plans/{planID}/publish | POST | /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/publish | admin | 200 | 200,400,409,422 | PASS | warn | expected |
| J | POST /api/v1/test-plans/{planID}/retire | POST | /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/retire | admin | 200 | 200,400,409,422 | PASS | warn | expected |
| J | POST /api/v1/test-plans/{planID}/maneuvers | POST | /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/maneuvers | admin | 409 | 201,400,409 | PASS | warn | expected |
| J | PATCH /api/v1/test-plans/{planID}/maneuvers/{maneuverID} | PATCH | /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/maneuvers/00000000-0000-0000-0000-000000000000 | admin | 404 | 200,400,409,404 | PASS | warn | expected |
| J | DELETE /api/v1/test-plans/{planID}/maneuvers/{maneuverID} | DELETE | /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/maneuvers/00000000-0000-0000-0000-000000000000 | admin | 404 | 200,400,404 | PASS | warn | expected |
| J | GET /api/v1/test-plans/{planID}/maneuvers/{maneuverID}/qr | GET | /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/maneuvers/00000000-0000-0000-0000-000000000000/qr | admin | 404 | 200,404,400 | PASS | warn | expected |
| J | GET /api/v1/test-plans/{planID}/maneuvers/{maneuverID}/qr-code | GET | /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/maneuvers/00000000-0000-0000-0000-000000000000/qr-code | admin | 404 | 200,404,400 | PASS | warn | expected |
| J | GET /api/v1/test-plans/{planID}/maneuvers/{maneuverID}/mask | GET | /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/maneuvers/00000000-0000-0000-0000-000000000000/mask | admin | 404 | 200,404,400 | PASS | warn | expected |
| K | GET /api/v1/tests (admin) | GET | /api/v1/tests | admin | 200 | 200,401,403,404 | PASS | critical | expected |
| K | GET /api/v1/tests?status=running | GET | /api/v1/tests?status=running | admin | 200 | 200,400,404 | PASS | critical | expected |
| K | GET /api/v1/tests/my | GET | /api/v1/tests/my | candidate | 200 | 200,404 | PASS | critical | expected |
| K | GET /api/v1/tests/my/stats | GET | /api/v1/tests/my/stats | candidate | 200 | 200,404 | PASS | critical | expected |
| K | GET /api/v1/tests/my/pending | GET | /api/v1/tests/my/pending | candidate | 404 | 200,404 | PASS | warn | expected |
| K | GET /api/v1/tests/{running} | GET | /api/v1/tests/10000000-0000-4000-8000-000000000141 | admin | 200 | 200,404,403 | PASS | critical | expected |
| K | GET /api/v1/tests/{completed}/status | GET | /api/v1/tests/10000000-0000-4000-8000-000000000142/status | admin | 200 | 200,404 | PASS | critical | expected |
| K | GET /api/v1/tests/{completed}/result (candidate) | GET | /api/v1/tests/10000000-0000-4000-8000-000000000142/result | candidate | 403 | 200,403,404 | PASS | critical | expected |
| K | GET /api/v1/tests/{completed}/result (admin) | GET | /api/v1/tests/10000000-0000-4000-8000-000000000142/result | admin | 200 | 200,403,404 | PASS | critical | expected |
| K | GET /api/v1/tests/{completed}/result (expert) | GET | /api/v1/tests/10000000-0000-4000-8000-000000000142/result | expert | 200 | 200,403,404 | PASS | critical | expected |
| K | GET /api/v1/tests/{completed}/sessions | GET | /api/v1/tests/10000000-0000-4000-8000-000000000142/sessions | admin | 200 | 200,404 | PASS | critical | expected |
| K | GET /api/v1/tests/{completed}/recording | GET | /api/v1/tests/10000000-0000-4000-8000-000000000142/recording | candidate | 404 | 200,404,500 | PASS | critical | expected |
| K | GET /api/v1/tests/{completed}/monitor/status | GET | /api/v1/tests/10000000-0000-4000-8000-000000000142/monitor/status | admin | 200 | 200,404,500 | PASS | critical | expected |
| K | GET /api/v1/tests/{completed}/monitor/live | GET | /api/v1/tests/10000000-0000-4000-8000-000000000142/monitor/live | admin | 200 | 200,404,500 | PASS | critical | expected |
| K | POST /api/v1/tests/{id}/abort | POST | /api/v1/tests/10000000-0000-4000-8000-000000000141/abort | admin | 200 | 200,400,409,422,404 | PASS | warn | expected |
| K | POST /api/v1/tests/device-checkin | POST | /api/v1/tests/device-checkin | candidate | 404 | 200,400,401,403,404 | PASS | warn | expected |
| K | POST /api/v1/tests/{id}/guidelines/acknowledge | POST | /api/v1/tests/10000000-0000-4000-8000-000000000142/guidelines/acknowledge | candidate | 200 | 200,400,409,404 | PASS | warn | expected |
| L | GET /api/v1/appeals?status=pending | GET | /api/v1/appeals?status=pending | expert | 200 | 200,401 | PASS | critical | expected |
| L | GET /api/v1/appeals/{id} | GET | /api/v1/appeals/10000000-0000-4000-8000-000000000141 | admin | 404 | 200,404 | PASS | warn | expected |
| L | PATCH /api/v1/appeals/{id}/resolve | PATCH | /api/v1/appeals/10000000-0000-4000-8000-000000000141/resolve | expert | 200 | 200,400,409,422,404 | PASS | warn | expected |
| L | POST /api/v1/appeals | POST | /api/v1/appeals | candidate | 500 | 201,400,403,404,409,422 | FAIL | warn | unexpected_status |
| M | GET /api/v1/recordings/{test_id}/play | GET | /api/v1/recordings/10000000-0000-4000-8000-000000000142/play | admin | 500 | 200,401,403,404 | WARN | critical | dependency |
| M | GET /api/v1/recordings/{test_id}/frames | GET | /api/v1/recordings/10000000-0000-4000-8000-000000000142/frames | admin | 500 | 200,401,403,404 | WARN | critical | dependency |
| N | POST /api/v1/reports/{test_id}/generate | POST | /api/v1/reports/10000000-0000-4000-8000-000000000142/generate | admin | 500 | 200,422,409,400 | WARN | critical | dependency |
| N | GET /api/v1/reports/{test_id}/pdf | GET | /api/v1/reports/10000000-0000-4000-8000-000000000142/pdf | admin | 500 | 200,404 | WARN | critical | dependency |
| O | POST /internal/tests (skip when INTERNAL_API_KEY unset) | POST | /internal/tests | internal | 403 | 401,403,404 | PASS | warn | expected |

## Critical failures
- Critical failures:
- B | PATCH /api/v1/auth/password/change -> FAIL (401)
- B | POST /api/v1/auth/candidates/register -> FAIL (422)


## Warnings/dependency failures
- Warnings:
- M | GET /api/v1/recordings/10000000-0000-4000-8000-000000000142/play -> WARN (500)
- M | GET /api/v1/recordings/10000000-0000-4000-8000-000000000142/frames -> WARN (500)
- N | POST /api/v1/reports/10000000-0000-4000-8000-000000000142/generate -> WARN (500)
- N | GET /api/v1/reports/10000000-0000-4000-8000-000000000142/pdf -> WARN (500)


## Missing endpoints
- Missing endpoints:
- F | POST /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/payments -> WARN (404)
- F | POST /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/payments/retry -> WARN (404)


## Role/RBAC issues
- Role/RBAC issues:
- B | GET /api/v1/candidates -> PASS (401)
- B | GET /api/v1/admins -> PASS (403)


## JSON envelope issues
- JSON envelope issues: `none`

## Mutating endpoints tested
- Mutating endpoints:
- B | POST /api/v1/auth/login -> PASS (200)
- B | POST /api/v1/auth/login -> PASS (200)
- B | POST /api/v1/auth/login -> PASS (200)
- B | POST /api/v1/auth/login -> PASS (200)
- B | POST /api/v1/auth/login -> PASS (200)
- B | POST /api/v1/auth/login -> PASS (200)
- B | POST /api/v1/auth/logout -> PASS (200)
- B | POST /api/v1/auth/token/refresh -> PASS (200)
- B | POST /api/v1/auth/password/forgot -> PASS (200)
- B | POST /api/v1/auth/password/reset -> PASS (400)
- B | PATCH /api/v1/auth/password/change -> FAIL (401)
- B | POST /api/v1/auth/candidates/register -> FAIL (422)
- B | POST /api/v1/auth/candidates/verify-otp -> PASS (400)
- B | POST /api/v1/auth/candidates/resend-otp -> PASS (200)
- B | POST /api/v1/auth/invitations/accept -> PASS (400)
- C | PATCH /api/v1/candidates/me -> PASS (200)
- C | PATCH /api/v1/experts/me -> PASS (200)
- C | PATCH /api/v1/institutes/me -> PASS (200)
- D | PATCH /api/v1/candidates/10000000-0000-4000-8000-000000000141/status -> PASS (200)
- E | POST /api/v1/bookings -> PASS (201)
- E | PATCH /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/verify -> PASS (400)
- G | POST /api/v1/slots -> PASS (201)
- G | POST /api/v1/slots -> PASS (201)
- E | PATCH /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/schedule -> PASS (200)
- E | PATCH /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/reschedule -> PASS (200)
- E | DELETE /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345 -> PASS (200)
- F | POST /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/payments -> WARN (404)
- F | POST /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/payments/retry -> WARN (404)
- F | POST /api/v1/bookings/9a733229-7000-42e5-a9ab-8f9a8678d345/payments/callback -> PASS (401)
- H | POST /api/v1/devices -> FAIL (409)
- H | PATCH /api/v1/devices/00000000-0000-0000-0000-000000000000/status -> PASS (200)
- H | PATCH /api/v1/devices/00000000-0000-0000-0000-000000000000 -> PASS (200)
- I | PUT /api/v1/test-level-mappings -> PASS (200)
- J | POST /api/v1/test-plans -> PASS (201)
- J | PATCH /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f -> PASS (200)
- J | POST /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/publish -> PASS (200)
- J | POST /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/retire -> PASS (200)
- J | POST /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/maneuvers -> PASS (409)
- J | PATCH /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/maneuvers/00000000-0000-0000-0000-000000000000 -> PASS (404)
- J | DELETE /api/v1/test-plans/5f32812e-c399-4b38-9c72-44067b8a9c8f/maneuvers/00000000-0000-0000-0000-000000000000 -> PASS (404)
- K | POST /api/v1/tests/10000000-0000-4000-8000-000000000141/abort -> PASS (200)
- K | POST /api/v1/tests/device-checkin -> PASS (404)
- K | POST /api/v1/tests/10000000-0000-4000-8000-000000000142/guidelines/acknowledge -> PASS (200)
- L | PATCH /api/v1/appeals/10000000-0000-4000-8000-000000000141/resolve -> PASS (200)
- L | POST /api/v1/appeals -> FAIL (500)
- N | POST /api/v1/reports/10000000-0000-4000-8000-000000000142/generate -> WARN (500)
- O | POST /internal/tests -> PASS (403)

## Recommended fixes
- Re-run once auth/session services are healthy and fix non-critical route/path regressions before frontend smoke.
- Investigate `non_json_error`/`envelope_missing` on critical endpoints first; frontend contracts should always return JSON envelopes.
- Dependency WARN items (`REPORT_FAILED`, `LIST_ERROR`, payment webhook/storage errors) are only considered non-blocking when route is mounted and response is JSON-formatted.
