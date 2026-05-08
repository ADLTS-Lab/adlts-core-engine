package app

import (
	"net/http"
	"time"

	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"

	"adlts/internal/admin"
	"adlts/internal/analytics"
	"adlts/internal/appeals"
	"adlts/internal/auth"
	"adlts/internal/authority"
	"adlts/internal/bookings"
	"adlts/internal/devices"
	"adlts/internal/exams"
	"adlts/internal/institutes"
	"adlts/internal/internalapi"
	"adlts/internal/iot"
	"adlts/internal/schedules"
	"adlts/internal/scoring"
	"adlts/internal/users"
	"adlts/internal/ws"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type App struct {
	router *chi.Mux
	deps   runtime.Dependencies
}

func New(deps runtime.Dependencies) *App {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(30 * time.Second))
	router.Use(middleware.Logger)

	router.Route("/auth", func(r chi.Router) { auth.RegisterAuthRoutes(r, deps) })
	router.Route("/users", func(r chi.Router) { users.RegisterUserRoutes(r, deps) })
	router.Route("/institutes", func(r chi.Router) { institutes.RegisterInstituteRoutes(r, deps) })
	router.Route("/bookings", func(r chi.Router) { bookings.RegisterBookingRoutes(r, deps) })
	router.Route("/schedules", func(r chi.Router) { schedules.RegisterScheduleRoutes(r, deps) })
	router.Route("/devices", func(r chi.Router) { devices.RegisterDeviceRoutes(r, deps) })
	router.Route("/admin", func(r chi.Router) { admin.RegisterAdminRoutes(r, deps) })
	router.Route("/internal", func(r chi.Router) { internalapi.RegisterInternalRoutes(r, deps) })
	router.Route("/scoring", func(r chi.Router) { scoring.RegisterScoringRoutes(r, deps) })
	router.Route("/iot", func(r chi.Router) { iot.RegisterIoTRoutes(r, deps) })
	router.Route("/exams", func(r chi.Router) { exams.RegisterExamRoutes(r, deps) })
	router.Route("/appeals", func(r chi.Router) { appeals.RegisterAppealRoutes(r, deps) })
	router.Route("/analytics", func(r chi.Router) { analytics.RegisterAnalyticsRoutes(r, deps) })
	router.Route("/authority", func(r chi.Router) { authority.RegisterAuthorityRoutes(r, deps) })
	router.Route("/ws", func(r chi.Router) { ws.RegisterWsRoutes(r, deps) })

	return &App{router: router, deps: deps}
}

func (a *App) Run(addr string) error {
	return http.ListenAndServe(addr, a.router)
}

func AuthMiddleware(deps runtime.Dependencies) func(http.Handler) http.Handler {
	return security.Authenticate(deps.Tokens, deps.Store)
}
