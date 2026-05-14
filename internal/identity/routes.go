package identity

import (
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

// Mount registers all user-management routes under the provided router.
// Called once by app.go during wiring.
func (h *Handler) Mount(r chi.Router) {
	auth := security.Authenticate(h.tokens)
	superOnly := security.RequireEntities(security.EntitySuperAdmin)
	adminOrSuper := security.RequireEntities(security.EntityAdmin, security.EntitySuperAdmin)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/candidates/register", h.registerCandidate)
		r.Post("/candidates/verify-otp", h.verifyOTP)
		r.Post("/candidates/resend-otp", h.resendOTP)
		r.Post("/invitations/accept", h.acceptInvitation)
		r.Post("/login", h.login)
		r.With(auth).Post("/logout", h.logout)
		r.With(auth).Post("/token/refresh", h.refreshToken)
		r.Post("/password/forgot", h.forgotPassword)
		r.Post("/password/reset", h.resetPassword)
		r.With(auth).Patch("/password/change", h.changePassword)
	})

	r.Route("/candidates", func(r chi.Router) {
		r.Use(auth)
		r.With(adminOrSuper).Get("/", h.listCandidates)
		r.With(security.RequireEntities(security.EntityCandidate)).Get("/me", h.candidateMe)
		r.With(security.RequireEntities(security.EntityCandidate)).Patch("/me", h.updateCandidateMe)
		r.With(security.RequireEntities(security.EntityCandidate)).Patch("/me/photo", h.uploadCandidatePhotoMe)
		r.With(security.RequireEntities(security.EntityCandidate)).Delete("/me", h.softDeleteCandidateMe)
		r.With(adminOrSuper).Get("/{id}", h.getCandidate)
		r.With(superOnly).Patch("/{id}", h.updateCandidateAdmin)
		r.With(adminOrSuper).Patch("/{id}/status", h.updateCandidateStatus)
		r.With(adminOrSuper).Patch("/{id}/photo", h.uploadCandidatePhotoAdmin)
		r.With(superOnly).Delete("/{id}", h.deleteCandidate)
	})

	r.Route("/experts", func(r chi.Router) {
		r.Use(auth)
		r.With(superOnly).Get("/", h.listExperts)
		r.With(security.RequireEntities(security.EntityExpert)).Get("/me", h.expertMe)
		r.With(security.RequireEntities(security.EntityExpert)).Patch("/me", h.updateExpertMe)
		r.With(security.RequireEntities(security.EntityExpert)).Patch("/me/photo", h.uploadExpertPhotoMe)
		r.With(superOnly).Get("/{id}", h.getExpert)
		r.With(superOnly).Patch("/{id}", h.updateExpertAdmin)
		r.With(superOnly).Patch("/{id}/status", h.updateExpertStatus)
		r.With(superOnly).Patch("/{id}/photo", h.uploadExpertPhotoAdmin)
		r.With(superOnly).Delete("/{id}", h.deleteExpert)
	})

	r.Route("/institutes", func(r chi.Router) {
		r.Use(auth)
		r.With(adminOrSuper).Get("/", h.listInstitutes)
		r.With(security.RequireEntities(security.EntityInstitute)).Get("/me", h.instituteMe)
		r.With(security.RequireEntities(security.EntityInstitute)).Patch("/me", h.updateInstituteMe)
		r.With(security.RequireEntities(security.EntityInstitute)).Patch("/me/logo", h.uploadInstituteLogoMe)
		r.With(security.RequireEntities(security.EntityInstitute)).Delete("/me", h.softDeleteInstituteMe)
		r.With(adminOrSuper).Get("/{id}", h.getInstitute)
		r.With(superOnly).Patch("/{id}", h.updateInstituteAdmin)
		r.With(adminOrSuper).Patch("/{id}/status", h.updateInstituteStatus)
		r.With(superOnly).Patch("/{id}/logo", h.uploadInstituteLogoAdmin)
		r.With(superOnly).Delete("/{id}", h.deleteInstitute)
	})

	r.Route("/transport-authorities", func(r chi.Router) {
		r.Use(auth)
		r.With(superOnly).Get("/", h.listAuthorities)
		r.With(security.RequireEntities(security.EntityTransportAuthority)).Get("/me", h.authorityMe)
		r.With(security.RequireEntities(security.EntityTransportAuthority)).Patch("/me", h.updateAuthorityMe)
		r.With(security.RequireEntities(security.EntityTransportAuthority)).Patch("/me/logo", h.uploadAuthorityLogoMe)
		r.With(superOnly).Get("/{id}", h.getAuthority)
		r.With(superOnly).Patch("/{id}", h.updateAuthorityAdmin)
		r.With(superOnly).Patch("/{id}/status", h.updateAuthorityStatus)
		r.With(superOnly).Patch("/{id}/logo", h.uploadAuthorityLogoAdmin)
		r.With(superOnly).Delete("/{id}", h.deleteAuthority)
	})

	r.Route("/admins", func(r chi.Router) {
		r.Use(auth)
		r.With(superOnly).Get("/", h.listAdmins)
		r.With(security.RequireEntities(security.EntityAdmin)).Get("/me", h.adminMe)
		r.With(security.RequireEntities(security.EntityAdmin)).Patch("/me", h.updateAdminMe)
		r.With(superOnly).Get("/{id}", h.getAdmin)
		r.With(superOnly).Patch("/{id}", h.updateAdminAdmin)
		r.With(superOnly).Patch("/{id}/status", h.updateAdminStatus)
		r.With(superOnly).Delete("/{id}", h.deleteAdmin)
	})

	r.Route("/super-admins", func(r chi.Router) {
		r.Use(auth)
		r.Use(superOnly)
		r.Get("/", h.listSuperAdmins)
		r.Get("/me", h.superAdminMe)
		r.Patch("/me", h.updateSuperAdminMe)
		r.Get("/{id}", h.getSuperAdmin)
		r.Patch("/{id}", h.updateSuperAdminAdmin)
		r.Delete("/{id}", h.deleteSuperAdmin)
	})

	r.Route("/invitations", func(r chi.Router) {
		r.Use(auth)
		r.Use(adminOrSuper)
		r.Post("/", h.createInvitation)
		r.Get("/", h.listInvitations)
		r.Get("/{id}", h.getInvitation)
		r.Post("/{id}/resend", h.resendInvitation)
		r.Delete("/{id}", h.cancelInvitation)
	})
}
