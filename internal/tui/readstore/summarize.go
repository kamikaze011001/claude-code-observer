package readstore

import (
	"encoding/json"
	"fmt"

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
		if d, ok := a["duration_ms"].(float64); ok {
			durStr = fmt.Sprintf("%dms", int(d))
		}
		mark := ""
		if ok, isBool := a["success"].(bool); isBool && !ok {
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
		hook, _ := a["hook"].(string)
		return truncRunes("hook start: "+hook, maxSummaryRunes)
	case domain.EventHookExecutionComplete:
		hook, _ := a["hook"].(string)
		dur := ""
		if d, ok := a["duration_ms"].(float64); ok {
			dur = fmt.Sprintf(" %dms", int(d))
		}
		return truncRunes("hook done: "+hook+dur, maxSummaryRunes)
	default:
		return truncRunes(eventName, maxSummaryRunes)
	}
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
