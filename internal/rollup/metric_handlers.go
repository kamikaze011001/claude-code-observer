package rollup

import (
	"math"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

// metricSessionOp wraps a sessionCounters into a single sessionCounterUpsert op
// keyed on the snapshot's session/ts. Returns nil when the session id is empty
// (cannot attribute the increment to any row).
func metricSessionOp(snap domain.MetricSnapshot, c sessionCounters) []Op {
	if snap.SessionID == "" {
		return nil
	}
	return []Op{{
		Query: sessionCounterUpsert,
		Args:  sessionCounterArgs(snap.SessionID, snap.TS, c),
	}}
}

// roundNonNeg rounds v to the nearest int64, clamping negatives to 0 (these are
// counts and durations; a negative delta is meaningless and ignored).
func roundNonNeg(v float64) int64 {
	if v <= 0 {
		return 0
	}
	return int64(math.Round(v))
}

func applyLinesOfCode(snap domain.MetricSnapshot) []Op {
	n := roundNonNeg(snap.Value)
	var c sessionCounters
	switch snapAttrString(snap.Attrs, "type") {
	case "added":
		c.LinesAdded = n
	case "removed":
		c.LinesRemoved = n
	default:
		return nil
	}
	return metricSessionOp(snap, c)
}

func applyCommit(snap domain.MetricSnapshot) []Op {
	return metricSessionOp(snap, sessionCounters{Commits: roundNonNeg(snap.Value)})
}

func applyPullRequest(snap domain.MetricSnapshot) []Op {
	return metricSessionOp(snap, sessionCounters{PullRequests: roundNonNeg(snap.Value)})
}

func applyActiveTime(snap domain.MetricSnapshot) []Op {
	if snapAttrString(snap.Attrs, "type") != "user" {
		return nil
	}
	return metricSessionOp(snap, sessionCounters{ActiveSeconds: roundNonNeg(snap.Value)})
}

func applyCodeEditDecision(snap domain.MetricSnapshot) []Op {
	n := roundNonNeg(snap.Value)
	var c sessionCounters
	switch snapAttrString(snap.Attrs, "decision") {
	case "accept":
		c.EditsAccepted = n
	case "reject":
		c.EditsRejected = n
	default:
		return nil
	}
	return metricSessionOp(snap, c)
}

// snapAttrString reads a string attr from a metric snapshot's attrs map.
func snapAttrString(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	if s, ok := attrs[key].(string); ok {
		return s
	}
	return ""
}

func init() {
	metricUpdaters[domain.MetricLinesOfCode] = applyLinesOfCode
	metricUpdaters[domain.MetricCommit] = applyCommit
	metricUpdaters[domain.MetricPullRequest] = applyPullRequest
	metricUpdaters[domain.MetricActiveTime] = applyActiveTime
	metricUpdaters[domain.MetricCodeEditToolDecision] = applyCodeEditDecision
}
