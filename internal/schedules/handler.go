package schedules

import (
	"net/http"
	sort "sort"

	"adlts/internal/platform/domain"
	"adlts/internal/platform/httpx"
	"adlts/internal/platform/runtime"
	"adlts/internal/platform/security"
	"adlts/internal/platform/store"
)

type Handler struct {
	deps runtime.Dependencies
}

func New(deps runtime.Dependencies) *Handler {
	return &Handler{deps: deps}
}

func (h *Handler) handleAvailableSlots(w http.ResponseWriter, r *http.Request) {
	current, ok := security.CurrentUser(r)
	if !ok {
		httpx.Failure(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing authenticated user", nil)
		return
	}
	// require verified booking
	hasVerified := false
	store.Read(h.deps.Store, func() struct{} {
		for _, booking := range h.deps.Store.Bookings {
			if booking.CandidateID == current.ID && booking.Status == domain.BookingVerified {
				hasVerified = true
				break
			}
		}
		return struct{}{}
	})
	if !hasVerified {
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
