package recording

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetTestRecording returns minio_prefix and frame_count for a saved recording
func (r *Repository) GetTestRecording(ctx context.Context, testID uuid.UUID) (string, int, error) {
	var prefix string
	var frameCount int
	err := r.db.QueryRow(ctx, `SELECT minio_prefix, frame_count FROM test_recordings WHERE test_id = $1 AND status = 'saved'`, testID).Scan(&prefix, &frameCount)
	if err != nil {
		return "", 0, err
	}
	return prefix, frameCount, nil
}
