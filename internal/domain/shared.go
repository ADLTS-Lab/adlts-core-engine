package domain

import (
	"time"

	"github.com/google/uuid"
)

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
