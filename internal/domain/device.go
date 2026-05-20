package domain

import (
	"time"

	"github.com/google/uuid"
)

type Device struct {
	ID            uuid.UUID    `db:"id"`
	MACAddress    string       `db:"mac_address"` // unique hardware identifier
	Name          string       `db:"name"`
	Secret        string       `db:"secret"`      // device auth token, never exposed in API
	TestCenterID  uuid.UUID    `db:"test_center_id"` // FK → test_centers
	Status        DeviceStatus `db:"status"`
	LastHeartbeat *time.Time   `db:"last_heartbeat"`
	Audit
}
