package booking

import (
	"errors"
	"time"
)

// -------------------------------------------------------
// Request DTOs
// -------------------------------------------------------

// CreateBookingRequest is sent by the candidate.
// candidate_id is injected from the JWT and never accepted from the body.
type CreateBookingRequest struct {
	InstituteID string `json:"institute_id"`
}

func (r *CreateBookingRequest) Validate() error {
	if r.InstituteID == "" {
		return ErrMissingInstituteID
	}
	return nil
}

// VerifyBookingRequest is sent by an admin or the institute.
type VerifyBookingRequest struct {
	Action          string `json:"action"`
	RejectionReason string `json:"rejection_reason"`
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
	SlotID string `json:"slot_id"`
}

func (r *ScheduleBookingRequest) Validate() error {
	if r.SlotID == "" {
		return ErrMissingSlotID
	}
	return nil
}

// RescheduleBookingRequest reassigns to a new slot.
type RescheduleBookingRequest struct {
	SlotID string `json:"slot_id"`
}

func (r *RescheduleBookingRequest) Validate() error {
	if r.SlotID == "" {
		return ErrMissingSlotID
	}
	return nil
}

// InitiatePaymentRequest is sent by the candidate to start a payment.
type InitiatePaymentRequest struct {
	AmountCents int    `json:"amount_cents"`
	Currency    string `json:"currency"`
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
	InstituteID string    `json:"institute_id"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	Capacity    int       `json:"capacity"`
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
	Available   int    `json:"available"`
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

// InitiatePaymentResponse includes the checkout URL.
type InitiatePaymentResponse struct {
	PaymentID   string `json:"payment_id"`
	CheckoutURL string `json:"checkout_url"`
	TxRef       string `json:"tx_ref"`
}

// -------------------------------------------------------
// Sentinel errors
// -------------------------------------------------------

var (
	ErrMissingInstituteID     = errors.New("institute_id is required")
	ErrMissingSlotID          = errors.New("slot_id is required")
	ErrMissingSlotTimes       = errors.New("starts_at and ends_at are required")
	ErrInvalidSlotTimes       = errors.New("ends_at must be after starts_at")
	ErrInvalidVerifyAction    = errors.New("action must be 'approve' or 'reject'")
	ErrMissingRejectionReason = errors.New("rejection_reason is required when rejecting")
	ErrInvalidAmount          = errors.New("amount_cents must be greater than zero")
	ErrInvalidCurrency        = errors.New("currency must be ETB")

	ErrBookingNotFound             = errors.New("booking not found")
	ErrSlotNotFound                = errors.New("slot not found")
	ErrPaymentNotFound             = errors.New("payment not found")
	ErrInstituteNotFound           = errors.New("institute not found")
	ErrCandidateNotFound           = errors.New("candidate not found")
	ErrSlotFull                    = errors.New("slot is fully booked")
	ErrForbidden                   = errors.New("forbidden")
	ErrInvalidStatusForAction      = errors.New("booking is not in the required status for this action")
	ErrMaxPaymentAttempts          = errors.New("maximum payment attempts reached")
	ErrAlreadyProcessed            = errors.New("payment already processed")
	ErrPaymentProvider             = errors.New("payment provider error")
	ErrPaymentVerificationMismatch = errors.New("payment verification data does not match the local payment")
)
