package repository

import (
	"context"
	"testing"
)

func TestDeleteEventsBefore_DeletesOnlyOldRows(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()

	ctx := context.Background()
	mustExec(t, repo, `INSERT INTO events (ts, session_id, event_name, attrs) VALUES (?, ?, ?, ?)`, 100, "s1", "user_prompt", "{}")
	mustExec(t, repo, `INSERT INTO events (ts, session_id, event_name, attrs) VALUES (?, ?, ?, ?)`, 500, "s1", "user_prompt", "{}")
	mustExec(t, repo, `INSERT INTO events (ts, session_id, event_name, attrs) VALUES (?, ?, ?, ?)`, 1000, "s1", "user_prompt", "{}")

	n, err := repo.DeleteEventsBefore(ctx, 500)
	if err != nil {
		t.Fatalf("DeleteEventsBefore: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows = %d, want 1 (only ts=100 < cutoff)", n)
	}

	var count int
	if err := repo.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("remaining rows = %d, want 2", count)
	}
}

func TestDeleteEventsBefore_EmptyTableNoop(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()
	n, err := repo.DeleteEventsBefore(context.Background(), 1000)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
}

func TestDeleteMetricSnapshotsBefore_DeletesOnlyOldRows(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()

	ctx := context.Background()
	mustExec(t, repo, `INSERT INTO metric_snapshots (ts, session_id, metric_name, value, attrs) VALUES (?, ?, ?, ?, ?)`, 100, "s1", "m", 1.0, "{}")
	mustExec(t, repo, `INSERT INTO metric_snapshots (ts, session_id, metric_name, value, attrs) VALUES (?, ?, ?, ?, ?)`, 500, "s1", "m", 1.0, "{}")
	mustExec(t, repo, `INSERT INTO metric_snapshots (ts, session_id, metric_name, value, attrs) VALUES (?, ?, ?, ?, ?)`, 1000, "s1", "m", 1.0, "{}")

	n, err := repo.DeleteMetricSnapshotsBefore(ctx, 500)
	if err != nil {
		t.Fatalf("DeleteMetricSnapshotsBefore: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
}

func TestDeleteMetricSnapshotsBefore_EmptyTableNoop(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()
	n, err := repo.DeleteMetricSnapshotsBefore(context.Background(), 1000)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 0 {
		t.Fatalf("rows = %d, want 0", n)
	}
}
