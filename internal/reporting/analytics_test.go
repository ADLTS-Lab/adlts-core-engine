package reporting

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)
func TestAnalyzeBuildsSummary(t *testing.T) {
	offset := 12.5
	score := 88.2
	passed := true
	weakest := "mirror_alignment"
	resultPassed := true

	ctx := ReportContext{
		Test: Test{ID: uuid.New()},
		Result: TestResult{
			TestID:            uuid.New(),
			CandidateID:       uuid.New(),
			WeightedTotalScore: 88.2,
			Passed:            resultPassed,
			PassThreshold:     70,
			AnyCriticalFail:   false,
			WeakestManeuver:   "Reverse Parking",
		},
		Sessions: []TestSession{
			{
				ID:                 uuid.New(),
				ManeuverType:       "reverse_parking",
				DisplayName:        "Reverse Parking",
				SequenceNumber:     1,
				Score:              &score,
				Passed:             &passed,
				CriticalFail:       false,
				MeanCenterOffsetPx: &offset,
				EventCountBySeverity: map[string]int{"minor": 1},
				DimensionScores:    map[string]float64{"centering": 92},
				WeakestPhase:       &weakest,
			},
		},
		Events: map[string][]ManeuverEvent{},
	}

	summary := NewAnalyticsEngine().Analyze(ctx)
	if summary.TestID == "" {
		t.Fatal("expected test id")
	}
	if !summary.Passed {
		t.Fatal("expected passing summary")
	}
	if len(summary.ManeuverSummaries) != 1 {
		t.Fatalf("expected 1 maneuver summary, got %d", len(summary.ManeuverSummaries))
	}
	if summary.WeakestManeuver == "" {
		t.Fatal("expected weakest maneuver")
	}
}

func TestAnalyticsPayloadAndPrompts(t *testing.T) {
	summary := AnalyticalSummary{
		Passed:            true,
		WeightedTotalScore: 83.4,
		PassThreshold:     70,
		WeakestManeuver:   "Reverse Parking",
		AnyCriticalFail:   false,
		MostCommonMistake: "lane_departure",
		Strengths:         []string{"Strong lane centering"},
		Weaknesses:        []string{"Reverse parking inconsistent"},
		Recommendations:   []string{"Practice reverse parking slowly"},
	}

	payload, err := AnalyticsPayload(summary)
	if err != nil {
		t.Fatalf("payload: %v", err)
	}
	system, user := BuildPrompts(string(payload))
	if system == "" || user == "" {
		t.Fatal("expected prompts")
	}
	if !strings.Contains(string(payload), "weighted_total_score") {
		t.Fatal("expected analytics subset in payload")
	}
}

func TestRendererRendersHTML(t *testing.T) {
	renderer, err := NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	generatedAt := time.Now().UTC()
	passed := true
	data := ReportData{
		GeneratedAt: generatedAt,
		Test: Test{ID: uuid.New(), TestLevelCode: "B"},
		Result: TestResult{WeightedTotalScore: 81.5, PassThreshold: 70, Passed: true},
		Candidate: CandidateProfile{FirstName: "Jane", LastName: "Doe", Email: "jane@example.com"},
		Analytics: AnalyticalSummary{Strengths: []string{"Strong control"}, Weaknesses: []string{"Needs work"}, Recommendations: []string{"Practice"}, ManeuverSummaries: []ManeuverSummary{{DisplayName: "Straight Line", ManeuverType: "straight_line", Score: 90, Passed: true, CriticalFail: false, MeanCenterOffsetPx: 8, EventCountBySeverity: map[string]int{}, DimensionScores: map[string]float64{}}}, Passed: passed},
		Narrative: Narrative{OverallNarrative: "Overall narrative", StrengthsNarrative: "Strengths", WeaknessesNarrative: "Weaknesses", RecommendedFocus: "Focus"},
	}

	html, err := renderer.RenderHTML(data)
	if err != nil {
		t.Fatalf("render html: %v", err)
	}
	if !strings.Contains(string(html), "Jane") || !strings.Contains(string(html), "ADLTS Reporting") {
		t.Fatal("rendered html missing expected content")
	}
	_ = generatedAt
}
