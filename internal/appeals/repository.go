package appeals

import (
	"adlts/internal/platform/domain"
	"adlts/internal/platform/store"
	"time"
)

type Repository struct {
	s *store.Store
}

func NewRepository(s *store.Store) *Repository {
	return &Repository{s: s}
}

func (r *Repository) Create(a *domain.Appeal) {
	store.Write(r.s, func() struct{} {
		r.s.Appeals[a.ID] = a
		return struct{}{}
	})
}

func (r *Repository) ListPending() []*domain.Appeal {
	return store.Read(r.s, func() []*domain.Appeal {
		result := make([]*domain.Appeal, 0)
		for _, a := range r.s.Appeals {
			if a.Status == domain.AppealPending {
				result = append(result, a)
			}
		}
		return result
	})
}

func (r *Repository) UpdateResolution(id string, updater func(*domain.Appeal) *domain.Appeal) *domain.Appeal {
	return store.Write(r.s, func() *domain.Appeal {
		a, exists := r.s.Appeals[id]
		if !exists {
			return nil
		}
		updated := updater(a)
		if updated != nil {
			updated.UpdatedAt = time.Now().UTC()
		}
		return updated
	})
}
