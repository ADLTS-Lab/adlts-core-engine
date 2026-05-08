package admin

import (
	"adlts/internal/platform/domain"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

func RegisterAdminRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	r.Post("/devices/heartbeat", h.handleDeviceHeartbeat)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleAdmin, domain.RoleAuthority)).Get("/analytics/map", h.handleHeatmap)
}
