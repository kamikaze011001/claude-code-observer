package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
	"github.com/kamikaze011001/claude-code-observer/internal/rollup"
)

// InsertEventsAndApplyRollups inserts a batch of events and applies all
// rollup updates inside a single SQLite transaction. Either every event +
// every rollup op lands, or nothing does.
func (r *Repository) InsertEventsAndApplyRollups(ctx context.Context, events []domain.Event) error {
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
	if err := applyRollupsTx(ctx, tx, events); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// applyRollupsTx executes rollup ops for each event on the given tx.
// Within each event's ops, session upserts are executed before prompt upserts
// to satisfy the prompts.session_id foreign-key constraint even when events
// arrive out of order (i.e., no prior session_start).
func applyRollupsTx(ctx context.Context, tx *sql.Tx, events []domain.Event) error {
	for i := range events {
		ev := events[i]
		ops := rollup.Apply(ev)
		if err := execOpsOrdered(ctx, tx, ev.EventName, ops); err != nil {
			return err
		}
	}
	return nil
}

// execOpsOrdered runs session-table ops before prompt-table ops so that FK
// constraints are satisfied even when a prompt op fires before any
// session_start has been seen.
func execOpsOrdered(ctx context.Context, tx *sql.Tx, eventName string, ops []rollup.Op) error {
	// First pass: session ops (upsert into sessions).
	for _, op := range ops {
		if isSessionOp(op) {
			if _, err := tx.ExecContext(ctx, op.Query, op.Args...); err != nil {
				return fmt.Errorf("rollup op for %s: %w", eventName, err)
			}
		}
	}
	// Second pass: all other ops (prompt upserts, etc.).
	for _, op := range ops {
		if !isSessionOp(op) {
			if _, err := tx.ExecContext(ctx, op.Query, op.Args...); err != nil {
				return fmt.Errorf("rollup op for %s: %w", eventName, err)
			}
		}
	}
	return nil
}

// isSessionOp reports whether op targets the sessions table.
func isSessionOp(op rollup.Op) bool {
	// All session upserts contain "INTO sessions" — a lightweight heuristic
	// that avoids importing a SQL parser.
	return len(op.Query) > 12 && containsSessionTarget(op.Query)
}

func containsSessionTarget(q string) bool {
	return strings.Contains(q, "INTO sessions")
}

// RebuildRollups truncates sessions and prompts, then replays every event
// in (ts, id) order through the rollup updaters. Single transaction.
// Caller must ensure no concurrent writers (i.e., stop `cco serve` first).
func (r *Repository) RebuildRollups(ctx context.Context) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `DELETE FROM prompts`); err != nil {
		return fmt.Errorf("delete prompts: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM sessions`); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, ts, session_id, COALESCE(prompt_id, ''), event_name, attrs
		FROM events
		ORDER BY ts ASC, id ASC`)
	if err != nil {
		return fmt.Errorf("select events: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ev domain.Event
		var attrs string
		if err = rows.Scan(&ev.ID, &ev.TS, &ev.SessionID, &ev.PromptID, &ev.EventName, &attrs); err != nil {
			return fmt.Errorf("scan event: %w", err)
		}
		ev.Attrs = map[string]any{}
		if attrs != "" {
			if jerr := jsonUnmarshal(attrs, &ev.Attrs); jerr != nil {
				return fmt.Errorf("unmarshal attrs id=%d: %w", ev.ID, jerr)
			}
		}
		if err = execOpsOrdered(ctx, tx, ev.EventName, rollup.Apply(ev)); err != nil {
			return fmt.Errorf("rollup id=%d: %w", ev.ID, err)
		}
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("rows: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// jsonUnmarshal is a thin wrapper to keep the import contained.
func jsonUnmarshal(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}
