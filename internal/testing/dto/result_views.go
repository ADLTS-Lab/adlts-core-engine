// Package dto contains response-shaping types for the testing module.
// These are distinct from domain structs — they flatten and filter fields
// based on the caller's role (see §13 of the implementation plan).
package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ── Shared sub-types ──────────────────────────────────────────────────────────

// ManeuverScoreSummary is a lightweight per-maneuver entry used in result views.
type ManeuverScoreSummary struct {
	ManeuverType string  `json:"maneuver_type"`
	DisplayName  string  `json:"display_name"`
	Score        float64 `json:"score"`
	Passed       bool    `json:"passed"`
	Weight       float64 `json:"weight"`
	CriticalFail bool    `json:"critical_fail"`
}

// DimensionScores holds the per-dimension scores returned from Python scorer.
type DimensionScores struct {
	Centering         float64 `json:"centering"`
	Direction         float64 `json:"direction"`
	Smoothness        float64 `json:"smoothness"`
	LaneQuality       float64 `json:"lane_quality"`
	ReverseCompliance float64 `json:"reverse_compliance"`
}

// PhaseScores holds entry/body/exit phase scores.
type PhaseScores struct {
	Entry float64 `json:"entry"`
	Body  float64 `json:"body"`
	Exit  float64 `json:"exit"`
}

// ManeuverEventView is a single scoring event entry exposed in the full result.
type ManeuverEventView struct {
	EventType  string          `json:"event_type"`
	Severity   string          `json:"severity"`
	StartFrame int64           `json:"start_frame"`
	EndFrame   *int64          `json:"end_frame,omitempty"`
	Detail     json.RawMessage `json:"detail,omitempty"`
}

// SessionResultView is the per-maneuver detail block used in FullResultView.
type SessionResultView struct {
	SessionID            uuid.UUID           `json:"session_id"`
	ManeuverType         string              `json:"maneuver_type"`
	DisplayName          string              `json:"display_name"`
	SequenceNumber       int                 `json:"sequence_number"`
	Score                float64             `json:"score"`
	Passed               bool                `json:"passed"`
	CriticalFail         bool                `json:"critical_fail"`
	Weight               float64             `json:"weight"`
	FrameCount           int                 `json:"frame_count"`
	LaneDetectedPct      float64             `json:"lane_detected_pct"`
	AvgIoU               float64             `json:"avg_iou"`
	MeanCenterOffsetPx   float64             `json:"mean_center_offset_px"`
	OffsetVariancePx     float64             `json:"offset_variance_px"`
	DimensionScores      *DimensionScores    `json:"dimension_scores,omitempty"`
	PhaseScores          *PhaseScores        `json:"phase_scores,omitempty"`
	WeakestPhase         string              `json:"weakest_phase,omitempty"`
	EventCountBySeverity map[string]int      `json:"event_count_by_severity,omitempty"`
	Events               []ManeuverEventView `json:"events,omitempty"`
}

// ── Role-based result views ───────────────────────────────────────────────────

// CandidateResultView is what a candidate sees after their visibility delay expires.
// Includes: score, pass/fail, narrative, and per-maneuver scores (no raw events).
// Rule: visible only after result_visible_to_candidate_at has elapsed.
type CandidateResultView struct {
	TestID             uuid.UUID `json:"test_id"`
	WeightedTotalScore float64   `json:"weighted_total_score"`
	Passed             bool      `json:"passed"`
	PassThreshold      float64   `json:"pass_threshold"`
	AnyCriticalFail    bool      `json:"any_critical_fail"`
	WeakestManeuver    string    `json:"weakest_maneuver,omitempty"`

	// Per-maneuver breakdown (scores only, no events or dimension detail)
	ManeuverScores []ManeuverScoreSummary `json:"maneuver_scores"`

	// LLM-generated narrative
	OverallNarrative    string `json:"overall_narrative"`
	StrengthsNarrative  string `json:"strengths_narrative"`
	WeaknessesNarrative string `json:"weaknesses_narrative"`
	RecommendedFocus    string `json:"recommended_focus"`

	ResultAvailableAt *time.Time `json:"result_available_at,omitempty"`
}

// InstituteResultView is what an institute/school sees.
// Includes: summary-level data only — no per-session events or dimension scores.
type InstituteResultView struct {
	TestID              uuid.UUID  `json:"test_id"`
	CandidateID         uuid.UUID  `json:"candidate_id"`
	TestLevelCode       string     `json:"test_level_code"`
	WeightedTotalScore  float64    `json:"weighted_total_score"`
	Passed              bool       `json:"passed"`
	PassThreshold       float64    `json:"pass_threshold"`
	WeakestManeuver     string     `json:"weakest_maneuver,omitempty"`
	StrengthsNarrative  string     `json:"strengths_narrative"`
	WeaknessesNarrative string     `json:"weaknesses_narrative"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
}

// FullResultView is what Admin, SuperAdmin, and Expert see.
// Includes: everything — dimension scores, event log, phase breakdown.
type FullResultView struct {
	TestID             uuid.UUID `json:"test_id"`
	CandidateID        uuid.UUID `json:"candidate_id"`
	TestLevelCode      string    `json:"test_level_code"`
	WeightedTotalScore float64   `json:"weighted_total_score"`
	Passed             bool      `json:"passed"`
	PassThreshold      float64   `json:"pass_threshold"`
	AnyCriticalFail    bool      `json:"any_critical_fail"`
	WeakestManeuver    string    `json:"weakest_maneuver,omitempty"`

	// Full per-session breakdown with dimension scores and events
	Sessions []SessionResultView `json:"sessions"`

	// LLM-generated narrative
	OverallNarrative    string `json:"overall_narrative"`
	StrengthsNarrative  string `json:"strengths_narrative"`
	WeaknessesNarrative string `json:"weaknesses_narrative"`
	RecommendedFocus    string `json:"recommended_focus"`
	NarrativeModel      string `json:"narrative_model"`

	// Test lifecycle timestamps
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	ScoredAt    *time.Time `json:"scored_at,omitempty"`

	// Score breakdown blob (raw JSON stored on test_results.score_breakdown)
	ScoreBreakdown json.RawMessage `json:"score_breakdown,omitempty"`
}
