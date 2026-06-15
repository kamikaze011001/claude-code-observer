package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
	"github.com/kamikaze011001/claude-code-observer/internal/rollup"
)

// InsertEvents inserts a batch of events in a single write transaction.
// Empty / nil input is a no-op. attrs is JSON-encoded.
func (r *Repository) InsertEvents(ctx context.Context, events []domain.Event) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertEventsTx(ctx, tx, events); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// InsertMetricSnapshots inserts a batch of metric datapoints in one tx.
func (r *Repository) InsertMetricSnapshots(ctx context.Context, snaps []domain.MetricSnapshot) error {
	if len(snaps) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertMetricSnapshotsTx(ctx, tx, snaps); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// InsertMetricsAndApplyRollups inserts a batch of metric snapshots and applies
// the metric rollup additively, in a single transaction. Either every snapshot
// + every rollup op lands, or nothing does. Mirrors InsertEventsAndApplyRollups.
func (r *Repository) InsertMetricsAndApplyRollups(ctx context.Context, snaps []domain.MetricSnapshot) error {
	if len(snaps) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertMetricSnapshotsTx(ctx, tx, snaps); err != nil {
		return err
	}
	if err := applyMetricRollupsTx(ctx, tx, snaps); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// applyMetricRollupsTx executes metric rollup ops for each snapshot on the tx.
// All ops target the sessions table, so execOpsOrdered's session-first pass
// handles them without FK ordering concerns.
func applyMetricRollupsTx(ctx context.Context, tx *sql.Tx, snaps []domain.MetricSnapshot) error {
	for i := range snaps {
		ops := rollup.ApplyMetric(snaps[i])
		if err := execOpsOrdered(ctx, tx, snaps[i].MetricName, ops); err != nil {
			return err
		}
	}
	return nil
}

func insertMetricSnapshotsTx(ctx context.Context, tx *sql.Tx, snaps []domain.MetricSnapshot) error {
	const q = `INSERT INTO metric_snapshots (ts, session_id, metric_name, value, attrs) VALUES (?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return fmt.Errorf("prepare metric_snapshots: %w", err)
	}
	defer stmt.Close()

	for i := range snaps {
		s := snaps[i]
		attrs := s.Attrs
		if attrs == nil {
			attrs = map[string]any{}
		}
		bs, err := json.Marshal(attrs)
		if err != nil {
			return fmt.Errorf("marshal attrs[%d]: %w", i, err)
		}
		var sessID any
		if s.SessionID != "" {
			sessID = s.SessionID
		}
		if _, err := stmt.ExecContext(ctx, s.TS, sessID, s.MetricName, s.Value, string(bs)); err != nil {
			return fmt.Errorf("insert metric_snapshots[%d]: %w", i, err)
		}
	}
	return nil
}

// insertEventsTx is the work shared between InsertEvents and the combined
// IngestBatch path used by service.Service. Caller owns the transaction.
func insertEventsTx(ctx context.Context, tx *sql.Tx, events []domain.Event) error {
	const q = `INSERT INTO events (ts, session_id, prompt_id, event_name, attrs) VALUES (?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return fmt.Errorf("prepare events: %w", err)
	}
	defer stmt.Close()

	for i := range events {
		ev := events[i]
		attrs := ev.Attrs
		if attrs == nil {
			attrs = map[string]any{}
		}
		bs, err := json.Marshal(attrs)
		if err != nil {
			return fmt.Errorf("marshal attrs[%d]: %w", i, err)
		}
		var promptID any
		if ev.PromptID != "" {
			promptID = ev.PromptID
		}
		if _, err := stmt.ExecContext(ctx, ev.TS, ev.SessionID, promptID, ev.EventName, string(bs)); err != nil {
			return fmt.Errorf("insert events[%d]: %w", i, err)
		}
	}
	return nil
}
