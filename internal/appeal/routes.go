package appeal

import (
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

// Mount registers appeal module routes with role-based auth enforcement
func (h *Handler) Mount(api chi.Router, tokens *security.Manager) {
	api.Group(func(r chi.Router) {
		// All appeal routes require valid JWT tokens
		r.Use(security.Authenticate(tokens))

		// Candidate can create appeals
		r.With(security.RequireEntities(security.EntityCandidate)).
			Post("/appeals", h.createAppeal)

		// Experts and Admins can resolve appeals
		r.With(security.RequireEntities(security.EntityExpert, security.EntityAdmin)).
			Patch("/appeals/{id}/resolve", h.resolveAppeal)

		// Experts/Admin/SuperAdmin can list appeals (e.g. pending queue)
		r.With(security.RequireEntities(security.EntityExpert, security.EntityAdmin, security.EntitySuperAdmin)).
			Get("/appeals", h.listAppeals)

		// Any authenticated user can get appeal by ID
		r.Get("/appeals/{id}", h.getAppeal)
	})
}
