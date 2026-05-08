package users

import (
	"net/http"

	"adlts/internal/platform/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
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

type userPatchRequest struct {
	Status      domain.AccountStatus `json:"status"`
	InstituteID string               `json:"institute_id,omitempty"`
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	httpx.Success(w, http.StatusOK, user, nil)
}

func (h *Handler) handlePatchUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "id is required", nil)
		return
	}
	var req userPatchRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid patch payload", err.Error())
		return
	}
	updated, ok := h.svc.PatchUser(id, req)
	if !ok || updated == nil {
		httpx.Failure(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, updated, nil)
}
