package rollup

import (
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApplyMetric_UnknownMetricReturnsNil(t *testing.T) {
	ops := ApplyMetric(domain.MetricSnapshot{MetricName: "claude_code.unknown.metric", Value: 5})
	if ops != nil {
		t.Fatalf("expected nil ops for unknown metric, got %d", len(ops))
	}
}

func TestApplyMetric_EmptyNameReturnsNil(t *testing.T) {
	if ops := ApplyMetric(domain.MetricSnapshot{}); ops != nil {
		t.Fatalf("expected nil for empty metric name")
	}
}

func TestApplyMetric_KnownMetricDispatches(t *testing.T) {
	ops := ApplyMetric(domain.MetricSnapshot{
		MetricName: domain.MetricCommit,
		SessionID:  "s1",
		TS:         1000,
		Value:      2,
	})
	if len(ops) != 1 {
		t.Fatalf("expected 1 op for commit metric, got %d", len(ops))
	}
}
