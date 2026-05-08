package users

import (
	"adlts/internal/platform/domain"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

// RegisterUserRoutes registers user-related routes for the users package.
func RegisterUserRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW).Get("/me", h.handleMe)
	r.With(authMW, security.RequireRoles(domain.RoleAuthority, domain.RoleAdmin)).Patch("/{id}", h.handlePatchUser)
}
