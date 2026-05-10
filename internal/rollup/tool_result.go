package rollup

import "github.com/kamikaze011001/claude-code-observer/internal/domain"

func applyToolResult(ev domain.Event) []Op {
	ops := make([]Op, 0, 2)
	if ev.PromptID != "" {
		ops = append(ops, Op{
			Query: promptCounterUpsert,
			Args:  promptCounterArgs(ev.PromptID, ev.SessionID, ev.TS, promptCounters{ToolCalls: 1}),
		})
	}
	ops = append(ops, Op{
		Query: sessionCounterUpsert,
		Args:  sessionCounterArgs(ev.SessionID, ev.TS, sessionCounters{ToolCalls: 1}),
	})
	return ops
}

func init() {
	updaters[domain.EventToolResult] = applyToolResult
}
