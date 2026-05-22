package reporting

import (
	"time"

	"github.com/google/uuid"
)

type CandidateProfile struct {
	ID         uuid.UUID `json:"id"`
	FirstName  string    `json:"first_name"`
	MiddleName string    `json:"middle_name"`
	LastName   string    `json:"last_name"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
}

type Test struct {
	ID                 uuid.UUID  `json:"id"`
	BookingID          uuid.UUID  `json:"booking_id"`
	CandidateID        uuid.UUID  `json:"candidate_id"`
	TestCenterID       uuid.UUID  `json:"test_center_id"`
	TestPlanID         uuid.UUID  `json:"test_plan_id"`
	TestLevelCode      string     `json:"test_level_code"`
	Status             string     `json:"status"`
	Passed             *bool      `json:"passed"`
	WeightedTotalScore *float64   `json:"weighted_total_score"`
	StartedAt          *time.Time `json:"started_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type TestResult struct {
	TestID              uuid.UUID `json:"test_id"`
	CandidateID         uuid.UUID `json:"candidate_id"`
	WeightedTotalScore   float64   `json:"weighted_total_score"`
	Passed              bool      `json:"passed"`
	PassThreshold       float64   `json:"pass_threshold"`
	AnyCriticalFail     bool      `json:"any_critical_fail"`
	WeakestManeuver     string    `json:"weakest_maneuver"`
	ResultReleasedAt    *time.Time `json:"result_released_at"`
	CompletedAt         *time.Time `json:"completed_at"`
}

type TestSession struct {
	ID                  uuid.UUID          `json:"id"`
	TestID              uuid.UUID          `json:"test_id"`
	ManeuverID          uuid.UUID          `json:"maneuver_id"`
	ManeuverType        string             `json:"maneuver_type"`
	DisplayName         string             `json:"display_name"`
	SequenceNumber      int                `json:"sequence_number"`
	StartedAt           time.Time          `json:"started_at"`
	EndedAt             *time.Time         `json:"ended_at"`
	FrameCount          int                `json:"frame_count"`
	Score               *float64           `json:"score"`
	Passed              *bool              `json:"passed"`
	CriticalFail        bool               `json:"critical_fail"`
	MeanCenterOffsetPx  *float64           `json:"mean_center_offset_px"`
	EventCountBySeverity map[string]int     `json:"event_count_by_severity"`
	DimensionScores     map[string]float64 `json:"dimension_scores"`
	WeakestPhase        *string            `json:"weakest_phase"`
}

type ManeuverEvent struct {
	ID         uuid.UUID              `json:"id"`
	SessionID  uuid.UUID              `json:"session_id"`
	EventType  string                 `json:"event_type"`
	Severity   string                 `json:"severity"`
	StartFrame int                    `json:"start_frame"`
	EndFrame   *int                   `json:"end_frame"`
	Detail     map[string]any         `json:"detail"`
	CreatedAt  time.Time              `json:"created_at"`
}

type ReportContext struct {
	Test      Test
	Result    TestResult
	Sessions  []TestSession
	Events    map[string][]ManeuverEvent
	Candidate CandidateProfile
}

type ManeuverSummary struct {
	DisplayName          string             `json:"display_name"`
	ManeuverType         string             `json:"maneuver_type"`
	Score                float64            `json:"score"`
	Passed               bool               `json:"passed"`
	CriticalFail         bool               `json:"critical_fail"`
	WeakestPhase         string             `json:"weakest_phase"`
	MeanCenterOffsetPx   float64            `json:"mean_center_offset_px"`
	EventCountBySeverity map[string]int     `json:"event_count_by_severity"`
	DimensionScores      map[string]float64 `json:"dimension_scores"`
}

type AnalyticalSummary struct {
	TestID             string            `json:"test_id"`
	Passed             bool              `json:"passed"`
	WeightedTotalScore  float64           `json:"weighted_total_score"`
	PassThreshold      float64           `json:"pass_threshold"`
	WeakestManeuver    string            `json:"weakest_maneuver"`
	AnyCriticalFail    bool              `json:"any_critical_fail"`
	MostCommonMistake  string            `json:"most_common_mistake"`
	CriticalEvents     []string          `json:"critical_events"`
	Strengths          []string          `json:"strengths"`
	Weaknesses         []string          `json:"weaknesses"`
	Recommendations    []string          `json:"recommendations"`
	ManeuverSummaries  []ManeuverSummary `json:"maneuver_summaries"`
}

type Narrative struct {
	OverallNarrative   string `json:"overall_narrative"`
	StrengthsNarrative string `json:"strengths_narrative"`
	WeaknessesNarrative string `json:"weaknesses_narrative"`
	RecommendedFocus   string `json:"recommended_focus"`
}

type ReportData struct {
	GeneratedAt time.Time       `json:"generated_at"`
	Test        Test            `json:"test"`
	Result      TestResult      `json:"result"`
	Candidate   CandidateProfile `json:"candidate"`
	Analytics   AnalyticalSummary `json:"analytics"`
	Narrative   Narrative      `json:"narrative"`
}
