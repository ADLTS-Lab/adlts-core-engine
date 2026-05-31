package testing

import (
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

// Mount registers all testing module routes.
func (h *Handler) Mount(api chi.Router, root chi.Router, tokens *security.Manager) {
	// ── Public (no auth) ──────────────────────────────────────────────────────
	api.Get("/test-level-types", h.listTestLevelTypes)
	api.Get("/guidelines", h.listGuidelines)
	api.Get("/guidelines/faq", h.listGuidelinesFAQ)

	// ── Internal service token ────────────────────────────────────────────────
	root.Group(func(r chi.Router) {
		r.Use(security.RequireInternalToken(h.internalAPIKey))
		r.Post("/internal/tests", h.createTestInternal)
		r.Patch("/internal/tests/by-booking/{bookingID}", h.rescheduleTestByBooking)
		r.Delete("/internal/tests/by-booking/{bookingID}", h.cancelTestByBooking)
		// Result webhook from ADLTS Python service
		r.Post("/internal/tests/{id}/result", h.resultWebhook)
	})

	// ── Admin / SuperAdmin: static maneuver types ─────────────────────────────
	api.With(
		security.Authenticate(tokens),
		security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
	).Get("/maneuver-types", h.listManeuverTypes)

	// ── Admin / SuperAdmin: devices ────────────────────────────────────────────
	api.Route("/devices", func(r chi.Router) {
		r.Use(security.Authenticate(tokens))
		r.Use(security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin))
		r.Get("/", h.listDevices)
		r.Post("/", h.registerDevice)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.getDevice)
			r.Patch("/", h.updateDevice)
			r.Patch("/status", h.updateDeviceStatus)
			r.Delete("/", h.deleteDevice)
			r.Get("/qr-code", h.downloadDeviceQR)
		})
	})

	// ── Admin / SuperAdmin: test plans ────────────────────────────────────────
	api.Route("/test-plans", func(r chi.Router) {
		r.Use(security.Authenticate(tokens))
		r.Use(security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin))
		r.Get("/", h.listTestPlans)
		r.Post("/", h.createTestPlan)
		r.Route("/{planID}", func(r chi.Router) {
			r.Get("/", h.getTestPlan)
			r.Patch("/", h.updateTestPlan)
			r.Delete("/", h.deleteTestPlan)
			r.Post("/publish", h.publishTestPlan)
			r.Post("/retire", h.retireTestPlan)
			r.Route("/maneuvers", func(r chi.Router) {
				r.Get("/", h.listManeuverConfigs)
				r.Post("/", h.createManeuverConfig)
				r.Post("/reorder", h.reorderManeuverConfigs)
				r.Route("/{maneuverID}", func(r chi.Router) {
					r.Put("/mask", h.uploadReferenceMask)
					r.Get("/mask", h.downloadReferenceMask)
					r.Get("/qr-code", h.downloadManeuverQR)
					r.Patch("/", h.updateManeuverConfig)
					r.Delete("/", h.deleteManeuverConfig)
					r.Get("/qr", h.downloadManeuverQRZip)
				})
			})
		})
	})

	// ── Admin / SuperAdmin: level mappings ────────────────────────────────────
	api.Route("/test-level-mappings", func(r chi.Router) {
		r.Use(security.Authenticate(tokens))
		r.Use(security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin))
		r.Get("/", h.listLevelMappings)
		r.Put("/", h.upsertLevelMapping)
	})

	// ── Tests ─────────────────────────────────────────────────────────────────
	api.Route("/tests", func(r chi.Router) {
		// Admin CRUD
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)).Get("/", h.listTests)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)).Post("/", h.createTestAdmin)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)).Get("/{id}", h.getTest)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)).Patch("/{id}", h.updateTestAdmin)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)).Delete("/{id}", h.deleteTestAdmin)

		// Status + Start + Abort
		r.With(security.Authenticate(tokens)).Get("/{id}/status", h.getTestStatus)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin, security.EntityCandidate)).Post("/{id}/start", h.startTest)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)).Post("/{id}/abort", h.adminAbortTest)

		// Monitoring
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)).Get("/{id}/monitor/status", h.monitorStatus)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)).Get("/{id}/monitor/live", h.monitorLive)

		// Candidate flow
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityCandidate)).Get("/my/pending", h.getMyPendingTest)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityCandidate)).Post("/device-checkin", h.deviceCheckin)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityCandidate)).Post("/{id}/guidelines/acknowledge", h.acknowledgeGuidelines)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityCandidate)).Get("/{id}/live", h.getCandidateLive)

		// Results
		r.With(security.Authenticate(tokens)).Get("/{id}/result", h.getTestResult)

		// Sessions & Events
		r.With(security.Authenticate(tokens)).Get("/{id}/sessions", h.listSessions)
		r.With(security.Authenticate(tokens)).Get("/{id}/sessions/{sessionID}", h.getSession)
		r.With(security.Authenticate(tokens)).Get("/{id}/sessions/{sessionID}/events", h.listSessionEvents)

		// Frame analyses (admin only)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)).Get("/{id}/sessions/{sessionID}/frames", h.getFrameAnalyses)

		// Recordings
		r.With(security.Authenticate(tokens)).Get("/{id}/recording", h.getTestRecording)
		r.With(security.Authenticate(tokens), security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)).Get("/{id}/sessions/{sessionID}/recording", h.getSessionRecording)
	})
}