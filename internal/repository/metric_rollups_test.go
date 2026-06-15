package repository

import (
	"context"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestInsertMetricsAndApplyRollups_SumsDeltaSnapshots(t *testing.T) {
	repo, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer repo.Close()
	ctx := context.Background()

	// Non-monotonic delta series for lines_added on one session: total = 156+201+11 = 368.
	snaps := []domain.MetricSnapshot{
		{TS: 100, SessionID: "s1", MetricName: domain.MetricLinesOfCode, Value: 156, Attrs: map[string]any{"type": "added"}},
		{TS: 200, SessionID: "s1", MetricName: domain.MetricLinesOfCode, Value: 201, Attrs: map[string]any{"type": "added"}},
		{TS: 300, SessionID: "s1", MetricName: domain.MetricLinesOfCode, Value: 11, Attrs: map[string]any{"type": "added"}},
		{TS: 300, SessionID: "s1", MetricName: domain.MetricLinesOfCode, Value: 30, Attrs: map[string]any{"type": "removed"}},
		{TS: 350, SessionID: "s1", MetricName: domain.MetricCommit, Value: 2},
		{TS: 360, SessionID: "s1", MetricName: domain.MetricActiveTime, Value: 45, Attrs: map[string]any{"type": "user"}},
		{TS: 360, SessionID: "s1", MetricName: domain.MetricActiveTime, Value: 9999, Attrs: map[string]any{"type": "cli"}},
		{TS: 370, SessionID: "s1", MetricName: domain.MetricCodeEditToolDecision, Value: 1, Attrs: map[string]any{"decision": "accept"}},
		{TS: 371, SessionID: "s1", MetricName: domain.MetricCodeEditToolDecision, Value: 1, Attrs: map[string]any{"decision": "reject"}},
	}
	if err := repo.InsertMetricsAndApplyRollups(ctx, snaps); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	var added, removed, commits, active, acc, rej int64
	row := repo.DB().QueryRowContext(ctx,
		`SELECT lines_added, lines_removed, commits, active_seconds, edits_accepted, edits_rejected
		 FROM sessions WHERE session_id = 's1'`)
	if err := row.Scan(&added, &removed, &commits, &active, &acc, &rej); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if added != 368 {
		t.Errorf("lines_added = %d, want 368 (SUM of deltas, not MAX=201)", added)
	}
	if removed != 30 || commits != 2 || active != 45 || acc != 1 || rej != 1 {
		t.Errorf("removed=%d commits=%d active=%d acc=%d rej=%d; want 30/2/45/1/1",
			removed, commits, active, acc, rej)
	}

	var n int
	if err := repo.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM metric_snapshots`).Scan(&n); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if n != len(snaps) {
		t.Errorf("metric_snapshots rows = %d, want %d", n, len(snaps))
	}
}
