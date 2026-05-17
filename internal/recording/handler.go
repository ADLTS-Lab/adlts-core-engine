package recording

import (
	"net/http"
	"time"

	"adlts/internal/platform/httpx"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Mount(api chi.Router) {
	api.Get("/recordings/{test_id}/play", h.playRecording)
	api.Get("/recordings/{test_id}/frames", h.listFrames)
}

func (h *Handler) playRecording(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required", nil)
		return
	}
	_ = auth // currently unused but kept for audit/authorization later
	idStr := chi.URLParam(r, "test_id")
	tid, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test_id must be a valid UUID", nil)
		return
	}
	ctx := r.Context()
	if err := h.svc.StreamMJPEG(ctx, w, tid); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "STREAM_ERROR", "could not stream recording", nil)
		return
	}
}

func (h *Handler) listFrames(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required", nil)
		return
	}
	_ = auth
	idStr := chi.URLParam(r, "test_id")
	tid, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test_id must be a valid UUID", nil)
		return
	}
	frames, err := h.svc.FrameList(r.Context(), tid, 15*time.Minute)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "LIST_ERROR", "could not list frames", nil)
		return
	}
	httpx.Success(w, http.StatusOK, frames, nil)
}
