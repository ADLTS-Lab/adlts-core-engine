package bookings

import (
	"time"

	"adlts/internal/platform/domain"
	"adlts/internal/platform/store"
)

type Repository struct {
	s *store.Store
}

func NewRepository(s *store.Store) *Repository {
	return &Repository{s: s}
}

func (r *Repository) Create(booking *domain.Booking) {
	store.Write(r.s, func() struct{} {
		r.s.Bookings[booking.ID] = booking
		return struct{}{}
	})
}

func (r *Repository) FindByID(id string) (*domain.Booking, bool) {
	return r.s.FindBooking(id)
}

func (r *Repository) ListPendingForInstitute(instituteID string) []*domain.Booking {
	return store.Read(r.s, func() []*domain.Booking {
		result := make([]*domain.Booking, 0)
		for _, booking := range r.s.Bookings {
			if booking.InstituteID == instituteID && booking.Status == domain.BookingPendingVerification {
				result = append(result, booking)
			}
		}
		return result
	})
}

func (r *Repository) UpdateVerification(id string, updater func(*domain.Booking) *domain.Booking) *domain.Booking {
	return store.Write(r.s, func() *domain.Booking {
		booking, exists := r.s.Bookings[id]
		if !exists {
			return nil
		}
		updated := updater(booking)
		if updated != nil {
			updated.UpdatedAt = time.Now().UTC()
		}
		return updated
	})
}

func (r *Repository) CandidateHasVerifiedBooking(candidateID string) bool {
	return store.Read(r.s, func() bool {
		for _, booking := range r.s.Bookings {
			if booking.CandidateID == candidateID && booking.Status == domain.BookingVerified {
				return true
			}
		}
		return false
	})
}
