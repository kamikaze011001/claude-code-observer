package rollup

import "github.com/kamikaze011001/claude-code-observer/internal/domain"

// applyToolDecision bumps the per-session tool_denied counter when the user
// rejects a tool invocation. Spec values are accept|reject (see
// docs/CLAUDE-CODE-OTEL.md §8.5).
func applyToolDecision(ev domain.Event) []Op {
	var denied int64
	if attrString(ev.Attrs, "decision") == "reject" {
		denied = 1
	}
	return []Op{{
		Query: sessionCounterUpsert,
		Args:  sessionCounterArgs(ev.SessionID, ev.TS, sessionCounters{ToolDenied: denied}),
	}}
}

func init() {
	updaters[domain.EventToolDecision] = applyToolDecision
}
