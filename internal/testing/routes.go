package testing

import (
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

// Mount registers all testing module routes.
// api  = /api/v1 subrouter
// root = the root chi router (for /internal/* routes)
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
					// Legacy endpoints (migration-002 schema) — kept for backward compat
					r.Put("/mask", h.uploadReferenceMask)
					r.Get("/mask", h.downloadReferenceMask)
					r.Get("/qr-code", h.downloadManeuverQR)
					// New endpoints (migration-003 schema)
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
		// ── Admin CRUD ────────────────────────────────────────────────────────
		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
		).Get("/", h.listTests)

		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
		).Post("/", h.createTestAdmin)

		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
		).Get("/{id}", h.getTest)

		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
		).Patch("/{id}", h.updateTestAdmin)

		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
		).Delete("/{id}", h.deleteTestAdmin)

		// ── Any authenticated: status ─────────────────────────────────────────
		r.With(security.Authenticate(tokens)).Get("/{id}/status", h.getTestStatus)

		// ── Admin-only: monitoring + abort ────────────────────────────────────
		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
		).Get("/{id}/monitor/status", h.monitorStatus)

		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
		).Get("/{id}/monitor/live", h.monitorLive)

		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
		).Post("/{id}/abort", h.adminAbortTest)

		// ── Candidate-only actions ────────────────────────────────────────────
		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityCandidate),
		).Get("/my/pending", h.getMyPendingTest)
		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityCandidate),
		).Get("/my", h.getMyTests)
		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityCandidate),
		).Get("/my/stats", h.getMyTestStats)

		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityCandidate),
		).Post("/device-checkin", h.deviceCheckin)

		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityCandidate),
		).Post("/{id}/guidelines/acknowledge", h.acknowledgeGuidelines)

		// ── Any authenticated: results ────────────────────────────────────────
		r.With(security.Authenticate(tokens)).Get("/{id}/result", h.getTestResult)

		// ── Any authenticated: sessions ───────────────────────────────────────
		r.With(security.Authenticate(tokens)).Get("/{id}/sessions", h.listSessions)
		r.With(security.Authenticate(tokens)).Get("/{id}/sessions/{sessionID}", h.getSession)
		r.With(security.Authenticate(tokens)).Get("/{id}/sessions/{sessionID}/events", h.listSessionEvents)

		// ── Admin-only: frame analyses (raw sensor data) ──────────────────────
		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
		).Get("/{id}/sessions/{sessionID}/frames", h.getFrameAnalyses)

		// ── Any authenticated: recordings (presigned MinIO URLs) ──────────────
		r.With(security.Authenticate(tokens)).Get("/{id}/recording", h.getTestRecording)

		// ── Admin-only: per-session recording ─────────────────────────────────
		r.With(
			security.Authenticate(tokens),
			security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin),
		).Get("/{id}/sessions/{sessionID}/recording", h.getSessionRecording)
	})
}
