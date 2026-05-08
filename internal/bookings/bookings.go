package bookings

import (
	"adlts/internal/platform/domain"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

func RegisterBookingRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW, security.RequireRoles(domain.RoleCandidate)).Post("/", h.handleCreateBooking)
	r.With(authMW, security.RequireRoles(domain.RoleInstituteAdmin)).Get("/verify", h.handleListPendingBookings)
	r.With(authMW, security.RequireRoles(domain.RoleInstituteAdmin)).Patch("/{id}/verify", h.handleVerifyBooking)
	r.With(authMW, security.RequireRoles(domain.RoleCandidate)).Get("/available-slots", h.handleAvailableSlots)
}
