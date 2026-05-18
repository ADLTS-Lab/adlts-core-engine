package testing

import (
	"context"
	"time"

	"adlts/internal/domain"
	"log/slog"

	"github.com/google/uuid"
)

// ExpiryWorker runs in the background and looks for tests that have been
// in 'pending' or 'ready' state for too long without completing, and aborts them.
type ExpiryWorker struct {
	repo         *Repository
	pollInterval time.Duration
	timeout      time.Duration
}

func NewExpiryWorker(repo *Repository, pollInterval time.Duration, timeout time.Duration) *ExpiryWorker {
	if pollInterval == 0 {
		pollInterval = 5 * time.Minute
	}
	if timeout == 0 {
		timeout = 2 * time.Hour
	}
	return &ExpiryWorker{
		repo:         repo,
		pollInterval: pollInterval,
		timeout:      timeout,
	}
}

// Start begins the background check loop. It terminates when ctx is cancelled.
func (w *ExpiryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.expireStaleTests(ctx)
		}
	}
}

func (w *ExpiryWorker) expireStaleTests(ctx context.Context) {
	// Any active tests (not final) older than our timeout
	cutoff := time.Now().Add(-w.timeout)

	rows, err := w.repo.db.Query(ctx, 
		`SELECT id, device_id FROM tests 
		 WHERE status NOT IN ('completed', 'aborted', 'failed') 
		 AND updated_at < $1`, cutoff)
	if err != nil {
		slog.Error("ExpiryWorker failed to query stale tests", "error", err)
		return
	}
	defer rows.Close()

	var toAbort []struct {
		TestID   uuid.UUID
		DeviceID *uuid.UUID
	}
	for rows.Next() {
		var id uuid.UUID
		var devID *uuid.UUID
		if err := rows.Scan(&id, &devID); err == nil {
			toAbort = append(toAbort, struct{TestID uuid.UUID; DeviceID *uuid.UUID}{TestID: id, DeviceID: devID})
		}
	}
	rows.Close()

	for _, t := range toAbort {
		_ = w.repo.AbortTest(ctx, t.TestID, domain.AbortSystemError, systemActorID)
		if t.DeviceID != nil {
			_ = w.repo.ReleaseDevice(ctx, *t.DeviceID, systemActorID)
		}
		slog.Info("ExpiryWorker aborted stale test", "test_id", t.TestID)
	}
}
