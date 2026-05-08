package exams

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

func (r *Repository) Create(e *domain.Exam) {
	store.Write(r.s, func() struct{} {
		r.s.Exams[e.ID] = e
		return struct{}{}
	})
}

func (r *Repository) FindByID(id string) (*domain.Exam, bool) {
	return r.s.FindExam(id)
}

func (r *Repository) Update(id string, updater func(*domain.Exam) *domain.Exam) *domain.Exam {
	return store.Write(r.s, func() *domain.Exam {
		e, exists := r.s.Exams[id]
		if !exists {
			return nil
		}
		updated := updater(e)
		if updated != nil {
			updated.UpdatedAt = time.Now().UTC()
		}
		return updated
	})
}

func (r *Repository) LatestForDevice(deviceID string) *domain.Exam {
	var matched *domain.Exam
	store.Read(r.s, func() struct{} {
		for _, exam := range r.s.Exams {
			if exam.DeviceID == deviceID && (exam.Status == domain.ExamActive || exam.Status == domain.ExamInitiating) {
				matched = exam
			}
		}
		return struct{}{}
	})
	return matched
}
