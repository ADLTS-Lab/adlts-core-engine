package testing

import (
	"time"

	"github.com/google/uuid"
)

// ── Device DTOs ───────────────────────────────────────────────────────────────

type RegisterDeviceRequest struct {
	DeviceCode    string   `json:"device_code"`
	Password      string   `json:"password"`
	TestCenterID  string   `json:"test_center_id"`
	AllowedLevels []string `json:"allowed_levels"`
	StreamURL     string   `json:"stream_url"`
}

type UpdateDeviceRequest struct {
	StreamURL     *string  `json:"stream_url,omitempty"`
	AllowedLevels []string `json:"allowed_levels,omitempty"`
}

type UpdateDeviceStatusRequest struct {
	Status string `json:"status"`
}

type DeviceResponse struct {
	ID            uuid.UUID  `json:"id"`
	DeviceCode    string     `json:"device_code"`
	TestCenterID  uuid.UUID  `json:"test_center_id"`
	AllowedLevels []string   `json:"allowed_levels"`
	StreamURL     string     `json:"stream_url"`
	Status        string     `json:"status"`
	CurrentTestID *uuid.UUID `json:"current_test_id,omitempty"`
	LastSeenAt    *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ── TestLevelType DTOs ────────────────────────────────────────────────────────

type TestLevelTypeResponse struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

// ── TestPlan DTOs ─────────────────────────────────────────────────────────────

type CreateTestPlanRequest struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	PassThreshold float64 `json:"pass_threshold"`
}

type UpdateTestPlanRequest struct {
	Name          *string  `json:"name,omitempty"`
	Description   *string  `json:"description,omitempty"`
	PassThreshold *float64 `json:"pass_threshold,omitempty"`
}

type TestPlanResponse struct {
	ID            uuid.UUID          `json:"id"`
	TestCenterID  uuid.UUID          `json:"test_center_id"`
	Name          string             `json:"name"`
	Description   string             `json:"description"`
	PassThreshold float64            `json:"pass_threshold"`
	Status        string             `json:"status"`
	PublishedAt   *time.Time         `json:"published_at,omitempty"`
	Maneuvers     []ManeuverResponse `json:"maneuvers,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
}

// ── Maneuver DTOs (legacy) ────────────────────────────────────────────────────

type CreateManeuverRequest struct {
	Name              string  `json:"name"`
	Description       string  `json:"description"`
	SequenceNumber    int     `json:"sequence_number"`
	QRCodeValue       string  `json:"qr_code_value"`
	TolerancePx       int     `json:"tolerance_px"`
	Weight            float64 `json:"weight"`
	MinFramesRequired int     `json:"min_frames_required"`
}

type UpdateManeuverRequest struct {
	Name              *string  `json:"name,omitempty"`
	Description       *string  `json:"description,omitempty"`
	TolerancePx       *int     `json:"tolerance_px,omitempty"`
	Weight            *float64 `json:"weight,omitempty"`
	MinFramesRequired *int     `json:"min_frames_required,omitempty"`
}

type ManeuverResponse struct {
	ID                uuid.UUID `json:"id"`
	TestPlanID        uuid.UUID `json:"test_plan_id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	SequenceNumber    int       `json:"sequence_number"`
	QRCodeValue       string    `json:"qr_code_value"`
	ReferenceMaskURL  string    `json:"reference_mask_url"`
	TolerancePx       int       `json:"tolerance_px"`
	Weight            float64   `json:"weight"`
	MinFramesRequired int       `json:"min_frames_required"`
	CreatedAt         time.Time `json:"created_at"`
}

// ── Maneuver Config DTOs (migration-003 schema) ───────────────────────────────

type CreateManeuverConfigRequest struct {
	ManeuverType      string  `json:"maneuver_type"`
	DisplayName       string  `json:"display_name"`
	Weight            float64 `json:"weight"`
	PassThreshold     float64 `json:"pass_threshold"`
	TolerancePx       int     `json:"tolerance_px"`
	MinFramesRequired int     `json:"min_frames_required"`
	SequenceNumber    int     `json:"sequence_number"`
}

