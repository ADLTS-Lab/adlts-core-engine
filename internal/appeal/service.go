package appeal

import (
	"context"
	"errors"
	"time"

	"adlts/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	repo *Repository
	db   *pgxpool.Pool
}

func NewService(r *Repository, db *pgxpool.Pool) *Service {
	return &Service{repo: r, db: db}
}

func (s *Service) CreateAppeal(ctx context.Context, a *domain.Appeal) error {
	// enforce appeal window
	window, err := s.repo.GetAppealWindow(ctx, a.TestID)
	if err != nil {
		return err
	}
	if time.Now().After(window) {
		return errors.New("appeal window closed")
	}
	return s.repo.CreateAppeal(ctx, a)
}

// Resolve appeal and, if accepted, update test_results and tests within a transaction
func (s *Service) ResolveAppeal(ctx context.Context, appealID uuid.UUID, status domain.AppealStatus, resolution string, expertID uuid.UUID, resolvedBy uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := s.repo.ResolveAppealTx(ctx, tx, appealID, string(status), resolution, expertID, resolvedBy); err != nil {
		return err
	}

	if status == domain.AppealAccepted {
		// set test_results.passed = true for the test referenced by this appeal session
		if _, err := tx.Exec(ctx, `
			UPDATE test_results
			SET passed = true
			WHERE test_id = (
				SELECT t.id
				FROM tests t
				JOIN sessions s ON s.booking_id = t.booking_id
				WHERE s.id = (SELECT session_id FROM appeals WHERE id = $1)
				LIMIT 1
			)
		`, appealID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE tests
			SET updated_at = NOW()
			WHERE id = (
				SELECT t.id
				FROM tests t
				JOIN sessions s ON s.booking_id = t.booking_id
				WHERE s.id = (SELECT session_id FROM appeals WHERE id = $1)
				LIMIT 1
			)
		`, appealID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
