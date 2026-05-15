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
		{"user_prompt with command", "user_prompt", `{"prompt_length":142,"command_name":"commit"}`, "prompt: 142ch /commit"},
		{"user_prompt no command", "user_prompt", `{"prompt_length":88}`, "prompt: 88ch"},
		{"user_prompt missing length", "user_prompt", `{}`, "prompt"},
		{"tool_result success", "tool_result", `{"tool_name":"Read","duration_ms":12,"success":true}`, "Read 12ms"},
		{"tool_result fail", "tool_result", `{"tool_name":"Bash","duration_ms":1245,"success":false}`, "Bash 1245ms ✗"},
		{"tool_result string-typed attrs", "tool_result", `{"tool_name":"Bash","duration_ms":"459","success":"false"}`, "Bash 459ms ✗"},
		{"tool_result string success true", "tool_result", `{"tool_name":"Read","duration_ms":"34","success":"true"}`, "Read 34ms"},
		{"tool_decision", "tool_decision", `{"decision":"reject","tool_name":"Bash"}`, "reject Bash"},
		{"api_request", "api_request", `{"model":"claude-opus-4-7","cost_usd":0.0021}`, "claude-opus-4-7 $0.0021"},
		{"api_error with message", "api_error", `{"error":"timeout"}`, "error: timeout"},
		{"api_error with status only", "api_error", `{"status_code":429}`, "error: 429"},
		{"unknown event", "something_else", `{}`, "something_else"},
		{"truncates over 60", "tool_result", `{"tool_name":"Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","duration_ms":1}`, "Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa…"},
		{"bad json", "api_request", `not json`, "api_request"},
		{"tool_result missing duration", "tool_result", `{"tool_name":"Read","success":true}`, "Read ?ms"},
		{"user_prompt fractional length (defensive)", "user_prompt", `{"prompt_length":88.7}`, "prompt: 88ch"},
		{"compaction", "compaction", `{"pre_tokens":12300,"post_tokens":4100,"trigger":"auto"}`, "compaction: 12300→4100 tok"},
		{"code_edit_decision", "claude_code.code_edit_tool.decision", `{"decision":"reject","tool_name":"Edit"}`, "Edit reject"},
		{"permission_mode", "permission_mode_changed", `{"to":"acceptEdits"}`, "permission_mode → acceptEdits"},
		{"auth", "auth", `{"event":"login"}`, "auth: login"},
		{"mcp_conn", "mcp_server_connection", `{"server_name":"github","state":"connected"}`, "mcp github: connected"},
		{"internal_err", "internal_error", `{"error":"oops"}`, "internal_error: oops"},
		{"plugin_installed", "plugin_installed", `{"name":"foo"}`, "plugin installed: foo"},
		{"skill_activated", "skill_activated", `{"name":"brainstorm"}`, "skill: brainstorm"},
		{"at_mention", "at_mention", `{"target":"file"}`, "@mention: file"},
		{"retries_exhausted", "api_retries_exhausted", `{"attempt":4}`, "api retries exhausted: 4"},
		{"hook_start", "hook_execution_start", `{"hook_name":"SessionStart:startup","hook_event":"SessionStart"}`, "hook start: SessionStart:startup"},
		{"hook_complete", "hook_execution_complete", `{"hook_name":"Stop","total_duration_ms":"9"}`, "hook done: Stop 9ms"},
		{"hook_complete falls back to hook_event", "hook_execution_complete", `{"hook_event":"PreToolUse","total_duration_ms":"12"}`, "hook done: PreToolUse 12ms"},
		{"hook_start missing name", "hook_execution_start", `{}`, "hook start: ?"},
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
