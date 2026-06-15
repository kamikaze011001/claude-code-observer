package rollup

import (
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func wantOneSessionOp(t *testing.T, snap domain.MetricSnapshot) []any {
	t.Helper()
	ops := ApplyMetric(snap)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Query != sessionCounterUpsert {
		t.Fatalf("expected sessionCounterUpsert, got different query")
	}
	return ops[0].Args
}

const (
	idxLinesAdded    = 15
	idxLinesRemoved  = 16
	idxCommits       = 17
	idxPullRequests  = 18
	idxActiveSeconds = 19
	idxEditsAccepted = 20
	idxEditsRejected = 21
)

func TestMetric_LinesOfCode(t *testing.T) {
	cases := []struct {
		name              string
		typ               string
		value             float64
		wantAdded, wantRm int64
	}{
		{"added", "added", 156, 156, 0},
		{"removed", "removed", 12, 0, 12},
		{"unknown type ignored", "weird", 99, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.wantAdded == 0 && c.wantRm == 0 {
				// unknown type → no op at all
				if ops := ApplyMetric(domain.MetricSnapshot{
					MetricName: domain.MetricLinesOfCode, SessionID: "s1", TS: 1000,
					Value: c.value, Attrs: map[string]any{"type": c.typ},
				}); ops != nil {
					t.Fatalf("expected nil ops for unknown type, got %d", len(ops))
				}
				return
			}
			args := wantOneSessionOp(t, domain.MetricSnapshot{
				MetricName: domain.MetricLinesOfCode, SessionID: "s1", TS: 1000,
				Value: c.value, Attrs: map[string]any{"type": c.typ},
			})
			if args[idxLinesAdded].(int64) != c.wantAdded {
				t.Errorf("lines_added = %v, want %d", args[idxLinesAdded], c.wantAdded)
			}
			if args[idxLinesRemoved].(int64) != c.wantRm {
				t.Errorf("lines_removed = %v, want %d", args[idxLinesRemoved], c.wantRm)
			}
		})
	}
}

func TestMetric_Commit(t *testing.T) {
	args := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricCommit, SessionID: "s1", TS: 1000, Value: 2,
	})
	if args[idxCommits].(int64) != 2 {
		t.Errorf("commits = %v, want 2", args[idxCommits])
	}
}

func TestMetric_PullRequest(t *testing.T) {
	args := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricPullRequest, SessionID: "s1", TS: 1000, Value: 1,
	})
	if args[idxPullRequests].(int64) != 1 {
		t.Errorf("pull_requests = %v, want 1", args[idxPullRequests])
	}
}

func TestMetric_ActiveTime_UserOnly(t *testing.T) {
	userArgs := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricActiveTime, SessionID: "s1", TS: 1000,
		Value: 42.6, Attrs: map[string]any{"type": "user"},
	})
	if userArgs[idxActiveSeconds].(int64) != 43 {
		t.Errorf("active_seconds = %v, want 43", userArgs[idxActiveSeconds])
	}
	// type=cli → ignored entirely (no op)
	if ops := ApplyMetric(domain.MetricSnapshot{
		MetricName: domain.MetricActiveTime, SessionID: "s1", TS: 1000,
		Value: 9999, Attrs: map[string]any{"type": "cli"},
	}); ops != nil {
		t.Errorf("active_time type=cli should produce no op, got %d", len(ops))
	}
}

func TestMetric_CodeEditDecision(t *testing.T) {
	acc := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricCodeEditToolDecision, SessionID: "s1", TS: 1000,
		Value: 1, Attrs: map[string]any{"decision": "accept"},
	})
	if acc[idxEditsAccepted].(int64) != 1 || acc[idxEditsRejected].(int64) != 0 {
		t.Errorf("accept: accepted=%v rejected=%v, want 1/0", acc[idxEditsAccepted], acc[idxEditsRejected])
	}
	rej := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricCodeEditToolDecision, SessionID: "s1", TS: 1000,
		Value: 1, Attrs: map[string]any{"decision": "reject"},
	})
	if rej[idxEditsRejected].(int64) != 1 || rej[idxEditsAccepted].(int64) != 0 {
		t.Errorf("reject: accepted=%v rejected=%v, want 0/1", rej[idxEditsAccepted], rej[idxEditsRejected])
	}
}

func TestMetric_AllProductivityMetricsHaveHandler(t *testing.T) {
	for _, name := range []string{
		domain.MetricLinesOfCode, domain.MetricCommit, domain.MetricPullRequest,
		domain.MetricActiveTime, domain.MetricCodeEditToolDecision,
	} {
		if _, ok := metricUpdaters[name]; !ok {
			t.Errorf("metricUpdaters missing handler for %q", name)
		}
	}
}

func TestMetric_CostAndTokenNotRolledUp(t *testing.T) {
	for _, name := range []string{domain.MetricCostUsage, domain.MetricTokenUsage, domain.MetricSessionCount} {
		if _, ok := metricUpdaters[name]; ok {
			t.Errorf("metricUpdaters must NOT handle %q (double-count risk)", name)
		}
	}
}