type UpdateManeuverConfigRequest struct {
	DisplayName       *string  `json:"display_name,omitempty"`
	Weight            *float64 `json:"weight,omitempty"`
	PassThreshold     *float64 `json:"pass_threshold,omitempty"`
	TolerancePx       *int     `json:"tolerance_px,omitempty"`
	MinFramesRequired *int     `json:"min_frames_required,omitempty"`
}

type ReorderManeuverConfigRequest struct {
	OrderedIDs []uuid.UUID `json:"ordered_ids"`
}

type ManeuverConfigResponse struct {
	ID                uuid.UUID `json:"id"`
	TestPlanID        uuid.UUID `json:"test_plan_id"`
	ManeuverType      string    `json:"maneuver_type"`
	DisplayName       string    `json:"display_name"`
	SequenceNumber    int       `json:"sequence_number"`
	Weight            float64   `json:"weight"`
	PassThreshold     float64   `json:"pass_threshold"`
	TolerancePx       int       `json:"tolerance_px"`
	MinFramesRequired int       `json:"min_frames_required"`
	QRStartValue      string    `json:"qr_start_value"`
	QREndValue        string    `json:"qr_end_value"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ── Admin Test CRUD DTOs ──────────────────────────────────────────────────────

type CreateTestAdminRequest struct {
	CandidateID   uuid.UUID `json:"candidate_id"`
	TestCenterID  uuid.UUID `json:"test_center_id"`
	TestLevelCode string    `json:"test_level_code"`
	BookingID     uuid.UUID `json:"booking_id"`
}

type UpdateTestAdminRequest struct {
	AppealWindowClosesAt       *time.Time `json:"appeal_window_closes_at,omitempty"`
	ResultVisibleToCandidateAt *time.Time `json:"result_visible_to_candidate_at,omitempty"`
	ResultVisibleToInstituteAt *time.Time `json:"result_visible_to_institute_at,omitempty"`
}

type RecordingURLResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"`
}

// ── TestLevelMapping DTOs ─────────────────────────────────────────────────────

type UpsertLevelMappingRequest struct {
	TestLevelCode string    `json:"test_level_code"`
	TestPlanID    uuid.UUID `json:"test_plan_id"`
}

