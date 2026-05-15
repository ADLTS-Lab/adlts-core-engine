package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port string
	DatabaseURL string 
	JWTSecret string 
	InternalAPIKey string

	SuperAdminName     string
	SuperAdminEmail    string
	SuperAdminPassword string

	UploadsDir       string
	MediaMaxSizeMB   int64

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

		SuperAdminName:     getEnv("SUPER_ADMIN_NAME", "Root Admin"),
		SuperAdminEmail:    getEnv("SUPER_ADMIN_EMAIL", "root@adlts.et"),
		SuperAdminPassword: getEnv("SUPER_ADMIN_PASSWORD", "SuperSecure123!"),

		UploadsDir:     getEnv("UPLOADS_DIR", "../uploads"),
		MediaMaxSizeMB: getEnvInt64("MEDIA_MAX_SIZE_MB", 5),

		SMTPHost:       getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:       getEnv("SMTP_PORT", "587"),
		SMTPUser:       getEnv("SMTP_USER", ""),
		SMTPPassword:   getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:       getEnv("SMTP_FROM", ""),
		SMTPFromName:   getEnv("SMTP_FROM_NAME", "ADLTS"),
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
