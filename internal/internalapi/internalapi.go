package internalapi

import (
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
)

func RegisterInternalRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	r.With(security.RequireInternalToken(deps.Config.InternalAPIKey)).Post("/scoring/frame-process", h.handleFrameProcess)
}
