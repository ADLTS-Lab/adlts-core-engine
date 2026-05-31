package reporting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type GeminiClient interface {
	GenerateNarrative(ctx context.Context, analytics AnalyticalSummary) (Narrative, error)
}

type HTTPGeminiClient struct {
	apiKey string
	model  string
	client *http.Client
}

func NewHTTPGeminiClient(apiKey, model string) *HTTPGeminiClient {
	if model == "" {
		model = "gemini-1.5-flash"
	}
	return &HTTPGeminiClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type geminiRequest struct {
	SystemInstruction struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"system_instruction,omitempty"`
	Contents []struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
	GenerationConfig struct {
		Temperature      float64 `json:"temperature"`
		ResponseMimeType string  `json:"responseMimeType"`
	} `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

func (c *HTTPGeminiClient) GenerateNarrative(ctx context.Context, analytics AnalyticalSummary) (Narrative, error) {
	analyticsJSON, err := AnalyticsPayload(analytics)
	if err != nil {
		return Narrative{}, err
	}

	systemPrompt, userPrompt := BuildPrompts(string(analyticsJSON))

	reqBody := geminiRequest{}
	reqBody.SystemInstruction.Parts = append(reqBody.SystemInstruction.Parts, struct {
		Text string `json:"text"`
	}{Text: systemPrompt})

	reqBody.Contents = append(reqBody.Contents, struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{
		Role: "user",
		Parts: []struct {
			Text string `json:"text"`
		}{{Text: userPrompt}},
	})
	
	reqBody.GenerationConfig.Temperature = 0.2
	reqBody.GenerationConfig.ResponseMimeType = "application/json"

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Narrative{}, err
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", c.model, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return Narrative{}, err
	}
	req.Header.Set("content-type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return Narrative{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Narrative{}, fmt.Errorf("gemini returned %s", resp.Status)
	}

	var out geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Narrative{}, err
	}
	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return Narrative{}, fmt.Errorf("gemini response missing content")
	}

	var narrative Narrative
	text := strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text)
	if err := json.Unmarshal([]byte(text), &narrative); err != nil {
		return Narrative{}, fmt.Errorf("invalid narrative json: %w", err)
	}
	return narrative, nil
}
