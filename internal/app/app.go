package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"adlts/internal/booking"
	"adlts/internal/identity"
	"adlts/internal/platform/config"
	"adlts/internal/platform/mailer"
	"adlts/internal/platform/media"
	"adlts/internal/platform/security"

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

	// TODO: bookingHandler  := booking.NewHandler(booking.NewService(booking.NewRepository(db)), tokens)
	// TODO: sessionHandler  := session.NewHandler(...)
	// TODO: iotHandler      := iot.NewHandler(...)
	// TODO: scoringHandler  := scoring.NewHandler(...)
	// TODO: appealHandler   := appeal.NewHandler(...)
	// TODO: reportingHandler := reporting.NewHandler(...)

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
		bookingHandler.Mount(api)
		// sessionHandler.Mount(api)
		// iotHandler.Mount(api)
		// scoringHandler.Mount(api)
		// appealHandler.Mount(api)
		// reportingHandler.Mount(api)
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
