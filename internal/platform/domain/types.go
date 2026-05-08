package domain

import "time"

type Role string

const (
	RoleAuthority      Role = "authority"
	RoleAdmin          Role = "admin"
	RoleInstituteAdmin Role = "institute_admin"
	RoleExaminer       Role = "examiner"
	RoleCandidate      Role = "candidate"
	RoleInternal       Role = "internal"
)

type AccountStatus string

const (
	AccountActive    AccountStatus = "active"
	AccountSuspended AccountStatus = "suspended"
	AccountPending   AccountStatus = "pending"
)

type BookingStatus string

const (
	BookingPendingVerification BookingStatus = "pending_verification"
	BookingVerified            BookingStatus = "verified"
	BookingRejected            BookingStatus = "rejected"
	BookingScheduled           BookingStatus = "scheduled"
)

type ExamStatus string

const (
	ExamScheduled      ExamStatus = "scheduled"
	ExamInitiating     ExamStatus = "initiating"
	ExamActive         ExamStatus = "active"
	ExamCompleted      ExamStatus = "completed"
	ExamReviewRequired ExamStatus = "review_required"
	ExamFinalized      ExamStatus = "finalized"
	ExamStopped        ExamStatus = "stopped"
)

type AppealStatus string

const (
	AppealPending  AppealStatus = "pending"
	AppealAccepted AppealStatus = "accepted"
	AppealRejected AppealStatus = "rejected"
)

type User struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Email         string        `json:"email"`
	PasswordHash  string        `json:"-"`
	Role          Role          `json:"role"`
	Status        AccountStatus `json:"status"`
	InstituteID   string        `json:"institute_id,omitempty"`
	OTPCode       string        `json:"-"`
	OTPVerifiedAt *time.Time    `json:"otp_verified_at,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

type Institute struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
}

type Invitation struct {
	Token     string    `json:"token"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type Booking struct {
	ID                  string        `json:"id"`
	CandidateID         string        `json:"candidate_id"`
	InstituteID         string        `json:"institute_id"`
	RequestedSlotID     string        `json:"requested_slot_id,omitempty"`
	ScheduledSlotID     string        `json:"scheduled_slot_id,omitempty"`
	Status              BookingStatus `json:"status"`
	TrainingHours       int           `json:"training_hours"`
	TrainingEvidenceURL string        `json:"training_evidence_url,omitempty"`
	VerifiedBy          string        `json:"verified_by,omitempty"`
	VerifiedAt          *time.Time    `json:"verified_at,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

type Slot struct {
	ID          string    `json:"id"`
	InstituteID string    `json:"institute_id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Capacity    int       `json:"capacity"`
	BookedCount int       `json:"booked_count"`
	Location    string    `json:"location"`
}

type Device struct {
	ID            string     `json:"id"`
	MACAddress    string     `json:"mac_address"`
	Name          string     `json:"name"`
	Secret        string     `json:"-"`
	Status        string     `json:"status"`
	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Violation struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"`
	FrameID   string    `json:"frame_id,omitempty"`
	Track     string    `json:"track,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type FrameAnalysis struct {
	ID              string      `json:"id"`
	ExamID          string      `json:"exam_id,omitempty"`
	DeviceID        string      `json:"device_id"`
	Frame           string      `json:"frame,omitempty"`
	DetectedObjects []string    `json:"detected_objects"`
	Violations      []Violation `json:"violations"`
	ScoreDelta      float64     `json:"score_delta"`
	Speed           float64     `json:"speed"`
	CreatedAt       time.Time   `json:"created_at"`
}

type ExamTelemetry struct {
	Health         string    `json:"health"`
	CurrentScore   float64   `json:"current_score"`
	LastFrameID    string    `json:"last_frame_id,omitempty"`
	ViolationCount int       `json:"violation_count"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Exam struct {
	ID               string        `json:"id"`
	BookingID        string        `json:"booking_id"`
	CandidateID      string        `json:"candidate_id"`
	DeviceID         string        `json:"device_id,omitempty"`
	ExaminerID       string        `json:"examiner_id,omitempty"`
	Status           ExamStatus    `json:"status"`
	Score            float64       `json:"score"`
	Violations       []Violation   `json:"violations"`
	Telemetry        ExamTelemetry `json:"telemetry"`
	ResultOverlayURL string        `json:"result_overlay_url,omitempty"`
	StartedAt        *time.Time    `json:"started_at,omitempty"`
	CompletedAt      *time.Time    `json:"completed_at,omitempty"`
	FinalizedAt      *time.Time    `json:"finalized_at,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

type Appeal struct {
	ID          string       `json:"id"`
	ExamID      string       `json:"exam_id"`
	CandidateID string       `json:"candidate_id"`
	Reason      string       `json:"reason"`
	Status      AppealStatus `json:"status"`
	Resolution  string       `json:"resolution,omitempty"`
	ResolvedBy  string       `json:"resolved_by,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}
