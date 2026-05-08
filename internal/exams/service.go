package exams

import "adlts/internal/platform/domain"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(e *domain.Exam) {
	s.repo.Create(e)
}

func (s *Service) FindByID(id string) (*domain.Exam, bool) {
	return s.repo.FindByID(id)
}

func (s *Service) Update(id string, updater func(*domain.Exam) *domain.Exam) *domain.Exam {
	return s.repo.Update(id, updater)
}

func (s *Service) LatestForDevice(deviceID string) *domain.Exam {
	return s.repo.LatestForDevice(deviceID)
}
