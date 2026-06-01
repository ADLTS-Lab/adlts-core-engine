package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"adlts/internal/appeal"
	"adlts/internal/booking"
	"adlts/internal/identity"
	"adlts/internal/platform/config"
	"adlts/internal/platform/mailer"
	"adlts/internal/platform/media"
	minioclient "adlts/internal/platform/minio"
	"adlts/internal/platform/security"
	"adlts/internal/recording"
	"adlts/internal/reporting"
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
	identity.BaseURL = cfg.BaseURL
	identitySvc := identity.NewService(identity.NewRepository(db), tokens, mail)

	// Seed root super-admin gracefully
	if err := identitySvc.SeedSuperAdmin(context.Background(), cfg.SuperAdminName, cfg.SuperAdminEmail, cfg.SuperAdminPassword); err != nil {
		logger.Error("failed to seed super admin", "error", err)
	}

	identityHandler := identity.NewHandler(identitySvc, tokens)
	bookingSvc := booking.NewService(
		booking.NewRepository(db),
		booking.NewChapaProvider(cfg.ChapaSecretKey, cfg.ChapaWebhookSecret, cfg.ChapaBaseURL),
		mail,
		cfg.BaseURL,
		cfg.FrontendBaseURL,
	)
	bookingHandler := booking.NewHandler(bookingSvc, tokens)

	// ── Appeal module ─────────────────────────────────────────────────────────
	appealRepo := appeal.NewRepository(db)
	appealSvc := appeal.NewService(appealRepo, db)
	appealHandler := appeal.NewHandler(appealSvc)

	// ── Recording module (playback-only) ──────────────────────────────────────
	recordingRepo := recording.NewRepository(db)
	// Recording module uses its own lightweight MinIO client read from env.
	// If env vars are missing it returns ErrMinioNotConfigured and we skip recording.
	recordingMinioClient, recordingMinioErr := recording.NewMinioClientFromEnv()
	if recordingMinioErr != nil {
		logger.Error("recording MinIO not configured — playback disabled", "error", recordingMinioErr)
	}
	recordingSvc := recording.NewService(recordingRepo, recordingMinioClient)
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
			testing_.NewScoreManeuverClient(cfg.LaneDetectorURL),
			testing_.NewNarrativeGenerator(cfg.GeminiAPIKey, cfg.GeminiModel),
			minioClient,
			testing_.NewIoTClient(""),
			minioClient,
		),
	)

	// ── Testing Expiry Cron ───────────────────────────────────────────────────
	testingExpiry := testing_.NewExpiryWorker(testing_.NewRepository(db), 0, 0)
	go testingExpiry.Start(context.Background())

	reportRenderer, err := reporting.NewRenderer()
	if err != nil {
		logger.Error("failed to load report template", "error", err)
	}
	reportingSvc := reporting.NewService(
		reporting.NewHTTPTestingCoreClient(cfg.TestingCoreBaseURL, cfg.TestingCoreToken),
		reporting.NewHTTPIdentityClient(cfg.IdentityBaseURL, cfg.IdentityToken),
		reporting.NewHTTPGeminiClient(cfg.GeminiAPIKey, cfg.GeminiModel),
		reportRenderer,
		cfg.ReportOutputDir,
		logger,
	)
	reportingHandler := reporting.NewHandler(reportingSvc)

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
		if db == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"degraded","db":"error"}`))
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"degraded","db":"error"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","db":"ok"}`))
	})

	// Static media file serving — public, UUID-obscured paths
	r.Get("/uploads/*", media.ServeHandler())

	r.Route("/api/v1", func(api chi.Router) {
		identityHandler.Mount(api)
		bookingHandler.Mount(api)

		// Testing Core: public + internal + authenticated routes (3-param Mount)
		testingHandler.Mount(api, r, tokens)

		// Appeal: routes, auth, and roles managed inside the module's Mount
		appealHandler.Mount(api, tokens)

		// Recording playback: JWT required
		api.Group(func(sub chi.Router) {
			sub.Use(security.Authenticate(tokens))
			recordingHandler.Mount(sub)
		})

		// Reports: Admin/SuperAdmin/Institute/Expert only
		api.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin, security.EntityInstitute, security.EntityExpert),
		).Route("/reports", func(r chi.Router) {
			reportingHandler.Mount(r)
		})
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
