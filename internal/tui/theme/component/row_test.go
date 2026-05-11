package component

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestSessionRow_Width_ASCII(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := SessionRowData{
		Index:       1,
		Started:     time.Date(2026, 5, 11, 9, 14, 0, 0, time.UTC),
		ProjectName: "claude-code-observer",
		DurationSec: 4320,
		CostUSD:     1.12,
		Prompts:     12,
		Tokens:      38000,
		Live:        true,
	}
	out := SessionRow(&th, r, true, 90)
	if got := lipgloss.Width(out); got != 90 {
		t.Errorf("session row width (ascii): got %d want 90", got)
	}
}

func TestSessionRow_Width_CJK(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := SessionRowData{
		Index: 2, Started: time.Now(), ProjectName: "日本語プロジェクト", // 9 wide chars
		DurationSec: 60, CostUSD: 0.10, Prompts: 1, Tokens: 100, Live: false,
	}
	out := SessionRow(&th, r, false, 90)
	if got := lipgloss.Width(out); got != 90 {
		t.Errorf("session row width (cjk): got %d want 90", got)
	}
}

func TestSessionRow_Width_Emoji(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := SessionRowData{
		Index: 3, Started: time.Now(), ProjectName: "🚀-rocket-fast",
		DurationSec: 60, CostUSD: 0.10, Prompts: 1, Tokens: 100, Live: false,
	}
	out := SessionRow(&th, r, false, 90)
	if got := lipgloss.Width(out); got != 90 {
		t.Errorf("session row width (emoji): got %d want 90", got)
	}
}

func TestEventRow_Width(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	e := EventRowData{
		Time:      time.Date(2026, 5, 11, 9, 14, 8, 0, time.UTC),
		EventName: "user_prompt",
		Summary:   `"refactor receiver pipeline"`,
		IsPrompt:  true,
	}
	out := EventRow(&th, e, true, 70)
	if got := lipgloss.Width(out); got != 70 {
		t.Errorf("event row width: got %d want 70", got)
	}
}

func TestAPIRequestRow_Width(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := APIRequestRowData{
		Time: time.Date(2026, 5, 11, 9, 15, 43, 0, time.UTC),
		Model: "claude-opus-4-7", CostUSD: 0.21,
		InputTokens: 8481, OutputTokens: 2140,
	}
	out := APIRequestRow(&th, r, 70)
	if got := lipgloss.Width(out); got != 70 {
		t.Errorf("api row width: got %d want 70", got)
	}
}

func TestToolCallRow_Width(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := ToolCallRowData{
		Time: time.Date(2026, 5, 11, 9, 15, 46, 0, time.UTC),
		ToolName: "Write", Success: true, DurationMS: 112,
	}
	out := ToolCallRow(&th, r, 70)
	if got := lipgloss.Width(out); got != 70 {
		t.Errorf("tool row width: got %d want 70", got)
	}
}
