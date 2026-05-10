package readstore

import "testing"

func TestSummarize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		eventName string
		attrs     string
		want      string
	}{
		{"user_prompt with command", "claude_code.user_prompt", `{"prompt_length":142,"command_name":"commit"}`, "prompt: 142ch /commit"},
		{"user_prompt no command", "claude_code.user_prompt", `{"prompt_length":88}`, "prompt: 88ch"},
		{"user_prompt missing length", "claude_code.user_prompt", `{}`, "prompt"},
		{"tool_result success", "claude_code.tool_result", `{"tool_name":"Read","duration_ms":12,"success":true}`, "Read 12ms"},
		{"tool_result fail", "claude_code.tool_result", `{"tool_name":"Bash","duration_ms":1245,"success":false}`, "Bash 1245ms ✗"},
		{"tool_decision", "claude_code.tool_decision", `{"decision":"reject","tool_name":"Bash"}`, "reject Bash"},
		{"api_request", "claude_code.api_request", `{"model":"claude-opus-4-7","cost_usd":0.0021}`, "claude-opus-4-7 $0.0021"},
		{"api_error with message", "claude_code.api_error", `{"error":"timeout"}`, "error: timeout"},
		{"api_error with status only", "claude_code.api_error", `{"status_code":429}`, "error: 429"},
		{"unknown event", "claude_code.something_else", `{}`, "claude_code.something_else"},
		{"truncates over 60", "claude_code.tool_result", `{"tool_name":"Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","duration_ms":1}`, "Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa…"},
		{"bad json", "claude_code.api_request", `not json`, "claude_code.api_request"},
		{"tool_result missing duration", "claude_code.tool_result", `{"tool_name":"Read","success":true}`, "Read ?ms"},
		{"user_prompt fractional length (defensive)", "claude_code.user_prompt", `{"prompt_length":88.7}`, "prompt: 88ch"},
		{"compaction", "claude_code.compaction", `{"pre_tokens":12300,"post_tokens":4100,"trigger":"auto"}`, "compaction: 12300→4100 tok"},
		{"code_edit_decision", "claude_code.code_edit_tool.decision", `{"decision":"reject","tool_name":"Edit"}`, "Edit reject"},
		{"permission_mode", "claude_code.permission_mode_changed", `{"to":"acceptEdits"}`, "permission_mode → acceptEdits"},
		{"auth", "claude_code.auth", `{"event":"login"}`, "auth: login"},
		{"mcp_conn", "claude_code.mcp_server_connection", `{"server_name":"github","state":"connected"}`, "mcp github: connected"},
		{"internal_err", "claude_code.internal_error", `{"error":"oops"}`, "internal_error: oops"},
		{"plugin_installed", "claude_code.plugin_installed", `{"name":"foo"}`, "plugin installed: foo"},
		{"skill_activated", "claude_code.skill_activated", `{"name":"brainstorm"}`, "skill: brainstorm"},
		{"at_mention", "claude_code.at_mention", `{"target":"file"}`, "@mention: file"},
		{"retries_exhausted", "claude_code.api_retries_exhausted", `{"attempt":4}`, "api retries exhausted: 4"},
		{"hook_start", "claude_code.hook_execution_start", `{"hook":"PreToolUse"}`, "hook start: PreToolUse"},
		{"hook_complete", "claude_code.hook_execution_complete", `{"hook":"PreToolUse","duration_ms":12}`, "hook done: PreToolUse 12ms"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := summarize(c.eventName, []byte(c.attrs))
			if got != c.want {
				t.Errorf("summarize(%q, %s) = %q; want %q", c.eventName, c.attrs, got, c.want)
			}
		})
	}
}
