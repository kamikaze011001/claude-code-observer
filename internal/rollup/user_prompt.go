package rollup

import "github.com/kamikaze011001/claude-code-observer/internal/domain"

const promptMetadataUpsert = `INSERT INTO prompts (
    prompt_id, session_id, started_at,
    prompt_length, command_name, command_source
) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(prompt_id) DO UPDATE SET
    started_at     = MIN(started_at, excluded.started_at),
    prompt_length  = COALESCE(prompt_length,  excluded.prompt_length),
    command_name   = COALESCE(command_name,   excluded.command_name),
    command_source = COALESCE(command_source, excluded.command_source)`

const sessionCounterUpsert = `INSERT INTO sessions (
    session_id, started_at, last_seen_at,
    input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
    cost_usd, api_requests, api_errors, subagent_requests, auxiliary_requests,
    tool_calls, tool_denied, prompts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET
    started_at            = MIN(started_at, excluded.started_at),
    last_seen_at          = MAX(last_seen_at, excluded.last_seen_at),
    input_tokens          = input_tokens          + excluded.input_tokens,
    output_tokens         = output_tokens         + excluded.output_tokens,
    cache_read_tokens     = cache_read_tokens     + excluded.cache_read_tokens,
    cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
    cost_usd              = cost_usd              + excluded.cost_usd,
    api_requests          = api_requests          + excluded.api_requests,
    api_errors            = api_errors            + excluded.api_errors,
    subagent_requests     = subagent_requests     + excluded.subagent_requests,
    auxiliary_requests    = auxiliary_requests    + excluded.auxiliary_requests,
    tool_calls            = tool_calls            + excluded.tool_calls,
    tool_denied           = tool_denied           + excluded.tool_denied,
    prompts               = prompts               + excluded.prompts`

// sessionCounterArgs builds the args slice for sessionCounterUpsert.
// Pass 0 for any counter that this updater does not bump.
type sessionCounters struct {
	InputTokens, OutputTokens, CacheReadTokens, CacheCreationTokens int64
	CostUSD                                                          float64
	APIRequests, APIErrors, SubagentRequests, AuxiliaryRequests       int64
	ToolCalls, ToolDenied, Prompts                                    int64
}

func sessionCounterArgs(sessionID string, ts int64, c sessionCounters) []any {
	return []any{
		sessionID, ts, ts,
		c.InputTokens, c.OutputTokens, c.CacheReadTokens, c.CacheCreationTokens,
		c.CostUSD, c.APIRequests, c.APIErrors, c.SubagentRequests, c.AuxiliaryRequests,
		c.ToolCalls, c.ToolDenied, c.Prompts,
	}
}

// promptCounterUpsert / promptCounterArgs are introduced by Task 6 (api_request).

func applyUserPrompt(ev domain.Event) []Op {
	ops := make([]Op, 0, 2)
	if ev.PromptID != "" {
		ops = append(ops, Op{
			Query: promptMetadataUpsert,
			Args: []any{
				ev.PromptID, ev.SessionID, ev.TS,
				attrInt64(ev.Attrs, "prompt_length"),
				attrString(ev.Attrs, "command_name"),
				attrString(ev.Attrs, "command_source"),
			},
		})
	}
	ops = append(ops, Op{
		Query: sessionCounterUpsert,
		Args:  sessionCounterArgs(ev.SessionID, ev.TS, sessionCounters{Prompts: 1}),
	})
	return ops
}

func init() {
	updaters["claude_code.user_prompt"] = applyUserPrompt
}
