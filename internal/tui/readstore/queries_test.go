package readstore_test

import (
	"context"
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
