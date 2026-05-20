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

type Session struct {
	ID               uuid.UUID       `db:"id"`
	BookingID        uuid.UUID       `db:"booking_id"`
	CandidateID      uuid.UUID       `db:"candidate_id"`
	TestCenterID     uuid.UUID       `db:"test_center_id"` // non-nullable — sessions belong to a center
	DeviceID         *uuid.UUID      `db:"device_id"`
	Status           SessionStatus   `db:"status"`
	Score            float64         `db:"score"`
	RecordingURL     string          `db:"recording_url"`
	ResultOverlayURL string          `db:"result_overlay_url"`
	StartedAt        *time.Time      `db:"started_at"`
	CompletedAt      *time.Time      `db:"completed_at"`
	FinalizedAt      *time.Time      `db:"finalized_at"`
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
	Severity   string    `db:"severity"` // "minor", "major", "critical"
	FrameIndex int       `db:"frame_index"`
	CreatedAt  time.Time `db:"created_at"`
}
