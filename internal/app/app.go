package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"adlts/internal/identity"
	"adlts/internal/platform/config"
	"adlts/internal/platform/mailer"
	"adlts/internal/platform/media"
	minioclient "adlts/internal/platform/minio"
	"adlts/internal/platform/security"
	testing_ "adlts/internal/testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Build(cfg config.Config, db *pgxpool.Pool, logger *slog.Logger) *http.Server {
	// Configure media engine from environment
	media.UploadsDir = cfg.UploadsDir
	media.MaxFileSize = cfg.MediaMaxSizeMB * 1024 * 1024

	tokens := security.NewManager(cfg.JWTSecret)
	mail := mailer.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPFrom, cfg.SMTPFromName)

	identitySvc := identity.NewService(identity.NewRepository(db), tokens, mail)

	// Seed root super-admin gracefully
	if err := identitySvc.SeedSuperAdmin(context.Background(), cfg.SuperAdminName, cfg.SuperAdminEmail, cfg.SuperAdminPassword); err != nil {
		logger.Error("failed to seed super admin", "error", err)
	}

	identityHandler := identity.NewHandler(identitySvc, tokens)
	appealRepo := appeal.NewRepository(db)
	appealSvc := appeal.NewService(appealRepo, db)
	appealHandler := appeal.NewHandler(appealSvc)
	// Recording (playback-only) handler
	recordingRepo := recording.NewRepository(db)
	minioClient, _ := recording.NewMinioClientFromEnv()
	recordingSvc := recording.NewService(recordingRepo, minioClient)
	recordingHandler := recording.NewHandler(recordingSvc)
	// ── Testing Core ──────────────────────────────────────────────────────────
	minioClient, err := minioclient.New(
		cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioBucket, cfg.MinioUseSSL)
	if err != nil {
		logger.Error("failed to connect to MinIO", "error", err)
	} else if err := minioClient.EnsureBucket(context.Background()); err != nil {
		logger.Error("failed to ensure MinIO bucket", "error", err)
	}

	testingHandler := testing_.NewHandler(
		testing_.NewRepository(db),
		minioClient,
		cfg,
		tokens,
	)

	// Wire the orchestrator into the handler (enables admin start/abort)
	testingHandler.SetOrchestrator(
		testing_.NewOrchestrator(
			testing_.NewRepository(db),
			testing_.NewLaneDetectorClient(cfg.LaneDetectorURL),
			testing_.NewScoreManeuverClient(cfg.LaneDetectorURL), // scoreClient
			testing_.NewNarrativeGenerator(cfg.GeminiAPIKey, cfg.GeminiModel),
			minioClient,
			testing_.NewIoTClient(""), // iotClient - stream URL should ideally be updated per test/device, but we pass empty string for now to avoid nil, it seems it gets overridden or handled? Let's check scoring.go orchestrator
			minioClient,               // minioFull
		),
	)

	// ── Testing Expiry Cron ───────────────────────────────────────────────────
	testingExpiry := testing_.NewExpiryWorker(testing_.NewRepository(db), 0, 0)
	go testingExpiry.Start(context.Background())

	// ── HTTP server ────────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(requestLogger(logger))
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(corsMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Static media file serving — public, UUID-obscured paths
	r.Get("/uploads/*", media.ServeHandler())

	r.Route("/api/v1", func(api chi.Router) {
		identityHandler.Mount(api)
		testingHandler.Mount(api, r, tokens)
		appealHandler.Mount(api)
		recordingHandler.Mount(api)
	})

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Internal-Token,X-Device-Secret")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}
