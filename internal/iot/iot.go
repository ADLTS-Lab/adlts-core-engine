package iot

import (
	"adlts/internal/platform/runtime"

	"github.com/go-chi/chi/v5"
)

func RegisterIoTRoutes(r chi.Router, deps runtime.Dependencies) {
	h := New(deps)
	r.Get("/stream/{device_id}", h.handleStream)
}
