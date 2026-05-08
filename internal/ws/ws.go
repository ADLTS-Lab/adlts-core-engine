package ws

import (
	"adlts/internal/platform/runtime"

	"github.com/go-chi/chi/v5"
)

func RegisterWsRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	r.Get("/iot/stream/{device_id}", h.handleStream)
	r.Get("/ws/iot/stream/{device_id}", h.handleStream)
}
