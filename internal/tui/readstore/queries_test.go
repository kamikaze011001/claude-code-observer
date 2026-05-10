package readstore_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

func TestDashboardSnapshot_AggregatesByWindow(t *testing.T) {
	home := t.TempDir()
	repo, err := repository.Open(home)
	if err != nil {
		t.Fatalf("repository.Open: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	startOfDay := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	twoDaysAgo := startOfDay.Add(-2 * 24 * time.Hour)
	tenDaysAgo := startOfDay.Add(-10 * 24 * time.Hour)
	fortyDaysAgo := startOfDay.Add(-40 * 24 * time.Hour)

	insertSession := func(id, project string, started time.Time, cost float64, prompts, tools, errors int) {
		_, err := repo.DB().ExecContext(context.Background(),
			`INSERT INTO sessions
			 (session_id, project_name, started_at, last_seen_at,
			  cost_usd, prompts, tool_calls, api_errors)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, project, started.UnixNano(), started.UnixNano(),
			cost, prompts, tools, errors)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	insertSession("today1", "obs", now, 1.50, 5, 20, 1)
	insertSession("today2", "obs", now.Add(time.Hour), 0.80, 3, 12, 0)
	insertSession("d2", "scratch", twoDaysAgo, 2.00, 8, 30, 0)
	insertSession("d10", "obs", tenDaysAgo, 4.00, 10, 40, 2)
	insertSession("d40", "obs", fortyDaysAgo, 99.00, 100, 500, 50)

	pool, err := readstore.OpenRO(filepath.Join(home, "db.sqlite"))
	if err != nil {
		t.Fatalf("OpenRO: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	snap, top, err := readstore.DashboardSnapshot(context.Background(), pool, now)
	if err != nil {
		t.Fatalf("DashboardSnapshot: %v", err)
	}

	if got, want := snap.Today.CostUSD, 2.30; got != want {
		t.Errorf("today cost: got %.2f want %.2f", got, want)
	}
	if got, want := snap.Today.Prompts, int64(8); got != want {
		t.Errorf("today prompts: got %d want %d", got, want)
	}
	if got, want := snap.D7.CostUSD, 4.30; got != want {
		t.Errorf("7d cost: got %.2f want %.2f", got, want)
	}
	if got, want := snap.D30.CostUSD, 8.30; got != want {
		t.Errorf("30d cost: got %.2f want %.2f", got, want)
	}
	if got, want := snap.D30.Errors, int64(3); got != want {
		t.Errorf("30d errors: got %d want %d", got, want)
	}

	if len(top) != 2 {
		t.Fatalf("top: got %d rows want 2", len(top))
	}
	if top[0].SessionID != "today1" || top[1].SessionID != "today2" {
		t.Errorf("top order wrong: %+v", top)
	}
}

// ---------------------------------------------------------------------------
// Helpers for SessionsPage tests
// ---------------------------------------------------------------------------

type seedSession struct {
	id        string
	project   string
	started   int64 // ns
	endedNull bool
	cost      float64
	prompts   int64
}

func tsNS(y int, m time.Month, d, h, mi, s int) int64 {
	return time.Date(y, m, d, h, mi, s, 0, time.UTC).UnixNano()
}

func seedSessions(t *testing.T, ss []seedSession) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := repository.Open(dir)
	if err != nil {
		t.Fatalf("repo open: %v", err)
	}
	defer repo.Close()
	for _, s := range ss {
		ended := sql.NullInt64{Int64: s.started + 60_000_000_000, Valid: !s.endedNull}
		_, err := repo.DB().Exec(`
			INSERT INTO sessions(session_id, project_name, started_at, last_seen_at, ended_at,
				cost_usd, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
				api_requests, api_errors, subagent_requests, auxiliary_requests, tool_calls, tool_denied, prompts)
			VALUES (?, NULLIF(?, ''), ?, ?, ?, ?, 0,0,0,0,0,0,0,0,0,0, ?)`,
			s.id, s.project, s.started, s.started+60_000_000_000, ended, s.cost, s.prompts)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	return filepath.Join(dir, "db.sqlite")
}

func openTestRO(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := readstore.OpenRO(path)
	if err != nil {
		t.Fatalf("openRO: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// SessionsPage tests
// ---------------------------------------------------------------------------

func TestSessionsPage_FirstPageNoCursor(t *testing.T) {
	t.Parallel()
	db := openTestRO(t, seedSessions(t, []seedSession{
		{id: "s1", project: "alpha", started: tsNS(2026, 5, 10, 12, 0, 0), endedNull: false, cost: 0.10, prompts: 1},
		{id: "s2", project: "beta", started: tsNS(2026, 5, 10, 11, 0, 0), endedNull: true, cost: 0.20, prompts: 2},
		{id: "s3", project: "", started: tsNS(2026, 5, 10, 10, 0, 0), endedNull: false, cost: 0.30, prompts: 3},
	}))
	rows, next, err := readstore.SessionsPage(t.Context(), db, nil, 50)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got, want := len(rows), 3; got != want {
		t.Fatalf("rows: got %d want %d", got, want)
	}
	if rows[0].SessionID != "s1" || rows[1].SessionID != "s2" || rows[2].SessionID != "s3" {
		t.Fatalf("DESC order broken: %#v", rows)
	}
	if !rows[1].Live {
		t.Fatal("s2 has ended_at NULL → Live should be true")
	}
	if rows[0].Live {
		t.Fatal("s1 has ended_at set → Live should be false")
	}
	if next != nil {
		t.Fatalf("len(rows) < limit, next must be nil; got %v", *next)
	}
}

func TestSessionsPage_KeysetPagination(t *testing.T) {
	t.Parallel()
	var seeds []seedSession
	for i := 0; i < 12; i++ {
		seeds = append(seeds, seedSession{
			id: fmt.Sprintf("s%02d", i), project: "p",
			started:   tsNS(2026, 5, 10, 0, 0, 0) + int64(i)*int64(time.Minute),
			cost:      0.01, prompts: 1, endedNull: false,
		})
	}
	db := openTestRO(t, seedSessions(t, seeds))
	page1, next1, err := readstore.SessionsPage(t.Context(), db, nil, 5)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 5 || next1 == nil {
		t.Fatalf("page1: rows=%d next=%v", len(page1), next1)
	}
	page2, next2, err := readstore.SessionsPage(t.Context(), db, next1, 5)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 5 || next2 == nil {
		t.Fatalf("page2: rows=%d next=%v", len(page2), next2)
	}
	page3, next3, err := readstore.SessionsPage(t.Context(), db, next2, 5)
	if err != nil {
		t.Fatalf("page3: %v", err)
	}
	if len(page3) != 2 || next3 != nil {
		t.Fatalf("page3 should be 2 rows + nil cursor; got rows=%d next=%v", len(page3), next3)
	}
	seen := map[string]bool{}
	for _, p := range [][]readstore.SessionRow{page1, page2, page3} {
		for _, r := range p {
			if seen[r.SessionID] {
				t.Fatalf("dup: %s", r.SessionID)
			}
			seen[r.SessionID] = true
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers for SessionEvents tests
// ---------------------------------------------------------------------------

type seedEvent struct {
	ts        int64
	sessionID string
	promptID  string // "" → NULL
	eventName string
	attrs     string // JSON
}

func seedEvents(t *testing.T, ss []seedSession, ee []seedEvent) string {
	t.Helper()
	path := seedSessions(t, ss)
	dir := filepath.Dir(path)
	repo, err := repository.Open(dir)
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	defer repo.Close()
	for _, e := range ee {
		var pid sql.NullString
		if e.promptID != "" {
			pid = sql.NullString{String: e.promptID, Valid: true}
		}
		_, err := repo.DB().Exec(
			`INSERT INTO events(ts, session_id, prompt_id, event_name, attrs) VALUES (?, ?, ?, ?, ?)`,
			e.ts, e.sessionID, pid, e.eventName, e.attrs,
		)
		if err != nil {
			t.Fatalf("event insert: %v", err)
		}
	}
	return path
}

// ---------------------------------------------------------------------------
// SessionEvents tests
// ---------------------------------------------------------------------------

func TestSessionEvents_DESCAndCursor(t *testing.T) {
	t.Parallel()
	base := tsNS(2026, 5, 10, 12, 0, 0)
	ss := []seedSession{{id: "s1", project: "p", started: base, cost: 0, prompts: 0, endedNull: true}}
	var ee []seedEvent
	for i := 0; i < 6; i++ {
		ee = append(ee, seedEvent{
			ts:        base + int64(i)*int64(time.Second),
			sessionID: "s1",
			eventName: "tool_result",
			attrs:     fmt.Sprintf(`{"tool_name":"Read","duration_ms":%d,"success":true}`, i),
		})
	}
	db := openTestRO(t, seedEvents(t, ss, ee))

	rows, hasMore, err := readstore.SessionEvents(t.Context(), db, "s1", nil, 4)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(rows) != 4 || !hasMore {
		t.Fatalf("page1 rows=%d hasMore=%v want rows=4 hasMore=true", len(rows), hasMore)
	}
	if !rows[0].TS.After(rows[1].TS) {
		t.Fatal("rows must be DESC by ts")
	}
	if rows[0].Summary != "Read 5ms" {
		t.Errorf("summary[0] = %q; want %q", rows[0].Summary, "Read 5ms")
	}

	cursor := rows[len(rows)-1].TS.UnixNano()
	rows2, hasMore2, err := readstore.SessionEvents(t.Context(), db, "s1", &cursor, 4)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(rows2) != 2 || hasMore2 {
		t.Fatalf("page2 rows=%d hasMore=%v want rows=2 hasMore=false", len(rows2), hasMore2)
	}
}

func TestSessionEvents_PromptIDPreserved(t *testing.T) {
	t.Parallel()
	base := tsNS(2026, 5, 10, 12, 0, 0)
	ss := []seedSession{{id: "s1", project: "p", started: base, endedNull: true}}
	ee := []seedEvent{
		{ts: base, sessionID: "s1", promptID: "p1", eventName: "user_prompt", attrs: `{"prompt_length":12}`},
		{ts: base + 1, sessionID: "s1", promptID: "", eventName: "api_error", attrs: `{"error":"x"}`},
	}
	db := openTestRO(t, seedEvents(t, ss, ee))
	rows, _, err := readstore.SessionEvents(t.Context(), db, "s1", nil, 50)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].PromptID != "" {
		t.Errorf("rows[0].PromptID=%q want empty", rows[0].PromptID)
	}
	if rows[1].PromptID != "p1" {
		t.Errorf("rows[1].PromptID=%q want p1", rows[1].PromptID)
	}
}

// ---------------------------------------------------------------------------
// Helpers for PromptDetail tests
// ---------------------------------------------------------------------------

func seedPrompt(t *testing.T, sessionID, promptID string, started int64, cost float64) string {
	t.Helper()
	path := seedSessions(t, []seedSession{{id: sessionID, project: "p", started: started, endedNull: true}})
	dir := filepath.Dir(path)
	repo, err := repository.Open(dir)
	if err != nil {
		t.Fatalf("repo: %v", err)
	}
	defer repo.Close()
	_, err = repo.DB().Exec(`
		INSERT INTO prompts(prompt_id, session_id, started_at, ended_at, prompt_length, command_name, command_source,
			cost_usd, input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
			api_requests, subagent_requests, tool_calls, had_error)
		VALUES (?, ?, ?, ?, 100, NULL, NULL, ?, 1240, 312, 0, 0, 2, 0, 2, 0)`,
		promptID, sessionID, started, started+int64(4*time.Second), cost)
	if err != nil {
		t.Fatalf("prompt insert: %v", err)
	}
	for i, attrs := range []string{
		`{"model":"claude-opus-4-7","cost_usd":0.0021,"input_tokens":800,"output_tokens":120}`,
		`{"model":"claude-opus-4-7","cost_usd":0.0021,"input_tokens":440,"output_tokens":192}`,
	} {
		_, err := repo.DB().Exec(`INSERT INTO events(ts, session_id, prompt_id, event_name, attrs) VALUES (?, ?, ?, 'api_request', ?)`,
			started+int64(i+1)*int64(time.Second), sessionID, promptID, attrs)
		if err != nil {
			t.Fatalf("api_request insert: %v", err)
		}
	}
	for i, attrs := range []string{
		`{"tool_name":"Read","duration_ms":12,"success":true}`,
		`{"tool_name":"Bash","duration_ms":1245,"success":false}`,
	} {
		_, err := repo.DB().Exec(`INSERT INTO events(ts, session_id, prompt_id, event_name, attrs) VALUES (?, ?, ?, 'tool_result', ?)`,
			started+int64(i+3)*int64(time.Second), sessionID, promptID, attrs)
		if err != nil {
			t.Fatalf("tool_result insert: %v", err)
		}
	}
	return path
}

// ---------------------------------------------------------------------------
// PromptDetail tests
// ---------------------------------------------------------------------------

func TestPromptDetail_Found(t *testing.T) {
	t.Parallel()
	started := tsNS(2026, 5, 10, 12, 0, 0)
	db := openTestRO(t, seedPrompt(t, "s1", "p1", started, 0.0042))
	pd, err := readstore.PromptDetail(t.Context(), db, "p1")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if pd.Prompt.PromptID != "p1" {
		t.Errorf("prompt_id=%q", pd.Prompt.PromptID)
	}
	if pd.Prompt.CostUSD != 0.0042 {
		t.Errorf("cost=%v", pd.Prompt.CostUSD)
	}
	if len(pd.APIRequests) != 2 {
		t.Errorf("api_requests=%d", len(pd.APIRequests))
	}
	if len(pd.ToolCalls) != 2 {
		t.Errorf("tool_calls=%d", len(pd.ToolCalls))
	}
	if !pd.APIRequests[0].TS.Before(pd.APIRequests[1].TS) {
		t.Error("api_requests must be ASC ts")
	}
	if !pd.ToolCalls[0].TS.Before(pd.ToolCalls[1].TS) {
		t.Error("tool_calls must be ASC ts")
	}
	if pd.ToolCalls[1].Success {
		t.Error("second tool_result was success=false")
	}
	if pd.APIRequests[0].Model != "claude-opus-4-7" {
		t.Errorf("model=%q", pd.APIRequests[0].Model)
	}
}

func TestPromptDetail_NotFound(t *testing.T) {
	t.Parallel()
	db := openTestRO(t, seedSessions(t, nil))
	_, err := readstore.PromptDetail(t.Context(), db, "missing")
	if !errors.Is(err, readstore.ErrNotFound) {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}
