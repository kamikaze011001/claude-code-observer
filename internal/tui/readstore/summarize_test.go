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
		{"tool_decision", "claude_code.tool_decision", `{"decision":"deny","tool_name":"Bash"}`, "deny Bash"},
		{"api_request", "claude_code.api_request", `{"model":"claude-opus-4-7","cost_usd":0.0021}`, "claude-opus-4-7 $0.0021"},
		{"api_error with message", "claude_code.api_error", `{"error":"timeout"}`, "error: timeout"},
		{"api_error with status only", "claude_code.api_error", `{"status_code":429}`, "error: 429"},
		{"unknown event", "claude_code.something_else", `{}`, "claude_code.something_else"},
		{"truncates over 60", "claude_code.tool_result", `{"tool_name":"Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","duration_ms":1}`, "Aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa…"},
		{"bad json", "claude_code.api_request", `not json`, "claude_code.api_request"},
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
