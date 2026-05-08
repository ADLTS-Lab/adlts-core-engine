package institutes

import "adlts/internal/platform/domain"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListAll() []*domain.Institute {
	return s.repo.ListAll()
}
