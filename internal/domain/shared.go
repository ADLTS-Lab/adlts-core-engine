package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JSONB maps to PostgreSQL's JSONB column type.
// pgx v5 can scan JSONB natively into json.RawMessage ([]byte).
// A nil value represents a NULL column.
type JSONB = json.RawMessage

type Audit struct {
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	CreatedBy uuid.UUID `db:"created_by"`
	UpdatedBy uuid.UUID `db:"updated_by"`
}

type Address struct {
	Street  string `db:"street"`
	City    string `db:"city"`
	Region  string `db:"region"`
	Country string `db:"country"`
}
