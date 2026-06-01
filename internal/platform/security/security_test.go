package security

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestAuthenticateMissingHeaderReturnsJSON(t *testing.T) {
	manager := NewManager("test-secret-min-32-chars-long-12345678")
	r := chi.NewRouter()
	r.With(Authenticate(manager)).Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("expected json content-type, got %q", got)
	}

	var payload struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if payload.Success {
		t.Fatalf("expected success=false")
	}
	if payload.Error.Code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q", payload.Error.Code)
	}
}

func TestRequireEntitiesWrongRoleReturnsJSONForbidden(t *testing.T) {
	manager := NewManager("test-secret-min-32-chars-long-12345678")
	candidateToken, err := manager.Sign(uuid.New(), EntityCandidate, "candidate@test.et", nil)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	r := chi.NewRouter()
	r.With(Authenticate(manager), RequireEntities(EntityAdmin)).Get("/admin-only", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	req.Header.Set("Authorization", "Bearer "+candidateToken)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rec.Code)
	}

	var payload struct {
		Success bool `json:"success"`
		Error   struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}
	if payload.Success {
		t.Fatalf("expected success=false")
	}
	if payload.Error.Code != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN, got %q", payload.Error.Code)
	}
}
