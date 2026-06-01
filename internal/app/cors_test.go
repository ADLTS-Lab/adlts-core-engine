package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func newCORSTestRouter(allowedOrigins []string) http.Handler {
	r := chi.NewRouter()
	r.Use(corsMiddleware(allowedOrigins))
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return r
}

func TestCORSAllowsConfiguredOriginPreflight(t *testing.T) {
	router := newCORSTestRouter([]string{"https://frontend.example.com"})
	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", "https://frontend.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected %d, got %d", http.StatusNoContent, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://frontend.example.com" {
		t.Fatalf("expected allow-origin header to echo origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET,POST,PATCH,PUT,DELETE,OPTIONS" {
		t.Fatalf("unexpected allow-methods header: %q", got)
	}
	headers := rec.Header().Get("Access-Control-Allow-Headers")
	for _, expected := range []string{"Authorization", "Content-Type", "X-Internal-Token", "X-Device-Secret"} {
		if !strings.Contains(headers, expected) {
			t.Fatalf("expected allow-headers to include %q, got %q", expected, headers)
		}
	}
}

func TestCORSRejectsUnknownOriginPreflight(t *testing.T) {
	router := newCORSTestRouter([]string{"https://frontend.example.com"})
	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow-origin header for blocked origin, got %q", got)
	}
}

func TestCORSAddsAllowOriginForConfiguredOriginRequest(t *testing.T) {
	router := newCORSTestRouter([]string{"https://frontend.example.com"})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://frontend.example.com")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://frontend.example.com" {
		t.Fatalf("expected allow-origin header to echo origin, got %q", got)
	}
}
