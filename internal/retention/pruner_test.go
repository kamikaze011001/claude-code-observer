package retention

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
	"github.com/kamikaze011001/claude-code-observer/internal/scheduler"
)

func TestPruner_Tick_DeletesEventsAndMetricsOlderThanRetention(t *testing.T) {
	repo := openTempRepo(t)
	ctx := context.Background()

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	dayNs := int64(24 * time.Hour)
	oldNs := now.Add(-31 * 24 * time.Hour).UnixNano()
	freshNs := now.UnixNano()

	mustExec(t, repo.DB(), `INSERT INTO events (ts, session_id, event_name, attrs) VALUES (?, ?, ?, ?)`, oldNs, "s1", "claude_code.user_prompt", "{}")
	mustExec(t, repo.DB(), `INSERT INTO events (ts, session_id, event_name, attrs) VALUES (?, ?, ?, ?)`, freshNs, "s1", "claude_code.user_prompt", "{}")
	mustExec(t, repo.DB(), `INSERT INTO metric_snapshots (ts, session_id, metric_name, value, attrs) VALUES (?, ?, ?, ?, ?)`, oldNs, "s1", "m", 1.0, "{}")
	mustExec(t, repo.DB(), `INSERT INTO metric_snapshots (ts, session_id, metric_name, value, attrs) VALUES (?, ?, ?, ?, ?)`, freshNs, "s1", "m", 1.0, "{}")
	mustExec(t, repo.DB(), `INSERT INTO sessions (session_id, started_at, last_seen_at) VALUES (?, ?, ?)`, "s1", oldNs, oldNs)
	mustExec(t, repo.DB(), `INSERT INTO prompts (prompt_id, session_id, started_at) VALUES (?, ?, ?)`, "p1", "s1", oldNs)

	clock := scheduler.NewFakeClock(now)
	p := New(repo, clock, 30*24*time.Hour, silentLogger())
	if err := p.Tick(ctx); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	assertCount(t, repo.DB(), "events", 1)
	assertCount(t, repo.DB(), "metric_snapshots", 1)
	assertCount(t, repo.DB(), "sessions", 1)
	assertCount(t, repo.DB(), "prompts", 1)

	_ = dayNs
}

func TestPruner_Tick_HonorsCustomRetention(t *testing.T) {
	repo := openTempRepo(t)
	ctx := context.Background()

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	tenAgo := now.Add(-10 * 24 * time.Hour).UnixNano()
	mustExec(t, repo.DB(), `INSERT INTO events (ts, session_id, event_name, attrs) VALUES (?, ?, ?, ?)`, tenAgo, "s1", "claude_code.user_prompt", "{}")

	clock := scheduler.NewFakeClock(now)
	p := New(repo, clock, 7*24*time.Hour, silentLogger())
	if err := p.Tick(ctx); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	assertCount(t, repo.DB(), "events", 0)
}

func TestPruner_Tick_Idempotent(t *testing.T) {
	repo := openTempRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	mustExec(t, repo.DB(), `INSERT INTO events (ts, session_id, event_name, attrs) VALUES (?, ?, ?, ?)`,
		now.Add(-31*24*time.Hour).UnixNano(), "s1", "claude_code.user_prompt", "{}")

	clock := scheduler.NewFakeClock(now)
	p := New(repo, clock, 30*24*time.Hour, silentLogger())
	if err := p.Tick(ctx); err != nil {
		t.Fatalf("first Tick: %v", err)
	}
	if err := p.Tick(ctx); err != nil {
		t.Fatalf("second Tick: %v", err)
	}
	assertCount(t, repo.DB(), "events", 0)
}

func TestPruner_Tick_EmptyTablesNoError(t *testing.T) {
	repo := openTempRepo(t)
	clock := scheduler.NewFakeClock(time.Now())
	p := New(repo, clock, 30*24*time.Hour, silentLogger())
	if err := p.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
}

func TestNew_NilLoggerUsesDefault(t *testing.T) {
	repo := openTempRepo(t)
	clock := scheduler.NewFakeClock(time.Now())
	// nil logger should not panic — falls back to slog.Default()
	p := New(repo, clock, 30*24*time.Hour, nil)
	if err := p.Tick(context.Background()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
}

func openTempRepo(t *testing.T) *repository.Repository {
	t.Helper()
	repo, err := repository.Open(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func assertCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s rows = %d, want %d", table, got, want)
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
