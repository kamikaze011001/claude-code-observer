package readstore

import (
	"encoding/json"
	"fmt"
)

// maxSummaryRunes is the column-width budget for event summary strings.
// The truncation produces at most maxSummaryRunes runes (including the
// ellipsis), so visible content is at most maxSummaryRunes-1 characters.
const maxSummaryRunes = 59

// summarize renders a one-line description of an event for the Session Detail
// timeline. It is total: any decoding failure falls back to event_name.
func summarize(eventName string, attrs []byte) string {
	var a map[string]any
	if err := json.Unmarshal(attrs, &a); err != nil {
		return truncRunes(eventName, maxSummaryRunes)
	}
	switch eventName {
	case "claude_code.user_prompt":
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
	case "claude_code.tool_result":
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
	case "claude_code.tool_decision":
		dec, _ := a["decision"].(string)
		tool, _ := a["tool_name"].(string)
		return truncRunes(fmt.Sprintf("%s %s", dec, tool), maxSummaryRunes)
	case "claude_code.api_request":
		model, _ := a["model"].(string)
		cost, _ := a["cost_usd"].(float64)
		return truncRunes(fmt.Sprintf("%s $%.4f", model, cost), maxSummaryRunes)
	case "claude_code.api_error":
		if msg, ok := a["error"].(string); ok && msg != "" {
			return truncRunes("error: "+msg, maxSummaryRunes)
		}
		if code := a["status_code"]; code != nil {
			return truncRunes(fmt.Sprintf("error: %v", code), maxSummaryRunes)
		}
		return "error"
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
