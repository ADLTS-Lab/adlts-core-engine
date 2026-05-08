package analytics

import (
	"adlts/internal/platform/domain"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

func RegisterAnalyticsRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority, domain.RoleAdmin)).Get("/global", h.handleGlobalAnalytics)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority, domain.RoleAdmin)).Get("/map", h.handleHeatmap)
}