type LevelMappingResponse struct {
	ID            uuid.UUID `json:"id"`
	TestCenterID  uuid.UUID `json:"test_center_id"`
	TestLevelCode string    `json:"test_level_code"`
	TestPlanID    uuid.UUID `json:"test_plan_id"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ── Internal test creation ────────────────────────────────────────────────────

type CreateTestInternalRequest struct {
	BookingID     uuid.UUID `json:"booking_id"`
	CandidateID   uuid.UUID `json:"candidate_id"`
	TestCenterID  uuid.UUID `json:"test_center_id"`
	TestLevelCode string    `json:"test_level_code"`
}

// ── Device checkin ────────────────────────────────────────────────────────────

type DeviceCheckinRequest struct {
	DeviceCode   string    `json:"device_code"`
	Password     string    `json:"password"`
	TestCenterID uuid.UUID `json:"test_center_id"`
}

// ── Test Start DTOs ───────────────────────────────────────────────────────────

type StartTestRequest struct {
	TestCenterID uuid.UUID `json:"test_center_id"`
}

type StartTestResponse struct {
	Ok      bool   `json:"ok"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ── Result Webhook DTOs ───────────────────────────────────────────────────────

type ResultWebhookRequest struct {
	TestID          string            `json:"test_id"`
	CandidateID     string            `json:"candidate_id"`
	TotalScore      float64           `json:"total_score"`
	Passed          bool              `json:"passed"`
	PassThreshold   float64           `json:"pass_threshold"`
	RecordingPrefix string            `json:"recording_prefix"`
	Maneuvers       []ManeuverWebhook `json:"maneuvers"`
}

type ManeuverWebhook struct {
	Name       string  `json:"name"`
	RawScore   float64 `json:"raw_score"`
	Penalty    float64 `json:"penalty"`
	FinalScore float64 `json:"final_score"`
	FrameCount int     `json:"frame_count"`
	Violations int     `json:"violations"`
}

// ── Test Result DTOs ──────────────────────────────────────────────────────────

type SessionResultResponse struct {
	ID              uuid.UUID `json:"id"`
	TestID          uuid.UUID `json:"test_id"`
	SessionID       uuid.UUID `json:"session_id"`
	ManeuverID      uuid.UUID `json:"maneuver_id"`
	SequenceNumber  int       `json:"sequence_number"`
	Score           float64   `json:"score"`
	Weight          float64   `json:"weight"`
	Passed          bool      `json:"passed"`
	FrameCount      int       `json:"frame_count"`
	LaneDetectedPct float64   `json:"lane_detected_pct"`
	AvgIoU          float64   `json:"avg_iou"`
}

type TestResultResponse struct {
	ID                  uuid.UUID `json:"id"`
	TestID              uuid.UUID `json:"test_id"`
	WeightedTotalScore  float64   `json:"weighted_total_score"`
	Passed              bool      `json:"passed"`
	PassThreshold       float64   `json:"pass_threshold"`
	OverallNarrative    string    `json:"overall_narrative"`
	StrengthsNarrative  string    `json:"strengths_narrative"`
	WeaknessesNarrative string    `json:"weaknesses_narrative"`
	RecommendedFocus    string    `json:"recommended_focus"`
	NarrativeModel      string    `json:"narrative_model"`
}

type TestResultDetailResponse struct {
	Test           TestResponse            `json:"test"`
	Result         *TestResultResponse     `json:"result"`
	SessionResults []SessionResultResponse `json:"session_results"`
}

type FrameAnalysisResponse struct {
	FrameSeqNo   int64     `json:"frame_seq_no"`
	CapturedAt   time.Time `json:"captured_at"`
	LaneDetected bool      `json:"lane_detected"`
	CurvatureDir string    `json:"curvature_dir"`
	IoUScore     float64   `json:"iou_score"`
	FrameScore   float64   `json:"frame_score"`
	IsMocked     bool      `json:"is_mocked"`
}

// ── Test response ─────────────────────────────────────────────────────────────

type TestResponse struct {
	ID                 uuid.UUID  `json:"id"`
	BookingID          uuid.UUID  `json:"booking_id"`
	CandidateID        uuid.UUID  `json:"candidate_id"`
	TestCenterID       uuid.UUID  `json:"test_center_id"`
	TestPlanID         uuid.UUID  `json:"test_plan_id"`
	DeviceID           *uuid.UUID `json:"device_id,omitempty"`
	TestLevelCode      string     `json:"test_level_code"`
	Status             string     `json:"status"`
	AbortReason        *string    `json:"abort_reason,omitempty"`
	ScheduledStartAt   *time.Time `json:"scheduled_start_at,omitempty"`
	ScheduledEndAt     *time.Time `json:"scheduled_end_at,omitempty"`
	BookingWindowHours *int       `json:"booking_window_hours,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

// ── Monitor DTOs ──────────────────────────────────────────────────────────────

type MonitorStatusResponse struct {
	TestID      string     `json:"test_id"`
	Status      string     `json:"status"`
	DeviceID    *string    `json:"device_id,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	AbortReason *string    `json:"abort_reason,omitempty"`
}

type MonitorLiveResponse struct {
	TestID         string  `json:"test_id"`
	Status         string  `json:"status"`
	CurrentSession *int    `json:"current_session,omitempty"`
	FrameCount     int64   `json:"frame_count"`
	RunningAvgIoU  float64 `json:"running_avg_iou"`
	DeviceHealthOK bool    `json:"device_health_ok"`
}