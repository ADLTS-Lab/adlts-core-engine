package appeals

import "adlts/internal/platform/domain"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(a *domain.Appeal) {
	s.repo.Create(a)
}

func (s *Service) ListPending() []*domain.Appeal {
	return s.repo.ListPending()
}

func (s *Service) UpdateResolution(id string, updater func(*domain.Appeal) *domain.Appeal) *domain.Appeal {
	return s.repo.UpdateResolution(id, updater)
}
