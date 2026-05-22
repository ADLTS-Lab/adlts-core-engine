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

type AnthropicClient interface {
	GenerateNarrative(ctx context.Context, analytics AnalyticalSummary) (Narrative, error)
}

type HTTPAnthropicClient struct {
	apiKey string
	model  string
	client *http.Client
}

func NewHTTPAnthropicClient(apiKey, model string) *HTTPAnthropicClient {
	if model == "" {
		model = "claude-3-5-sonnet-latest"
	}
	return &HTTPAnthropicClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type anthropicMessageRequest struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	System    string `json:"system"`
	Messages  []struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"messages"`
}

type anthropicMessageResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (c *HTTPAnthropicClient) GenerateNarrative(ctx context.Context, analytics AnalyticalSummary) (Narrative, error) {
	analyticsJSON, err := AnalyticsPayload(analytics)
	if err != nil {
		return Narrative{}, err
	}

	systemPrompt, userPrompt := BuildPrompts(string(analyticsJSON))
	reqBody := anthropicMessageRequest{Model: c.model, MaxTokens: 600, System: systemPrompt}
	reqBody.Messages = append(reqBody.Messages, struct {
		Role    string `json:"role"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}{
		Role: "user",
		Content: []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{{Type: "text", Text: userPrompt}},
	})

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Narrative{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return Narrative{}, err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return Narrative{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Narrative{}, fmt.Errorf("anthropic returned %s", resp.Status)
	}

	var out anthropicMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Narrative{}, err
	}
	if len(out.Content) == 0 {
		return Narrative{}, fmt.Errorf("anthropic response missing content")
	}

	var narrative Narrative
	text := strings.TrimSpace(out.Content[0].Text)
	if err := json.Unmarshal([]byte(text), &narrative); err != nil {
		return Narrative{}, fmt.Errorf("invalid narrative json: %w", err)
	}
	return narrative, nil
}
