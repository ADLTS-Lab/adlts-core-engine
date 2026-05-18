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

	prompt := g.buildPrompt(input)
	result, err := g.callGemini(ctx, prompt)
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

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type parsedNarrative struct {
	Overall          string
	Strengths        string
	Weaknesses       string
	RecommendedFocus string
}

func (g *NarrativeGenerator) callGemini(ctx context.Context, prompt string) (*parsedNarrative, error) {
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.model, g.apiKey)

	body, _ := json.Marshal(geminiRequest{
		Contents: []geminiContent{{Parts: []geminiPart{{Text: prompt}}}},
	})
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
	sb.WriteString("You are an expert driving instructor analyzing a student's driving test results.\n\n")
	sb.WriteString(fmt.Sprintf("Test level: %s\n", inp.LevelCode))
	sb.WriteString(fmt.Sprintf("Overall score: %.1f / 100 (threshold: %.1f)\n", inp.TotalScore, inp.PassThreshold))
	if inp.Passed {
		sb.WriteString("Result: PASSED\n\n")
	} else {
		sb.WriteString("Result: FAILED\n\n")
	}
	sb.WriteString("Session breakdown:\n")
	for _, sr := range inp.SessionResults {
		sb.WriteString(fmt.Sprintf(
			"- Session %d: score=%.1f, lane_detected=%.1f%%, avg_iou=%.3f, passed=%v\n",
			sr.SequenceNumber, sr.Score, sr.LaneDetectedPct, sr.AvgIoU, sr.Passed,
		))
	}
	sb.WriteString(`
Please provide a JSON response with exactly these 4 fields (no markdown, just raw JSON):
{
  "overall": "<2-3 sentence overall narrative>",
  "strengths": "<strengths observed>",
  "weaknesses": "<areas needing improvement>",
  "recommended_focus": "<top 1-2 things to practice>"
}`)
	return sb.String()
}

// parseNarrativeText extracts the JSON narrative from the model's text response.
func parseNarrativeText(text string) *parsedNarrative {
	// Strip markdown code fences if present
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var out struct {
		Overall          string `json:"overall"`
		Strengths        string `json:"strengths"`
		Weaknesses       string `json:"weaknesses"`
		RecommendedFocus string `json:"recommended_focus"`
	}
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return &parsedNarrative{Overall: text}
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
