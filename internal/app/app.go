package app

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
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
	mail := mailer.NewSMTP(mailer.Config{
		Host:       cfg.SMTPHost,
		Port:       cfg.SMTPPort,
		User:       cfg.SMTPUser,
		Password:   cfg.SMTPPassword,
		From:       cfg.SMTPFrom,
		FromName:   cfg.SMTPFromName,
		Encryption: mailer.Encryption(cfg.SMTPEncryption),
		Timeout:    time.Duration(cfg.SMTPTimeoutSeconds) * time.Second,
	})
	identity.BaseURL = cfg.FrontendBaseURL
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
		cfg.InternalAPIKey,
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
	testingHandler.SetHealthChecker(testing_.NewHealthChecker(testing_.NewRepository(db)))
	testingHandler.SetNarrativeGenerator(testing_.NewNarrativeGenerator(cfg.GeminiAPIKey, cfg.GeminiModel))

	// Pass the ADLTS service URL into the handler for test start / webhook calls
	testingHandler.SetAdltsServiceURL(cfg.AdltsServiceURL)

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
	r.Use(corsMiddleware(cfg.CORSAllowedOrigins))

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

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		o := strings.TrimSpace(origin)
		if o != "" {
			allowed[o] = struct{}{}
		}
	}

	const allowedMethods = "GET,POST,PATCH,PUT,DELETE,OPTIONS"
	const allowedHeaders = "Authorization,Content-Type,X-Internal-Token,X-Device-Secret"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			originAllowed := false
			if origin != "" {
				w.Header().Add("Vary", "Origin")
				_, originAllowed = allowed[origin]
				if originAllowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
					w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
				}
			}

			if r.Method == http.MethodOptions {
				if origin != "" && !originAllowed {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				if origin == "" {
					w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
					w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}
