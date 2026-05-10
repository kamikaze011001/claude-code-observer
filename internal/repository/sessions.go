package repository

import (
	"context"
	"fmt"
)

// CloseIdleSessions sets ended_at = last_seen_at for any open session whose
// last_seen_at is strictly before cutoff (unix nanoseconds). Returns the
// number of sessions closed.
func (r *Repository) CloseIdleSessions(ctx context.Context, cutoff int64) (int64, error) {
	const q = `UPDATE sessions SET ended_at = last_seen_at WHERE ended_at IS NULL AND last_seen_at < ?`
	res, err := r.db.ExecContext(ctx, q, cutoff)
	if err != nil {
		return 0, fmt.Errorf("close idle sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}
