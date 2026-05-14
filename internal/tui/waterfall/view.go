package waterfall

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

const (
	laneLabelWidth = 11 // "auxiliary  " padded
	barGlyph       = "█"
)

// View renders the waterfall body. The shell renders chrome separately, but we
// mirror prompt.Detail and render an inner header + help bar for consistency.
func (m *Model) View(width, height int) string {
	th := m.th()
	if width <= 0 {
		width = 90
	}

	brand := th.Title.Render(th.Glyphs.Brand + " cco")
	bread := th.Muted.Render(" · waterfall " + shortID(m.promptID))
	pill := component.StatusPill(th, m.Status())
	headerRight := lipgloss.NewStyle().
		Width(width - lipgloss.Width(brand) - lipgloss.Width(bread)).
		Align(lipgloss.Right).Render(pill)
	header := lipgloss.JoinHorizontal(lipgloss.Top, brand, bread, headerRight)

	help := component.HelpBar(th, []component.KeyHint{
		{Key: "↑↓", Desc: "select"},
		{Key: "b", Desc: "back"},
		{Key: "r", Desc: "refresh"},
		{Key: "?", Desc: "about"},
		{Key: "q", Desc: "quit"},
	}, width)

	if m.notFound {
		body := th.Muted.Render("prompt not found — it may have been pruned")
		return strings.Join([]string{header, "", component.Card(th, "", body, width), "", help}, "\n")
	}
	if len(m.bars) == 0 {
		body := th.Muted.Render("no api requests for this prompt")
		return strings.Join([]string{header, "", component.Card(th, "", body, width), "", help}, "\n")
	}

	contentWidth := width - laneLabelWidth
	if contentWidth < 10 {
		contentWidth = 10
	}

	axis := m.renderAxis(th, contentWidth)
	lanes := []string{
		m.renderLane(th, LaneMain, contentWidth),
		m.renderLane(th, LaneSubagent, contentWidth),
		m.renderLane(th, LaneAuxiliary, contentWidth),
	}
	detail := m.renderDetail(th, width)

	parts := []string{header, "", axis, ""}
	parts = append(parts, lanes...)
	parts = append(parts, "", detail, "", help)
	return strings.Join(parts, "\n")
}

// renderAxis draws the relative time-axis header: "0ms ...... <span>ms".
func (m *Model) renderAxis(th *theme.Theme, contentWidth int) string {
	left := "0ms"
	right := fmt.Sprintf("%dms", m.totalSpanMS)
	gap := contentWidth - len(left) - len(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat("─", gap) + right
	return strings.Repeat(" ", laneLabelWidth) + th.Muted.Render(line)
}

// renderLane renders one lane: a padded label plus one or more packed sub-rows
// of bars. An empty lane renders a single "(none)" row.
func (m *Model) renderLane(th *theme.Theme, lane LaneKind, contentWidth int) string {
	var laneBars []Bar
	for _, b := range m.bars {
		if b.Lane == lane {
			laneBars = append(laneBars, b)
		}
	}
	label := padRight(lane.String(), laneLabelWidth)

	if len(laneBars) == 0 {
		return th.Label.Render(label) + th.Muted.Render("(none)")
	}

	rows := packLane(laneBars)
	var out []string
	for i, row := range rows {
		prefix := strings.Repeat(" ", laneLabelWidth)
		if i == 0 {
			prefix = th.Label.Render(label)
		}
		out = append(out, prefix+m.renderBarRow(th, row, contentWidth))
	}
	return strings.Join(out, "\n")
}

// renderBarRow paints one sub-row of non-overlapping bars onto a rune buffer.
// The bar under the cursor is rendered with the accent style; error bars use red.
func (m *Model) renderBarRow(th *theme.Theme, row []Bar, contentWidth int) string {
	cells := make([]string, contentWidth)
	for i := range cells {
		cells[i] = " "
	}
	for _, b := range row {
		startCol, w := scaleBar(b.OffsetMS, b.Req.DurationMS, m.totalSpanMS, contentWidth)
		style := th.Muted
		if b.Req.IsError {
			style = lipgloss.NewStyle().Foreground(th.Palette.Red)
		} else if m.isSelected(b) {
			style = th.Accent
		}
		for c := startCol; c < startCol+w && c < contentWidth; c++ {
			cells[c] = style.Render(barGlyph)
		}
	}
	return strings.Join(cells, "")
}

// renderDetail renders the "selected" panel for the bar under the cursor.
func (m *Model) renderDetail(th *theme.Theme, width int) string {
	if m.cursor < 0 || m.cursor >= len(m.bars) {
		return component.Card(th, "selected", th.Muted.Render("(no selection)"), width)
	}
	b := m.bars[m.cursor]
	r := b.Req
	qs := r.QuerySource
	if qs == "" {
		qs = "(unset)"
	}
	status := "ok"
	if r.IsError {
		status = "error"
	}
	lines := []string{
		labelValue(th, "model", orDash(r.Model), width-6),
		labelValue(th, "query_source", qs, width-6),
		labelValue(th, "duration", fmt.Sprintf("%d ms", r.DurationMS), width-6),
		labelValue(th, "cost", fmt.Sprintf("$%.4f", r.CostUSD), width-6),
		labelValue(th, "tokens", fmt.Sprintf("in %d / out %d", r.InputTokens, r.OutputTokens), width-6),
		labelValue(th, "started", startOf(r).Format("15:04:05")+" · "+status, width-6),
	}
	return component.Card(th, "selected", strings.Join(lines, "\n"), width)
}

func (m *Model) isSelected(b Bar) bool {
	if m.cursor < 0 || m.cursor >= len(m.bars) {
		return false
	}
	sel := m.bars[m.cursor]
	return sel.OffsetMS == b.OffsetMS &&
		sel.Lane == b.Lane &&
		sel.Req.TS.Equal(b.Req.TS) &&
		sel.Req.DurationMS == b.Req.DurationMS
}

// labelValue renders a "label   value" line padded to width.
func labelValue(th *theme.Theme, label, value string, width int) string {
	lbl := th.Label.Render(label)
	gap := width - lipgloss.Width(lbl) - lipgloss.Width(value)
	if gap < 1 {
		gap = 1
	}
	return lbl + strings.Repeat(" ", gap) + value
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
