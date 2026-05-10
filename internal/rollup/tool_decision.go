package rollup

import "github.com/kamikaze011001/claude-code-observer/internal/domain"

func applyToolDecision(ev domain.Event) []Op {
	var denied int64
	if attrString(ev.Attrs, "decision") == "deny" {
		denied = 1
	}
	return []Op{{
		Query: sessionCounterUpsert,
		Args:  sessionCounterArgs(ev.SessionID, ev.TS, sessionCounters{ToolDenied: denied}),
	}}
}

func init() {
	updaters["claude_code.tool_decision"] = applyToolDecision
}
