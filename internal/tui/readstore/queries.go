package readstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SessionRow is one row in the sessions list page.
type SessionRow struct {
	SessionID   string
	ProjectName string // "" rendered as "(unlabeled)" by the view
	StartedAt   time.Time
	LastSeenAt  time.Time
	EndedAt     time.Time // zero if Live
	DurationSec int64
	CostUSD     float64
	Prompts     int64
	Live        bool
}

// SessionsPage returns one page of sessions newest-first. cursor is a started_at
// (unix ns) — pass nil for the first page. The returned next-cursor is nil when
// the page is the last one.
func SessionsPage(ctx context.Context, db *sql.DB, cursor *int64, limit int) ([]SessionRow, *int64, error) {
	if limit <= 0 {
		limit = 50
	}
	const q = `
SELECT session_id,
       COALESCE(project_name, ''),
       started_at,
       last_seen_at,
       ended_at,
       cost_usd,
       prompts
FROM sessions
WHERE (? IS NULL OR started_at < ?)
ORDER BY started_at DESC
LIMIT ?`
	var cur sql.NullInt64
	if cursor != nil {
		cur = sql.NullInt64{Int64: *cursor, Valid: true}
	}
	rows, err := db.QueryContext(ctx, q, cur, cur, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("sessions page: %w", err)
	}
	defer rows.Close()

	out := make([]SessionRow, 0, limit)
	for rows.Next() {
		var (
			r        SessionRow
			started  int64
			lastSeen int64
			ended    sql.NullInt64
		)
		if err := rows.Scan(&r.SessionID, &r.ProjectName, &started, &lastSeen, &ended, &r.CostUSD, &r.Prompts); err != nil {
			return nil, nil, fmt.Errorf("sessions page scan: %w", err)
		}
		r.StartedAt = time.Unix(0, started).UTC()
		r.LastSeenAt = time.Unix(0, lastSeen).UTC()
		if ended.Valid {
			r.EndedAt = time.Unix(0, ended.Int64).UTC()
			r.DurationSec = (ended.Int64 - started) / int64(time.Second)
		} else {
			r.Live = true
			r.DurationSec = (lastSeen - started) / int64(time.Second)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("sessions page iter: %w", err)
	}
	var next *int64
	if len(out) == limit {
		v := out[len(out)-1].StartedAt.UnixNano()
		next = &v
	}
	return out, next, nil
}

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

// EventRow is a row in the Session Detail timeline. Summary is derived from
// attrs by summarize().
type EventRow struct {
	TS        time.Time
	EventName string
	PromptID  string // "" for session-level events
	Summary   string
}

// SessionEvents returns up to limit rows for sessionID newest-first. beforeTS
// is the keyset cursor (ts) — pass nil for the first page. The hasMore flag is
// true when len(returned) == limit.
func SessionEvents(ctx context.Context, db *sql.DB, sessionID string, beforeTS *int64, limit int) ([]EventRow, bool, error) {
	if limit <= 0 {
		limit = 200
	}
	const q = `
SELECT ts, event_name, COALESCE(prompt_id, ''), attrs
FROM events
WHERE session_id = ?
  AND (? IS NULL OR ts < ?)
ORDER BY ts DESC
LIMIT ?`
	var cur sql.NullInt64
	if beforeTS != nil {
		cur = sql.NullInt64{Int64: *beforeTS, Valid: true}
	}
	rows, err := db.QueryContext(ctx, q, sessionID, cur, cur, limit)
	if err != nil {
		return nil, false, fmt.Errorf("session events: %w", err)
	}
	defer rows.Close()

	out := make([]EventRow, 0, limit)
	for rows.Next() {
		var (
			r     EventRow
			ts    int64
			attrs []byte
		)
		if err := rows.Scan(&ts, &r.EventName, &r.PromptID, &attrs); err != nil {
			return nil, false, fmt.Errorf("session events scan: %w", err)
		}
		r.TS = time.Unix(0, ts).UTC()
		r.Summary = summarize(r.EventName, attrs)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("session events iter: %w", err)
	}
	return out, len(out) == limit, nil
}
