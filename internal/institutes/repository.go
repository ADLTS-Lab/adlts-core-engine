package institutes

import (
	"adlts/internal/platform/domain"
	"adlts/internal/platform/store"
)

type Repository struct {
	s *store.Store
}

func NewRepository(s *store.Store) *Repository {
	return &Repository{s: s}
}

func (r *Repository) ListAll() []*domain.Institute {
	return store.Read(r.s, func() []*domain.Institute {
		result := make([]*domain.Institute, 0, len(r.s.Institutes))
		for _, inst := range r.s.Institutes {
			result = append(result, inst)
		}
		return result
	})
}
