package bookings

import "adlts/internal/platform/domain"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(booking *domain.Booking) {
	s.repo.Create(booking)
}

func (s *Service) FindByID(id string) (*domain.Booking, bool) {
	return s.repo.FindByID(id)
}

func (s *Service) ListPending(instituteID string) []*domain.Booking {
	return s.repo.ListPendingForInstitute(instituteID)
}

func (s *Service) UpdateVerification(id string, updater func(*domain.Booking) *domain.Booking) *domain.Booking {
	return s.repo.UpdateVerification(id, updater)
}

func (s *Service) CandidateHasVerifiedBooking(candidateID string) bool {
	return s.repo.CandidateHasVerifiedBooking(candidateID)
}
