package domain

// Event names — emitted by Claude Code as OTLP log records.
// Source of truth: docs/CLAUDE-CODE-OTEL.md §8.
const (
	EventUserPrompt            = "claude_code.user_prompt"
	EventAPIRequest            = "claude_code.api_request"
	EventAPIError              = "claude_code.api_error"
	EventToolResult            = "claude_code.tool_result"
	EventToolDecision          = "claude_code.tool_decision"
	EventCompaction            = "claude_code.compaction"
	EventPermissionModeChanged = "claude_code.permission_mode_changed"
	EventAuth                  = "claude_code.auth"
	EventMCPServerConnection   = "claude_code.mcp_server_connection"
	EventInternalError         = "claude_code.internal_error"
	EventPluginInstalled       = "claude_code.plugin_installed"
	EventSkillActivated        = "claude_code.skill_activated"
	EventAtMention             = "claude_code.at_mention"
	EventAPIRetriesExhausted   = "claude_code.api_retries_exhausted"
	EventHookExecutionStart    = "claude_code.hook_execution_start"
	EventHookExecutionComplete = "claude_code.hook_execution_complete"
	EventAPIRequestBody        = "claude_code.api_request_body"
	EventAPIResponseBody       = "claude_code.api_response_body"

	// Community-observed event names (not in official docs §8.8).
	// Retained because the existing rollup pipeline uses them.
	EventSessionStart = "claude_code.session_start"
	EventSessionEnd   = "claude_code.session_end"
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
