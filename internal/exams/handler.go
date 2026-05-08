package exams

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

type examInitiateRequest struct {
	BookingID string `json:"booking_id"`
	DeviceID  string `json:"device_id"`
	QRCode    string `json:"qr_code,omitempty"`
}

func (h *Handler) readExam(r *http.Request) (*domain.Exam, *domain.User, bool) {
	current, ok := security.CurrentUser(r)
	if !ok {
		return nil, nil, false
	}
	id := chi.URLParam(r, "id")
	exam, exists := h.deps.Store.FindExam(id)
	if !exists {
		return nil, current, false
	}
	return exam, current, true
}

func (h *Handler) readExamForViewer(w http.ResponseWriter, r *http.Request, examinerOnly bool) (*domain.Exam, bool) {
	exam, current, ok := h.readExam(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return nil, false
	}
	if current.Role == domain.RoleCandidate && exam.CandidateID != current.ID {
		httpx.Failure(w, http.StatusForbidden, "FORBIDDEN", "candidates can only access their own exam data", nil)
		return nil, false
	}
	if examinerOnly && current.Role == domain.RoleCandidate {
		httpx.Failure(w, http.StatusForbidden, "FORBIDDEN", "examiner or authority access required", nil)
		return nil, false
	}
	return exam, true
}

func (h *Handler) handleInitiateExam(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	var req examInitiateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid exam initiation payload", err.Error())
		return
	}
	booking, exists := h.deps.Store.FindBooking(req.BookingID)
	if !exists || booking.CandidateID != current.ID || booking.Status != domain.BookingVerified {
		httpx.Failure(w, http.StatusBadRequest, "BOOKING_NOT_READY", "verified booking required for exam initiation", nil)
		return
	}
	if _, ok := h.deps.Store.FindDevice(req.DeviceID); !ok {
		httpx.Failure(w, http.StatusBadRequest, "DEVICE_NOT_FOUND", "device not found", nil)
		return
	}
	if req.QRCode != "" && req.QRCode != req.BookingID && req.QRCode != req.DeviceID {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_QR", "qr code does not match the booking or device linkage", nil)
		return
	}
	now := time.Now().UTC()
	exam := &domain.Exam{ID: store.NewID(), BookingID: booking.ID, CandidateID: current.ID, DeviceID: req.DeviceID, Status: domain.ExamInitiating, Score: 100, Telemetry: domain.ExamTelemetry{Health: "initiating", CurrentScore: 100, UpdatedAt: now}, StartedAt: &now, CreatedAt: now, UpdatedAt: now}
	h.svc.Create(exam)
	// update booking
	store.Write(h.deps.Store, func() struct{} {
		booking.Status = domain.BookingScheduled
		booking.ScheduledSlotID = req.DeviceID
		booking.UpdatedAt = now
		return struct{}{}
	})
	exam.Status = domain.ExamActive
	exam.Telemetry.Health = "active"
	httpx.Success(w, http.StatusCreated, exam, nil)
}

func (h *Handler) handleLiveExam(w http.ResponseWriter, r *http.Request) {
	exam, ok := h.readExamForViewer(w, r, true)
	if !ok {
		return
	}
	httpx.Success(w, http.StatusOK, exam, nil)
}

func (h *Handler) handleStopExam(w http.ResponseWriter, r *http.Request) {
	exam, ok := h.readExamForViewer(w, r, true)
	if !ok {
		return
	}
	now := time.Now().UTC()
	h.svc.Update(exam.ID, func(e *domain.Exam) *domain.Exam {
		e.Status = domain.ExamStopped
		e.CompletedAt = &now
		e.UpdatedAt = now
		e.Telemetry.Health = "stopped"
		if e.Score < 70 {
			e.Status = domain.ExamReviewRequired
		}
		if e.ResultOverlayURL == "" {
			e.ResultOverlayURL = fmt.Sprintf("https://storage.local/exams/%s/overlay.mp4", e.ID)
		}
		return e
	})
	httpx.Success(w, http.StatusOK, exam, nil)
}

func (h *Handler) handleExamResults(w http.ResponseWriter, r *http.Request) {
	exam, ok := h.readExamForViewer(w, r, false)
	if !ok {
		return
	}
	httpx.Success(w, http.StatusOK, exam, nil)
}

func (h *Handler) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	exam, ok := h.readExamForViewer(w, r, true)
	if !ok {
		return
	}
	httpx.Success(w, http.StatusOK, exam.Telemetry, nil)
}

func (h *Handler) handleResultOverlay(w http.ResponseWriter, r *http.Request) {
	exam, ok := h.readExamForViewer(w, r, false)
	if !ok {
		return
	}
	httpx.Success(w, http.StatusOK, map[string]any{"exam_id": exam.ID, "overlay_url": exam.ResultOverlayURL, "violations": exam.Violations}, nil)
}
