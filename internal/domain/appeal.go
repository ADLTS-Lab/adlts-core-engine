package domain

import (
	"time"

	"github.com/google/uuid"
)

type Appeal struct {
	ID          uuid.UUID    `db:"id"`
	SessionID   uuid.UUID    `db:"session_id"`
	TestID      uuid.UUID    `db:"test_id"`
	CandidateID uuid.UUID    `db:"candidate_id"` // stored — hidden from expert view in DTO
	ExpertID    *uuid.UUID   `db:"expert_id"`    // system-assigned; nil until assigned
	Reason      string       `db:"reason"`
	Status      AppealStatus `db:"status"`
	Resolution  string       `db:"resolution"` // expert's written verdict
	Audit
}

type ExpertReview struct {
	AppealID   uuid.UUID `db:"appeal_id"`
	ExpertID   uuid.UUID `db:"expert_id"`
	Decision   string    `db:"decision"` // "accepted" | "rejected"
	Notes      string    `db:"notes"`
	ReviewedAt time.Time `db:"reviewed_at"`
}
