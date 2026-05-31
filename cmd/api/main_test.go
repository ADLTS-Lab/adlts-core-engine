package main

import (
	"testing"

	"adlts/internal/platform/config"
)

func TestDatabaseURLUsesLocalhost(t *testing.T) {
	cases := []struct {
		name        string
		databaseURL string
		want        bool
	}{
		{
			name:        "localhost URL",
			databaseURL: "postgres://adlts:adlts@localhost:5432/adlts_dev?sslmode=disable",
			want:        true,
		},
		{
			name:        "loopback URL",
			databaseURL: "postgres://adlts:adlts@127.0.0.1:5432/adlts_dev?sslmode=disable",
			want:        true,
		},
		{
			name:        "keyword localhost DSN",
			databaseURL: "user=adlts password=adlts host=localhost port=5432 dbname=adlts_dev",
			want:        true,
		},
		{
			name:        "render internal URL",
			databaseURL: "postgresql://user:password@dpg-example-a:5432/adlts",
			want:        false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := databaseURLUsesLocalhost(tc.databaseURL); got != tc.want {
				t.Fatalf("databaseURLUsesLocalhost() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateRuntimeConfigRejectsLocalhostOnRender(t *testing.T) {
	t.Setenv("RENDER", "true")

	err := validateRuntimeConfig(config.Config{
		DatabaseURL: "postgres://adlts:adlts@localhost:5432/adlts_dev?sslmode=disable",
	})
	if err == nil {
		t.Fatal("expected localhost DATABASE_URL to be rejected on Render")
	}
}

func TestValidateRuntimeConfigAllowsLocalhostOffRender(t *testing.T) {
	t.Setenv("RENDER", "")

	err := validateRuntimeConfig(config.Config{
		DatabaseURL: "postgres://adlts:adlts@localhost:5432/adlts_dev?sslmode=disable",
	})
	if err != nil {
		t.Fatalf("expected localhost DATABASE_URL to be allowed off Render, got %v", err)
	}
}
