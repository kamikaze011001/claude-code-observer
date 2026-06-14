package component

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// Column widths for SessionRow and formatColHeader — single source of truth.
// Both SessionRow and sessions.formatColHeader reference these exported constants
// so header labels and row cells always stay aligned.
const (
	ColIdxW        = 4
	ColStartW      = 18
	ColDurW        = 10
	ColCostW       = 8
	ColBarW        = 10
	ColPrW         = 8
	ColTokW        = 7
	ColLiveW       = 8
	ColGutterCount = 8 // single-space gutters between the 9 columns
)

// ProjMinW is the minimum project-column display width before the spend bar
// begins to shrink. The bar absorbs the width deficit first so the project
// name stays readable on narrower terminals.
const ProjMinW = 8

// TruncToWidth is the exported form of truncToWidth, for use by sibling
// packages that render header labels with the same truncation rules.
func TruncToWidth(s string, w int) string { return truncToWidth(s, w) }

// SessionRowData is one row in the sessions list or in a dashboard panel.
type SessionRowData struct {
	Index       int       // 1-based position in the page (0 to omit)
	Started     time.Time
	ProjectName string
	DurationSec int64 // 0 to omit
	CostUSD     float64
	MaxCostUSD  float64 // largest cost on the page; scales the spend bar (0 => empty bar)
	Prompts     int64
	Tokens      int64 // 0 to omit
	Live        bool
}

// SessionRow renders one row inside a sessions table card. width is the
// content area inside the card border (caller computes via Budget). The
// returned line satisfies lipgloss.Width(out) == width.
func SessionRow(t *theme.Theme, r SessionRowData, selected bool, width int) string {
	// Compute project column width with full bar first.
	projW := width - ColIdxW - ColStartW - ColDurW - ColCostW - ColBarW - ColPrW - ColTokW - ColLiveW - ColGutterCount
	effectiveBarW := ColBarW

	// On narrow terminals the spend bar shrinks first so the project column
	// stays readable down to ProjMinW cells.
	if projW < ProjMinW {
		deficit := ProjMinW - projW
		effectiveBarW = ColBarW - deficit
		if effectiveBarW < 0 {
			effectiveBarW = 0
		}
		projW = ProjMinW
	}

	// Safety clamp: if even with zero bar we can't reach ProjMinW, shrink
	// projW so the total content never exceeds width.
	maxProjW := width - ColIdxW - ColStartW - ColDurW - ColCostW - effectiveBarW - ColPrW - ColTokW - ColLiveW - ColGutterCount
	if maxProjW < projW {
		projW = maxProjW
		if projW < 0 {
			projW = 0
		}
	}

	idx := padRight(fmt.Sprintf("%d", r.Index), ColIdxW)
	start := padRight(r.Started.Format("2006-01-02 15:04"), ColStartW)
	project := padRight(truncToWidth(r.ProjectName, projW), projW)
	dur := padRight(humanDuration(r.DurationSec), ColDurW)
	cost := padRight(CostText(t, r.CostUSD), ColCostW)
	bar := padRight(costBar(t, r.CostUSD, r.MaxCostUSD, effectiveBarW), effectiveBarW)
	prompts := padRight(fmt.Sprintf("%d", r.Prompts), ColPrW)
	tokens := padRight(HumanInt(r.Tokens), ColTokW)
	live := padRight("", ColLiveW)
	if r.Live {
		live = padRight(StatusPill(t, StatusLive), ColLiveW)
	}

	line := lipgloss.JoinHorizontal(lipgloss.Top,
		idx, " ", start, " ", project, " ", dur, " ", cost, " ", bar, " ", prompts, " ", tokens, " ", live,
	)
	if selected {
		line = lipgloss.NewStyle().Background(t.Palette.BgAlt).Width(width).Render(line)
	} else {
		line = lipgloss.NewStyle().Width(width).Render(line)
	}
	return line
}

// costBar renders a proportional spend bar w cells wide: filled cells are
// tier-colored, the remainder is muted track. max<=0 => empty track.
func costBar(t *theme.Theme, cost, max float64, w int) string {
	if w <= 0 {
		return ""
	}
	filled := 0
	if max > 0 {
		filled = int(math.Round(cost / max * float64(w)))
		if filled > w {
			filled = w
		}
		// filled < 0 is unreachable (cost >= 0 and max > 0), kept as defensive clamp.
		if filled < 0 {
			filled = 0
		}
	}
	full := lipgloss.NewStyle().Foreground(CostColor(t, cost)).Render(strings.Repeat("█", filled))
	track := lipgloss.NewStyle().Foreground(t.Palette.BgAlt).Render(strings.Repeat("░", w-filled))
	return full + track
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
