# Booking, Scheduling & Payment — Full Implementation Plan
> **For:** OpenAI Codex / automated implementation  
> **Project:** `adlts-core-engine` (Go modular monolith)  
> **Scope:** `internal/booking/` module + migration + wiring into existing app  
> **Must not break:** `internal/identity/` or any existing routes/tests

---

## Table of Contents

1. [Overview & Constraints](#1-overview--constraints)
2. [File Tree to Create](#2-file-tree-to-create)
3. [Step 1 — Database Migration](#3-step-1--database-migration)
4. [Step 2 — Domain Models](#4-step-2--domain-models)
5. [Step 3 — Config Updates](#5-step-3--config-updates)
6. [Step 4 — Module Scaffold](#6-step-4--module-scaffold)
7. [Step 5 — DTOs](#7-step-5--dtos)
8. [Step 6 — Payment Provider Interface](#8-step-6--payment-provider-interface)
9. [Step 7 — Chapa Provider Implementation](#9-step-7--chapa-provider-implementation)
10. [Step 8 — Repository](#10-step-8--repository)
11. [Step 9 — Service Layer](#11-step-9--service-layer)
12. [Step 10 — Handlers](#12-step-10--handlers)
13. [Step 11 — Routes](#13-step-11--routes)
14. [Step 12 — Wire into app.go](#14-step-12--wire-into-appgo)
15. [Step 13 — Auth Middleware Fix](#15-step-13--auth-middleware-fix)
16. [Step 14 — Environment Variables](#16-step-14--environment-variables)
17. [Step 15 — docker-compose Updates](#17-step-15--docker-compose-updates)
18. [Step 16 — Tests](#18-step-16--tests)
19. [Step 17 — go.mod Dependencies](#19-step-17--gomod-dependencies)
20. [Limitations & Future Work](#20-limitations--future-work)
21. [Implementation Order for Codex](#21-implementation-order-for-codex)

---

## 1. Overview & Constraints

### What this module does

The booking module implements the full lifecycle shown in the whiteboard diagram:

```
Candidate creates booking
  └─► System sets status = drafted
        └─► Institute requires verification?
              ├─ YES ─► Booking enters pending_verification
              │           └─► Institute reviews candidate details
              │                 ├─ REJECT ─► Candidate notified, flow ends
              │                 └─ APPROVE ─► Status = verified
              └─ NO  ─► Status = verified immediately
                          └─► Admin assigns slot (scheduling)
                                └─► Candidate initiates payment (Chapa)
                                      ├─ RETRY LOOP (on failure, up to 3 attempts)
                                      └─ SUCCESS ─► Status = confirmed
                                                      └─► Test created
                                                            ├─ Reschedule option
                                                            └─► Test Ready / Booking Archived
                                                                  └─► Hands off to Testing Flow
```

### Hard constraints (do not violate)

- Do **not** modify any file inside `internal/identity/`
- Do **not** modify `migrations/001_schema.sql`
- Do **not** change existing routes in `internal/app/app.go` — only append new wiring
- Do **not** add columns to existing tables — all schema changes go in a new migration file
- Follow the same layering pattern: `handler → service → repository`
- Use the same `httpx.Success` / `httpx.Failure` JSON envelope — never `http.Error`
- Pagination is fixed at 20 per page using `page` query param (same as identity module)
- All UUIDs are `github.com/google/uuid`
- Database driver is `pgx/v5` pool (same as identity)
- No ORM — raw SQL with named parameters

### Architecture summary of new files

```
internal/booking/
  dto.go                 ← request/response structs
  payment_provider.go    ← PaymentProvider interface + types
  chapa_provider.go      ← Chapa implementation of PaymentProvider
  repository.go          ← all SQL queries
  service.go             ← all business logic
  handler.go             ← HTTP handlers
  routes.go              ← route registration

internal/domain/
  booking.go             ← UPDATE: add new types and constants (slot, payment, booking status)

internal/platform/config/
  config.go              ← UPDATE: add Chapa + frontend config fields

internal/app/
  app.go                 ← UPDATE: wire booking module (append only)

internal/platform/security/
  security.go            ← FIX: replace http.Error with httpx.Failure

migrations/
  002_booking_scheduling_payment.sql  ← new file, never modify 001

tests/booking/
  booking_suite_test.go  ← integration tests
```

---

## 2. File Tree to Create

```
adlts-core-engine/
├── migrations/
│   └── 002_booking_scheduling_payment.sql   [CREATE NEW]
├── internal/
│   ├── domain/
│   │   └── booking.go                       [UPDATE - add new types]
│   ├── platform/
│   │   └── config/
│   │       └── config.go                    [UPDATE - add fields]
│   ├── app/
│   │   └── app.go                           [UPDATE - append wiring]
│   ├── platform/
│   │   └── security/
│   │       └── security.go                  [FIX - http.Error → httpx.Failure]
│   └── booking/
│       ├── dto.go                           [CREATE NEW]
│       ├── payment_provider.go              [CREATE NEW]
│       ├── chapa_provider.go                [CREATE NEW]
│       ├── repository.go                    [CREATE NEW]
│       ├── service.go                       [CREATE NEW]
│       ├── handler.go                       [CREATE NEW]
│       └── routes.go                        [CREATE NEW]
└── tests/
    └── booking/
        └── booking_suite_test.go            [CREATE NEW]
```

---

## 3. Step 1 — Database Migration

**File:** `migrations/002_booking_scheduling_payment.sql`

This file must be run after `001_schema.sql`. The existing `bookings` and `slots` tables in `001_schema.sql` are scaffolded (no columns beyond basic structure). This migration adds all operational columns.

```sql
-- ============================================================
-- 002_booking_scheduling_payment.sql
-- Adds all columns and tables needed for booking, scheduling,
-- and payment features. Run after 001_schema.sql.
-- ============================================================

-- -------------------------------------------------------
-- Extend the bookings table
-- -------------------------------------------------------
ALTER TABLE bookings
  ADD COLUMN IF NOT EXISTS status                TEXT        NOT NULL DEFAULT 'drafted',
  ADD COLUMN IF NOT EXISTS requires_verification BOOLEAN     NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS verified_by           UUID        REFERENCES admins(id),
  ADD COLUMN IF NOT EXISTS verified_at           TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS rejection_reason      TEXT,
  ADD COLUMN IF NOT EXISTS slot_id               UUID,
  ADD COLUMN IF NOT EXISTS scheduled_at          TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS payment_ref           TEXT,
  ADD COLUMN IF NOT EXISTS payment_status        TEXT        NOT NULL DEFAULT 'unpaid',
  ADD COLUMN IF NOT EXISTS payment_amount_cents  INTEGER,
  ADD COLUMN IF NOT EXISTS payment_attempts      INTEGER     NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS archived_at           TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ADD COLUMN IF NOT EXISTS updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- -------------------------------------------------------
-- Extend the slots table
-- -------------------------------------------------------
ALTER TABLE slots
  ADD COLUMN IF NOT EXISTS institute_id UUID        REFERENCES institutes(id),
  ADD COLUMN IF NOT EXISTS test_id      UUID,
  ADD COLUMN IF NOT EXISTS starts_at    TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS ends_at      TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS capacity     INTEGER     NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS booked_count INTEGER     NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ADD COLUMN IF NOT EXISTS updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- -------------------------------------------------------
-- Payments table (new — audit log of every payment attempt)
-- -------------------------------------------------------
CREATE TABLE IF NOT EXISTS payments (
  id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id     UUID        NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
  amount_cents   INTEGER     NOT NULL,
  currency       TEXT        NOT NULL DEFAULT 'ETB',
  status         TEXT        NOT NULL DEFAULT 'pending',
  -- status values: pending | success | failed | refunded
  provider       TEXT        NOT NULL DEFAULT 'chapa',
  -- provider values: chapa | manual
  -- future: telebirr_direct | cbe_direct | awash_direct | dashen_direct
  provider_ref   TEXT,
  -- provider_ref = tx_ref sent to Chapa (format: adlts_booking_{uuid}_{unix})
  attempt_number INTEGER     NOT NULL DEFAULT 1,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- -------------------------------------------------------
-- Indexes
-- -------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_bookings_status      ON bookings(status);
CREATE INDEX IF NOT EXISTS idx_bookings_candidate   ON bookings(candidate_id);
CREATE INDEX IF NOT EXISTS idx_bookings_institute   ON bookings(institute_id);
CREATE INDEX IF NOT EXISTS idx_bookings_slot        ON bookings(slot_id);
CREATE INDEX IF NOT EXISTS idx_slots_institute      ON slots(institute_id);
CREATE INDEX IF NOT EXISTS idx_payments_booking     ON payments(booking_id);
CREATE INDEX IF NOT EXISTS idx_payments_provider_ref ON payments(provider_ref);
```

> **Note for Codex:** Check if `candidate_id` and `institute_id` already exist on the `bookings` table from `001_schema.sql`. If they do, skip adding them. Only add columns that are missing.

---

## 4. Step 2 — Domain Models

**File:** `internal/domain/booking.go`

Replace the entire file with the following (the original scaffolded version only had empty structs):

```go
package domain

import (
	"time"

	"github.com/google/uuid"
)

// -------------------------------------------------------
// Booking status lifecycle
// drafted → pending_verification → verified → scheduled
//        → payment_pending → payment_failed → confirmed
//        → archived | cancelled | rejected
// -------------------------------------------------------

type BookingStatus string

const (
	BookingDrafted             BookingStatus = "drafted"
	BookingPendingVerification BookingStatus = "pending_verification"
	BookingVerified            BookingStatus = "verified"
	BookingRejected            BookingStatus = "rejected"
	BookingScheduled           BookingStatus = "scheduled"
	BookingPaymentPending      BookingStatus = "payment_pending"
	BookingPaymentFailed       BookingStatus = "payment_failed"
	BookingConfirmed           BookingStatus = "confirmed"
	BookingArchived            BookingStatus = "archived"
	BookingCancelled           BookingStatus = "cancelled"
)

// Booking is the core entity.
type Booking struct {
	ID                   uuid.UUID     `db:"id"`
	CandidateID          uuid.UUID     `db:"candidate_id"`
	InstituteID          uuid.UUID     `db:"institute_id"`
	TestID               *uuid.UUID    `db:"test_id"`
	SlotID               *uuid.UUID    `db:"slot_id"`
	Status               BookingStatus `db:"status"`
	RequiresVerification bool          `db:"requires_verification"`
	VerifiedBy           *uuid.UUID    `db:"verified_by"`
	VerifiedAt           *time.Time    `db:"verified_at"`
	RejectionReason      *string       `db:"rejection_reason"`
	ScheduledAt          *time.Time    `db:"scheduled_at"`
	PaymentRef           *string       `db:"payment_ref"`
	PaymentStatus        string        `db:"payment_status"`
	PaymentAmountCents   *int          `db:"payment_amount_cents"`
	PaymentAttempts      int           `db:"payment_attempts"`
	ArchivedAt           *time.Time    `db:"archived_at"`
	CreatedAt            time.Time     `db:"created_at"`
	UpdatedAt            time.Time     `db:"updated_at"`
}

// Slot represents a scheduled testing window at an institute.
type Slot struct {
	ID          uuid.UUID  `db:"id"`
	InstituteID uuid.UUID  `db:"institute_id"`
	TestID      *uuid.UUID `db:"test_id"`
	StartsAt    time.Time  `db:"starts_at"`
	EndsAt      time.Time  `db:"ends_at"`
	Capacity    int        `db:"capacity"`
	BookedCount int        `db:"booked_count"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

// Payment is an audit record of every payment attempt.
type Payment struct {
	ID            uuid.UUID `db:"id"`
	BookingID     uuid.UUID `db:"booking_id"`
	AmountCents   int       `db:"amount_cents"`
	Currency      string    `db:"currency"`
	Status        string    `db:"status"`
	Provider      string    `db:"provider"`
	ProviderRef   *string   `db:"provider_ref"`
	AttemptNumber int       `db:"attempt_number"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}
```

---

## 5. Step 3 — Config Updates

**File:** `internal/platform/config/config.go`

Add the following fields to the existing `Config` struct. Do not remove any existing fields:

```go
// Chapa payment gateway
ChapaSecretKey     string // env: CHAPA_SECRET_KEY
ChapaWebhookSecret string // env: CHAPA_WEBHOOK_SECRET
ChapaBaseURL       string // env: CHAPA_BASE_URL, default: https://api.chapa.co/v1

// Frontend (used to build return_url after payment)
FrontendBaseURL string // env: FRONTEND_BASE_URL, default: http://localhost:3000
```

In the config loading function (wherever `os.Getenv` or `viper` calls are made), add:

```go
cfg.ChapaSecretKey     = getEnv("CHAPA_SECRET_KEY", "")
cfg.ChapaWebhookSecret = getEnv("CHAPA_WEBHOOK_SECRET", "")
cfg.ChapaBaseURL       = getEnv("CHAPA_BASE_URL", "https://api.chapa.co/v1")
cfg.FrontendBaseURL    = getEnv("FRONTEND_BASE_URL", "http://localhost:3000")
```

> **Note for Codex:** `getEnv` is the helper already used in `config.go` for falling back to defaults. If it does not exist by that name, use whatever pattern is already used in that file (e.g., `os.Getenv` with a manual default check).

---

## 6. Step 4 — Module Scaffold

Create the directory `internal/booking/` with seven empty `.go` files:

```
internal/booking/dto.go
internal/booking/payment_provider.go
internal/booking/chapa_provider.go
internal/booking/repository.go
internal/booking/service.go
internal/booking/handler.go
internal/booking/routes.go
```

All files start with `package booking`.

---

## 7. Step 5 — DTOs

**File:** `internal/booking/dto.go`

```go
package booking

import "time"

// -------------------------------------------------------
// Request DTOs
// -------------------------------------------------------

// CreateBookingRequest is sent by the candidate.
// candidate_id is injected from the JWT — never accepted from the request body.
type CreateBookingRequest struct {
	InstituteID string `json:"institute_id"` // required
}

func (r *CreateBookingRequest) Validate() error {
	if r.InstituteID == "" {
		return ErrMissingInstituteID
	}
	return nil
}

// VerifyBookingRequest is sent by an admin or the institute.
type VerifyBookingRequest struct {
	Action          string `json:"action"`           // "approve" | "reject"
	RejectionReason string `json:"rejection_reason"` // required when action == "reject"
}

func (r *VerifyBookingRequest) Validate() error {
	if r.Action != "approve" && r.Action != "reject" {
		return ErrInvalidVerifyAction
	}
	if r.Action == "reject" && r.RejectionReason == "" {
		return ErrMissingRejectionReason
	}
	return nil
}

// ScheduleBookingRequest is sent by an admin to assign a slot.
type ScheduleBookingRequest struct {
	SlotID string `json:"slot_id"` // UUID of an existing slot
}

func (r *ScheduleBookingRequest) Validate() error {
	if r.SlotID == "" {
		return ErrMissingSlotID
	}
	return nil
}

// RescheduleBookingRequest reassigns to a new slot.
type RescheduleBookingRequest struct {
	SlotID string `json:"slot_id"` // UUID of the new slot
}

func (r *RescheduleBookingRequest) Validate() error {
	if r.SlotID == "" {
		return ErrMissingSlotID
	}
	return nil
}

// InitiatePaymentRequest is sent by the candidate to start a Chapa checkout.
type InitiatePaymentRequest struct {
	AmountCents int    `json:"amount_cents"` // amount in Ethiopian cents (1 ETB = 100 cents)
	Currency    string `json:"currency"`     // must be "ETB"
}

func (r *InitiatePaymentRequest) Validate() error {
	if r.AmountCents <= 0 {
		return ErrInvalidAmount
	}
	if r.Currency == "" {
		r.Currency = "ETB"
	}
	if r.Currency != "ETB" {
		return ErrInvalidCurrency
	}
	return nil
}

// CreateSlotRequest is sent by an admin.
type CreateSlotRequest struct {
	InstituteID string    `json:"institute_id"` // required
	StartsAt    time.Time `json:"starts_at"`    // required, RFC3339
	EndsAt      time.Time `json:"ends_at"`      // required, RFC3339
	Capacity    int       `json:"capacity"`     // default 1 if zero
}

func (r *CreateSlotRequest) Validate() error {
	if r.InstituteID == "" {
		return ErrMissingInstituteID
	}
	if r.StartsAt.IsZero() || r.EndsAt.IsZero() {
		return ErrMissingSlotTimes
	}
	if !r.EndsAt.After(r.StartsAt) {
		return ErrInvalidSlotTimes
	}
	if r.Capacity <= 0 {
		r.Capacity = 1
	}
	return nil
}

// -------------------------------------------------------
// Response DTOs
// -------------------------------------------------------

type BookingResponse struct {
	ID                   string  `json:"id"`
	CandidateID          string  `json:"candidate_id"`
	InstituteID          string  `json:"institute_id"`
	TestID               *string `json:"test_id,omitempty"`
	SlotID               *string `json:"slot_id,omitempty"`
	Status               string  `json:"status"`
	RequiresVerification bool    `json:"requires_verification"`
	VerifiedBy           *string `json:"verified_by,omitempty"`
	VerifiedAt           *string `json:"verified_at,omitempty"`
	RejectionReason      *string `json:"rejection_reason,omitempty"`
	ScheduledAt          *string `json:"scheduled_at,omitempty"`
	PaymentStatus        string  `json:"payment_status"`
	PaymentAmountCents   *int    `json:"payment_amount_cents,omitempty"`
	PaymentAttempts      int     `json:"payment_attempts"`
	ArchivedAt           *string `json:"archived_at,omitempty"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type SlotResponse struct {
	ID          string `json:"id"`
	InstituteID string `json:"institute_id"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	Capacity    int    `json:"capacity"`
	BookedCount int    `json:"booked_count"`
	Available   int    `json:"available"` // computed: capacity - booked_count
}

type PaymentResponse struct {
	ID            string  `json:"id"`
	BookingID     string  `json:"booking_id"`
	AmountCents   int     `json:"amount_cents"`
	Currency      string  `json:"currency"`
	Status        string  `json:"status"`
	Provider      string  `json:"provider"`
	ProviderRef   *string `json:"provider_ref,omitempty"`
	AttemptNumber int     `json:"attempt_number"`
	CreatedAt     string  `json:"created_at"`
}

// InitiatePaymentResponse includes the Chapa checkout URL.
type InitiatePaymentResponse struct {
	PaymentID   string `json:"payment_id"`
	CheckoutURL string `json:"checkout_url"` // redirect candidate here
	TxRef       string `json:"tx_ref"`
}

// -------------------------------------------------------
// Sentinel errors
// -------------------------------------------------------

import "errors"

var (
	ErrMissingInstituteID    = errors.New("institute_id is required")
	ErrMissingSlotID         = errors.New("slot_id is required")
	ErrMissingSlotTimes      = errors.New("starts_at and ends_at are required")
	ErrInvalidSlotTimes      = errors.New("ends_at must be after starts_at")
	ErrInvalidVerifyAction   = errors.New("action must be 'approve' or 'reject'")
	ErrMissingRejectionReason = errors.New("rejection_reason is required when rejecting")
	ErrInvalidAmount         = errors.New("amount_cents must be greater than zero")
	ErrInvalidCurrency       = errors.New("currency must be ETB")

	ErrBookingNotFound       = errors.New("booking not found")
	ErrSlotNotFound          = errors.New("slot not found")
	ErrPaymentNotFound       = errors.New("payment not found")
	ErrSlotFull              = errors.New("slot is fully booked")
	ErrForbidden             = errors.New("forbidden")
	ErrInvalidStatusForAction = errors.New("booking is not in the required status for this action")
	ErrMaxPaymentAttempts    = errors.New("maximum payment attempts reached")
	ErrAlreadyProcessed      = errors.New("payment already processed")
)
```

> **Note for Codex:** The `import "errors"` block inside the `var` block is a placeholder for clarity. In the actual file, put the `import` at the top of the file with other imports.

---

## 8. Step 6 — Payment Provider Interface

**File:** `internal/booking/payment_provider.go`

This interface is the abstraction layer. Chapa implements it today. Any future direct integration (Telebirr direct API, CBE direct API, Awash Bank, Dashen/Amole) implements it tomorrow — zero changes to service or handler code.

```go
package booking

import "context"

// PaymentProvider abstracts any payment gateway.
//
// Current implementation: Chapa (Ethiopian payment gateway).
// Chapa already aggregates Telebirr, CBE Birr, bank accounts, Visa/Mastercard
// on their hosted checkout page. Candidates are redirected to Chapa's page
// and pick their preferred method there.
//
// LIMITATION: This backend does not integrate directly with Telebirr,
// CBE Birr, Awash Bank, Dashen/Amole, or any individual bank's own API.
// Those are future work items. See FUTURE_INTEGRATIONS below.
//
// FUTURE_INTEGRATIONS:
//   - Telebirr Direct API  (Ethio Telecom standalone)
//   - CBE Birr Direct API  (Commercial Bank of Ethiopia)
//   - Awash Bank API
//   - Dashen Bank / Amole API
//   - International: Stripe (for foreign card payments)
//
// To add a new provider: implement this interface and wire it in app.go.
// No changes to service.go or handler.go are required.
type PaymentProvider interface {
	// InitiatePayment creates a hosted payment session with the provider.
	// Returns a CheckoutURL that the candidate must be redirected to.
	// The TxRef in the result is the unique reference stored in the payments table.
	InitiatePayment(ctx context.Context, req PaymentInitRequest) (PaymentInitResult, error)

	// VerifyTransaction confirms with the provider that a given tx_ref
	// was genuinely completed. Always call this from the webhook handler
	// before updating the database — never trust the webhook payload alone.
	VerifyTransaction(ctx context.Context, txRef string) (PaymentVerifyResult, error)

	// ValidateWebhookSignature verifies that an incoming webhook request
	// genuinely originated from this provider.
	// payload is the raw request body bytes.
	// signature is the value of the provider's signature header.
	ValidateWebhookSignature(payload []byte, signature string) bool
}

// PaymentInitRequest contains everything needed to start a payment session.
type PaymentInitRequest struct {
	TxRef       string // unique ref, format: adlts_booking_{bookingUUID}_{unixTimestamp}
	AmountCents int    // amount in cents (divide by 100 for ETB)
	Currency    string // "ETB"
	Email       string // candidate's email
	FirstName   string // candidate's first name
	LastName    string // candidate's last name
	Phone       string // candidate's phone number
	CallbackURL string // your webhook endpoint (Chapa POSTs here after payment)
	ReturnURL   string // frontend page candidate sees after completing payment
}

// PaymentInitResult is returned by InitiatePayment.
type PaymentInitResult struct {
	CheckoutURL string // redirect the candidate's browser to this URL
	TxRef       string // echo of the tx_ref (for storage)
}

// PaymentVerifyResult is returned by VerifyTransaction.
type PaymentVerifyResult struct {
	TxRef       string // matches what was sent in InitiatePayment
	Status      string // "success" | "failed" | "pending"
	AmountCents int    // verified amount from the provider (cents)
}
```

---

## 9. Step 7 — Chapa Provider Implementation

**File:** `internal/booking/chapa_provider.go`

Chapa API reference:
- Initialize: `POST https://api.chapa.co/v1/transaction/initialize`
- Verify: `GET https://api.chapa.co/v1/transaction/verify/{tx_ref}`
- Webhook signature header: `x-chapa-signature` (HMAC-SHA256 of payload, key = webhook secret)

```go
package booking

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ChapaProvider implements PaymentProvider using the Chapa payment gateway.
// Chapa is Ethiopia's primary payment aggregator. It supports:
//   - Telebirr (via Chapa's hosted page)
//   - CBE Birr (via Chapa's hosted page)
//   - Amole (via Chapa's hosted page)
//   - Visa / Mastercard (via Chapa's hosted page)
//   - Bank transfers (via Chapa's hosted page)
//
// The backend does NOT talk to any of these providers directly.
// All payment method selection happens on Chapa's hosted checkout page.
type ChapaProvider struct {
	secretKey     string
	webhookSecret string
	baseURL       string // default: https://api.chapa.co/v1
	httpClient    *http.Client
}

// NewChapaProvider constructs a ready-to-use ChapaProvider.
func NewChapaProvider(secretKey, webhookSecret, baseURL string) *ChapaProvider {
	if baseURL == "" {
		baseURL = "https://api.chapa.co/v1"
	}
	return &ChapaProvider{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		baseURL:       baseURL,
		httpClient:    &http.Client{Timeout: 15 * time.Second},
	}
}

// InitiatePayment calls POST /transaction/initialize on Chapa.
// On success it returns the CheckoutURL that the candidate must visit.
func (c *ChapaProvider) InitiatePayment(ctx context.Context, req PaymentInitRequest) (PaymentInitResult, error) {
	body := map[string]any{
		"amount":       fmt.Sprintf("%.2f", float64(req.AmountCents)/100.0),
		"currency":     req.Currency,
		"email":        req.Email,
		"first_name":   req.FirstName,
		"last_name":    req.LastName,
		"phone_number": req.Phone,
		"tx_ref":       req.TxRef,
		"callback_url": req.CallbackURL, // Chapa POSTs to this URL after payment
		"return_url":   req.ReturnURL,   // candidate is redirected here after completing
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return PaymentInitResult{}, fmt.Errorf("chapa: marshal init body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/transaction/initialize", bytes.NewReader(rawBody))
	if err != nil {
		return PaymentInitResult{}, fmt.Errorf("chapa: build init request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.secretKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return PaymentInitResult{}, fmt.Errorf("chapa: init http call: %w", err)
	}
	defer resp.Body.Close()

	var chapaResp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			CheckoutURL string `json:"checkout_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chapaResp); err != nil {
		return PaymentInitResult{}, fmt.Errorf("chapa: decode init response: %w", err)
	}
	if chapaResp.Status != "success" {
		return PaymentInitResult{}, fmt.Errorf("chapa: init failed: %s", chapaResp.Message)
	}

	return PaymentInitResult{
		CheckoutURL: chapaResp.Data.CheckoutURL,
		TxRef:       req.TxRef,
	}, nil
}

// VerifyTransaction calls GET /transaction/verify/{tx_ref} on Chapa.
// Always call this from the webhook handler — never trust the webhook body alone.
func (c *ChapaProvider) VerifyTransaction(ctx context.Context, txRef string) (PaymentVerifyResult, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/transaction/verify/"+txRef, nil)
	if err != nil {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: build verify request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.secretKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: verify http call: %w", err)
	}
	defer resp.Body.Close()

	var chapaResp struct {
		Status string `json:"status"`
		Data   struct {
			Status string  `json:"status"`
			TxRef  string  `json:"tx_ref"`
			Amount float64 `json:"amount"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chapaResp); err != nil {
		return PaymentVerifyResult{}, fmt.Errorf("chapa: decode verify response: %w", err)
	}

	return PaymentVerifyResult{
		TxRef:       chapaResp.Data.TxRef,
		Status:      chapaResp.Data.Status,
		AmountCents: int(chapaResp.Data.Amount * 100),
	}, nil
}

// ValidateWebhookSignature verifies the x-chapa-signature header.
// Chapa signs the raw request body with the webhook secret using HMAC-SHA256.
// The computed hex digest must match the header value exactly.
func (c *ChapaProvider) ValidateWebhookSignature(payload []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(c.webhookSecret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// generateTxRef produces a unique, traceable transaction reference.
// Format: adlts_booking_{bookingUUID}_{unixTimestamp}
// This is stored in payments.provider_ref and sent to Chapa as tx_ref.
func generateTxRef(bookingID string) string {
	return fmt.Sprintf("adlts_booking_%s_%d", bookingID, time.Now().Unix())
}

// discardBody reads and closes a response body to enable connection reuse.
func discardBody(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, body)
	body.Close()
}
```

---

## 10. Step 8 — Repository

**File:** `internal/booking/repository.go`

The repository handles all SQL. It follows the exact same pattern as `internal/identity/repository.go`: named parameters with `pgx/v5`, a `updateFields` helper for patch-style updates, and no ORM.

```go
package booking

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"<module>/internal/domain"
)

// Replace <module> with the actual Go module path from go.mod.

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// -------------------------------------------------------
// Booking queries
// -------------------------------------------------------

// CreateBooking inserts a new booking record and returns it with server-generated fields.
func (r *Repository) CreateBooking(ctx context.Context, b domain.Booking) (domain.Booking, error) {
	b.ID = uuid.New()
	now := time.Now()
	b.CreatedAt = now
	b.UpdatedAt = now

	const q = `
		INSERT INTO bookings
			(id, candidate_id, institute_id, status, requires_verification,
			 payment_status, payment_attempts, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING *`

	row, err := r.db.Query(ctx, q,
		b.ID, b.CandidateID, b.InstituteID, b.Status,
		b.RequiresVerification, b.PaymentStatus, b.PaymentAttempts,
		b.CreatedAt, b.UpdatedAt)
	if err != nil {
		return domain.Booking{}, err
	}
	defer row.Close()

	return scanBooking(row)
}

// BookingByID fetches a single booking by its UUID.
// Returns ErrBookingNotFound if it does not exist.
func (r *Repository) BookingByID(ctx context.Context, id uuid.UUID) (domain.Booking, error) {
	const q = `SELECT * FROM bookings WHERE id = $1 LIMIT 1`
	rows, err := r.db.Query(ctx, q, id)
	if err != nil {
		return domain.Booking{}, err
	}
	defer rows.Close()
	b, err := scanBooking(rows)
	if err != nil {
		return domain.Booking{}, ErrBookingNotFound
	}
	return b, nil
}

// BookingFilter constrains ListBookings results.
type BookingFilter struct {
	CandidateID *uuid.UUID
	InstituteID *uuid.UUID
	Status      *domain.BookingStatus
}

// ListBookings returns a paginated list of bookings (20 per page).
func (r *Repository) ListBookings(ctx context.Context, f BookingFilter, page int) ([]domain.Booking, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 20

	clauses := []string{"1=1"}
	args := []any{}
	idx := 1

	if f.CandidateID != nil {
		clauses = append(clauses, fmt.Sprintf("candidate_id = $%d", idx))
		args = append(args, *f.CandidateID)
		idx++
	}
	if f.InstituteID != nil {
		clauses = append(clauses, fmt.Sprintf("institute_id = $%d", idx))
		args = append(args, *f.InstituteID)
		idx++
	}
	if f.Status != nil {
		clauses = append(clauses, fmt.Sprintf("status = $%d", idx))
		args = append(args, string(*f.Status))
		idx++
	}

	args = append(args, 20, offset)
	q := fmt.Sprintf(
		`SELECT * FROM bookings WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		strings.Join(clauses, " AND "), idx, idx+1,
	)

	rows, err := r.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []domain.Booking
	for rows.Next() {
		b, err := scanBookingRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, b)
	}
	return results, nil
}

// UpdateBookingFields performs a patch-style update using only the provided fields.
// Keys in fields must exactly match column names in the bookings table.
func (r *Repository) UpdateBookingFields(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now()
	setClauses := make([]string, 0, len(fields))
	args := make([]any, 0, len(fields)+1)
	i := 1
	for col, val := range fields {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	q := fmt.Sprintf(`UPDATE bookings SET %s WHERE id = $%d`,
		strings.Join(setClauses, ", "), i)
	_, err := r.db.Exec(ctx, q, args...)
	return err
}

// DeleteBooking permanently deletes a booking record.
func (r *Repository) DeleteBooking(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM bookings WHERE id = $1`, id)
	return err
}

// -------------------------------------------------------
// Slot queries
// -------------------------------------------------------

// CreateSlot inserts a new slot record.
func (r *Repository) CreateSlot(ctx context.Context, s domain.Slot) (domain.Slot, error) {
	s.ID = uuid.New()
	now := time.Now()
	s.CreatedAt = now
	s.UpdatedAt = now

	const q = `
		INSERT INTO slots
			(id, institute_id, starts_at, ends_at, capacity, booked_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING *`

	rows, err := r.db.Query(ctx, q,
		s.ID, s.InstituteID, s.StartsAt, s.EndsAt,
		s.Capacity, s.BookedCount, s.CreatedAt, s.UpdatedAt)
	if err != nil {
		return domain.Slot{}, err
	}
	defer rows.Close()
	return scanSlot(rows)
}

// SlotByID fetches a single slot by UUID.
func (r *Repository) SlotByID(ctx context.Context, id uuid.UUID) (domain.Slot, error) {
	rows, err := r.db.Query(ctx, `SELECT * FROM slots WHERE id = $1 LIMIT 1`, id)
	if err != nil {
		return domain.Slot{}, err
	}
	defer rows.Close()
	s, err := scanSlot(rows)
	if err != nil {
		return domain.Slot{}, ErrSlotNotFound
	}
	return s, nil
}

// ListSlots returns paginated slots for an institute.
func (r *Repository) ListSlots(ctx context.Context, instituteID uuid.UUID, page int) ([]domain.Slot, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * 20
	rows, err := r.db.Query(ctx,
		`SELECT * FROM slots WHERE institute_id = $1 ORDER BY starts_at ASC LIMIT 20 OFFSET $2`,
		instituteID, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.Slot
	for rows.Next() {
		s, err := scanSlotRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, nil
}

// IncrementSlotBookedCount atomically increments booked_count.
// Must be called when a booking is successfully scheduled into a slot.
func (r *Repository) IncrementSlotBookedCount(ctx context.Context, slotID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE slots SET booked_count = booked_count + 1, updated_at = NOW() WHERE id = $1`,
		slotID)
	return err
}

// DecrementSlotBookedCount atomically decrements booked_count.
// Must be called when a booking is rescheduled away from a slot or cancelled.
// Floor is 0 — will not go negative.
func (r *Repository) DecrementSlotBookedCount(ctx context.Context, slotID uuid.UUID) error {
	_, err := r.db.Exec(ctx,
		`UPDATE slots SET booked_count = GREATEST(booked_count - 1, 0), updated_at = NOW() WHERE id = $1`,
		slotID)
	return err
}

// -------------------------------------------------------
// Payment queries
// -------------------------------------------------------

// CreatePayment inserts a new payment audit record.
func (r *Repository) CreatePayment(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	p.ID = uuid.New()
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	const q = `
		INSERT INTO payments
			(id, booking_id, amount_cents, currency, status, provider,
			 provider_ref, attempt_number, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING *`

	rows, err := r.db.Query(ctx, q,
		p.ID, p.BookingID, p.AmountCents, p.Currency, p.Status,
		p.Provider, p.ProviderRef, p.AttemptNumber, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return domain.Payment{}, err
	}
	defer rows.Close()
	return scanPayment(rows)
}

// PaymentByID fetches a single payment by UUID.
func (r *Repository) PaymentByID(ctx context.Context, id uuid.UUID) (domain.Payment, error) {
	rows, err := r.db.Query(ctx, `SELECT * FROM payments WHERE id = $1 LIMIT 1`, id)
	if err != nil {
		return domain.Payment{}, err
	}
	defer rows.Close()
	p, err := scanPayment(rows)
	if err != nil {
		return domain.Payment{}, ErrPaymentNotFound
	}
	return p, nil
}

// PaymentByProviderRef fetches a payment by the tx_ref stored in provider_ref.
// Used by the webhook handler for idempotency checks.
func (r *Repository) PaymentByProviderRef(ctx context.Context, providerRef string) (domain.Payment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT * FROM payments WHERE provider_ref = $1 ORDER BY created_at DESC LIMIT 1`,
		providerRef)
	if err != nil {
		return domain.Payment{}, err
	}
	defer rows.Close()
	p, err := scanPayment(rows)
	if err != nil {
		return domain.Payment{}, ErrPaymentNotFound
	}
	return p, nil
}

// ListPaymentsByBooking returns all payment attempts for a booking.
func (r *Repository) ListPaymentsByBooking(ctx context.Context, bookingID uuid.UUID) ([]domain.Payment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT * FROM payments WHERE booking_id = $1 ORDER BY attempt_number ASC`,
		bookingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []domain.Payment
	for rows.Next() {
		p, err := scanPaymentRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, p)
	}
	return results, nil
}

// UpdatePaymentFields performs a patch-style update on the payments table.
func (r *Repository) UpdatePaymentFields(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	fields["updated_at"] = time.Now()
	setClauses := make([]string, 0, len(fields))
	args := make([]any, 0, len(fields)+1)
	i := 1
	for col, val := range fields {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
		i++
	}
	args = append(args, id)
	q := fmt.Sprintf(`UPDATE payments SET %s WHERE id = $%d`,
		strings.Join(setClauses, ", "), i)
	_, err := r.db.Exec(ctx, q, args...)
	return err
}

// LatestPaymentAttemptNumber returns the highest attempt_number for a booking.
// Returns 0 if no payments exist yet.
func (r *Repository) LatestPaymentAttemptNumber(ctx context.Context, bookingID uuid.UUID) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(attempt_number), 0) FROM payments WHERE booking_id = $1`,
		bookingID).Scan(&n)
	return n, err
}

// -------------------------------------------------------
// Row scanners (internal helpers)
// -------------------------------------------------------
// Codex: implement scanBooking, scanBookingRow, scanSlot, scanSlotRow,
// scanPayment, scanPaymentRow using pgx's rows.Scan() in the same style
// as identity/repository.go. Each scanner maps columns to domain struct fields.
```

---

## 11. Step 9 — Service Layer

**File:** `internal/booking/service.go`

The service contains all business logic. It calls the repository for data and the mailer for emails. It holds a `PaymentProvider` interface — never a concrete Chapa type.

```go
package booking

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"<module>/internal/domain"
	"<module>/internal/platform/mailer"
)

const maxPaymentAttempts = 3

type Service struct {
	repo     *Repository
	provider PaymentProvider
	mailer   *mailer.Mailer
	// baseURL is the API's own public URL, used to build the callback_url for Chapa.
	// Read from config.BaseURL (already exists in config.go).
	baseURL string
	// frontendBaseURL is used to build the return_url (where candidate goes after paying).
	frontendBaseURL string
}

func NewService(
	repo *Repository,
	provider PaymentProvider,
	mailer *mailer.Mailer,
	baseURL string,
	frontendBaseURL string,
) *Service {
	return &Service{
		repo:            repo,
		provider:        provider,
		mailer:          mailer,
		baseURL:         baseURL,
		frontendBaseURL: frontendBaseURL,
	}
}

// -------------------------------------------------------
// Booking lifecycle
// -------------------------------------------------------

// CreateBooking is called by a candidate.
// It checks the institute's requires_verification setting, sets the initial status,
// and sends a confirmation email.
func (s *Service) CreateBooking(ctx context.Context, candidateID uuid.UUID, req CreateBookingRequest) (domain.Booking, error) {
	if err := req.Validate(); err != nil {
		return domain.Booking{}, err
	}

	instituteID, err := uuid.Parse(req.InstituteID)
	if err != nil {
		return domain.Booking{}, ErrMissingInstituteID
	}

	// Determine initial status based on institute's verification requirement.
	// TODO: fetch institute record here to read requires_verification.
	// For now, default to false (no verification required).
	// When identity module exposes InstituteByID, call it here.
	requiresVerification := false
	status := domain.BookingVerified
	if requiresVerification {
		status = domain.BookingPendingVerification
	}

	b := domain.Booking{
		CandidateID:          candidateID,
		InstituteID:          instituteID,
		Status:               status,
		RequiresVerification: requiresVerification,
		PaymentStatus:        "unpaid",
		PaymentAttempts:      0,
	}

	created, err := s.repo.CreateBooking(ctx, b)
	if err != nil {
		return domain.Booking{}, err
	}

	// Send email: booking received
	// s.mailer.Send(candidateEmail, "Booking Received", ...)
	// (fetch candidate email from identity module or include it in CreateBookingRequest)

	return created, nil
}

// GetBooking returns a booking if the caller is authorized to view it.
// - Candidate: can only see own bookings (candidate_id == callerID)
// - Institute: can only see bookings for their institute
// - Admin / super-admin: can see all
func (s *Service) GetBooking(ctx context.Context, callerID uuid.UUID, callerType string, bookingID uuid.UUID) (domain.Booking, error) {
	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return domain.Booking{}, ErrBookingNotFound
	}
	if err := s.authorizeBookingAccess(callerID, callerType, b); err != nil {
		return domain.Booking{}, err
	}
	return b, nil
}

// ListBookings returns paginated bookings scoped to the caller's access level.
func (s *Service) ListBookings(ctx context.Context, callerID uuid.UUID, callerType string, statusFilter *domain.BookingStatus, page int) ([]domain.Booking, error) {
	f := BookingFilter{}
	switch callerType {
	case "candidate":
		f.CandidateID = &callerID
	case "institute":
		f.InstituteID = &callerID
	case "admin", "super_admin":
		// no filter — admins see all
	default:
		return nil, ErrForbidden
	}
	if statusFilter != nil {
		f.Status = statusFilter
	}
	return s.repo.ListBookings(ctx, f, page)
}

// VerifyBooking is called by an admin or institute user.
// Approving transitions status to verified.
// Rejecting transitions status to rejected and stores the reason.
func (s *Service) VerifyBooking(ctx context.Context, callerID uuid.UUID, callerType string, bookingID uuid.UUID, req VerifyBookingRequest) (domain.Booking, error) {
	if err := req.Validate(); err != nil {
		return domain.Booking{}, err
	}
	if callerType != "admin" && callerType != "super_admin" && callerType != "institute" {
		return domain.Booking{}, ErrForbidden
	}

	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return domain.Booking{}, ErrBookingNotFound
	}
	if b.Status != domain.BookingPendingVerification {
		return domain.Booking{}, ErrInvalidStatusForAction
	}

	now := time.Now()
	fields := map[string]any{}

	if req.Action == "approve" {
		fields["status"] = string(domain.BookingVerified)
		fields["verified_by"] = callerID
		fields["verified_at"] = now
		// TODO: send approval email to candidate
	} else {
		fields["status"] = string(domain.BookingRejected)
		fields["rejection_reason"] = req.RejectionReason
		// TODO: send rejection email to candidate
	}

	if err := s.repo.UpdateBookingFields(ctx, bookingID, fields); err != nil {
		return domain.Booking{}, err
	}
	return s.repo.BookingByID(ctx, bookingID)
}

// ScheduleBooking is called by an admin to assign a slot to a verified booking.
// Prerequisite: booking status must be "verified".
// Effect: increments the slot's booked_count, sets booking status to "scheduled".
func (s *Service) ScheduleBooking(ctx context.Context, callerType string, bookingID uuid.UUID, req ScheduleBookingRequest) (domain.Booking, error) {
	if err := req.Validate(); err != nil {
		return domain.Booking{}, err
	}
	if callerType != "admin" && callerType != "super_admin" {
		return domain.Booking{}, ErrForbidden
	}

	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return domain.Booking{}, ErrBookingNotFound
	}
	if b.Status != domain.BookingVerified {
		return domain.Booking{}, ErrInvalidStatusForAction
	}

	slotID, err := uuid.Parse(req.SlotID)
	if err != nil {
		return domain.Booking{}, ErrMissingSlotID
	}

	slot, err := s.repo.SlotByID(ctx, slotID)
	if err != nil {
		return domain.Booking{}, ErrSlotNotFound
	}
	if slot.BookedCount >= slot.Capacity {
		return domain.Booking{}, ErrSlotFull
	}

	if err := s.repo.IncrementSlotBookedCount(ctx, slotID); err != nil {
		return domain.Booking{}, err
	}

	fields := map[string]any{
		"slot_id":      slotID,
		"scheduled_at": slot.StartsAt,
		"status":       string(domain.BookingScheduled),
	}
	if err := s.repo.UpdateBookingFields(ctx, bookingID, fields); err != nil {
		return domain.Booking{}, err
	}
	// TODO: send scheduling confirmation email to candidate

	return s.repo.BookingByID(ctx, bookingID)
}

// RescheduleBooking moves a booking to a new slot.
// The old slot's booked_count is decremented; the new slot's is incremented.
// Sends a warning email to the candidate about the change.
func (s *Service) RescheduleBooking(ctx context.Context, callerID uuid.UUID, callerType string, bookingID uuid.UUID, req RescheduleBookingRequest) (domain.Booking, error) {
	if err := req.Validate(); err != nil {
		return domain.Booking{}, err
	}

	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return domain.Booking{}, ErrBookingNotFound
	}

	// Only admins can reschedule any booking; candidates can only reschedule their own
	if callerType == "candidate" && b.CandidateID != callerID {
		return domain.Booking{}, ErrForbidden
	}
	if callerType != "admin" && callerType != "super_admin" && callerType != "candidate" {
		return domain.Booking{}, ErrForbidden
	}

	allowedStatuses := map[domain.BookingStatus]bool{
		domain.BookingScheduled: true,
		domain.BookingConfirmed: true,
	}
	if !allowedStatuses[b.Status] {
		return domain.Booking{}, ErrInvalidStatusForAction
	}

	newSlotID, err := uuid.Parse(req.SlotID)
	if err != nil {
		return domain.Booking{}, ErrMissingSlotID
	}
	newSlot, err := s.repo.SlotByID(ctx, newSlotID)
	if err != nil {
		return domain.Booking{}, ErrSlotNotFound
	}
	if newSlot.BookedCount >= newSlot.Capacity {
		return domain.Booking{}, ErrSlotFull
	}

	// Decrement old slot
	if b.SlotID != nil {
		_ = s.repo.DecrementSlotBookedCount(ctx, *b.SlotID)
	}
	// Increment new slot
	if err := s.repo.IncrementSlotBookedCount(ctx, newSlotID); err != nil {
		return domain.Booking{}, err
	}

	fields := map[string]any{
		"slot_id":      newSlotID,
		"scheduled_at": newSlot.StartsAt,
		"status":       string(domain.BookingScheduled),
	}
	if err := s.repo.UpdateBookingFields(ctx, bookingID, fields); err != nil {
		return domain.Booking{}, err
	}
	// TODO: send reschedule warning email to candidate

	return s.repo.BookingByID(ctx, bookingID)
}

// DeleteBooking permanently removes a booking.
// Candidates may only delete their own bookings in "drafted" status.
// Admins may delete any booking.
// Frees the slot's booked_count if a slot was assigned.
func (s *Service) DeleteBooking(ctx context.Context, callerID uuid.UUID, callerType string, bookingID uuid.UUID) error {
	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return ErrBookingNotFound
	}

	if callerType == "candidate" {
		if b.CandidateID != callerID {
			return ErrForbidden
		}
		if b.Status != domain.BookingDrafted {
			return ErrInvalidStatusForAction
		}
	} else if callerType != "admin" && callerType != "super_admin" {
		return ErrForbidden
	}

	if b.SlotID != nil {
		_ = s.repo.DecrementSlotBookedCount(ctx, *b.SlotID)
	}
	// TODO: if deleted by admin, send warning email to candidate

	return s.repo.DeleteBooking(ctx, bookingID)
}

// ArchiveBooking is called by the Testing Flow module when a test session ends.
// It transitions the booking to "archived" status.
func (s *Service) ArchiveBooking(ctx context.Context, bookingID uuid.UUID) error {
	fields := map[string]any{
		"status":      string(domain.BookingArchived),
		"archived_at": time.Now(),
	}
	return s.repo.UpdateBookingFields(ctx, bookingID, fields)
}

// -------------------------------------------------------
// Payment lifecycle
// -------------------------------------------------------

// InitiatePayment starts a Chapa payment session for a scheduled booking.
// The candidate must be the owner of the booking.
// Returns the CheckoutURL for Chapa's hosted page.
func (s *Service) InitiatePayment(
	ctx context.Context,
	candidateID uuid.UUID,
	candidateEmail, candidateFirstName, candidateLastName, candidatePhone string,
	bookingID uuid.UUID,
	req InitiatePaymentRequest,
) (InitiatePaymentResponse, error) {
	if err := req.Validate(); err != nil {
		return InitiatePaymentResponse{}, err
	}

	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return InitiatePaymentResponse{}, ErrBookingNotFound
	}
	if b.CandidateID != candidateID {
		return InitiatePaymentResponse{}, ErrForbidden
	}
	if b.Status != domain.BookingScheduled {
		return InitiatePaymentResponse{}, ErrInvalidStatusForAction
	}

	// Get latest attempt number for this booking
	lastAttempt, err := s.repo.LatestPaymentAttemptNumber(ctx, bookingID)
	if err != nil {
		return InitiatePaymentResponse{}, err
	}
	attemptNumber := lastAttempt + 1

	if attemptNumber > maxPaymentAttempts {
		return InitiatePaymentResponse{}, ErrMaxPaymentAttempts
	}

	txRef := generateTxRef(bookingID.String())

	// Build URLs
	callbackURL := fmt.Sprintf("%s/api/v1/bookings/%s/payments/callback", s.baseURL, bookingID)
	returnURL := fmt.Sprintf("%s/bookings/%s/payment/success", s.frontendBaseURL, bookingID)

	// Call Chapa
	result, err := s.provider.InitiatePayment(ctx, PaymentInitRequest{
		TxRef:       txRef,
		AmountCents: req.AmountCents,
		Currency:    req.Currency,
		Email:       candidateEmail,
		FirstName:   candidateFirstName,
		LastName:    candidateLastName,
		Phone:       candidatePhone,
		CallbackURL: callbackURL,
		ReturnURL:   returnURL,
	})
	if err != nil {
		return InitiatePaymentResponse{}, fmt.Errorf("initiate payment: %w", err)
	}

	// Create payment record
	payment, err := s.repo.CreatePayment(ctx, domain.Payment{
		BookingID:     bookingID,
		AmountCents:   req.AmountCents,
		Currency:      req.Currency,
		Status:        "pending",
		Provider:      "chapa",
		ProviderRef:   &txRef,
		AttemptNumber: attemptNumber,
	})
	if err != nil {
		return InitiatePaymentResponse{}, err
	}

	// Update booking status
	if err := s.repo.UpdateBookingFields(ctx, bookingID, map[string]any{
		"status":               string(domain.BookingPaymentPending),
		"payment_status":       "pending",
		"payment_amount_cents": req.AmountCents,
		"payment_attempts":     attemptNumber,
	}); err != nil {
		return InitiatePaymentResponse{}, err
	}

	return InitiatePaymentResponse{
		PaymentID:   payment.ID.String(),
		CheckoutURL: result.CheckoutURL,
		TxRef:       txRef,
	}, nil
}

// HandleChapaWebhook processes the Chapa webhook event.
//
// SECURITY: The raw body and signature must be verified by the handler BEFORE
// calling this method. This method assumes the signature has already been validated.
//
// IDEMPOTENCY: If the payment has already been processed (status != "pending"),
// this method returns nil without doing anything. Chapa may retry webhooks.
//
// DOUBLE-VERIFICATION: Even though the webhook is signed, this method calls
// VerifyTransaction on Chapa's API to independently confirm the payment status.
// Never trust the webhook payload alone.
func (s *Service) HandleChapaWebhook(ctx context.Context, txRef, webhookStatus string) error {
	// 1. Look up payment record by provider_ref (tx_ref)
	payment, err := s.repo.PaymentByProviderRef(ctx, txRef)
	if err != nil {
		return ErrPaymentNotFound
	}

	// 2. Idempotency: if already finalized, do nothing
	if payment.Status == "success" || payment.Status == "failed" {
		return nil
	}

	// 3. Double-verify with Chapa API
	verified, err := s.provider.VerifyTransaction(ctx, txRef)
	if err != nil {
		return fmt.Errorf("verify transaction: %w", err)
	}

	if verified.Status == "success" {
		// Update payment record
		_ = s.repo.UpdatePaymentFields(ctx, payment.ID, map[string]any{
			"status": "success",
		})
		// Update booking
		_ = s.repo.UpdateBookingFields(ctx, payment.BookingID, map[string]any{
			"status":         string(domain.BookingConfirmed),
			"payment_status": "paid",
			"payment_ref":    txRef,
		})
		// TODO: send payment confirmation email to candidate
		// TODO: trigger test creation logic (or emit an event for the testing module)
	} else {
		// Payment failed
		_ = s.repo.UpdatePaymentFields(ctx, payment.ID, map[string]any{
			"status": "failed",
		})
		_ = s.repo.UpdateBookingFields(ctx, payment.BookingID, map[string]any{
			"status":         string(domain.BookingPaymentFailed),
			"payment_status": "failed",
		})
		// TODO: send payment failure email to candidate with retry instructions
	}

	return nil
}

// RetryPayment allows a candidate to attempt payment again after failure.
// Maximum 3 total attempts (controlled by maxPaymentAttempts constant).
func (s *Service) RetryPayment(
	ctx context.Context,
	candidateID uuid.UUID,
	candidateEmail, candidateFirstName, candidateLastName, candidatePhone string,
	bookingID uuid.UUID,
	req InitiatePaymentRequest,
) (InitiatePaymentResponse, error) {
	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return InitiatePaymentResponse{}, ErrBookingNotFound
	}
	if b.CandidateID != candidateID {
		return InitiatePaymentResponse{}, ErrForbidden
	}
	if b.Status != domain.BookingPaymentFailed {
		return InitiatePaymentResponse{}, ErrInvalidStatusForAction
	}
	if b.PaymentAttempts >= maxPaymentAttempts {
		return InitiatePaymentResponse{}, ErrMaxPaymentAttempts
	}

	// Reset booking to scheduled so InitiatePayment's status check passes
	_ = s.repo.UpdateBookingFields(ctx, bookingID, map[string]any{
		"status": string(domain.BookingScheduled),
	})

	return s.InitiatePayment(ctx,
		candidateID, candidateEmail, candidateFirstName, candidateLastName, candidatePhone,
		bookingID, req)
}

// ListPayments returns all payment attempts for a booking.
func (s *Service) ListPayments(ctx context.Context, callerID uuid.UUID, callerType string, bookingID uuid.UUID) ([]domain.Payment, error) {
	b, err := s.repo.BookingByID(ctx, bookingID)
	if err != nil {
		return nil, ErrBookingNotFound
	}
	if err := s.authorizeBookingAccess(callerID, callerType, b); err != nil {
		return nil, err
	}
	return s.repo.ListPaymentsByBooking(ctx, bookingID)
}

// -------------------------------------------------------
// Slot management
// -------------------------------------------------------

// CreateSlot is called by an admin to add a testing slot for an institute.
func (s *Service) CreateSlot(ctx context.Context, callerType string, req CreateSlotRequest) (domain.Slot, error) {
	if err := req.Validate(); err != nil {
		return domain.Slot{}, err
	}
	if callerType != "admin" && callerType != "super_admin" {
		return domain.Slot{}, ErrForbidden
	}

	instituteID, err := uuid.Parse(req.InstituteID)
	if err != nil {
		return domain.Slot{}, ErrMissingInstituteID
	}

	return s.repo.CreateSlot(ctx, domain.Slot{
		InstituteID: instituteID,
		StartsAt:    req.StartsAt,
		EndsAt:      req.EndsAt,
		Capacity:    req.Capacity,
		BookedCount: 0,
	})
}

// ListSlots returns paginated slots for an institute.
func (s *Service) ListSlots(ctx context.Context, instituteID uuid.UUID, page int) ([]domain.Slot, error) {
	return s.repo.ListSlots(ctx, instituteID, page)
}

// GetSlot returns a single slot by ID.
func (s *Service) GetSlot(ctx context.Context, slotID uuid.UUID) (domain.Slot, error) {
	return s.repo.SlotByID(ctx, slotID)
}

// -------------------------------------------------------
// Internal helpers
// -------------------------------------------------------

func (s *Service) authorizeBookingAccess(callerID uuid.UUID, callerType string, b domain.Booking) error {
	switch callerType {
	case "candidate":
		if b.CandidateID != callerID {
			return ErrForbidden
		}
	case "institute":
		if b.InstituteID != callerID {
			return ErrForbidden
		}
	case "admin", "super_admin":
		// full access
	default:
		return ErrForbidden
	}
	return nil
}
```

---

## 12. Step 10 — Handlers

**File:** `internal/booking/handler.go`

All handlers use `httpx.DecodeJSON` + `httpx.Success` / `httpx.Failure`. Never use `http.Error`. All caller identity comes from the JWT claims injected by `security.go` middleware.

```go
package booking

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"<module>/internal/platform/httpx"
	"<module>/internal/platform/security"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// -------------------------------------------------------
// Booking handlers
// -------------------------------------------------------

// CreateBooking handles POST /bookings
// Auth: candidate only
func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())

	var req CreateBookingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, err.Error())
		return
	}

	candidateID, err := uuid.Parse(claims.Subject)
	if err != nil {
		httpx.Failure(w, http.StatusUnauthorized, "invalid token subject")
		return
	}

	b, err := h.service.CreateBooking(r.Context(), candidateID, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusCreated, toBookingResponse(b))
}

// GetBooking handles GET /bookings/{id}
// Auth: candidate (own), institute (own), admin, super_admin
func (h *Handler) GetBooking(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())
	bookingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	callerID, _ := uuid.Parse(claims.Subject)
	b, err := h.service.GetBooking(r.Context(), callerID, claims.EntityType, bookingID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, toBookingResponse(b))
}

// ListBookings handles GET /bookings
// Auth: all authenticated entities (scoped by caller type)
// Query params: status (optional), page (optional, default 1)
func (h *Handler) ListBookings(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())
	callerID, _ := uuid.Parse(claims.Subject)

	page := parsePageParam(r)

	var statusFilter *domain.BookingStatus
	if s := r.URL.Query().Get("status"); s != "" {
		bs := domain.BookingStatus(s)
		statusFilter = &bs
	}

	bookings, err := h.service.ListBookings(r.Context(), callerID, claims.EntityType, statusFilter, page)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	resp := make([]BookingResponse, 0, len(bookings))
	for _, b := range bookings {
		resp = append(resp, toBookingResponse(b))
	}
	httpx.Success(w, http.StatusOK, resp)
}

// VerifyBooking handles PATCH /bookings/{id}/verify
// Auth: admin, super_admin, institute
func (h *Handler) VerifyBooking(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())
	bookingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var req VerifyBookingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, err.Error())
		return
	}

	callerID, _ := uuid.Parse(claims.Subject)
	b, err := h.service.VerifyBooking(r.Context(), callerID, claims.EntityType, bookingID, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, toBookingResponse(b))
}

// ScheduleBooking handles PATCH /bookings/{id}/schedule
// Auth: admin, super_admin
func (h *Handler) ScheduleBooking(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())
	bookingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var req ScheduleBookingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, err.Error())
		return
	}

	b, err := h.service.ScheduleBooking(r.Context(), claims.EntityType, bookingID, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, toBookingResponse(b))
}

// RescheduleBooking handles PATCH /bookings/{id}/reschedule
// Auth: admin, super_admin, candidate (own booking only)
func (h *Handler) RescheduleBooking(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())
	bookingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var req RescheduleBookingRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, err.Error())
		return
	}

	callerID, _ := uuid.Parse(claims.Subject)
	b, err := h.service.RescheduleBooking(r.Context(), callerID, claims.EntityType, bookingID, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, toBookingResponse(b))
}

// DeleteBooking handles DELETE /bookings/{id}
// Auth: candidate (own, drafted only), admin, super_admin
func (h *Handler) DeleteBooking(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())
	bookingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	callerID, _ := uuid.Parse(claims.Subject)
	if err := h.service.DeleteBooking(r.Context(), callerID, claims.EntityType, bookingID); err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "booking deleted"})
}

// -------------------------------------------------------
// Payment handlers
// -------------------------------------------------------

// InitiatePayment handles POST /bookings/{id}/payments
// Auth: candidate only
// Body: InitiatePaymentRequest
// Response: InitiatePaymentResponse (includes checkout_url)
func (h *Handler) InitiatePayment(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())
	bookingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var req InitiatePaymentRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, err.Error())
		return
	}

	candidateID, _ := uuid.Parse(claims.Subject)

	// Candidate profile fields (email, name, phone) must come from JWT claims
	// or be fetched from the identity module. For now, read from custom JWT claims.
	// Adjust when identity module exposes a CandidateByID method.
	email := claims.Email
	firstName := claims.FirstName
	lastName := claims.LastName
	phone := claims.Phone

	resp, err := h.service.InitiatePayment(r.Context(),
		candidateID, email, firstName, lastName, phone, bookingID, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusCreated, resp)
}

// HandleChapaWebhook handles POST /bookings/{id}/payments/callback
// Auth: NONE — this is a public endpoint called by Chapa's servers.
// It MUST return 200 OK to prevent Chapa from retrying.
// Signature verification happens here before calling the service.
func (h *Handler) HandleChapaWebhook(w http.ResponseWriter, r *http.Request) {
	// 1. Read raw body — must be done before any decoding
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "cannot read body")
		return
	}

	// 2. Verify Chapa signature — check both header names Chapa may use
	sig := r.Header.Get("x-chapa-signature")
	if sig == "" {
		sig = r.Header.Get("chapa-signature")
	}
	if !h.service.provider.ValidateWebhookSignature(rawBody, sig) {
		// Return 401 but log the attempt — do not process
		httpx.Failure(w, http.StatusUnauthorized, "invalid webhook signature")
		return
	}

	// 3. Parse the webhook event
	var event struct {
		TxRef  string `json:"tx_ref"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rawBody, &event); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "invalid webhook body")
		return
	}

	// 4. Process (idempotent — safe to call multiple times)
	if err := h.service.HandleChapaWebhook(r.Context(), event.TxRef, event.Status); err != nil {
		// Log error but still return 200 to prevent Chapa from retrying forever
		// (Chapa retries on non-200 responses; retrying a processing error
		//  would just fail again. Log and move on.)
		_ = err
	}

	// 5. Must return 200 — Chapa retries on any other status
	w.WriteHeader(http.StatusOK)
}

// RetryPayment handles POST /bookings/{id}/payments/retry
// Auth: candidate only
func (h *Handler) RetryPayment(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())
	bookingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	var req InitiatePaymentRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, err.Error())
		return
	}

	candidateID, _ := uuid.Parse(claims.Subject)
	resp, err := h.service.RetryPayment(r.Context(),
		candidateID, claims.Email, claims.FirstName, claims.LastName, claims.Phone,
		bookingID, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusCreated, resp)
}

// ListPayments handles GET /bookings/{id}/payments
// Auth: all authenticated entities with booking access
func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())
	bookingID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "invalid booking id")
		return
	}

	callerID, _ := uuid.Parse(claims.Subject)
	payments, err := h.service.ListPayments(r.Context(), callerID, claims.EntityType, bookingID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	resp := make([]PaymentResponse, 0, len(payments))
	for _, p := range payments {
		resp = append(resp, toPaymentResponse(p))
	}
	httpx.Success(w, http.StatusOK, resp)
}

// -------------------------------------------------------
// Slot handlers
// -------------------------------------------------------

// CreateSlot handles POST /slots
// Auth: admin, super_admin
func (h *Handler) CreateSlot(w http.ResponseWriter, r *http.Request) {
	claims := security.ClaimsFromContext(r.Context())

	var req CreateSlotRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, err.Error())
		return
	}

	slot, err := h.service.CreateSlot(r.Context(), claims.EntityType, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusCreated, toSlotResponse(slot))
}

// ListSlots handles GET /slots
// Query params: institute_id (required), page (optional)
func (h *Handler) ListSlots(w http.ResponseWriter, r *http.Request) {
	instituteIDStr := r.URL.Query().Get("institute_id")
	instituteID, err := uuid.Parse(instituteIDStr)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "institute_id query param required")
		return
	}

	slots, err := h.service.ListSlots(r.Context(), instituteID, parsePageParam(r))
	if err != nil {
		handleServiceError(w, err)
		return
	}

	resp := make([]SlotResponse, 0, len(slots))
	for _, s := range slots {
		resp = append(resp, toSlotResponse(s))
	}
	httpx.Success(w, http.StatusOK, resp)
}

// GetSlot handles GET /slots/{id}
func (h *Handler) GetSlot(w http.ResponseWriter, r *http.Request) {
	slotID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "invalid slot id")
		return
	}

	slot, err := h.service.GetSlot(r.Context(), slotID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	httpx.Success(w, http.StatusOK, toSlotResponse(slot))
}

// -------------------------------------------------------
// Internal handler helpers
// -------------------------------------------------------

func handleServiceError(w http.ResponseWriter, err error) {
	switch err {
	case ErrBookingNotFound, ErrSlotNotFound, ErrPaymentNotFound:
		httpx.Failure(w, http.StatusNotFound, err.Error())
	case ErrForbidden:
		httpx.Failure(w, http.StatusForbidden, err.Error())
	case ErrInvalidStatusForAction, ErrSlotFull, ErrMaxPaymentAttempts,
		ErrInvalidVerifyAction, ErrMissingRejectionReason,
		ErrInvalidAmount, ErrInvalidCurrency, ErrMissingInstituteID,
		ErrMissingSlotID, ErrInvalidSlotTimes, ErrMissingSlotTimes:
		httpx.Failure(w, http.StatusBadRequest, err.Error())
	case ErrAlreadyProcessed:
		httpx.Failure(w, http.StatusConflict, err.Error())
	default:
		httpx.Failure(w, http.StatusInternalServerError, "internal error")
	}
}

func parsePageParam(r *http.Request) int {
	p := 1
	if s := r.URL.Query().Get("page"); s != "" {
		if n, err := fmt.Sscanf(s, "%d", &p); n != 1 || err != nil {
			p = 1
		}
	}
	if p < 1 {
		p = 1
	}
	return p
}

// -------------------------------------------------------
// Response mappers
// -------------------------------------------------------

func toBookingResponse(b domain.Booking) BookingResponse {
	resp := BookingResponse{
		ID:                   b.ID.String(),
		CandidateID:          b.CandidateID.String(),
		InstituteID:          b.InstituteID.String(),
		Status:               string(b.Status),
		RequiresVerification: b.RequiresVerification,
		PaymentStatus:        b.PaymentStatus,
		PaymentAmountCents:   b.PaymentAmountCents,
		PaymentAttempts:      b.PaymentAttempts,
		CreatedAt:            b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            b.UpdatedAt.Format(time.RFC3339),
	}
	if b.TestID != nil {
		s := b.TestID.String(); resp.TestID = &s
	}
	if b.SlotID != nil {
		s := b.SlotID.String(); resp.SlotID = &s
	}
	if b.VerifiedBy != nil {
		s := b.VerifiedBy.String(); resp.VerifiedBy = &s
	}
	if b.VerifiedAt != nil {
		s := b.VerifiedAt.Format(time.RFC3339); resp.VerifiedAt = &s
	}
	if b.RejectionReason != nil {
		resp.RejectionReason = b.RejectionReason
	}
	if b.ScheduledAt != nil {
		s := b.ScheduledAt.Format(time.RFC3339); resp.ScheduledAt = &s
	}
	if b.ArchivedAt != nil {
		s := b.ArchivedAt.Format(time.RFC3339); resp.ArchivedAt = &s
	}
	return resp
}

func toSlotResponse(s domain.Slot) SlotResponse {
	return SlotResponse{
		ID:          s.ID.String(),
		InstituteID: s.InstituteID.String(),
		StartsAt:    s.StartsAt.Format(time.RFC3339),
		EndsAt:      s.EndsAt.Format(time.RFC3339),
		Capacity:    s.Capacity,
		BookedCount: s.BookedCount,
		Available:   s.Capacity - s.BookedCount,
	}
}

func toPaymentResponse(p domain.Payment) PaymentResponse {
	resp := PaymentResponse{
		ID:            p.ID.String(),
		BookingID:     p.BookingID.String(),
		AmountCents:   p.AmountCents,
		Currency:      p.Currency,
		Status:        p.Status,
		Provider:      p.Provider,
		AttemptNumber: p.AttemptNumber,
		CreatedAt:     p.CreatedAt.Format(time.RFC3339),
	}
	if p.ProviderRef != nil {
		resp.ProviderRef = p.ProviderRef
	}
	return resp
}
```

> **Note for Codex:** `claims.Email`, `claims.FirstName`, `claims.LastName`, `claims.Phone` assume these fields exist on the JWT claims struct in `security.go`. If they do not, add them when issuing tokens in `identity/service.go` (`issueToken`), and read them in `security.go`'s claims parser.

---

## 13. Step 11 — Routes

**File:** `internal/booking/routes.go`

```go
package booking

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"<module>/internal/platform/security"
)

// RegisterRoutes mounts all booking, slot, and payment routes onto the provided router.
// It is called from app.go on the /api/v1 subrouter.
func (h *Handler) RegisterRoutes(r chi.Router, auth *security.Manager) {

	// ----------------------------------------------------------
	// Booking routes
	// ----------------------------------------------------------
	r.Route("/bookings", func(r chi.Router) {

		// Candidate creates a booking
		r.With(auth.RequireEntityType("candidate")).
			Post("/", h.CreateBooking)

		// Any authenticated entity lists bookings (scope applied in service)
		r.With(auth.RequireAuth()).
			Get("/", h.ListBookings)

		r.Route("/{id}", func(r chi.Router) {

			// Get booking details
			r.With(auth.RequireAuth()).
				Get("/", h.GetBooking)

			// Delete a booking
			// Candidates may delete own drafts; admins may delete any
			r.With(auth.RequireAuth()).
				Delete("/", h.DeleteBooking)

			// Institute / admin approves or rejects a pending booking
			r.With(auth.RequireEntityType("admin", "super_admin", "institute")).
				Patch("/verify", h.VerifyBooking)

			// Admin assigns a slot to a verified booking
			r.With(auth.RequireEntityType("admin", "super_admin")).
				Patch("/schedule", h.ScheduleBooking)

			// Admin or candidate reschedules to a different slot
			r.With(auth.RequireAuth()).
				Patch("/reschedule", h.RescheduleBooking)

			// ----------------------------------------------------------
			// Payment sub-routes (nested under /bookings/{id}/payments)
			// ----------------------------------------------------------
			r.Route("/payments", func(r chi.Router) {

				// Candidate initiates a Chapa payment session
				r.With(auth.RequireEntityType("candidate")).
					Post("/", h.InitiatePayment)

				// Chapa posts here after payment completes — NO auth middleware
				// Signature verification is done inside the handler
				r.Post("/callback", h.HandleChapaWebhook)

				// Candidate retries after a failed payment
				r.With(auth.RequireEntityType("candidate")).
					Post("/retry", h.RetryPayment)

				// Any authorized entity views payment history for this booking
				r.With(auth.RequireAuth()).
					Get("/", h.ListPayments)
			})
		})
	})

	// ----------------------------------------------------------
	// Slot routes
	// ----------------------------------------------------------
	r.Route("/slots", func(r chi.Router) {

		// Admin creates a slot
		r.With(auth.RequireEntityType("admin", "super_admin")).
			Post("/", h.CreateSlot)

		// Any authenticated entity can list or view slots
		r.With(auth.RequireAuth()).
			Get("/", h.ListSlots)

		r.With(auth.RequireAuth()).
			Get("/{id}", h.GetSlot)
	})
}
```

> **Note for Codex:** `auth.RequireEntityType(...)` may not exist yet in `security.go`. It is a middleware that checks `claims.EntityType` against the allowed list and returns 403 if not matched. If it does not exist, implement it as:
> ```go
> func (m *Manager) RequireEntityType(types ...string) func(http.Handler) http.Handler {
>     return func(next http.Handler) http.Handler {
>         return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
>             claims := ClaimsFromContext(r.Context())
>             for _, t := range types {
>                 if claims.EntityType == t {
>                     next.ServeHTTP(w, r)
>                     return
>                 }
>             }
>             httpx.Failure(w, http.StatusForbidden, "forbidden")
>         })
>     }
> }
> ```

---

## 14. Step 12 — Wire into app.go

**File:** `internal/app/app.go`

Find the section where `apiV1` router is set up and the identity module is mounted. Append the booking module wiring **after** all existing identity wiring. Do not change any existing lines.

```go
// --- ADD AFTER existing identity module wiring ---

// Booking, scheduling & payment module
bookingRepo    := booking.NewRepository(dbPool)
chapaProvider  := booking.NewChapaProvider(cfg.ChapaSecretKey, cfg.ChapaWebhookSecret, cfg.ChapaBaseURL)
bookingSvc     := booking.NewService(bookingRepo, chapaProvider, mailer, cfg.BaseURL, cfg.FrontendBaseURL)
bookingHandler := booking.NewHandler(bookingSvc)
bookingHandler.RegisterRoutes(apiV1, jwtManager)

// --- END booking wiring ---
```

Add the import at the top of `app.go`:
```go
booking "<module>/internal/booking"
```

---

## 15. Step 13 — Auth Middleware Fix

**File:** `internal/platform/security/security.go`

This is an existing bug documented in the analysis: auth middleware uses `http.Error` instead of the JSON envelope. Fix all occurrences.

Find every call to `http.Error(w, ...)` inside middleware functions (`RequireAuth`, token parsing, etc.) and replace with:

```go
// BEFORE (existing buggy code):
http.Error(w, "unauthorized", http.StatusUnauthorized)
http.Error(w, "forbidden", http.StatusForbidden)

// AFTER (correct — uses JSON envelope):
httpx.Failure(w, http.StatusUnauthorized, "unauthorized")
httpx.Failure(w, http.StatusForbidden, "forbidden")
```

Add the httpx import if not already present:
```go
"<module>/internal/platform/httpx"
```

Also ensure `ClaimsFromContext` exists and returns a struct with at minimum:
```go
type Claims struct {
    Subject    string // entity UUID
    EntityType string // "candidate" | "expert" | "admin" | "super_admin" | "institute" | "transport_authority"
    Email      string
    FirstName  string
    LastName   string
    Phone      string
    // any existing fields
}
```

If `Email`, `FirstName`, `LastName`, `Phone` are not in the claims yet, add them to the JWT payload in `identity/service.go`'s `issueToken` function, and parse them back in `security.go`'s `ValidateToken` function.

---

## 16. Step 14 — Environment Variables

Add to `.env` (development) and document in `README.md`:

```env
# Chapa Payment Gateway
# Get these from: https://dashboard.chapa.co → Settings → API
CHAPA_SECRET_KEY=CHASECK_test_xxxxxxxxxxxxxxxxxxxxxxxx
CHAPA_WEBHOOK_SECRET=your_random_secret_string_here
CHAPA_BASE_URL=https://api.chapa.co/v1

# Frontend base URL (used to build the return_url after payment)
FRONTEND_BASE_URL=http://localhost:3000
```

For test/sandbox mode: use a key starting with `CHASECK_test_`. For production: use a key starting with `CHASECK_`. Toggle via Chapa dashboard → bottom-left switch.

**Chapa Webhook Setup (manual step — done in Chapa dashboard):**
1. Log into https://dashboard.chapa.co
2. Go to Settings → Webhooks
3. Add webhook URL: `https://yourdomain.com/api/v1/bookings/{booking_id}/payments/callback`
4. Set the Secret Hash — this must match `CHAPA_WEBHOOK_SECRET` in your env

---

## 17. Step 15 — docker-compose Updates

**File:** `docker-compose.yml`

Add the new env vars to the `api` service environment section:

```yaml
services:
  api:
    environment:
      # --- existing vars ---
      # ... keep all existing ones ...

      # --- ADD: Chapa payment ---
      CHAPA_SECRET_KEY: ${CHAPA_SECRET_KEY:-CHASECK_test_placeholder}
      CHAPA_WEBHOOK_SECRET: ${CHAPA_WEBHOOK_SECRET:-dev_webhook_secret}
      CHAPA_BASE_URL: ${CHAPA_BASE_URL:-https://api.chapa.co/v1}
      FRONTEND_BASE_URL: ${FRONTEND_BASE_URL:-http://localhost:3000}
```

---

## 18. Step 16 — Tests

**File:** `tests/booking/booking_suite_test.go`

Uses the same pattern as `tests/identity/identity_suite_test.go`: real Postgres, full migration (both 001 and 002), HTTP test server, and direct API calls.

```go
package booking_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SetupTestServer spins up the full app with a real Postgres DB (same pattern as identity tests).
// Run both migrations before tests.

// -------------------------------------------------------
// Test cases — implement all of these
// -------------------------------------------------------

// TestCreateBooking_NoVerification
// Given: institute does not require verification
// When:  candidate POSTs /bookings
// Then:  booking created with status "verified"
func TestCreateBooking_NoVerification(t *testing.T) { /* ... */ }

// TestCreateBooking_WithVerification
// Given: institute requires verification
// When:  candidate POSTs /bookings
// Then:  booking created with status "pending_verification"
func TestCreateBooking_WithVerification(t *testing.T) { /* ... */ }

// TestVerifyBooking_Approve
// Given: booking in "pending_verification"
// When:  admin PATCHes /bookings/{id}/verify with action="approve"
// Then:  booking status = "verified", verified_at is set, verified_by = admin UUID
func TestVerifyBooking_Approve(t *testing.T) { /* ... */ }

// TestVerifyBooking_Reject
// Given: booking in "pending_verification"
// When:  admin PATCHes /bookings/{id}/verify with action="reject", rejection_reason="Documents missing"
// Then:  booking status = "rejected", rejection_reason stored
func TestVerifyBooking_Reject(t *testing.T) { /* ... */ }

// TestVerifyBooking_WrongStatus
// Given: booking already "verified"
// When:  admin tries to verify again
// Then:  400 Bad Request with ErrInvalidStatusForAction
func TestVerifyBooking_WrongStatus(t *testing.T) { /* ... */ }

// TestScheduleBooking
// Given: booking in "verified", slot exists with capacity 2
// When:  admin PATCHes /bookings/{id}/schedule with slot_id
// Then:  booking status = "scheduled", slot booked_count incremented to 1
func TestScheduleBooking(t *testing.T) { /* ... */ }

// TestScheduleBooking_SlotFull
// Given: slot capacity = 1, booked_count = 1
// When:  admin tries to schedule another booking into it
// Then:  400 Bad Request with ErrSlotFull
func TestScheduleBooking_SlotFull(t *testing.T) { /* ... */ }

// TestRescheduleBooking
// Given: booking in "scheduled" with slot A (booked_count=1)
// When:  admin reschedules to slot B
// Then:  slot A booked_count = 0, slot B booked_count = 1, booking slot_id = slot B
func TestRescheduleBooking(t *testing.T) { /* ... */ }

// TestInitiatePayment
// Given: booking in "scheduled"
// When:  candidate POSTs /bookings/{id}/payments
// Then:  payment record created (status="pending"), booking status = "payment_pending"
//        response contains checkout_url (may be mocked if no real Chapa key)
func TestInitiatePayment(t *testing.T) { /* ... */ }

// TestHandleChapaWebhook_Success
// Given: payment in "pending", valid Chapa signature on webhook
// When:  Chapa POSTs /bookings/{id}/payments/callback with status="success"
// Then:  service double-verifies (mock VerifyTransaction returns success)
//        payment status = "success", booking status = "confirmed"
func TestHandleChapaWebhook_Success(t *testing.T) { /* ... */ }

// TestHandleChapaWebhook_Failure
// Given: payment in "pending", valid signature
// When:  Chapa POSTs callback with status="failed"
// Then:  payment status = "failed", booking status = "payment_failed"
func TestHandleChapaWebhook_Failure(t *testing.T) { /* ... */ }

// TestHandleChapaWebhook_InvalidSignature
// Given: webhook with wrong signature
// When:  Chapa POSTs callback
// Then:  401 Unauthorized, payment record unchanged
func TestHandleChapaWebhook_InvalidSignature(t *testing.T) { /* ... */ }

// TestHandleChapaWebhook_Idempotent
// Given: payment already in "success"
// When:  same webhook arrives again (Chapa retry)
// Then:  200 OK, no duplicate updates, booking status unchanged
func TestHandleChapaWebhook_Idempotent(t *testing.T) { /* ... */ }

// TestRetryPayment
// Given: booking in "payment_failed", 1 previous attempt
// When:  candidate POSTs /bookings/{id}/payments/retry
// Then:  new payment record created with attempt_number=2, booking back to "payment_pending"
func TestRetryPayment(t *testing.T) { /* ... */ }

// TestRetryPayment_MaxAttempts
// Given: booking in "payment_failed", 3 previous attempts
// When:  candidate tries to retry again
// Then:  400 Bad Request with ErrMaxPaymentAttempts
func TestRetryPayment_MaxAttempts(t *testing.T) { /* ... */ }

// TestDeleteBooking_CandidateDraft
// Given: candidate's own booking in "drafted"
// When:  candidate DELETEs /bookings/{id}
// Then:  204 / 200, booking gone from DB
func TestDeleteBooking_CandidateDraft(t *testing.T) { /* ... */ }

// TestDeleteBooking_CandidateNotDraft
// Given: candidate's booking in "confirmed"
// When:  candidate DELETEs /bookings/{id}
// Then:  400 Bad Request with ErrInvalidStatusForAction
func TestDeleteBooking_CandidateNotDraft(t *testing.T) { /* ... */ }

// TestDeleteBooking_OtherCandidate
// Given: booking owned by candidate A
// When:  candidate B tries to delete it
// Then:  403 Forbidden
func TestDeleteBooking_OtherCandidate(t *testing.T) { /* ... */ }

// TestListBookings_CandidateScope
// Given: 3 bookings (2 owned by candidate A, 1 by candidate B)
// When:  candidate A GETs /bookings
// Then:  only 2 bookings returned
func TestListBookings_CandidateScope(t *testing.T) { /* ... */ }

// TestListBookings_InstituteScope
// Given: 3 bookings (2 for institute X, 1 for institute Y)
// When:  institute X GETs /bookings
// Then:  only 2 bookings returned
func TestListBookings_InstituteScope(t *testing.T) { /* ... */ }

// TestListBookings_AdminScope
// Given: 3 bookings across different candidates and institutes
// When:  admin GETs /bookings
// Then:  all 3 returned
func TestListBookings_AdminScope(t *testing.T) { /* ... */ }

// TestCreateSlot
// Given: admin authenticated
// When:  admin POSTs /slots with valid data
// Then:  slot created with booked_count=0, available=capacity
func TestCreateSlot(t *testing.T) { /* ... */ }

// TestCreateSlot_CandidateForbidden
// Given: candidate authenticated
// When:  candidate POSTs /slots
// Then:  403 Forbidden
func TestCreateSlot_CandidateForbidden(t *testing.T) { /* ... */ }

// TestArchiveBooking
// Given: booking in "confirmed"
// When:  ArchiveBooking() is called (internal, triggered by testing flow)
// Then:  booking status = "archived", archived_at is set
func TestArchiveBooking(t *testing.T) { /* ... */ }
```

---

## 19. Step 17 — go.mod Dependencies

No new external dependencies are required. The implementation uses only:

- `github.com/google/uuid` — already in go.mod
- `github.com/jackc/pgx/v5` — already in go.mod
- `github.com/go-chi/chi/v5` — already in go.mod (used by identity routes)
- Standard library: `crypto/hmac`, `crypto/sha256`, `encoding/hex`, `encoding/json`, `fmt`, `net/http`, `io`, `strings`, `time`

Run `go mod tidy` after adding the new package to ensure imports are clean.

---

## 20. Limitations & Future Work

Add the following as a comment block at the top of `internal/booking/chapa_provider.go` and as a section in `README.md`:

```
PAYMENT GATEWAY — CURRENT IMPLEMENTATION & KNOWN LIMITATIONS
=============================================================

Provider: Chapa (https://chapa.co) — Ethiopia's primary payment aggregator.

What Chapa handles for us (on their hosted checkout page):
  ✓ Telebirr (Ethio Telecom mobile wallet)
  ✓ CBE Birr (Commercial Bank of Ethiopia wallet)
  ✓ Amole (Dashen Bank wallet)
  ✓ Visa / Mastercard (international cards)
  ✓ Bank transfers (local Ethiopian banks)

This backend only calls TWO Chapa endpoints:
  1. POST /transaction/initialize → get checkout URL
  2. GET  /transaction/verify/{tx_ref} → confirm payment status

KNOWN LIMITATIONS (future direct integrations, out of scope for v1):
  - Direct Telebirr API (Ethio Telecom) — requires separate merchant agreement
  - Direct CBE API (Commercial Bank of Ethiopia) — not publicly available
  - Direct Awash Bank API
  - Direct Dashen Bank / Amole API
  - Direct Stripe integration (for international card payments outside Chapa)

To add a direct provider integration:
  1. Create a new file: internal/booking/{provider}_provider.go
  2. Implement the PaymentProvider interface
  3. Wire it in internal/app/app.go (replace or add alongside ChapaProvider)
  No changes to service.go or handler.go are needed.
```

---

## 21. Implementation Order for Codex

Execute these steps in strict sequence. Each step is a discrete unit of work. Do not start a step until the previous one compiles without errors.

| # | Action | File(s) |
|---|--------|---------|
| 1 | Write migration file | `migrations/002_booking_scheduling_payment.sql` |
| 2 | Update domain booking model | `internal/domain/booking.go` |
| 3 | Update config struct | `internal/platform/config/config.go` |
| 4 | Create empty booking package files | `internal/booking/*.go` |
| 5 | Write DTOs | `internal/booking/dto.go` |
| 6 | Write PaymentProvider interface | `internal/booking/payment_provider.go` |
| 7 | Write Chapa provider | `internal/booking/chapa_provider.go` |
| 8 | Write repository | `internal/booking/repository.go` |
| 9 | Write service | `internal/booking/service.go` |
| 10 | Write handlers | `internal/booking/handler.go` |
| 11 | Write routes | `internal/booking/routes.go` |
| 12 | Fix auth middleware (http.Error → httpx.Failure) | `internal/platform/security/security.go` |
| 13 | Add RequireEntityType middleware if missing | `internal/platform/security/security.go` |
| 14 | Add JWT claim fields (Email, FirstName, etc.) if missing | `internal/platform/security/security.go`, `internal/identity/service.go` |
| 15 | Wire booking module into app | `internal/app/app.go` |
| 16 | Add env vars to docker-compose | `docker-compose.yml` |
| 17 | Write tests | `tests/booking/booking_suite_test.go` |
| 18 | Run `go mod tidy` | — |
| 19 | Run `go build ./...` and confirm zero errors | — |
| 20 | Run `go test ./tests/identity/...` to confirm no regressions | — |
| 21 | Run `go test ./tests/booking/...` | — |

---

*End of implementation plan.*
