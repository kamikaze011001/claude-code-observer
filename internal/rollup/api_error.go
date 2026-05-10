package rollup

import "github.com/kamikaze011001/claude-code-observer/internal/domain"

const promptHadErrorUpsert = `INSERT INTO prompts (
    prompt_id, session_id, started_at, had_error
) VALUES (?, ?, ?, ?)
ON CONFLICT(prompt_id) DO UPDATE SET
    started_at = MIN(started_at, excluded.started_at),
    had_error  = 1`

func applyAPIError(ev domain.Event) []Op {
	ops := make([]Op, 0, 2)
	if ev.PromptID != "" {
		ops = append(ops, Op{
			Query: promptHadErrorUpsert,
			Args:  []any{ev.PromptID, ev.SessionID, ev.TS, int64(1)},
		})
	}
	ops = append(ops, Op{
		Query: sessionCounterUpsert,
		Args:  sessionCounterArgs(ev.SessionID, ev.TS, sessionCounters{APIErrors: 1}),
	})
	return ops
}

func init() {
	updaters[domain.EventAPIError] = applyAPIError
}
