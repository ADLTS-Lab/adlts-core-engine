package config

import "os"

type Config struct {
	Port           string
	JWTSecret      string
	InternalAPIKey string
	SeedDemoData   bool
}

func Load() Config {
	return Config{
		Port:           getEnv("PORT", "8080"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
		InternalAPIKey: getEnv("INTERNAL_API_KEY", "change-me-in-production-internal"),
		SeedDemoData:   getEnv("SEED_DEMO_DATA", "true") == "true",
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
