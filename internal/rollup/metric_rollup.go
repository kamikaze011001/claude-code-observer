package rollup

import (
	"log/slog"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

// MetricUpdater turns one metric datapoint into zero or more SQL ops. Like
// Updater, it is a pure function over the snapshot and never touches the DB.
//
// Claude Code exports these counters with DELTA temporality: each datapoint is
// the increment for its interval, so handlers accumulate additively via
// sessionCounterUpsert (col = col + excluded.col).
type MetricUpdater func(snap domain.MetricSnapshot) []Op

// metricUpdaters maps fully-qualified Claude Code metric names to their updater.
// Unknown names get no entry — ApplyMetric silently ignores them so the ingest
// path never breaks when upstream adds a metric.
var metricUpdaters = map[string]MetricUpdater{}

// ApplyMetric looks up the updater for snap.MetricName and returns its ops.
// Returns nil for unknown or empty names, after a debug log.
func ApplyMetric(snap domain.MetricSnapshot) []Op {
	if snap.MetricName == "" {
		return nil
	}
	u, ok := metricUpdaters[snap.MetricName]
	if !ok || u == nil {
		slog.Debug("rollup: no handler for metric", "name", snap.MetricName)
		return nil
	}
	return u(snap)
}
