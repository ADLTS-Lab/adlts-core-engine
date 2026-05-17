package appeal

import (
	"context"
	"time"

	"adlts/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateAppeal(ctx context.Context, a *domain.Appeal) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO appeals (id, test_id, session_id, candidate_id, reason, status, created_at, updated_at, created_by, updated_by)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
    `,
		a.ID, a.TestID, a.SessionID, a.CandidateID, a.Reason, a.Status,
		a.CreatedAt, a.UpdatedAt, a.Audit.CreatedBy, a.Audit.UpdatedBy,
	)
	return err
}

func (r *Repository) GetAppealWindow(ctx context.Context, testID uuid.UUID) (time.Time, error) {
	var t time.Time
	err := r.db.QueryRow(ctx, `SELECT appeal_window_closes_at FROM tests WHERE id=$1`, testID).Scan(&t)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func (r *Repository) ResolveAppealTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, status string, resolution string, expertID uuid.UUID, resolvedBy uuid.UUID) error {
	_, err := tx.Exec(ctx, `
        UPDATE appeals SET status=$2, resolution=$3, expert_id=$4, updated_at=NOW(), updated_by=$5 WHERE id=$1
    `, id, status, resolution, expertID, resolvedBy)
	return err
}
