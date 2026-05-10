package repository

import (
	"context"
	"testing"
)

func TestCloseIdleSessions_ClosesOnlyIdleOpenSessions(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()

	ctx := context.Background()

	// Three sessions:
	//   open-recent: open, last_seen_at = 1000  (not idle, cutoff = 500)
	//   open-idle:   open, last_seen_at = 100   (idle)
	//   closed:      already ended, last_seen_at = 50
	mustExec(t, repo, `INSERT INTO sessions (session_id, started_at, last_seen_at) VALUES (?, ?, ?)`, "open-recent", 1000, 1000)
	mustExec(t, repo, `INSERT INTO sessions (session_id, started_at, last_seen_at) VALUES (?, ?, ?)`, "open-idle", 50, 100)
	mustExec(t, repo, `INSERT INTO sessions (session_id, started_at, last_seen_at, ended_at) VALUES (?, ?, ?, ?)`, "closed", 10, 50, 50)

	n, err := repo.CloseIdleSessions(ctx, 500)
	if err != nil {
		t.Fatalf("CloseIdleSessions: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows affected = %d, want 1", n)
	}

	var endedAt int64
	if err := repo.DB().QueryRowContext(ctx,
		`SELECT ended_at FROM sessions WHERE session_id = ?`, "open-idle").Scan(&endedAt); err != nil {
		t.Fatalf("query open-idle: %v", err)
	}
	if endedAt != 100 {
		t.Fatalf("open-idle ended_at = %d, want 100", endedAt)
	}

	var endedNullable any
	if err := repo.DB().QueryRowContext(ctx,
		`SELECT ended_at FROM sessions WHERE session_id = ?`, "open-recent").Scan(&endedNullable); err != nil {
		t.Fatalf("query open-recent: %v", err)
	}
	if endedNullable != nil {
		t.Fatalf("open-recent ended_at = %v, want nil", endedNullable)
	}

	if err := repo.DB().QueryRowContext(ctx,
		`SELECT ended_at FROM sessions WHERE session_id = ?`, "closed").Scan(&endedAt); err != nil {
		t.Fatalf("query closed: %v", err)
	}
	if endedAt != 50 {
		t.Fatalf("closed ended_at = %d, want 50 (untouched)", endedAt)
	}
}

func TestCloseIdleSessions_EmptyTableNoop(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()
	n, err := repo.CloseIdleSessions(context.Background(), 1000)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
}

func mustExec(t *testing.T, r *Repository, q string, args ...any) {
	t.Helper()
	if _, err := r.DB().Exec(q, args...); err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}
