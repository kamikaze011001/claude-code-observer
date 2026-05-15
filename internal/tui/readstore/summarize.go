package readstore

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

const maxSummaryRunes = 59

func summarize(eventName string, attrs []byte) string {
	var a map[string]any
	if err := json.Unmarshal(attrs, &a); err != nil {
		return truncRunes(eventName, maxSummaryRunes)
	}
	switch eventName {
	case domain.EventUserPrompt:
		if length, ok := a["prompt_length"]; ok {
			var lenStr string
			if f, isFloat := length.(float64); isFloat {
				lenStr = fmt.Sprintf("%dch", int(f))
			} else {
				lenStr = fmt.Sprintf("%vch", length)
			}
			if cmd, hasCmd := a["command_name"].(string); hasCmd && cmd != "" {
				return truncRunes("prompt: "+lenStr+" /"+cmd, maxSummaryRunes)
			}
			return truncRunes("prompt: "+lenStr, maxSummaryRunes)
		}
		return "prompt"
	case domain.EventToolResult:
		tool, _ := a["tool_name"].(string)
		durStr := "?ms"
		if d, ok := attrInt(a, "duration_ms"); ok {
			durStr = fmt.Sprintf("%dms", d)
		}
		mark := ""
		if ok, isBool := attrBool(a, "success"); isBool && !ok {
			mark = " ✗"
		}
		return truncRunes(fmt.Sprintf("%s %s%s", tool, durStr, mark), maxSummaryRunes)
	case domain.EventToolDecision:
		dec, _ := a["decision"].(string)
		tool, _ := a["tool_name"].(string)
		return truncRunes(fmt.Sprintf("%s %s", dec, tool), maxSummaryRunes)
	case domain.EventAPIRequest:
		model, _ := a["model"].(string)
		cost, _ := a["cost_usd"].(float64)
		return truncRunes(fmt.Sprintf("%s $%.4f", model, cost), maxSummaryRunes)
	case domain.EventAPIError:
		if msg, ok := a["error"].(string); ok && msg != "" {
			return truncRunes("error: "+msg, maxSummaryRunes)
		}
		if code := a["status_code"]; code != nil {
			return truncRunes(fmt.Sprintf("error: %v", code), maxSummaryRunes)
		}
		return "error"
	case "claude_code.code_edit_tool.decision":
		dec, _ := a["decision"].(string)
		tool, _ := a["tool_name"].(string)
		return truncRunes(fmt.Sprintf("%s %s", tool, dec), maxSummaryRunes)
	case domain.EventCompaction:
		pre, _ := a["pre_tokens"].(float64)
		post, _ := a["post_tokens"].(float64)
		return truncRunes(fmt.Sprintf("compaction: %d→%d tok", int(pre), int(post)), maxSummaryRunes)
	case domain.EventPermissionModeChanged:
		to, _ := a["to"].(string)
		return truncRunes("permission_mode → "+to, maxSummaryRunes)
	case domain.EventAuth:
		evt, _ := a["event"].(string)
		return truncRunes("auth: "+evt, maxSummaryRunes)
	case domain.EventMCPServerConnection:
		name, _ := a["server_name"].(string)
		state, _ := a["state"].(string)
		return truncRunes(fmt.Sprintf("mcp %s: %s", name, state), maxSummaryRunes)
	case domain.EventInternalError:
		msg, _ := a["error"].(string)
		return truncRunes("internal_error: "+msg, maxSummaryRunes)
	case domain.EventPluginInstalled:
		name, _ := a["name"].(string)
		return truncRunes("plugin installed: "+name, maxSummaryRunes)
	case domain.EventSkillActivated:
		name, _ := a["name"].(string)
		return truncRunes("skill: "+name, maxSummaryRunes)
	case domain.EventAtMention:
		target, _ := a["target"].(string)
		return truncRunes("@mention: "+target, maxSummaryRunes)
	case domain.EventAPIRetriesExhausted:
		var attempt int
		if f, ok := a["attempt"].(float64); ok {
			attempt = int(f)
		}
		return truncRunes(fmt.Sprintf("api retries exhausted: %d", attempt), maxSummaryRunes)
	case domain.EventHookExecutionStart:
		return truncRunes("hook start: "+hookLabel(a), maxSummaryRunes)
	case domain.EventHookExecutionComplete:
		dur := ""
		if d, ok := attrInt(a, "total_duration_ms"); ok {
			dur = fmt.Sprintf(" %dms", d)
		}
		return truncRunes("hook done: "+hookLabel(a)+dur, maxSummaryRunes)
	default:
		return truncRunes(eventName, maxSummaryRunes)
	}
}

// hookLabel picks the most descriptive identifier from a hook_execution event.
// Claude Code emits hook_name (e.g. "SessionStart:startup") and hook_event
// (e.g. "SessionStart"); prefer hook_name, fall back to hook_event, then "?".
func hookLabel(a map[string]any) string {
	if s, ok := a["hook_name"].(string); ok && s != "" {
		return s
	}
	if s, ok := a["hook_event"].(string); ok && s != "" {
		return s
	}
	return "?"
}

// attrInt coerces a JSON attribute to int64. Claude Code emits some numeric
// attributes (tool_result.duration_ms, hook total_duration_ms) as quoted
// strings rather than JSON numbers, so accept both forms.
func attrInt(a map[string]any, key string) (int64, bool) {
	switch v := a[key].(type) {
	case float64:
		return int64(v), true
	case string:
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

// attrBool coerces a JSON attribute to bool, accepting both native booleans
// and the quoted-string form ("true"/"false") that Claude Code emits on
// tool_result records.
func attrBool(a map[string]any, key string) (bool, bool) {
	switch v := a[key].(type) {
	case bool:
		return v, true
	case string:
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, false
		}
		return b, true
	default:
		return false, false
	}
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
