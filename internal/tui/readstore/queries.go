package readstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
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
	Tokens      int64
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
       prompts,
       input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens AS tokens
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
		if err := rows.Scan(&r.SessionID, &r.ProjectName, &started, &lastSeen, &ended, &r.CostUSD, &r.Prompts, &r.Tokens); err != nil {
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
	Sessions int64
	CostUSD  float64
	Prompts  int64
	Tokens   int64
	Tools    int64
	Errors   int64
}

// Snapshot is the dashboard's three-window rollup.
type Snapshot struct {
	Today         WindowStats
	Yesterday     WindowStats
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
  COALESCE(SUM(CASE WHEN started_at >= ? THEN 1                                                                                    END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd                                                                             END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts                                                                              END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens             END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls                                                                           END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors                                                                           END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN 1                                                                                    END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd                                                                             END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts                                                                              END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens             END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls                                                                           END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors                                                                           END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN 1                                                                                    END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd                                                                             END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts                                                                              END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens             END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls                                                                           END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors                                                                           END), 0)
FROM sessions
WHERE started_at >= ?`

	var s Snapshot
	err := db.QueryRowContext(ctx, q,
		today, today, today, today, today, today,
		d7, d7, d7, d7, d7, d7,
		d30, d30, d30, d30, d30, d30,
		d30,
	).Scan(
		&s.Today.Sessions, &s.Today.CostUSD, &s.Today.Prompts, &s.Today.Tokens, &s.Today.Tools, &s.Today.Errors,
		&s.D7.Sessions, &s.D7.CostUSD, &s.D7.Prompts, &s.D7.Tokens, &s.D7.Tools, &s.D7.Errors,
		&s.D30.Sessions, &s.D30.CostUSD, &s.D30.Prompts, &s.D30.Tokens, &s.D30.Tools, &s.D30.Errors,
	)
	if err != nil {
		return Snapshot{}, nil, fmt.Errorf("snapshot query: %w", err)
	}

	yStart := startOfDay.Add(-24 * time.Hour).UnixNano()
	yEnd := today
	const yQ = `
SELECT
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN 1                                                                                END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN cost_usd                                                                         END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN prompts                                                                          END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens         END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN tool_calls                                                                       END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN api_errors                                                                       END), 0)
FROM sessions
WHERE started_at >= ? AND started_at < ?`
	if err := db.QueryRowContext(ctx, yQ,
		yStart, yEnd, yStart, yEnd, yStart, yEnd,
		yStart, yEnd, yStart, yEnd, yStart, yEnd,
		yStart, yEnd,
	).Scan(
		&s.Yesterday.Sessions, &s.Yesterday.CostUSD, &s.Yesterday.Prompts,
		&s.Yesterday.Tokens, &s.Yesterday.Tools, &s.Yesterday.Errors,
	); err != nil {
		return Snapshot{}, nil, fmt.Errorf("yesterday snapshot: %w", err)
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

// RecentSessionsToday returns up to limit sessions started since the start of
// the UTC day containing now, newest-first. The shape is the same as
// TopSession so the dashboard can reuse the same row renderer.
func RecentSessionsToday(ctx context.Context, db *sql.DB, now time.Time, limit int) ([]TopSession, error) {
	if limit <= 0 {
		limit = 5
	}
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).UnixNano()
	const q = `
