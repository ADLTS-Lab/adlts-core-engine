package institutes

import (
	"net/http"

	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
)

type Handler struct {
	deps runtime.Dependencies
	svc  *Service
}

func New(deps runtime.Dependencies) *Handler {
	repo := NewRepository(deps.Store)
	svc := NewService(repo)
	return &Handler{deps: deps, svc: svc}
}

func (h *Handler) handleListInstitutes(w http.ResponseWriter, r *http.Request) {
	institutes := h.svc.ListAll()
	httpx.Success(w, http.StatusOK, institutes, &httpx.Meta{Total: len(institutes)})
}
