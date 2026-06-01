package config

import "testing"

func TestLoadUsesDefaultCORSOriginsWhenUnset(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	cfg := Load()

	if len(cfg.CORSAllowedOrigins) == 0 {
		t.Fatalf("expected default CORS allowed origins to be set")
	}
	expected := map[string]bool{
		"http://localhost:3000": true,
		"http://127.0.0.1:3000": true,
		"http://localhost:5173": true,
		"http://127.0.0.1:5173": true,
	}
	for _, origin := range cfg.CORSAllowedOrigins {
		delete(expected, origin)
	}
	if len(expected) != 0 {
		t.Fatalf("missing expected default origins: %#v", expected)
	}
}

func TestLoadParsesConfiguredCORSOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com, https://admin.example.com ")

	cfg := Load()

	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("expected 2 origins, got %d", len(cfg.CORSAllowedOrigins))
	}
	if cfg.CORSAllowedOrigins[0] != "https://app.example.com" {
		t.Fatalf("unexpected first origin: %q", cfg.CORSAllowedOrigins[0])
	}
	if cfg.CORSAllowedOrigins[1] != "https://admin.example.com" {
		t.Fatalf("unexpected second origin: %q", cfg.CORSAllowedOrigins[1])
	}
}
