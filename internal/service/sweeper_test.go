package service

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/scheduler"
)

func TestSweeper_Tick_ClosesIdleSessionsOnly(t *testing.T) {
	repo := openTempRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	idleNs := now.Add(-31 * time.Minute).UnixNano()
	mustExec(t, repo.DB(), `INSERT INTO sessions (session_id, started_at, last_seen_at) VALUES (?, ?, ?)`,
		"idle", idleNs, idleNs)
	mustExec(t, repo.DB(), `INSERT INTO sessions (session_id, started_at, last_seen_at) VALUES (?, ?, ?)`,
		"fresh", now.UnixNano(), now.UnixNano())

	clock := scheduler.NewFakeClock(now)
	sw := NewSweeper(repo, clock, 30*time.Minute, silentLogger())
	if err := sw.Tick(ctx); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	var endedAt int64
	if err := repo.DB().QueryRowContext(ctx,
		`SELECT ended_at FROM sessions WHERE session_id = 'idle'`).Scan(&endedAt); err != nil {
		t.Fatalf("query idle: %v", err)
	}
	if endedAt == 0 {
		t.Fatalf("idle session not closed")
	}

	var freshEnded any
	if err := repo.DB().QueryRowContext(ctx,
		`SELECT ended_at FROM sessions WHERE session_id = 'fresh'`).Scan(&freshEnded); err != nil {
		t.Fatalf("query fresh: %v", err)
	}
	if freshEnded != nil {
		t.Fatalf("fresh session was closed: %v", freshEnded)
	}
}

func TestSweeper_Tick_HonorsCustomIdle(t *testing.T) {
	repo := openTempRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	sixAgo := now.Add(-6 * time.Minute).UnixNano()
	mustExec(t, repo.DB(), `INSERT INTO sessions (session_id, started_at, last_seen_at) VALUES (?, ?, ?)`,
		"s1", sixAgo, sixAgo)

	clock := scheduler.NewFakeClock(now)
	sw := NewSweeper(repo, clock, 5*time.Minute, silentLogger())
	if err := sw.Tick(ctx); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	var endedAt int64
	if err := repo.DB().QueryRowContext(ctx,
		`SELECT ended_at FROM sessions WHERE session_id = 's1'`).Scan(&endedAt); err != nil {
		t.Fatalf("query: %v", err)
	}
	if endedAt == 0 {
		t.Fatalf("session not closed under 5-min threshold")
	}
}

func TestSweeper_Tick_EmptyTableNoError(t *testing.T) {
	repo := openTempRepo(t)
	clock := scheduler.NewFakeClock(time.Now())
	sw := NewSweeper(repo, clock, 30*time.Minute, silentLogger())
	if err := sw.Tick(context.Background()); err != nil {
		t.Fatalf("Tick on empty table: %v", err)
	}
}

func mustExec(t *testing.T, db *sql.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
