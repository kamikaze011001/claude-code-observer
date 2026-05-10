package domain

// Event names — emitted by Claude Code as OTLP log records.
// Source of truth: docs/CLAUDE-CODE-OTEL.md §8.
//
// Claude Code emits these as bare strings on LogRecord.event_name (no
// "claude_code." prefix). The eventparser also strips a leading
// "claude_code." defensively (see internal/eventparser/parser.go), so
// updaters keyed on these constants match either form on the wire.
const (
	EventUserPrompt            = "user_prompt"
	EventAPIRequest            = "api_request"
	EventAPIError              = "api_error"
	EventToolResult            = "tool_result"
	EventToolDecision          = "tool_decision"
	EventCompaction            = "compaction"
	EventPermissionModeChanged = "permission_mode_changed"
	EventAuth                  = "auth"
	EventMCPServerConnection   = "mcp_server_connection"
	EventInternalError         = "internal_error"
	EventPluginInstalled       = "plugin_installed"
	EventSkillActivated        = "skill_activated"
	EventAtMention             = "at_mention"
	EventAPIRetriesExhausted   = "api_retries_exhausted"
	EventHookExecutionStart    = "hook_execution_start"
	EventHookExecutionComplete = "hook_execution_complete"
	EventAPIRequestBody        = "api_request_body"
	EventAPIResponseBody       = "api_response_body"

	// Community-observed event names (not in official docs §8.8).
	// Retained because the existing rollup pipeline uses them.
	EventSessionStart = "session_start"
	EventSessionEnd   = "session_end"
)

// Metric names — emitted by Claude Code as OTLP metric datapoints.
// Source of truth: docs/CLAUDE-CODE-OTEL.md §7.1.
const (
	MetricSessionCount         = "claude_code.session.count"
	MetricLinesOfCode          = "claude_code.lines_of_code.count"
	MetricPullRequest          = "claude_code.pull_request.count"
	MetricCommit               = "claude_code.commit.count"
	MetricCostUsage            = "claude_code.cost.usage"
	MetricTokenUsage           = "claude_code.token.usage"
	MetricActiveTime           = "claude_code.active_time.total"
	MetricCodeEditToolDecision = "claude_code.code_edit_tool.decision"
)

// AllEventNames is the canonical list of Claude Code event names this build
// recognises. The rollup registry test asserts every entry has a handler.
var AllEventNames = []string{
	EventUserPrompt, EventAPIRequest, EventAPIError, EventToolResult,
	EventToolDecision, EventCompaction, EventPermissionModeChanged, EventAuth,
	EventMCPServerConnection, EventInternalError, EventPluginInstalled,
	EventSkillActivated, EventAtMention, EventAPIRetriesExhausted,
	EventHookExecutionStart, EventHookExecutionComplete, EventAPIRequestBody,
	EventAPIResponseBody, EventSessionStart, EventSessionEnd,
}
