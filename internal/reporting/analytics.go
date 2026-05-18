package reporting

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type AnalyticsEngine struct{}

func NewAnalyticsEngine() *AnalyticsEngine { return &AnalyticsEngine{} }

func (e *AnalyticsEngine) Analyze(ctx ReportContext) AnalyticalSummary {
	summary := AnalyticalSummary{
		TestID:            ctx.Test.ID.String(),
		Passed:            ctx.Result.Passed,
		WeightedTotalScore: ctx.Result.WeightedTotalScore,
		PassThreshold:     ctx.Result.PassThreshold,
		WeakestManeuver:   ctx.Result.WeakestManeuver,
		AnyCriticalFail:   ctx.Result.AnyCriticalFail,
		CriticalEvents:    []string{},
		Strengths:         []string{},
		Weaknesses:        []string{},
		Recommendations:   []string{},
		ManeuverSummaries: []ManeuverSummary{},
	}

	mistakeCounts := map[string]int{}
	criticalCount := 0
	weakestScore := math.MaxFloat64
	weakestName := ctx.Result.WeakestManeuver

	for _, session := range ctx.Sessions {
		score := 0.0
		if session.Score != nil {
			score = *session.Score
		}
		passed := session.Passed != nil && *session.Passed
		meanOffset := 0.0
		if session.MeanCenterOffsetPx != nil {
			meanOffset = *session.MeanCenterOffsetPx
		}
		weakestPhase := ""
		if session.WeakestPhase != nil {
			weakestPhase = *session.WeakestPhase
		}
		if score < weakestScore {
			weakestScore = score
			weakestName = displayName(session.DisplayName, session.ManeuverType)
		}

		critical := session.CriticalFail
		if critical {
			criticalCount++
			summary.CriticalEvents = append(summary.CriticalEvents, fmt.Sprintf("%s had a critical failure", displayName(session.DisplayName, session.ManeuverType)))
		}

		for _, event := range ctx.Events[session.ID.String()] {
			mistakeCounts[event.EventType]++
			if strings.EqualFold(event.Severity, "critical") {
				criticalCount++
				summary.CriticalEvents = append(summary.CriticalEvents, fmt.Sprintf("%s: %s", displayName(session.DisplayName, session.ManeuverType), event.EventType))
			}
		}

		maneuverSummary := ManeuverSummary{
			DisplayName:          displayName(session.DisplayName, session.ManeuverType),
			ManeuverType:         session.ManeuverType,
			Score:                score,
			Passed:               passed,
			CriticalFail:         critical,
			WeakestPhase:         weakestPhase,
			MeanCenterOffsetPx:   meanOffset,
			EventCountBySeverity: safeMap(session.EventCountBySeverity),
			DimensionScores:      safeScoreMap(session.DimensionScores),
		}
		summary.ManeuverSummaries = append(summary.ManeuverSummaries, maneuverSummary)
	}

	if weakestName == "" && len(summary.ManeuverSummaries) > 0 {
		weakestName = summary.ManeuverSummaries[0].DisplayName
	}
	summary.WeakestManeuver = weakestName
	summary.AnyCriticalFail = criticalCount > 0 || ctx.Result.AnyCriticalFail
	summary.MostCommonMistake = mostCommon(mistakeCounts)

	if summary.WeightedTotalScore >= 85 {
		summary.Strengths = append(summary.Strengths, "High overall score with strong execution across the test")
	}
	if criticalCount == 0 {
		summary.Strengths = append(summary.Strengths, "No critical failures were recorded")
	}
	if avgCenterOffset(summary.ManeuverSummaries) < 20 {
		summary.Strengths = append(summary.Strengths, "Consistent lane centering was maintained")
	}
	if summary.WeightedTotalScore >= summary.PassThreshold && !summary.AnyCriticalFail {
		summary.Strengths = append(summary.Strengths, "Smooth execution supported a passing result")
	}

	if summary.WeightedTotalScore < summary.PassThreshold {
		summary.Weaknesses = append(summary.Weaknesses, fmt.Sprintf("Overall score %.1f is below the pass threshold %.1f", summary.WeightedTotalScore, summary.PassThreshold))
		summary.Recommendations = append(summary.Recommendations, "Focus on consistent execution of low-scoring maneuvers to raise the overall score")
	}
	if summary.AnyCriticalFail {
		summary.Weaknesses = append(summary.Weaknesses, "Critical failures affected the final result")
		summary.Recommendations = append(summary.Recommendations, "Reduce risky inputs and stabilize maneuver control to avoid critical events")
	}
	if avgCenterOffset(summary.ManeuverSummaries) >= 20 {
		summary.Weaknesses = append(summary.Weaknesses, "Centering consistency needs improvement")
		summary.Recommendations = append(summary.Recommendations, "Practice lane centering and use mirror references more consistently")
	}
	if summary.MostCommonMistake != "" {
		summary.Recommendations = append(summary.Recommendations, recommendationForMistake(summary.MostCommonMistake))
	}
	if summary.WeakestManeuver != "" {
		summary.Weaknesses = append(summary.Weaknesses, fmt.Sprintf("%s was the weakest maneuver", summary.WeakestManeuver))
	}

	return summary
}

func displayName(name, maneuverType string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return strings.Title(strings.ReplaceAll(maneuverType, "_", " "))
}

func safeMap(src map[string]int) map[string]int {
	if src == nil {
		return map[string]int{}
	}
	out := make(map[string]int, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func safeScoreMap(src map[string]float64) map[string]float64 {
	if src == nil {
		return map[string]float64{}
	}
	out := make(map[string]float64, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func mostCommon(counts map[string]int) string {
	var best string
	bestCount := 0
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if counts[k] > bestCount {
			best = k
			bestCount = counts[k]
		}
	}
	return best
}

func avgCenterOffset(summaries []ManeuverSummary) float64 {
	if len(summaries) == 0 {
		return 0
	}
	total := 0.0
	for _, s := range summaries {
		total += math.Abs(s.MeanCenterOffsetPx)
	}
	return total / float64(len(summaries))
}

func recommendationForMistake(mistake string) string {
	switch strings.ToLower(mistake) {
	case "reverse_parking", "parking_reverse", "reverse_parking_issue":
		return "Practice reverse parking using mirrors and slower steering adjustments"
	case "wrong_direction", "direction_error":
		return "Improve steering anticipation and align direction changes earlier"
	case "lane_departure":
		return "Focus on lane centering and keep sight of reference markers"
	case "sudden_brake", "hard_braking":
		return "Use smoother pedal input and anticipate stopping distance"
	default:
		return "Review the weakest maneuver carefully and rehearse it at reduced speed"
	}
}
