package domain

import (
	"time"

	"github.com/google/uuid"
)

type InstituteVerification struct {
	ID          uuid.UUID `db:"id"`
	CandidateID uuid.UUID `db:"candidate_id"` // FK → candidates
	InstituteID uuid.UUID `db:"institute_id"` // FK → institutes
	IssuedAt    time.Time `db:"issued_at"`
	ExpiresAt   time.Time `db:"expires_at"` // not permanent
	Audit
}
// -------------------------------------------------------
// Booking status lifecycle
// drafted -> pending_verification -> verified -> scheduled
//        -> payment_pending -> payment_failed -> confirmed
//        -> archived | cancelled | rejected
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
	TestLevelCode        string        `db:"test_level_code"`
	ScheduledAt          *time.Time    `db:"scheduled_at"`
	PaymentRef           *string       `db:"payment_ref"`
	PaymentStatus        string        `db:"payment_status"`
	PaymentAmountCents   *int          `db:"payment_amount_cents"`
	PaymentAttempts      int           `db:"payment_attempts"`
	ArchivedAt           *time.Time    `db:"archived_at"`
	CreatedAt            time.Time     `db:"created_at"`
	UpdatedAt            time.Time     `db:"updated_at"`
}

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