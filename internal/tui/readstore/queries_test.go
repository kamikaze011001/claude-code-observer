package readstore_test

import (
	"context"
	"database/sql"
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
