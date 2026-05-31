package identity

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func buildIdentityRouterForRefreshTests() (*security.Manager, http.Handler) {
	tokens := security.NewManager("test-secret-min-32-chars-long-12345678")
	svc := NewService(nil, tokens, nil)
	handler := NewHandler(svc, tokens)

	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		handler.Mount(api)
	})
	return tokens, r
}

func TestRefreshTokenEndpointSuccessWithoutAuthorizationHeader(t *testing.T) {
	tokens, router := buildIdentityRouterForRefreshTests()

	refreshToken, err := tokens.SignRefreshToken(uuid.New(), security.EntityCandidate, "candidate@test.et", nil)
	if err != nil {
		t.Fatalf("failed to create refresh token: %v", err)
	}
	body, _ := json.Marshal(RefreshRequest{RefreshToken: refreshToken})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}

	var payload struct {
		Success bool          `json:"success"`
		Data    LoginResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !payload.Success {
		t.Fatalf("expected success=true")
	}
	if payload.Data.AccessToken == "" || payload.Data.RefreshToken == "" {
		t.Fatalf("expected both access and refresh tokens in response")
	}
	if payload.Data.EntityType != string(security.EntityCandidate) {
		t.Fatalf("expected entity_type %q, got %q", security.EntityCandidate, payload.Data.EntityType)
	}
}

func TestRefreshTokenEndpointRejectsAccessToken(t *testing.T) {
	tokens, router := buildIdentityRouterForRefreshTests()

	accessToken, err := tokens.SignAccessToken(uuid.New(), security.EntityCandidate, "candidate@test.et", nil)
	if err != nil {
		t.Fatalf("failed to create access token: %v", err)
	}
	body, _ := json.Marshal(RefreshRequest{RefreshToken: accessToken})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}

	var payload struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Success {
		t.Fatalf("expected success=false")
	}
	if payload.Error.Code != "INVALID_REFRESH_TOKEN" {
		t.Fatalf("expected INVALID_REFRESH_TOKEN, got %q", payload.Error.Code)
	}
}

func TestRefreshTokenEndpointRejectsMalformedToken(t *testing.T) {
	_, router := buildIdentityRouterForRefreshTests()

	body, _ := json.Marshal(RefreshRequest{RefreshToken: "not-a-jwt"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
}
