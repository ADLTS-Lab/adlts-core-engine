package appeal

import (
	"context"
	"fmt"
	"strings"
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

func (r *Repository) ListAppeals(ctx context.Context, status string, page, limit int) ([]domain.Appeal, int, error) {
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	conds := []string{}
	args := []any{}
	if status != "" {
		args = append(args, status)
		conds = append(conds, fmt.Sprintf("status=$%d", len(args)))
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	countQuery := `SELECT COUNT(*) FROM appeals ` + where
	var total int
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, session_id, candidate_id, expert_id, reason, status, resolution, created_at, updated_at, created_by, updated_by
		FROM appeals %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]domain.Appeal, 0, limit)
	for rows.Next() {
		var a domain.Appeal
		var (
			sessionID  *uuid.UUID
			expertID   *uuid.UUID
			resolution *string
			createdAt  time.Time
			updatedAt  time.Time
			createdBy  uuid.UUID
			updatedBy  uuid.UUID
		)
		if err := rows.Scan(
			&a.ID, &sessionID, &a.CandidateID, &expertID, &a.Reason, &a.Status, &resolution,
			&createdAt, &updatedAt, &createdBy, &updatedBy,
		); err != nil {
			return nil, 0, err
		}
		if sessionID != nil {
			a.SessionID = *sessionID
		}
		a.ExpertID = expertID
		if resolution != nil {
			a.Resolution = *resolution
		}
		a.Audit = domain.Audit{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			CreatedBy: createdBy,
			UpdatedBy: updatedBy,
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
