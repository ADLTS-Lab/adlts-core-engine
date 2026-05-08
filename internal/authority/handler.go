package authority

import (
	"fmt"
	"net/http"
	"time"

	"adlts/internal/platform/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"
	"adlts/internal/platform/store"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	deps runtime.Dependencies
}

func New(deps runtime.Dependencies) *Handler {
	return &Handler{deps: deps}
}

type appealResolveRequest struct {
	Status     domain.AppealStatus `json:"status"`
	Resolution string              `json:"resolution"`
}

func (h *Handler) handleResolveAppeal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "id is required", nil)
		return
	}
	var req appealResolveRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid appeal resolution payload", err.Error())
		return
	}
	current, _ := security.CurrentUser(r)
	updated := store.Write(h.deps.Store, func() *domain.Appeal {
		appeal, exists := h.deps.Store.Appeals[id]
		if !exists {
			return nil
		}
		now := time.Now().UTC()
		appeal.Status = req.Status
		appeal.Resolution = req.Resolution
		appeal.ResolvedBy = current.ID
		appeal.UpdatedAt = now
		if exam, exists := h.deps.Store.Exams[appeal.ExamID]; exists {
			exam.FinalizedAt = &now
			exam.UpdatedAt = now
			if req.Status == domain.AppealAccepted {
				exam.Status = domain.ExamFinalized
				exam.Score = maxFloat(exam.Score, 75)
				exam.Telemetry.Health = "appeal_accepted"
			} else {
				exam.Status = domain.ExamFinalized
				exam.Telemetry.Health = "appeal_rejected"
			}
			if exam.ResultOverlayURL == "" {
				exam.ResultOverlayURL = fmt.Sprintf("https://storage.local/exams/%s/overlay.mp4", exam.ID)
			}
		}
		return appeal
	})
	if updated == nil {
		httpx.Failure(w, http.StatusNotFound, "APPEAL_NOT_FOUND", "appeal not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, updated, nil)
}

func maxFloat(current, threshold float64) float64 {
	if current > threshold {
		return current
	}
	return threshold
}
