package booking

import (
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

// Mount registers all booking routes under the provided router.
func (h *Handler) Mount(r chi.Router) {
	auth := security.Authenticate(h.tokens)
	candidateOnly := security.RequireEntities(security.EntityCandidate)
	adminOrSuper := security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)
	instituteOrAdmin := security.RequireEntities(security.EntityInstitute, security.EntityAdmin, security.EntitySuperAdmin)

	// Chapa webhook callback (public)
	r.Get("/bookings/{id}/payments/callback", h.handleChapaCallback)
	r.Post("/bookings/{id}/payments/callback", h.handleChapaWebhook)
	r.Get("/bookings/{id}/payments/callback/", h.handleChapaCallback)
	r.Post("/bookings/{id}/payments/callback/", h.handleChapaWebhook)

	r.Route("/bookings", func(r chi.Router) {
		r.Use(auth)

		r.With(candidateOnly).Post("/", h.createBooking)
		r.Get("/", h.listBookings)

		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.getBooking)
			r.Delete("/", h.deleteBooking)
			r.With(instituteOrAdmin).Patch("/verify", h.verifyBooking)
			r.With(adminOrSuper).Patch("/schedule", h.scheduleBooking)
			r.Patch("/reschedule", h.rescheduleBooking)

			r.With(candidateOnly).Post("/payments", h.initiatePayment)
			r.With(candidateOnly).Post("/payments/", h.initiatePayment)
			r.With(candidateOnly).Post("/payments/retry", h.retryPayment)
			r.With(candidateOnly).Post("/payments/retry/", h.retryPayment)
			r.Get("/payments", h.listPayments)
			r.Get("/payments/", h.listPayments)
		})
	})

	r.Route("/slots", func(r chi.Router) {
		r.Use(auth)

		r.With(adminOrSuper).Post("/", h.createSlot)
		r.Get("/", h.listSlots)
		r.Get("/{id}", h.getSlot)
	})
}
