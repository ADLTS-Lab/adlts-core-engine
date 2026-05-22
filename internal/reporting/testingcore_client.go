package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TestingCoreClient interface {
	GetTest(ctx context.Context, testID string) (Test, error)
	GetResult(ctx context.Context, testID string) (TestResult, error)
	ListSessions(ctx context.Context, testID string) ([]TestSession, error)
	ListEvents(ctx context.Context, testID, sessionID string) ([]ManeuverEvent, error)
}

type HTTPTestingCoreClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPTestingCoreClient(baseURL, token string) *HTTPTestingCoreClient {
	return &HTTPTestingCoreClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *HTTPTestingCoreClient) GetTest(ctx context.Context, testID string) (Test, error) {
	var out struct {
		Data Test `json:"data"`
	}
	if err := c.getJSON(ctx, "/tests/"+url.PathEscape(testID), &out); err != nil {
		return Test{}, err
	}
	return out.Data, nil
}

func (c *HTTPTestingCoreClient) GetResult(ctx context.Context, testID string) (TestResult, error) {
	var out struct {
		Data TestResult `json:"data"`
	}
	if err := c.getJSON(ctx, "/tests/"+url.PathEscape(testID)+"/result", &out); err != nil {
		return TestResult{}, err
	}
	return out.Data, nil
}

func (c *HTTPTestingCoreClient) ListSessions(ctx context.Context, testID string) ([]TestSession, error) {
	var out struct {
		Data []TestSession `json:"data"`
	}
	if err := c.getJSON(ctx, "/tests/"+url.PathEscape(testID)+"/sessions", &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

func (c *HTTPTestingCoreClient) ListEvents(ctx context.Context, testID, sessionID string) ([]ManeuverEvent, error) {
	var out struct {
		Data []ManeuverEvent `json:"data"`
	}
	path := fmt.Sprintf("/tests/%s/sessions/%s/events", url.PathEscape(testID), url.PathEscape(sessionID))
	if err := c.getJSON(ctx, path, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

func (c *HTTPTestingCoreClient) getJSON(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("testing core %s returned %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
