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

func TestSessionRow_WidthWithBar(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := SessionRowData{
		Index: 1, Started: time.Date(2026, 6, 14, 15, 4, 0, 0, time.UTC),
		ProjectName: "claude-code-observer", DurationSec: 662,
		CostUSD: 2.84, MaxCostUSD: 2.84, Prompts: 14, Tokens: 1_200_000, Live: false,
	}
	out := SessionRow(&th, r, false, 100)
	if got := lipgloss.Width(out); got != 100 {
		t.Errorf("session row width with bar: got %d want 100", got)
	}
}

func TestSessionRow_ZeroMaxCostNoPanic(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := SessionRowData{Index: 1, Started: time.Now(), ProjectName: "x", CostUSD: 0, MaxCostUSD: 0}
	out := SessionRow(&th, r, false, 100) // must not divide by zero
	if got := lipgloss.Width(out); got != 100 {
		t.Errorf("zero-max width: got %d want 100", got)
	}
}

// TestSessionRow_NarrowWidthInvariant asserts that SessionRow always returns
// exactly `width` display cells at various terminal widths, including those
// narrow enough to shrink or eliminate the spend bar.
func TestSessionRow_NarrowWidthInvariant(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := SessionRowData{
		Index:       1,
		Started:     time.Date(2026, 6, 14, 15, 4, 0, 0, time.UTC),
		ProjectName: "claude-code-observer",
		DurationSec: 662,
		CostUSD:     2.84,
		MaxCostUSD:  2.84,
		Prompts:     14,
		Tokens:      1_200_000,
		Live:        false,
	}
	widths := []int{60, 80, 90, 100, 120}
	for _, w := range widths {
		out := SessionRow(&th, r, false, w)
		if got := lipgloss.Width(out); got != w {
			t.Errorf("width %d: lipgloss.Width(SessionRow) = %d, want %d", w, got, w)
		}
	}
}

func TestAPIRequestRow_WidthWithCumulative(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := APIRequestRowData{
		Time: time.Date(2026, 6, 14, 15, 4, 9, 0, time.UTC),
		Model: "claude-opus-4-8", CostUSD: 0.031, CumulativeUSD: 0.035,
		InputTokens: 4800, OutputTokens: 910,
	}
	out := APIRequestRow(&th, r, 80)
	if got := lipgloss.Width(out); got != 80 {
		t.Errorf("api row width with cumulative: got %d want 80", got)
	}
}

func TestAPIRequestRow_NarrowWidthInvariant(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := APIRequestRowData{
		Time: time.Date(2026, 6, 14, 15, 4, 9, 0, time.UTC),
		Model: "claude-opus-4-8", CostUSD: 0.031, CumulativeUSD: 0.035,
		InputTokens: 4800, OutputTokens: 910,
	}
	widths := []int{40, 60, 80, 120}
	for _, w := range widths {
		out := APIRequestRow(&th, r, w)
		if got := lipgloss.Width(out); got != w {
			t.Errorf("width %d: lipgloss.Width(APIRequestRow) = %d, want %d", w, got, w)
		}
	}
}
