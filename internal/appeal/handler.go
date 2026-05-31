package appeal

import (
	"net/http"
	"time"

	"adlts/internal/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/security"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}



type createAppealReq struct {
	TestID    string `json:"test_id"`
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"`
}

func (h *Handler) createAppeal(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required", nil)
		return
	}
	var req createAppealReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body malformed", nil)
		return
	}
	testID, err := uuid.Parse(req.TestID)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "test_id must be a valid UUID", nil)
		return
	}
	sessionID, err := uuid.Parse(req.SessionID)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "session_id must be a valid UUID", nil)
		return
	}

	// Check appeal window
	window, err := h.svc.repo.GetAppealWindow(r.Context(), testID)
	if err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "DB_ERROR", "could not check appeal window", nil)
		return
	}
	if time.Now().After(window) {
		httpx.Failure(w, http.StatusForbidden, "APPEAL_WINDOW_CLOSED", "appeal window has closed", nil)
		return
	}

	a := &domain.Appeal{
		ID:          uuid.New(),
		TestID:      testID,
		SessionID:   sessionID,
		CandidateID: auth.SubjectID,
		Reason:      req.Reason,
		Status:      domain.AppealPending,
		Audit: domain.Audit{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			CreatedBy: auth.SubjectID,
			UpdatedBy: auth.SubjectID,
		},
	}

	if err := h.svc.CreateAppeal(r.Context(), a); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "CREATE_FAILED", "could not create appeal", nil)
		return
	}
	httpx.Success(w, http.StatusCreated, map[string]string{"id": a.ID.String()}, nil)
}

type resolveReq struct {
	Decision   string `json:"decision"`
	Resolution string `json:"resolution"`
}

func parseAppealStatus(s string) (domain.AppealStatus, bool) {
	switch s {
	case "accepted":
		return domain.AppealAccepted, true
	case "rejected":
		return domain.AppealRejected, true
	default:
		return "", false
	}
}

func (h *Handler) resolveAppeal(w http.ResponseWriter, r *http.Request) {
	auth, ok := security.CurrentAuth(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required", nil)
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}
	var req resolveReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_BODY", "request body malformed", nil)
		return
	}
	status, ok2 := parseAppealStatus(req.Decision)
	if !ok2 {
		httpx.Failure(w, http.StatusUnprocessableEntity, "INVALID_DECISION", "decision must be accepted or rejected", nil)
		return
	}

	if err := h.svc.ResolveAppeal(r.Context(), id, status, req.Resolution, auth.SubjectID, auth.SubjectID); err != nil {
		httpx.Failure(w, http.StatusInternalServerError, "RESOLVE_FAILED", "could not resolve appeal", nil)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"message": "appeal resolved"}, nil)
}

func (h *Handler) getAppeal(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	appealID, err := uuid.Parse(id)
	if err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_ID", "id must be a valid UUID", nil)
		return
	}

	var a domain.Appeal
	var expertID *uuid.UUID
	var resolution *string
	var createdAt, updatedAt time.Time
	var createdBy, updatedBy uuid.UUID

	err = h.svc.repo.db.QueryRow(r.Context(), `
		SELECT id, test_id, session_id, candidate_id, expert_id, reason, status, resolution, created_at, updated_at, created_by, updated_by
		FROM appeals WHERE id = $1
	`, appealID).Scan(&a.ID, &a.TestID, &a.SessionID, &a.CandidateID, &expertID, &a.Reason, &a.Status, &resolution, &createdAt, &updatedAt, &createdBy, &updatedBy)
	if err != nil {
		httpx.Failure(w, http.StatusNotFound, "NOT_FOUND", "appeal not found", nil)
		return
	}
	a.ExpertID = expertID
	if resolution != nil {
		a.Resolution = *resolution
	}
	a.Audit = domain.Audit{CreatedAt: createdAt, UpdatedAt: updatedAt, CreatedBy: createdBy, UpdatedBy: updatedBy}

	httpx.Success(w, http.StatusOK, a, nil)
}
