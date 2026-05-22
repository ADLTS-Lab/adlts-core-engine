package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type IdentityClient interface {
	GetCandidate(ctx context.Context, candidateID uuid.UUID) (CandidateProfile, error)
}

type HTTPIdentityClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewHTTPIdentityClient(baseURL, token string) *HTTPIdentityClient {
	return &HTTPIdentityClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *HTTPIdentityClient) GetCandidate(ctx context.Context, candidateID uuid.UUID) (CandidateProfile, error) {
	var out struct {
		Data CandidateProfile `json:"data"`
	}
	if err := c.getJSON(ctx, "/candidates/"+url.PathEscape(candidateID.String()), &out); err != nil {
		return CandidateProfile{}, err
	}
	return out.Data, nil
}

func (c *HTTPIdentityClient) getJSON(ctx context.Context, path string, dst any) error {
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
		return fmt.Errorf("identity %s returned %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}
