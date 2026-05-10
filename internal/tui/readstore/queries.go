package readstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// WindowStats is the rollup over a single time window.
type WindowStats struct {
	CostUSD float64
	Prompts int64
	Tools   int64
	Errors  int64
}

// Snapshot is the dashboard's three-window rollup.
type Snapshot struct {
	Today         WindowStats
	D7            WindowStats
	D30           WindowStats
	LatestEventTS int64
}

// TopSession is a row in the "top sessions today" panel.
type TopSession struct {
	SessionID   string
	ProjectName string
	StartedAt   int64
	CostUSD     float64
	Prompts     int64
	Live        bool
}

// DashboardSnapshot returns the three-window rollup plus the top-3 most
// expensive sessions started today (UTC). now is injected for testability.
func DashboardSnapshot(ctx context.Context, db *sql.DB, now time.Time) (Snapshot, []TopSession, error) {
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	today := startOfDay.UnixNano()
	d7 := startOfDay.Add(-7 * 24 * time.Hour).UnixNano()
	d30 := startOfDay.Add(-30 * 24 * time.Hour).UnixNano()

	const q = `
SELECT
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd     END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts      END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls   END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors   END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd     END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts      END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls   END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors   END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd     END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts      END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls   END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors   END), 0)
FROM sessions
WHERE started_at >= ?`

	var s Snapshot
	err := db.QueryRowContext(ctx, q,
		today, today, today, today,
		d7, d7, d7, d7,
		d30, d30, d30, d30,
		d30,
	).Scan(
		&s.Today.CostUSD, &s.Today.Prompts, &s.Today.Tools, &s.Today.Errors,
		&s.D7.CostUSD, &s.D7.Prompts, &s.D7.Tools, &s.D7.Errors,
		&s.D30.CostUSD, &s.D30.Prompts, &s.D30.Tools, &s.D30.Errors,
	)
	if err != nil {
		return Snapshot{}, nil, fmt.Errorf("snapshot query: %w", err)
	}

	var ts sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT MAX(ts) FROM events").Scan(&ts); err != nil {
		return Snapshot{}, nil, fmt.Errorf("latest event ts: %w", err)
	}
	if ts.Valid {
		s.LatestEventTS = ts.Int64
	}

	const topQ = `
SELECT session_id, COALESCE(project_name, ''), started_at, cost_usd, prompts, ended_at IS NULL
FROM sessions
WHERE started_at >= ?
ORDER BY cost_usd DESC
LIMIT 3`
	rows, err := db.QueryContext(ctx, topQ, today)
	if err != nil {
		return Snapshot{}, nil, fmt.Errorf("top sessions query: %w", err)
	}
	defer rows.Close()

	var top []TopSession
	for rows.Next() {
		var t TopSession
		var live int
		if err := rows.Scan(&t.SessionID, &t.ProjectName, &t.StartedAt, &t.CostUSD, &t.Prompts, &live); err != nil {
			return Snapshot{}, nil, fmt.Errorf("top session scan: %w", err)
		}
		t.Live = live == 1
		top = append(top, t)
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, nil, fmt.Errorf("top sessions iter: %w", err)
	}
	return s, top, nil
}
