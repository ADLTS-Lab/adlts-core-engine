package appeals

import (
	"adlts/internal/modules"
	"adlts/internal/platform/runtime"

	"github.com/go-chi/chi/v5"
)

func RegisterAppealRoutes(r chi.Router, deps runtime.Dependencies) {
	modules.RegisterAppealRoutes(r, deps)
}
