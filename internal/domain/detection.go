package domain

import (
	"time"

	"github.com/google/uuid"
)

type DetectionEvent struct {
	ID         uuid.UUID        `db:"id"`
	SessionID  uuid.UUID        `db:"session_id"`
	DeviceID   uuid.UUID        `db:"device_id"`
	FrameIndex int              `db:"frame_index"`
	Objects    []DetectedObject `db:"-"` // stored as JSONB
	Violations []Violation      `db:"-"` // stored as JSONB or child rows
	ScoreDelta float64          `db:"score_delta"`
	Speed      float64          `db:"speed"`
	CreatedAt  time.Time        `db:"created_at"`
}

type DetectedObject struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	BoundingBox *BBox  `json:"bounding_box,omitempty"`
}

type BBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}
