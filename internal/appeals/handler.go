package appeals

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
	svc  *Service
}

func New(deps runtime.Dependencies) *Handler {
	repo := NewRepository(deps.Store)
	svc := NewService(repo)
	return &Handler{deps: deps, svc: svc}
}

type appealCreateRequest struct {
	ExamID string `json:"exam_id"`
	Reason string `json:"reason"`
}

type appealResolveRequest struct {
	Status     domain.AppealStatus `json:"status"`
	Resolution string              `json:"resolution"`
}

func (h *Handler) handleCreateAppeal(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	var req appealCreateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid appeal payload", err.Error())
		return
	}
	exam, exists := h.deps.Store.FindExam(req.ExamID)
	if !exists || exam.CandidateID != current.ID {
		httpx.Failure(w, http.StatusBadRequest, "EXAM_NOT_FOUND", "exam not found for candidate", nil)
		return
	}
	now := time.Now().UTC()
	appeal := &domain.Appeal{ID: store.NewID(), ExamID: exam.ID, CandidateID: current.ID, Reason: req.Reason, Status: domain.AppealPending, CreatedAt: now, UpdatedAt: now}
	h.svc.Create(appeal)
	httpx.Success(w, http.StatusCreated, appeal, nil)
}

func (h *Handler) handlePendingAppeals(w http.ResponseWriter, r *http.Request) {
	appeals := h.svc.ListPending()
	httpx.Success(w, http.StatusOK, appeals, &httpx.Meta{Total: len(appeals)})
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
	updated := h.svc.UpdateResolution(id, func(a *domain.Appeal) *domain.Appeal {
		if a == nil {
			return nil
		}
		now := time.Now().UTC()
		a.Status = req.Status
		a.Resolution = req.Resolution
		a.ResolvedBy = current.ID
		a.UpdatedAt = now
		if exam, exists := h.deps.Store.Exams[a.ExamID]; exists {
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
		return a
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
