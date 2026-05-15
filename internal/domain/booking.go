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

type Booking struct {
	ID                       uuid.UUID     `db:"id"`
	CandidateID              uuid.UUID     `db:"candidate_id"`  // FK → candidates
	TestCenterID             uuid.UUID     `db:"test_center_id"` // FK → test_centers
	InstituteVerificationID  uuid.UUID     `db:"institute_verification_id"`
	SlotID                   *uuid.UUID    `db:"slot_id"`     // assigned after scheduling
	Status                   BookingStatus `db:"status"`
	TrainingHours            int           `db:"training_hours"`
	TrainingEvidenceURL      string        `db:"training_evidence_url"`
	VerifiedBy               *uuid.UUID    `db:"verified_by"` // Institute user who approved
	VerifiedAt               *time.Time    `db:"verified_at"`
	Audit
}

type Slot struct {
	ID           uuid.UUID `db:"id"`
	TestCenterID uuid.UUID `db:"test_center_id"` // FK → test_centers
	StartTime    time.Time `db:"start_time"`
	EndTime      time.Time `db:"end_time"`
	Capacity     int       `db:"capacity"`
	BookedCount  int       `db:"booked_count"`
	Audit
}
