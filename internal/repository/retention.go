package repository

import (
	"context"
	"fmt"
)

// DeleteEventsBefore removes rows from events whose ts is strictly less than
// cutoff (unix nanoseconds). Returns the number of rows deleted.
func (r *Repository) DeleteEventsBefore(ctx context.Context, cutoff int64) (int64, error) {
	return r.deleteBefore(ctx, "events", cutoff)
}

// DeleteMetricSnapshotsBefore removes rows from metric_snapshots whose ts is
// strictly less than cutoff (unix nanoseconds). Returns the number of rows
// deleted.
func (r *Repository) DeleteMetricSnapshotsBefore(ctx context.Context, cutoff int64) (int64, error) {
	return r.deleteBefore(ctx, "metric_snapshots", cutoff)
}

func (r *Repository) deleteBefore(ctx context.Context, table string, cutoff int64) (int64, error) {
	// table is a fixed in-package literal — never user input — so string
	// substitution is safe. Parameterized binding is used for the value.
	q := fmt.Sprintf(`DELETE FROM %s WHERE ts < ?`, table)
	res, err := r.db.ExecContext(ctx, q, cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete from %s: %w", table, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}
