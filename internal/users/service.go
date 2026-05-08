package users

import (
	"adlts/internal/platform/domain"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetByID(id string) (*domain.User, bool) {
	return s.repo.FindByID(id)
}

func (s *Service) PatchUser(id string, req userPatchRequest) (*domain.User, bool) {
	updated := s.repo.UpdatePatch(id, req.Status, req.InstituteID)
	if updated == nil {
		return nil, false
	}
	return updated, true
}
