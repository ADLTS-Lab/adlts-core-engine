package main

import (
	"log/slog"
	"os"

	"adlts/internal/app"
	"adlts/internal/platform/config"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"
	"adlts/internal/platform/store"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	appStore := store.New()
	if cfg.SeedDemoData {
		store.SeedDemoData(appStore)
	}
	tokens := security.NewManager(cfg.JWTSecret)
	deps := runtime.Dependencies{
		Config: cfg,
		Store:  appStore,
		Tokens: tokens,
		Logger: logger,
	}

	handler := app.New(deps)
	logger.Info("ADLTS API starting", "port", cfg.Port)
	if err := handler.Run(":" + cfg.Port); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
