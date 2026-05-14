package domain

import (
	"time"

	"github.com/google/uuid"
)

type Invitation struct {
	ID           uuid.UUID  `db:"id"`
	Token        string     `db:"token"`
	Email        string     `db:"email"`
	EntityType   string     `db:"entity_type"`
	TestCenterID *uuid.UUID `db:"test_center_id"`
	ExpiresAt    time.Time  `db:"expires_at"`
	UsedAt       *time.Time `db:"used_at"`
	CreatedBy    uuid.UUID  `db:"created_by"`
	CreatedAt    time.Time  `db:"created_at"`
}