SELECT session_id, COALESCE(project_name, ''), started_at, cost_usd, prompts, ended_at IS NULL
FROM sessions
WHERE started_at >= ?
ORDER BY started_at DESC
LIMIT ?`
	rows, err := db.QueryContext(ctx, q, startOfDay, limit)
	if err != nil {
		return nil, fmt.Errorf("recent sessions query: %w", err)
	}
	defer rows.Close()
	var out []TopSession
	for rows.Next() {
		var r TopSession
		var live int
		if err := rows.Scan(&r.SessionID, &r.ProjectName, &r.StartedAt, &r.CostUSD, &r.Prompts, &live); err != nil {
			return nil, fmt.Errorf("recent session scan: %w", err)
		}
		r.Live = live == 1
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recent sessions iter: %w", err)
	}
	return out, nil
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

// ErrNotFound is returned when a lookup-by-id query finds no row.
var ErrNotFound = errors.New("readstore: not found")

// Prompt is the row from the prompts rollup table.
type Prompt struct {
	PromptID            string
	SessionID           string
	StartedAt           time.Time
	EndedAt             time.Time
	PromptLength        int64
	CommandName         string
	CommandSource       string
	CostUSD             float64
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
	APIRequests         int64
	SubagentRequests    int64
	ToolCalls           int64
	HadError            bool
}

// ToolCall is one tool_result event associated with a prompt.
type ToolCall struct {
	TS         time.Time
	ToolName   string
	DurationMS int64
	Success    bool
}

// APIRequest is one api_request event associated with a prompt.
type APIRequest struct {
	TS           time.Time
	Model        string
	CostUSD      float64
	InputTokens  int64
	OutputTokens int64
}

// PromptDetailResult bundles a prompt row with its partitioned events.
type PromptDetailResult struct {
	Prompt      Prompt
	ToolCalls   []ToolCall
	APIRequests []APIRequest
}

// PromptDetail fetches the prompt row plus its tool_result and api_request
// events ordered ASC by ts. Returns ErrNotFound when no such prompt exists.
func PromptDetail(ctx context.Context, db *sql.DB, promptID string) (PromptDetailResult, error) {
	const q = `
SELECT prompt_id, session_id, started_at, ended_at,
       COALESCE(prompt_length, 0), COALESCE(command_name, ''), COALESCE(command_source, ''),
       cost_usd, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
       api_requests, subagent_requests, tool_calls, had_error
FROM prompts WHERE prompt_id = ?`
	var (
		p       Prompt
		started int64
		ended   sql.NullInt64
		hadErr  int
	)
	err := db.QueryRowContext(ctx, q, promptID).Scan(
		&p.PromptID, &p.SessionID, &started, &ended,
		&p.PromptLength, &p.CommandName, &p.CommandSource,
		&p.CostUSD, &p.InputTokens, &p.OutputTokens, &p.CacheReadTokens, &p.CacheCreationTokens,
		&p.APIRequests, &p.SubagentRequests, &p.ToolCalls, &hadErr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PromptDetailResult{}, ErrNotFound
	}
	if err != nil {
		return PromptDetailResult{}, fmt.Errorf("prompt detail: %w", err)
	}
	p.StartedAt = time.Unix(0, started).UTC()
	if ended.Valid {
		p.EndedAt = time.Unix(0, ended.Int64).UTC()
	}
	p.HadError = hadErr == 1

	evRows, err := db.QueryContext(ctx, `
SELECT ts, event_name, attrs
FROM events
WHERE prompt_id = ? AND event_name IN ('tool_result','api_request')
ORDER BY ts`, promptID)
	if err != nil {
		return PromptDetailResult{}, fmt.Errorf("prompt events: %w", err)
	}
	defer evRows.Close()

	out := PromptDetailResult{Prompt: p}
	for evRows.Next() {
		var (
			ts        int64
			eventName string
			attrs     []byte
		)
		if err := evRows.Scan(&ts, &eventName, &attrs); err != nil {
			return PromptDetailResult{}, fmt.Errorf("prompt event scan: %w", err)
		}
		ev := time.Unix(0, ts).UTC()
		var a map[string]any
		_ = json.Unmarshal(attrs, &a)
		switch eventName {
		case domain.EventToolResult:
			tc := ToolCall{TS: ev}
			tc.ToolName, _ = a["tool_name"].(string)
			if v, ok := a["duration_ms"].(float64); ok {
				tc.DurationMS = int64(v)
			}
			if v, ok := a["success"].(bool); ok {
				tc.Success = v
			}
			out.ToolCalls = append(out.ToolCalls, tc)
		case domain.EventAPIRequest:
			r := APIRequest{TS: ev}
			r.Model, _ = a["model"].(string)
			if v, ok := a["cost_usd"].(float64); ok {
				r.CostUSD = v
			}
			if v, ok := a["input_tokens"].(float64); ok {
				r.InputTokens = int64(v)
			}
			if v, ok := a["output_tokens"].(float64); ok {
				r.OutputTokens = int64(v)
			}
			out.APIRequests = append(out.APIRequests, r)
		}
	}
	if err := evRows.Err(); err != nil {
		return PromptDetailResult{}, fmt.Errorf("prompt events iter: %w", err)
	}
	return out, nil
}
