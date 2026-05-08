package exams

import (
	"adlts/internal/modules"
	"adlts/internal/platform/runtime"

	"github.com/go-chi/chi/v5"
)

func RegisterExamRoutes(r chi.Router, deps runtime.Dependencies) {
	modules.RegisterExamRoutes(r, deps)
}
