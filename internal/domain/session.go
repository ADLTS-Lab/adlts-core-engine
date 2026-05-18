package domain

import (
	"time"

	"github.com/google/uuid"
)

// SessionStatus is kept for the appeals module which references the legacy sessions table.
// New testing core code uses domain.TestStatus instead.
type SessionStatus string

const (
	SessionScheduled      SessionStatus = "scheduled"
	SessionInitiating     SessionStatus = "initiating"
	SessionActive         SessionStatus = "active"
	SessionCompleted      SessionStatus = "completed"
	SessionReviewRequired SessionStatus = "review_required"
	SessionFinalized      SessionStatus = "finalized"
	SessionAborted        SessionStatus = "aborted"
)

// the old monolith. They reference the old `sessions` table which has been
// superseded by the Testing Core module (tests, test_sessions, frame_analyses).
//
// The appeals team may reference Session for backward compat with the existing
// `appeals` table. New testing core code uses domain.Test and domain.TestSession.

type Session struct {
	ID               uuid.UUID     `db:"id"`
	BookingID        uuid.UUID     `db:"booking_id"`
	CandidateID      uuid.UUID     `db:"candidate_id"`
	TestCenterID     uuid.UUID     `db:"test_center_id"`
	DeviceID         *uuid.UUID    `db:"device_id"`
	Status           SessionStatus `db:"status"`
	Score            float64       `db:"score"`
	RecordingURL     string        `db:"recording_url"`
	ResultOverlayURL string        `db:"result_overlay_url"`
	StartedAt        *time.Time    `db:"started_at"`
	CompletedAt      *time.Time    `db:"completed_at"`
	FinalizedAt      *time.Time    `db:"finalized_at"`
	Audit
}

type SessionTelemetry struct {
	SessionID      uuid.UUID `db:"session_id"`
	CurrentScore   float64   `db:"current_score"`
	ViolationCount int       `db:"violation_count"`
	LastFrameIndex int       `db:"last_frame_index"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type Violation struct {
	ID         uuid.UUID `db:"id"`
	SessionID  uuid.UUID `db:"session_id"`
	Code       string    `db:"code"`
	Message    string    `db:"message"`
	Severity   string    `db:"severity"`
	FrameIndex int       `db:"frame_index"`
	CreatedAt  time.Time `db:"created_at"`
}
