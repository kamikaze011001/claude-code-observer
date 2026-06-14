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

func TestTurnHeaderRow_Width(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := TurnHeaderRowData{
		Time: time.Date(2026, 6, 14, 15, 4, 1, 0, time.UTC),
		Label: "/refactor", PromptLength: 412, DurationSec: 11,
		Calls: 3, CostUSD: 0.036, Expanded: true,
	}
	for _, w := range []int{60, 90, 120} {
		out := TurnHeaderRow(&th, r, true, w)
		if got := lipgloss.Width(out); got != w {
			t.Errorf("turn header width: got %d want %d", got, w)
		}
	}
}

func TestTurnChildRow_Width(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	api := TurnChildRowData{Kind: "api", Model: "claude-opus-4-8", CostUSD: 0.031, InputTokens: 4800, OutputTokens: 910, Last: false}
	tool := TurnChildRowData{Kind: "tool", ToolName: "Read", Success: true, DurationMS: 38, Last: true}
	for _, w := range []int{40, 60, 90, 120} {
		for _, rd := range []TurnChildRowData{api, tool} {
			out := TurnChildRow(&th, rd, w)
			if got := lipgloss.Width(out); got != w {
				t.Errorf("turn child width (%s) at %d: got %d want %d", rd.Kind, w, got, w)
			}
		}
	}
}

// TestTurnChildRow_ToolBranch_NarrowWidths covers the degenerate-width guard
// (Fix 1) and the mark-column approach (Fix 2) across both glyph modes.
//
// Width arithmetic for "38ms" (durLen=4): overhead = tagCW(5)+1+markCW(2)+1+durLen = 13.
// bodyW = width - connW(4) - costW(8) - gutter(2) = width - 14.
//   width=24 → bodyW=10 → overhead(13) > bodyW → degenerate path.
//   width=28 → bodyW=14 → overhead(13) ≤ bodyW → normal path (nameAvail=1).
// For "10000ms" (durLen=7): overhead=16 → degenerate at width=24 and width=28.
func TestTurnChildRow_ToolBranch_NarrowWidths(t *testing.T) {
	glyphModes := []struct {
		name   string
		glyphs theme.Glyphs
	}{
		{"unicode", theme.UnicodeGlyphs()},
		{"nerd", theme.NerdGlyphs()},
	}

	rowCases := []struct {
		name  string
		rd    TurnChildRowData
		width int
	}{
		// Overflow trigger: overhead=13 > bodyW=10 → degenerate path.
		{"tool_38ms_w24", TurnChildRowData{Kind: "tool", ToolName: "Read", Success: true, DurationMS: 38}, 24},
		// Just above trigger: bodyW=14, overhead=13 → normal path (nameAvail=1).
		{"tool_38ms_w28", TurnChildRowData{Kind: "tool", ToolName: "Read", Success: true, DurationMS: 38}, 28},
		// Large duration: "10000ms" → overhead=16; degenerate at both widths.
		{"tool_10000ms_w24", TurnChildRowData{Kind: "tool", ToolName: "Write", Success: false, DurationMS: 10000}, 24},
		{"tool_10000ms_w28", TurnChildRowData{Kind: "tool", ToolName: "Write", Success: false, DurationMS: 10000}, 28},
		// Long tool name at degenerate width — name is truncated into the fallback body.
		{"long_toolname_w24", TurnChildRowData{Kind: "tool", ToolName: "SomeVeryLongToolNameThatExceeds", Success: true, DurationMS: 38}, 24},
		// Long tool name at normal width — name is truncated to nameAvail.
		{"long_toolname_w40", TurnChildRowData{Kind: "tool", ToolName: "SomeVeryLongToolNameThatExceeds", Success: true, DurationMS: 38}, 40},
	}

	for _, gm := range glyphModes {
		th := theme.Build(theme.MochaPalette(), gm.glyphs)
		for _, tc := range rowCases {
			t.Run(gm.name+"/"+tc.name, func(t *testing.T) {
				out := TurnChildRow(&th, tc.rd, tc.width)
				if got := lipgloss.Width(out); got != tc.width {
					t.Errorf("TurnChildRow(%s, w=%d): got width %d, want %d",
						tc.name, tc.width, got, tc.width)
				}
			})
		}
	}
}
