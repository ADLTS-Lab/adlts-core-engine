package domain

import (
	"time"

	"github.com/google/uuid"
)

// ── ManeuverType — static enum (not admin-configurable) ─────────────────────

// ManeuverType identifies the kind of driving maneuver being evaluated.
// Values are seeded into the maneuver_types DB table and mirrored here in Go.
type ManeuverType string

const (
	ManeuverTypeStraightLine      ManeuverType = "straight_line"
	ManeuverTypeFigure8           ManeuverType = "figure_8"
	ManeuverTypeLeftCurve         ManeuverType = "left_curve"
	ManeuverTypeRightCurve        ManeuverType = "right_curve"
	ManeuverTypeLeftCurveReverse  ManeuverType = "left_curve_reverse"
	ManeuverTypeRightCurveReverse ManeuverType = "right_curve_reverse"
	ManeuverTypeParking           ManeuverType = "parking"
	ManeuverTypeReverseParking    ManeuverType = "reverse_parking"
)

// ManeuverTypeInfo carries the human-readable metadata for a ManeuverType.
type ManeuverTypeInfo struct {
	Code            ManeuverType `json:"code"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	RequiresReverse bool         `json:"requires_reverse"`
	SortOrder       int          `json:"sort_order"`
}

// AllManeuverTypes is the authoritative Go-side list served by GET /maneuver-types.
// It mirrors the maneuver_types seed rows in migration 003.
var AllManeuverTypes = []ManeuverTypeInfo{
	{
		Code: ManeuverTypeStraightLine, Name: "Straight Line", SortOrder: 1,
		Description:     "Drive in a straight line maintaining lane center",
		RequiresReverse: false,
	},
	{
		Code: ManeuverTypeFigure8, Name: "Figure 8", SortOrder: 2,
		Description:     "Complete two arcs forming a figure-8 pattern",
		RequiresReverse: false,
	},
	{
		Code: ManeuverTypeLeftCurve, Name: "Left Curve", SortOrder: 3,
		Description:     "Navigate a left-bending curved path",
		RequiresReverse: false,
	},
	{
		Code: ManeuverTypeRightCurve, Name: "Right Curve", SortOrder: 4,
		Description:     "Navigate a right-bending curved path",
		RequiresReverse: false,
	},
	{
		Code: ManeuverTypeLeftCurveReverse, Name: "Left Curve (Reverse)", SortOrder: 5,
		Description:     "Reverse through a left-bending curved path",
		RequiresReverse: true,
	},
	{
		Code: ManeuverTypeRightCurveReverse, Name: "Right Curve (Reverse)", SortOrder: 6,
		Description:     "Reverse through a right-bending curved path",
		RequiresReverse: true,
	},
	{
		Code: ManeuverTypeParking, Name: "Parking", SortOrder: 7,
		Description:     "Park the vehicle within a marked bay",
		RequiresReverse: false,
	},
	{
		Code: ManeuverTypeReverseParking, Name: "Reverse Parking", SortOrder: 8,
		Description:     "Reverse the vehicle into a marked parking bay",
		RequiresReverse: true,
	},
}

// ── Enums ─────────────────────────────────────────────────────────────────────

type DeviceStatus string

const (
	DeviceStatusActive      DeviceStatus = "active"
	DeviceStatusInactive    DeviceStatus = "inactive"
	DeviceStatusInUse       DeviceStatus = "in_use"
	DeviceStatusMaintenance DeviceStatus = "maintenance"
)

type TestPlanStatus string

const (
	TestPlanDraft   TestPlanStatus = "draft"
	TestPlanActive  TestPlanStatus = "active"
	TestPlanRetired TestPlanStatus = "retired"
)

type TestStatus string

const (
	TestStatusPending      TestStatus = "pending"
	TestStatusReady        TestStatus = "ready"
	TestStatusGuidelines   TestStatus = "guidelines"
	TestStatusAcknowledged TestStatus = "acknowledged"
	TestStatusIoTHealth    TestStatus = "iot_health"
	TestStatusRunning      TestStatus = "running"
	TestStatusFinishing    TestStatus = "finishing"
	TestStatusCompleted    TestStatus = "completed"
	TestStatusAborted      TestStatus = "aborted"
	TestStatusExpired      TestStatus = "expired"
)

type AbortReason string

const (
	AbortHealthCheckFailed AbortReason = "health_check_failed"
	AbortStreamLost        AbortReason = "stream_lost"
	AbortAdminIntervention AbortReason = "admin_intervention"
	AbortCandidateNoShow   AbortReason = "candidate_no_show"
	AbortDeviceFailure     AbortReason = "device_failure"
	AbortSystemError       AbortReason = "system_error"
)

type HealthStatus string

const (
	HealthOk       HealthStatus = "ok"
	HealthDegraded HealthStatus = "degraded"
	HealthFailed   HealthStatus = "failed"
)

type VerdictType string

const (
	VerdictPass    VerdictType = "pass"
	VerdictFail    VerdictType = "fail"
	VerdictPending VerdictType = "pending"
)

type CurvatureDir string

const (
	CurvatureStraight CurvatureDir = "straight"
	CurvatureLeft     CurvatureDir = "left"
	CurvatureRight    CurvatureDir = "right"
	CurvatureNone     CurvatureDir = "none"
)

// ── Domain Structs ────────────────────────────────────────────────────────────

// TestLevelType is a seeded reference for driving license classes.
type TestLevelType struct {
	Code        string `db:"code"`
	Name        string `db:"name"`
	Description string `db:"description"`
	SortOrder   int    `db:"sort_order"`
}

// Device represents an in-car testing unit (ESP32-CAM + sensors).
type Device struct {
	ID            uuid.UUID    `db:"id"`
	DeviceCode    string       `db:"device_code"`
	PasswordHash  string       `db:"password_hash"`
	TestCenterID  uuid.UUID    `db:"test_center_id"`
	AllowedLevels string       `db:"allowed_levels"` // JSON array of level codes
	StreamURL     string       `db:"stream_url"`
	Status        DeviceStatus `db:"status"`
	CurrentTestID *uuid.UUID   `db:"current_test_id"`
	LastSeenAt    *time.Time   `db:"last_seen_at"`
	Audit
}

// TestPlan defines a set of maneuvers for a specific test center.
type TestPlan struct {
	ID            uuid.UUID        `db:"id"`
	TestCenterID  uuid.UUID        `db:"test_center_id"`
	Name          string           `db:"name"`
	Description   string           `db:"description"`
	PassThreshold float64          `db:"pass_threshold"`
	Status        TestPlanStatus   `db:"status"`
	PublishedAt   *time.Time       `db:"published_at"`
	Maneuvers     []ManeuverConfig `db:"-"` // loaded separately when needed
	Audit
}

// ManeuverConfig is a maneuver slot in a test plan, referencing a static ManeuverType.
// Legacy fields (Name, Description, QRCodeValue, ReferenceMaskURL) are kept for
// backward compatibility — DO NOT remove them.
type ManeuverConfig struct {
	ID             uuid.UUID `db:"id"`
	TestPlanID     uuid.UUID `db:"test_plan_id"`
	SequenceNumber int       `db:"sequence_number"`
	TolerancePx    int       `db:"tolerance_px"`
	Weight         float64   `db:"weight"`

	// Legacy fields — present in migration 002, kept for backward compat
	Name             string `db:"name"`
	Description      string `db:"description"`
	QRCodeValue      string `db:"qr_code_value"`
	ReferenceMaskURL string `db:"reference_mask_url"`

	// New fields — added in migration 003
	ManeuverType      ManeuverType `db:"maneuver_type"`       // FK to maneuver_types
	DisplayName       string       `db:"display_name"`        // optional UI label override
	PassThreshold     float64      `db:"pass_threshold"`      // 0–100, default 70
	MinFramesRequired int          `db:"min_frames_required"` // minimum valid frames
	QRStartValue      string       `db:"qr_start_value"`      // auto: "ADLTS:S:{type}:{id}"
	QREndValue        string       `db:"qr_end_value"`        // auto: "ADLTS:E:{type}:{id}"
	Audit
}

// TestLevelMapping maps a license class to the currently active test plan for a center.
type TestLevelMapping struct {
	ID            uuid.UUID `db:"id"`
	TestCenterID  uuid.UUID `db:"test_center_id"`
	TestLevelCode string    `db:"test_level_code"`
	TestPlanID    uuid.UUID `db:"test_plan_id"`
	Audit
}

// Test is the central aggregate for one candidate's testing attempt.
type Test struct {
	ID            uuid.UUID    `db:"id"`
	BookingID     uuid.UUID    `db:"booking_id"`
	CandidateID   uuid.UUID    `db:"candidate_id"`
	TestCenterID  uuid.UUID    `db:"test_center_id"`
	TestPlanID    uuid.UUID    `db:"test_plan_id"`
	DeviceID      *uuid.UUID   `db:"device_id"`
	TestLevelCode string       `db:"test_level_code"`
	Status        TestStatus   `db:"status"`
	AbortReason   *AbortReason `db:"abort_reason"`

	ScheduledStartAt   *time.Time `db:"scheduled_start_at"`
	ScheduledEndAt     *time.Time `db:"scheduled_end_at"`
	BookingWindowHours *int       `db:"booking_window_hours"`

	// Lifecycle timestamps — set as the test progresses through states
	DeviceScannedAt   *time.Time `db:"device_scanned_at"`
	GuidelinesStartAt *time.Time `db:"guidelines_start_at"`
	AcknowledgedAt    *time.Time `db:"acknowledged_at"`
	IoTHealthStartAt  *time.Time `db:"iot_health_start_at"`
	IoTHealthPassedAt *time.Time `db:"iot_health_passed_at"`
	StartedAt         *time.Time `db:"started_at"`
	FinishingAt       *time.Time `db:"finishing_at"`
	CompletedAt       *time.Time `db:"completed_at"`
	AbortedAt         *time.Time `db:"aborted_at"`

	// Result summary
	WeightedTotalScore         *float64   `db:"weighted_total_score"`
	Passed                     *bool      `db:"passed"`
	AppealWindowClosesAt       *time.Time `db:"appeal_window_closes_at"`
	ResultVisibleToCandidateAt *time.Time `db:"result_visible_to_candidate_at"`
	ResultVisibleToInstituteAt *time.Time `db:"result_visible_to_institute_at"`

	Audit
}

// IoTHealthCheck records the result of the pre-test device health verification.
// Does not embed Audit — it is a standalone log record.
type IoTHealthCheck struct {
	ID               uuid.UUID    `db:"id"`
	TestID           uuid.UUID    `db:"test_id"`
	Passed           bool         `db:"passed"`
	StreamReachable  bool         `db:"stream_reachable"`
	NetworkLatencyMs int          `db:"network_latency_ms"`
	CameraStatus     HealthStatus `db:"camera_status"`
	NetworkStatus    HealthStatus `db:"network_status"`
	ErrorMessage     string       `db:"error_message"`
	Attempts         int          `db:"attempts"`
	CheckedAt        time.Time    `db:"checked_at"`
}

// TestSession represents a single QR-gated maneuver window during a test.
type TestSession struct {
	ID             uuid.UUID  `db:"id"`
	TestID         uuid.UUID  `db:"test_id"`
	ManeuverID     uuid.UUID  `db:"maneuver_id"`
	SequenceNumber int        `db:"sequence_number"`
	StartFrameSeq  int64      `db:"start_frame_seq"`
	EndFrameSeq    *int64     `db:"end_frame_seq"`
	StartedAt      time.Time  `db:"started_at"`
	EndedAt        *time.Time `db:"ended_at"`

	// New fields — added in migration 003
	ManeuverType ManeuverType `db:"maneuver_type"` // denormalized for queries
	QRStartData  string       `db:"qr_start_data"` // raw QR value that opened this session
	QREndData    *string      `db:"qr_end_data"`   // raw QR value that closed it (nil if still open)

	// Scored fields — populated after session closes
	Score             *float64   `db:"score"`
	SessionPassed     *bool      `db:"passed"`
	CriticalFail      bool       `db:"critical_fail"`
	ScoredAt          *time.Time `db:"scored_at"`
	FrameCount        int        `db:"frame_count"`
	LaneDetectedCount int        `db:"lane_detected_count"`
	AvgIoUScore       *float64   `db:"avg_iou_score"`

	// Extended scoring fields — added in migration 003
	MeanCenterOffset     *float64 `db:"mean_center_offset_px"`
	OffsetVariance       *float64 `db:"offset_variance_px"`
	DimensionScores      JSONB    `db:"dimension_scores"`        // {centering, direction, smoothness, ...}
	EventCountBySeverity JSONB    `db:"event_count_by_severity"` // {minor, major, critical}
	WeakestPhase         string   `db:"weakest_phase"`
}

// FrameAnalysis is one scored frame written in 2-second batches.
type FrameAnalysis struct {
	ID         uuid.UUID `db:"id"`
	TestID     uuid.UUID `db:"test_id"`
	SessionID  uuid.UUID `db:"session_id"`
	FrameSeqNo int64     `db:"frame_seq_no"`
	CapturedAt time.Time `db:"captured_at"`

	// Legacy detection outputs — present from migration 002
	LaneDetected bool         `db:"lane_detected"`
	CurvatureDir CurvatureDir `db:"curvature_dir"`
	IoUScore     float64      `db:"iou_score"`
	FrameScore   float64      `db:"frame_score"`
	SpeedKmh     float64      `db:"speed_kmh"`
	HeadingDeg   float64      `db:"heading_deg"`
	IsMocked     bool         `db:"is_mocked"`

	// Extended detection outputs — added in migration 003
	LaneDetectorMode string  `db:"lane_detector_mode"` // "classical" | "onnx_fallback"
	CenterOffsetPx   float64 `db:"center_offset_px"`   // signed px from lane center
	CurvatureR       float64 `db:"curvature_r"`        // signed radius in px
	LaneSymmetry     float64 `db:"lane_symmetry"`      // 0–1
	MotionDir        string  `db:"motion_dir"`         // "forward"|"backward"|"stopped"
	QREvent          *string `db:"qr_event"`           // non-nil when QR code detected
	ManeuverPhase    string  `db:"maneuver_phase"`     // "entry"|"body"|"exit"|"approach"|"stop"

	CreatedAt time.Time `db:"created_at"`
}

// ManeuverEvent records a discrete scoring event during a maneuver session.
// One row per detected infraction/observation (e.g. lane departure, wrong direction).
type ManeuverEvent struct {
	ID           uuid.UUID `db:"id"`
	TestID       uuid.UUID `db:"test_id"`
	SessionID    uuid.UUID `db:"session_id"`
	ManeuverType string    `db:"maneuver_type"`
	EventType    string    `db:"event_type"` // "lane_departure"|"wrong_direction"|etc.
	Severity     string    `db:"severity"`   // "minor"|"major"|"critical"
	StartFrame   int64     `db:"start_frame"`
	EndFrame     *int64    `db:"end_frame"`
	Detail       JSONB     `db:"detail"` // {"offset_px":-45,"phase":"body"}
	CreatedAt    time.Time `db:"created_at"`
}

// SessionResult is computed after a TestSession closes.
type SessionResult struct {
	ID              uuid.UUID `db:"id"`
	TestID          uuid.UUID `db:"test_id"`
	SessionID       uuid.UUID `db:"session_id"`
	ManeuverID      uuid.UUID `db:"maneuver_id"`
	SequenceNumber  int       `db:"sequence_number"`
	Score           float64   `db:"score"`
	Weight          float64   `db:"weight"`
	Passed          bool      `db:"passed"`
	FrameCount      int       `db:"frame_count"`
	LaneDetectedPct float64   `db:"lane_detected_pct"`
	AvgIoU          float64   `db:"avg_iou"`

	// New fields — added in migration 003
	ManeuverType         ManeuverType `db:"maneuver_type"`
	CriticalFail         bool         `db:"critical_fail"`
	MeanCenterOffset     float64      `db:"mean_center_offset_px"`
	OffsetVariance       float64      `db:"offset_variance_px"`
	DimensionScores      JSONB        `db:"dimension_scores"`        // {centering, direction, ...}
	EventCountBySeverity JSONB        `db:"event_count_by_severity"` // {minor, major, critical}
	WeakestPhase         string       `db:"weakest_phase"`

	CreatedAt time.Time `db:"created_at"`
}

// TestResult is the final aggregate written when a test completes.
type TestResult struct {
	ID                 uuid.UUID `db:"id"`
	TestID             uuid.UUID `db:"test_id"`
	WeightedTotalScore float64   `db:"weighted_total_score"`
	Passed             bool      `db:"passed"`
	PassThreshold      float64   `db:"pass_threshold"`

	// New fields — added in migration 003
	AnyCriticalFail bool   `db:"any_critical_fail"`
	WeakestManeuver string `db:"weakest_maneuver"` // maneuver_type code of lowest-scoring maneuver
	ScoreBreakdown  JSONB  `db:"score_breakdown"`  // array of per-session score summaries

	// LLM-generated narratives (written asynchronously after test_results committed)
	OverallNarrative    string `db:"overall_narrative"`
	StrengthsNarrative  string `db:"strengths_narrative"`
	WeaknessesNarrative string `db:"weaknesses_narrative"`
	RecommendedFocus    string `db:"recommended_focus"`
	NarrativeModel      string `db:"narrative_model"` // "gemini-1.5-flash"|"fallback"

	CreatedAt time.Time `db:"created_at"`
}

// TestRecording tracks dashcam JPEG frames stored in MinIO (owned by testing core).
// The replay team reads minio_prefix to serve playback.
type TestRecording struct {
	ID          uuid.UUID  `db:"id"`
	TestID      uuid.UUID  `db:"test_id"`
	MinioPrefix string     `db:"minio_prefix"`
	FrameCount  int        `db:"frame_count"`
	SizeBytes   int64      `db:"size_bytes"`
	StartedAt   time.Time  `db:"started_at"`
	EndedAt     *time.Time `db:"ended_at"`
	Status      string     `db:"status"` // recording | saved | failed
	CreatedAt   time.Time  `db:"created_at"`

	// New fields — added in migration 003
	ManeuverType string     `db:"maneuver_type"` // NULL = full test recording
	SessionID    *uuid.UUID `db:"session_id"`    // NULL = full test recording
	VideoKey     string     `db:"video_key"`     // MinIO key for stitched MP4
}
