package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"adlts/internal/domain"
)

// NarrativeInput is passed to the NarrativeGenerator for each completed test.
type NarrativeInput struct {
	TestID         string
	LevelCode      string
	TotalScore     float64
	Passed         bool
	PassThreshold  float64
	SessionResults []*domain.SessionResult
}

// NarrativeOutput contains the generated paragraphs.
type NarrativeOutput struct {
	Overall          string
	Strengths        string
	Weaknesses       string
	RecommendedFocus string
	ModelUsed        string
}

// NarrativeGenerator is the pluggable interface for LLM narrative generation.
// Currently backed by Gemini; can be swapped for local LLM or Azure AI later.
type NarrativeGenerator struct {
	apiKey string
	model  string
	client *http.Client
}

func NewNarrativeGenerator(apiKey, model string) *NarrativeGenerator {
	if model == "" {
		model = "gemini-1.5-flash"
	}
	return &NarrativeGenerator{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Generate calls the Gemini API and returns a structured narrative.
// Falls back to a minimal static summary if the API call fails.
func (g *NarrativeGenerator) Generate(ctx context.Context, input *NarrativeInput) (*NarrativeOutput, error) {
	if g.apiKey == "" {
		return g.fallback(input), nil
	}

	userPrompt := g.buildPrompt(input)
	result, err := g.callGemini(ctx, narrativeSystemPrompt, userPrompt)
	if err != nil {
		return g.fallback(input), nil
	}

	return &NarrativeOutput{
		Overall:          result.Overall,
		Strengths:        result.Strengths,
		Weaknesses:       result.Weaknesses,
		RecommendedFocus: result.RecommendedFocus,
		ModelUsed:        g.model,
	}, nil
}

// ── Gemini REST API call ─────────────────────────────────────────────────────

// geminiRequest is the full request body sent to the Gemini REST API.
// Uses system_instruction + JSON response mode for structured output.
type geminiRequest struct {
	SystemInstruction *geminiSystemInstruction `json:"system_instruction,omitempty"`
	Contents          []geminiContent          `json:"contents"`
	GenerationConfig  geminiGenerationConfig   `json:"generationConfig"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature      float64 `json:"temperature"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

const narrativeSystemPrompt = `You are an expert driving instructor producing a structured written assessment of a student's practical driving test for the student to read.

Rules:
- Use specific, technical driving terminology (e.g. lane centering, steering correction, vehicle positioning, speed modulation, traffic awareness, mirror checks, road positioning, observation).
- Base all statements strictly on the provided numerical data. Do not invent facts.
- Be professional, objective, and constructive — this is for the student's learning.
- Keep overall, strengths, weaknesses at 2-4 sentences. Keep recommended_focus at 3-5 sentences with actionable drills.
- Return ONLY valid JSON with exactly these 4 keys: overall, strengths, weaknesses, recommended_focus.`

type parsedNarrative struct {
	Overall          string
	Strengths        string
	Weaknesses       string
	RecommendedFocus string
}

func (g *NarrativeGenerator) callGemini(ctx context.Context, systemPrompt, userPrompt string) (*parsedNarrative, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.model, g.apiKey)

	reqBody := geminiRequest{
		SystemInstruction: &geminiSystemInstruction{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		Contents: []geminiContent{
			{Role: "user", Parts: []geminiPart{{Text: userPrompt}}},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:      0.2,
			ResponseMimeType: "application/json",
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini returned HTTP %d", resp.StatusCode)
	}

	var gr geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, err
	}
	if len(gr.Candidates) == 0 || len(gr.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	return parseNarrativeText(gr.Candidates[0].Content.Parts[0].Text), nil
}

func (g *NarrativeGenerator) buildPrompt(inp *NarrativeInput) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Test level: %s\n", inp.LevelCode))
	sb.WriteString(fmt.Sprintf("Overall score: %.1f / 100 (pass threshold: %.1f)\n", inp.TotalScore, inp.PassThreshold))
	if inp.Passed {
		sb.WriteString("Result: PASSED\n\n")
	} else {
		sb.WriteString("Result: FAILED\n\n")
	}
	sb.WriteString("Per-maneuver breakdown with technical metrics:\n")
	for _, sr := range inp.SessionResults {
		critStr := ""
		if sr.CriticalFail {
			critStr = " [CRITICAL FAIL]"
		}
		phaseStr := ""
		if sr.WeakestPhase != "" {
			phaseStr = fmt.Sprintf(", weakest_phase=%s", sr.WeakestPhase)
		}
		dimStr := ""
		if len(sr.DimensionScores) > 0 {
			dimStr = fmt.Sprintf(", dimension_scores=%s", string(sr.DimensionScores))
		}
		evtStr := ""
		if len(sr.EventCountBySeverity) > 0 {
			evtStr = fmt.Sprintf(", events=%s", string(sr.EventCountBySeverity))
		}
		sb.WriteString(fmt.Sprintf(
			"- #%d %s: score=%.1f, lane_detected=%.1f%%, avg_iou=%.3f, mean_center_offset=%.1fpx, offset_variance=%.1f, passed=%v%s%s%s%s\n",
			sr.SequenceNumber, string(sr.ManeuverType), sr.Score,
			sr.LaneDetectedPct, sr.AvgIoU, sr.MeanCenterOffset, sr.OffsetVariance,
			sr.Passed, critStr, phaseStr, dimStr, evtStr,
		))
	}
	sb.WriteString(`
Generate the assessment as JSON with exactly 4 keys:
{
  "overall": "...",
  "strengths": "...",
  "weaknesses": "...",
  "recommended_focus": "..."
}`)
	return sb.String()
}

// parseNarrativeText extracts the JSON narrative from the model's text response.
func parseNarrativeText(text string) *parsedNarrative {
	// JSON mode should return pure JSON, but strip fences defensively
	text = strings.TrimSpace(text)
	if idx := strings.Index(text, "{"); idx > 0 {
		text = text[idx:]
	}
	if idx := strings.LastIndex(text, "}"); idx >= 0 && idx < len(text)-1 {
		text = text[:idx+1]
	}

	var out struct {
		Overall          string `json:"overall"`
		Strengths        string `json:"strengths"`
		Weaknesses       string `json:"weaknesses"`
		RecommendedFocus string `json:"recommended_focus"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return &parsedNarrative{Overall: strings.TrimSpace(text)}
	}
	return &parsedNarrative{
		Overall:          out.Overall,
		Strengths:        out.Strengths,
		Weaknesses:       out.Weaknesses,
		RecommendedFocus: out.RecommendedFocus,
	}
}

// fallback returns a minimal static narrative when the API is unavailable.
func (g *NarrativeGenerator) fallback(inp *NarrativeInput) *NarrativeOutput {
	verdict := "did not pass"
	if inp.Passed {
		verdict = "passed"
	}
	return &NarrativeOutput{
		Overall:          fmt.Sprintf("The candidate %s the test with a score of %.1f%%.", verdict, inp.TotalScore),
		Strengths:        "Data available in session breakdown.",
		Weaknesses:       "Data available in session breakdown.",
		RecommendedFocus: "Review individual session results for details.",
		ModelUsed:        "fallback",
	}
}
