package bookings

import (
	"net/http"
	"sort"
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

type bookingCreateRequest struct {
	InstituteID     string `json:"institute_id"`
	RequestedSlotID string `json:"requested_slot_id,omitempty"`
	TrainingHours   int    `json:"training_hours,omitempty"`
}

type bookingVerifyRequest struct {
	Approved            bool   `json:"approved"`
	TrainingHours       int    `json:"training_hours,omitempty"`
	TrainingEvidenceURL string `json:"training_evidence_url,omitempty"`
	ScheduledSlotID     string `json:"scheduled_slot_id,omitempty"`
}

func (h *Handler) handleCreateBooking(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	var req bookingCreateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid booking payload", err.Error())
		return
	}
	if req.InstituteID == "" {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "institute_id is required", nil)
		return
	}
	if _, ok := h.deps.Store.FindInstitute(req.InstituteID); !ok {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_INSTITUTE", "institute does not exist", nil)
		return
	}
	if req.RequestedSlotID != "" {
		if slot, ok := h.deps.Store.Slots[req.RequestedSlotID]; !ok || slot.InstituteID != req.InstituteID {
			httpx.Failure(w, http.StatusBadRequest, "INVALID_SLOT", "requested slot is not available for the selected institute", nil)
			return
		}
	}
	now := time.Now().UTC()
	booking := &domain.Booking{
		ID:              store.NewID(),
		CandidateID:     current.ID,
		InstituteID:     req.InstituteID,
		RequestedSlotID: req.RequestedSlotID,
		Status:          domain.BookingPendingVerification,
		TrainingHours:   req.TrainingHours,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	h.svc.Create(booking)
	httpx.Success(w, http.StatusCreated, booking, nil)
}

func (h *Handler) handleListPendingBookings(w http.ResponseWriter, r *http.Request) {
	current, _ := security.CurrentUser(r)
	bookings := h.svc.ListPending(current.InstituteID)
	httpx.Success(w, http.StatusOK, bookings, &httpx.Meta{Total: len(bookings)})
}

func (h *Handler) handleVerifyBooking(w http.ResponseWriter, r *http.Request) {
	current, _ := security.CurrentUser(r)
	id := chi.URLParam(r, "id")
	var req bookingVerifyRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		httpx.Failure(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid booking verification payload", err.Error())
		return
	}
	updated := h.svc.UpdateVerification(id, func(booking *domain.Booking) *domain.Booking {
		if booking == nil || booking.InstituteID != current.InstituteID {
			return nil
		}
		now := time.Now().UTC()
		if req.Approved {
			booking.Status = domain.BookingVerified
			booking.TrainingHours = req.TrainingHours
			booking.TrainingEvidenceURL = req.TrainingEvidenceURL
			booking.VerifiedBy = current.ID
			booking.VerifiedAt = &now
			if req.ScheduledSlotID != "" {
				booking.ScheduledSlotID = req.ScheduledSlotID
			}
		} else {
			booking.Status = domain.BookingRejected
		}
		booking.UpdatedAt = now
		return booking
	})
	if updated == nil {
		httpx.Failure(w, http.StatusNotFound, "BOOKING_NOT_FOUND", "booking not found", nil)
		return
	}
	httpx.Success(w, http.StatusOK, updated, nil)
}

func (h *Handler) handleAvailableSlots(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	if !h.svc.CandidateHasVerifiedBooking(current.ID) {
		httpx.Failure(w, http.StatusForbidden, "BOOKING_NOT_VERIFIED", "verified booking required to view available slots", nil)
		return
	}
	slots := store.Read(h.deps.Store, func() []*domain.Slot {
		result := make([]*domain.Slot, 0)
		for _, slot := range h.deps.Store.Slots {
			if slot.InstituteID == current.InstituteID && slot.BookedCount < slot.Capacity {
				result = append(result, slot)
			}
		}
		sort.Slice(result, func(i, j int) bool { return result[i].StartTime.Before(result[j].StartTime) })
		return result
	})
	httpx.Success(w, http.StatusOK, slots, &httpx.Meta{Total: len(slots)})
}
