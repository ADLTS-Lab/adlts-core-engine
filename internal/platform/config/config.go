package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	InternalAPIKey string
	BaseURL        string

	TestingCoreBaseURL string
	TestingCoreToken   string
	IdentityBaseURL    string
	IdentityToken      string
	AnthropicAPIKey    string
	AnthropicModel     string
	ReportOutputDir    string

	SuperAdminName     string
	SuperAdminEmail    string
	SuperAdminPassword string

	UploadsDir         string
	MediaMaxSizeMB     int64
	FrontendBaseURL    string
	ChapaSecretKey     string
	ChapaWebhookSecret string
	ChapaBaseURL       string

	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	SMTPFromName string
}

func Load() Config {
	return Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		InternalAPIKey: getEnv("INTERNAL_API_KEY", ""),
		BaseURL:        getEnv("APP_URL", "http://localhost:8080"),

		TestingCoreBaseURL: getEnv("TESTING_CORE_BASE_URL", "https://api.adlts.et/api/v1"),
		TestingCoreToken:   getEnv("TESTING_CORE_TOKEN", ""),
		IdentityBaseURL:    getEnv("IDENTITY_BASE_URL", "https://api.adlts.et/api/v1"),
		IdentityToken:      getEnv("IDENTITY_TOKEN", ""),
		AnthropicAPIKey:    getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicModel:     getEnv("ANTHROPIC_MODEL", "claude-3-5-sonnet-latest"),
		ReportOutputDir:    getEnv("REPORT_OUTPUT_DIR", "../generated-reports"),

		SuperAdminName:     getEnv("SUPER_ADMIN_NAME", "Root Admin"),
		SuperAdminEmail:    getEnv("SUPER_ADMIN_EMAIL", "root@adlts.et"),
		SuperAdminPassword: getEnv("SUPER_ADMIN_PASSWORD", "SuperSecure123!"),

		UploadsDir:         getEnv("UPLOADS_DIR", "../uploads"),
		MediaMaxSizeMB:     getEnvInt64("MEDIA_MAX_SIZE_MB", 5),
		FrontendBaseURL:    getEnv("FRONTEND_BASE_URL", "http://localhost:3000"),
		ChapaSecretKey:     getEnv("CHAPA_SECRET_KEY", ""),
		ChapaWebhookSecret: getEnv("CHAPA_WEBHOOK_SECRET", ""),
		ChapaBaseURL:       getEnv("CHAPA_BASE_URL", "https://api.chapa.co/v1"),

		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),
		SMTPFromName: getEnv("SMTP_FROM_NAME", "ADLTS"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}
