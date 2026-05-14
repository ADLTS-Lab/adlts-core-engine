package domain

import (
	"time"

	"github.com/google/uuid"
)

type Recording struct {
	ID        uuid.UUID  `db:"id"`
	SessionID uuid.UUID  `db:"session_id"`
	URL       string     `db:"url"`
	SizeBytes int64      `db:"size_bytes"`
	CreatedAt time.Time  `db:"created_at"`
	ExpiresAt *time.Time `db:"expires_at"`
}
