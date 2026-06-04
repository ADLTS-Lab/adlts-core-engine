package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"adlts/internal/app"
	"adlts/internal/platform/config"
	"adlts/internal/platform/db"

	"github.com/joho/godotenv"
	"github.com/lmittmann/tint"
)

func main() {
	loadLocalEnv()

	cfg := config.Load()
	if err := validateRuntimeConfig(cfg); err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.Kitchen,
	}))
	srv := app.Build(cfg, pool, logger)

	logger.Info("ADLTS API starting", "port", cfg.Port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Error("server stopped", "err", err)
		}
	}()
	<-quit

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutCancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("shutdown error", "err", err)
	}
	logger.Info("goodbye")
}

func loadLocalEnv() {
	if os.Getenv("RENDER") == "true" {
		return
	}
	_ = godotenv.Load()
}

func validateRuntimeConfig(cfg config.Config) error {
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if os.Getenv("RENDER") == "true" && databaseURLUsesLocalhost(cfg.DatabaseURL) {
		return fmt.Errorf("DATABASE_URL points to localhost on Render; set DATABASE_URL to the Render Postgres internal database URL instead of the local development value")
	}
	return nil
}

func databaseURLUsesLocalhost(databaseURL string) bool {
	if parsed, err := url.Parse(databaseURL); err == nil && parsed.Hostname() != "" {
		return isLocalhost(parsed.Hostname())
	}

	// pgx also supports keyword/value DSNs such as "host=localhost dbname=...".
	for _, part := range strings.Fields(databaseURL) {
		key, value, ok := strings.Cut(part, "=")
		if !ok || strings.TrimSpace(key) != "host" {
			continue
		}
		return isLocalhost(strings.Trim(strings.TrimSpace(value), `"'`))
	}
	return false
}

func isLocalhost(host string) bool {
	switch strings.ToLower(strings.Trim(host, "[]")) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
