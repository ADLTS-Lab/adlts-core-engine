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

	TestingCoreBaseURL string
	TestingCoreToken string
	IdentityBaseURL string
	IdentityToken string
	AnthropicAPIKey string
	AnthropicModel string
	ReportOutputDir string

	SuperAdminName     string
	SuperAdminEmail    string
	SuperAdminPassword string

	UploadsDir     string
	MediaMaxSizeMB int64

	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	SMTPFromName string

	// ── Testing Core ─────────────────────────────────────────────────────────
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	LaneDetectorURL string

	GeminiAPIKey string
	GeminiModel  string

	BookingWindowHours         int // ±window for device checkin vs scheduled_at
	InstituteResultDelayHours  int // delay before institute can see result
}

func Load() Config {
	return Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		JWTSecret:      getEnv("JWT_SECRET", ""),
		InternalAPIKey: getEnv("INTERNAL_API_KEY", ""),
		TestingCoreBaseURL: getEnv("TESTING_CORE_BASE_URL", "https://api.adlts.et/api/v1"),
		TestingCoreToken: getEnv("TESTING_CORE_TOKEN", ""),
		IdentityBaseURL: getEnv("IDENTITY_BASE_URL", "https://api.adlts.et/api/v1"),
		IdentityToken: getEnv("IDENTITY_TOKEN", ""),
		AnthropicAPIKey: getEnv("ANTHROPIC_API_KEY", ""),
		AnthropicModel: getEnv("ANTHROPIC_MODEL", "claude-3-5-sonnet-latest"),
		ReportOutputDir: getEnv("REPORT_OUTPUT_DIR", "../generated-reports"),

		SuperAdminName:     getEnv("SUPER_ADMIN_NAME", "Root Admin"),
		SuperAdminEmail:    getEnv("SUPER_ADMIN_EMAIL", "root@adlts.et"),
		SuperAdminPassword: getEnv("SUPER_ADMIN_PASSWORD", "SuperSecure123!"),

		UploadsDir:     getEnv("UPLOADS_DIR", "../uploads"),
		MediaMaxSizeMB: getEnvInt64("MEDIA_MAX_SIZE_MB", 5),

		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", ""),
		SMTPFromName: getEnv("SMTP_FROM_NAME", "ADLTS"),

		// Testing Core
		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:    getEnv("MINIO_BUCKET", "adlts"),
		MinioUseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",

		LaneDetectorURL: getEnv("LANE_DETECTOR_URL", "http://localhost:8001"),

		GeminiAPIKey: getEnv("GEMINI_API_KEY", ""),
		GeminiModel:  getEnv("GEMINI_MODEL", "gemini-1.5-flash"),

		BookingWindowHours:        getEnvInt("BOOKING_WINDOW_HOURS", 2),
		InstituteResultDelayHours: getEnvInt("INSTITUTE_RESULT_DELAY_HOURS", 0),
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

func getEnvInt(key string, fallback int) int {
	return int(getEnvInt64(key, int64(fallback)))
}
