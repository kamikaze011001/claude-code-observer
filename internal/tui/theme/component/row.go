package component

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// SessionRowData is one row in the sessions list or in a dashboard panel.
type SessionRowData struct {
	Index       int       // 1-based position in the page (0 to omit)
	Started     time.Time
	ProjectName string
	DurationSec int64 // 0 to omit
	CostUSD     float64
	Prompts     int64
	Tokens      int64 // 0 to omit
	Live        bool
}

// SessionRow renders one row inside a sessions table card. width is the
// content area inside the card border (caller computes via Budget). The
// returned line satisfies lipgloss.Width(out) == width.
func SessionRow(t *theme.Theme, r SessionRowData, selected bool, width int) string {
	// Column widths. 7 single-space gutters between 8 columns.
	const (
		idxW        = 4
		startW      = 18
		durW        = 10
		costW       = 8
		prW         = 8
		tokW        = 7
		liveW       = 8
		gutterCount = 7 // spaces between idx, start, project, dur, cost, prompts, tokens, live
	)
	projW := width - idxW - startW - durW - costW - prW - tokW - liveW - gutterCount
	if projW < 4 {
		projW = 4
	}

	idx := padRight(fmt.Sprintf("%d", r.Index), idxW)
	start := padRight(r.Started.Format("2006-01-02 15:04"), startW)
	project := padRight(truncToWidth(r.ProjectName, projW), projW)
	dur := padRight(humanDuration(r.DurationSec), durW)
	cost := padRight(fmt.Sprintf("$%.2f", r.CostUSD), costW)
	prompts := padRight(fmt.Sprintf("%d", r.Prompts), prW)
	tokens := padRight(humanInt(r.Tokens), tokW)
	live := padRight("", liveW)
	if r.Live {
		live = padRight(StatusPill(t, StatusLive), liveW)
	}

	line := lipgloss.JoinHorizontal(lipgloss.Top,
		idx, " ", start, " ", project, " ", dur, " ", cost, " ", prompts, " ", tokens, " ", live,
	)
	if selected {
		line = lipgloss.NewStyle().Background(t.Palette.BgAlt).Width(width).Render(line)
	} else {
		line = lipgloss.NewStyle().Width(width).Render(line)
	}
	return line
}

// padRight returns s padded with spaces to exactly `w` display cells.
// s must already be at most w cells wide (use truncToWidth first if not).
func padRight(s string, w int) string {
	return lipgloss.NewStyle().Width(w).Render(s)
}

func truncToWidth(s string, w int) string {
	if runewidth.StringWidth(s) <= w {
		return s
	}
	return runewidth.Truncate(s, w, "…")
}

// EventRowData is one row in the session detail timeline.
type EventRowData struct {
	Time      time.Time
	EventName string
	Summary   string
	IsPrompt  bool // user_prompt rows get a tinted background
}

func EventRow(t *theme.Theme, e EventRowData, selected bool, width int) string {
	const (
		timeW   = 8  // "15:04:05"
		nameW   = 22
		gutters = 2 // two single-space gutters
	)
	sumW := width - timeW - nameW - gutters
	if sumW < 8 {
		sumW = 8
	}
	timeCol := padRight(e.Time.Format("15:04:05"), timeW)
	nameCol := padRight(truncToWidth(e.EventName, nameW), nameW)
	sumCol := padRight(truncToWidth(e.Summary, sumW), sumW)
	line := lipgloss.JoinHorizontal(lipgloss.Top, timeCol, " ", nameCol, " ", sumCol)
	s := lipgloss.NewStyle().Width(width)
	if selected {
		s = s.Background(t.Palette.BgAlt).Foreground(t.Palette.Accent)
	} else if e.IsPrompt {
		s = s.Background(t.Palette.BgAlt)
	} else {
		s = s.Foreground(t.Palette.FgMuted)
	}
	return s.Render(line)
}

// APIRequestRowData renders one api_request event.
type APIRequestRowData struct {
	Time         time.Time
	Model        string
	CostUSD      float64
	InputTokens  int64
	OutputTokens int64
}

func APIRequestRow(t *theme.Theme, r APIRequestRowData, width int) string {
	const (
		timeW   = 8
		modelW  = 18
		costW   = 8
		gutters = 3 // three single-space gutters
	)
	tailW := width - timeW - modelW - costW - gutters
	if tailW < 8 {
		tailW = 8
	}
	timeCol := padRight(r.Time.Format("15:04:05"), timeW)
	modelCol := padRight(truncToWidth(r.Model, modelW), modelW)
	costCol := padRight(t.Value.Render(fmt.Sprintf("$%.2f", r.CostUSD)), costW)
	tail := padRight(fmt.Sprintf("in %d  out %d", r.InputTokens, r.OutputTokens), tailW)
	line := lipgloss.JoinHorizontal(lipgloss.Top, timeCol, " ", modelCol, " ", costCol, " ", tail)
	return lipgloss.NewStyle().Width(width).Render(line)
}

// ToolCallRowData renders one tool_result event.
type ToolCallRowData struct {
	Time       time.Time
	ToolName   string
	Success    bool
	DurationMS int64
	Note       string
}

func ToolCallRow(t *theme.Theme, r ToolCallRowData, width int) string {
	const (
		timeW   = 8
		nameW   = 12
		markW   = 2
		durW    = 10
		gutters = 4 // four single-space gutters
	)
	noteW := width - timeW - nameW - markW - durW - gutters
	if noteW < 0 {
		noteW = 0
	}
	timeCol := padRight(r.Time.Format("15:04:05"), timeW)
	nameCol := padRight(truncToWidth(r.ToolName, nameW), nameW)
	mark := t.Glyphs.Check
	markStyle := lipgloss.NewStyle().Foreground(t.Palette.Green)
	if !r.Success {
		mark = t.Glyphs.Cross
		markStyle = lipgloss.NewStyle().Foreground(t.Palette.Red)
	}
	markCol := padRight(markStyle.Render(mark), markW)
	durCol := padRight(fmt.Sprintf("%dms", r.DurationMS), durW)
	noteCol := padRight(truncToWidth(r.Note, noteW), noteW)
	line := lipgloss.JoinHorizontal(lipgloss.Top, timeCol, " ", nameCol, " ", markCol, " ", durCol, " ", noteCol)
	return lipgloss.NewStyle().Width(width).Render(line)
}
