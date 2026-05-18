package reporting

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const reportSystemPrompt = `You are a professional driving assessment report writer.

Given structured analysis data for a practical driving test, write concise, objective, and constructive narratives.

Return valid JSON only with the following fields:
- overall_narrative
- strengths_narrative
- weaknesses_narrative
- recommended_focus

Rules:
- Be professional and encouraging.
- Base all statements strictly on the provided data.
- Do not invent facts.
- Keep each field between 2 and 5 sentences.
- Return only valid JSON.`

func BuildPrompts(analyticsJSON string) (string, string) {
	return reportSystemPrompt, fmt.Sprintf("Generate the report narrative from the following structured analysis:\n\n%s", analyticsJSON)
}

func AnalyticsPayload(summary AnalyticalSummary) ([]byte, error) {
	payload := map[string]any{
		"passed":              summary.Passed,
		"weighted_total_score": summary.WeightedTotalScore,
		"pass_threshold":      summary.PassThreshold,
		"weakest_maneuver":    summary.WeakestManeuver,
		"any_critical_fail":   summary.AnyCriticalFail,
		"most_common_mistake": summary.MostCommonMistake,
		"strengths":           summary.Strengths,
		"weaknesses":          summary.Weaknesses,
		"recommendations":     summary.Recommendations,
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buf.Bytes()), nil
}
