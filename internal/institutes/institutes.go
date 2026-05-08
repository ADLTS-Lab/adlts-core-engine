package institutes

import (
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

func RegisterInstituteRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	authMW := security.Authenticate(deps.Tokens, deps.Store)
	r.With(authMW).Get("/", h.handleListInstitutes)
}
