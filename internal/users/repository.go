package users

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

func (r *Repository) FindByID(id string) (*domain.User, bool) {
	return r.s.FindUser(id)
}

func (r *Repository) UpdatePatch(id string, status domain.AccountStatus, instituteID string) *domain.User {
	return store.Write(r.s, func() *domain.User {
		user, exists := r.s.Users[id]
		if !exists {
			return nil
		}
		if status != "" {
			user.Status = status
		}
		if instituteID != "" {
			user.InstituteID = instituteID
		}
		user.UpdatedAt = time.Now().UTC()
		return user
	})
}
