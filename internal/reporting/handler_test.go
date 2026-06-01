package reporting

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func mountReportRoutesForTest() http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		api.Route("/reports", func(reports chi.Router) {
			NewHandler(nil).Mount(reports)
		})
	})
	return r
}

func TestMountUsesSingleReportsPrefix(t *testing.T) {
	router := mountReportRoutesForTest()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/not-a-uuid/generate", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}

	reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/reports/reports/not-a-uuid/generate", nil)
	recDup := httptest.NewRecorder()
	router.ServeHTTP(recDup, reqDup)

	if recDup.Code != http.StatusNotFound {
		t.Fatalf("expected %d for duplicated /reports prefix, got %d", http.StatusNotFound, recDup.Code)
	}
}

func TestGenerateReportInvalidUUIDReturnsJSON(t *testing.T) {
	router := mountReportRoutesForTest()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports/not-a-uuid/generate", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
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
	if payload.Error.Code != "INVALID_ID" {
		t.Fatalf("expected INVALID_ID, got %q", payload.Error.Code)
	}
}
