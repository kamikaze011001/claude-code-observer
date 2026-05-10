package rollup

import "github.com/kamikaze011001/claude-code-observer/internal/domain"

const promptCounterUpsert = `INSERT INTO prompts (
    prompt_id, session_id, started_at,
    input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
    cost_usd, api_requests, subagent_requests, tool_calls
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(prompt_id) DO UPDATE SET
    started_at            = MIN(started_at, excluded.started_at),
    input_tokens          = input_tokens          + excluded.input_tokens,
    output_tokens         = output_tokens         + excluded.output_tokens,
    cache_read_tokens     = cache_read_tokens     + excluded.cache_read_tokens,
    cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
    cost_usd              = cost_usd              + excluded.cost_usd,
    api_requests          = api_requests          + excluded.api_requests,
    subagent_requests     = subagent_requests     + excluded.subagent_requests,
    tool_calls            = tool_calls            + excluded.tool_calls`

type promptCounters struct {
	InputTokens, OutputTokens, CacheReadTokens, CacheCreationTokens int64
	CostUSD                                                          float64
	APIRequests, SubagentRequests, ToolCalls                         int64
}

func promptCounterArgs(promptID, sessionID string, ts int64, c promptCounters) []any {
	return []any{
		promptID, sessionID, ts,
		c.InputTokens, c.OutputTokens, c.CacheReadTokens, c.CacheCreationTokens,
		c.CostUSD, c.APIRequests, c.SubagentRequests, c.ToolCalls,
	}
}

func applyAPIRequest(ev domain.Event) []Op {
	in := attrInt64(ev.Attrs, "input_tokens")
	out := attrInt64(ev.Attrs, "output_tokens")
	cr := attrInt64(ev.Attrs, "cache_read_tokens")
	cc := attrInt64(ev.Attrs, "cache_creation_tokens")
	cost := attrFloat64(ev.Attrs, "cost_usd")

	var subagent, auxiliary int64
	switch attrString(ev.Attrs, "query_source") {
	case "subagent":
		subagent = 1
	case "auxiliary":
		auxiliary = 1
	}

	ops := make([]Op, 0, 2)
	if ev.PromptID != "" {
		ops = append(ops, Op{
			Query: promptCounterUpsert,
			Args: promptCounterArgs(ev.PromptID, ev.SessionID, ev.TS, promptCounters{
				InputTokens: in, OutputTokens: out,
				CacheReadTokens: cr, CacheCreationTokens: cc,
				CostUSD:     cost,
				APIRequests: 1, SubagentRequests: subagent,
			}),
		})
	}
	ops = append(ops, Op{
		Query: sessionCounterUpsert,
		Args: sessionCounterArgs(ev.SessionID, ev.TS, sessionCounters{
			InputTokens: in, OutputTokens: out,
			CacheReadTokens: cr, CacheCreationTokens: cc,
			CostUSD:     cost,
			APIRequests: 1, SubagentRequests: subagent, AuxiliaryRequests: auxiliary,
		}),
	})
	return ops
}

func init() {
	updaters[domain.EventAPIRequest] = applyAPIRequest
}
