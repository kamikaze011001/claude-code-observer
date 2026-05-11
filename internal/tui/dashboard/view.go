package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

// resolvedTheme returns the model's theme pointer if set, else a pointer to
// the package-level default. This avoids a nil-deref in tests that set only
// specific model fields.
var fallbackTheme = func() *theme.Theme { t := theme.Default(); return &t }()

func (m *Model) th() *theme.Theme {
	if m.theme != nil {
		return m.theme
	}
	return fallbackTheme
}

// View renders the dashboard body at the given terminal dimensions.
func (m *Model) View(width, height int) string {
	if width <= 0 {
		width = 80
	}
	t := m.th()

	var sections []string

	sections = append(sections, m.renderHeader(t, width))
	sections = append(sections, m.renderWindowCards(t, width))

	if delta := m.renderDeltaStrip(t, width); delta != "" {
		sections = append(sections, delta)
	}

	sections = append(sections, m.renderTopSessions(t, width))
	sections = append(sections, m.renderRecentSessions(t, width))
	sections = append(sections, m.renderHelpBar(t, width))

	return strings.Join(sections, "\n")
}

func (m *Model) renderHeader(t *theme.Theme, width int) string {
	brand := t.Title.Render(t.Glyphs.Brand + " cco")
	breadcrumb := t.Muted2.Render(" · dashboard")
	left := brand + breadcrumb

	var st component.Status
	switch m.Status() {
	case theme.PillLive:
		st = component.StatusLive
	case theme.PillStale:
		st = component.StatusStale
	default:
		st = component.StatusNoDaemon
	}
	pill := component.StatusPill(t, st)
	pillW := lipgloss.Width(pill)
	leftW := width - pillW
	if leftW < 0 {
		leftW = 0
	}
	leftPadded := lipgloss.NewStyle().Width(leftW).Render(left)
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPadded, pill)
}

func (m *Model) renderWindowCards(t *theme.Theme, width int) string {
	cardW := (width - 2) / 3 // 3 cards + 2 single-space gutters = width
	if cardW < 16 {
		cardW = 16
	}
	today := renderWindowCard(t, "TODAY", m.snap.Today, cardW)
	d7 := renderWindowCard(t, "7 DAYS", m.snap.D7, cardW)
	d30 := renderWindowCard(t, "30 DAYS", m.snap.D30, cardW)
	return lipgloss.JoinHorizontal(lipgloss.Top, today, " ", d7, " ", d30)
}

func renderWindowCard(t *theme.Theme, title string, ws readstore.WindowStats, cardW int) string {
	// inner content width = cardW - 2 (border) - 4 (padding*2)
	inner := cardW - 6
	if inner < 8 {
		inner = 8
	}

	var b strings.Builder
	writeKV := func(label, value string) {
		row := t.Label.Render(label) + "  " + t.Value.Render(value)
		b.WriteString(lipgloss.NewStyle().Width(inner).Render(row))
		b.WriteString("\n")
	}

	writeKV("sessions", fmt.Sprintf("%d", ws.Sessions))
	writeKV("prompts", fmt.Sprintf("%d", ws.Prompts))
	writeKV("tokens", component.HumanInt(ws.Tokens))
	writeKV("tools", component.HumanInt(ws.Tools))
	writeKV("cost", fmt.Sprintf("$%.2f", ws.CostUSD))

	errVal := fmt.Sprintf("%d", ws.Errors)
	errLabel := t.Label.Render("errors")
	var errStyled string
	if ws.Errors > 0 {
		errStyled = lipgloss.NewStyle().Foreground(t.Palette.Red).Render(errVal)
	} else {
		errStyled = t.Value.Render(errVal)
	}
	row := errLabel + "  " + errStyled
	b.WriteString(lipgloss.NewStyle().Width(inner).Render(row))

	return component.Card(t, title, b.String(), cardW)
}

func (m *Model) renderDeltaStrip(t *theme.Theme, width int) string {
	y := m.snap.Yesterday
	tod := m.snap.Today
	if y.Sessions == 0 {
		return ""
	}

	deltaDir := func(curr, prev int64) component.Direction {
		switch {
		case curr > prev:
			return component.DeltaUp
		case curr < prev:
			return component.DeltaDown
		default:
			return component.DeltaFlat
		}
	}
	deltaDirF := func(curr, prev float64) component.Direction {
		switch {
		case curr > prev:
			return component.DeltaUp
		case curr < prev:
			return component.DeltaDown
		default:
			return component.DeltaFlat
		}
	}

	sessD := component.RenderDeltaInline(t, deltaDir(tod.Sessions, y.Sessions),
		fmt.Sprintf("%d→%d sessions", y.Sessions, tod.Sessions))
	promD := component.RenderDeltaInline(t, deltaDir(tod.Prompts, y.Prompts),
		fmt.Sprintf("%d→%d prompts", y.Prompts, tod.Prompts))
	tokD := component.RenderDeltaInline(t, deltaDir(tod.Tokens, y.Tokens),
		fmt.Sprintf("%s→%s tokens", component.HumanInt(y.Tokens), component.HumanInt(tod.Tokens)))
	costD := component.RenderDeltaInline(t, deltaDirF(tod.CostUSD, y.CostUSD),
		fmt.Sprintf("$%.2f→$%.2f cost", y.CostUSD, tod.CostUSD))

	body := lipgloss.JoinHorizontal(lipgloss.Top, sessD, "   ", promD, "   ", tokD, "   ", costD)
	return component.Card(t, "today vs yesterday", body, width)
}

func (m *Model) renderTopSessions(t *theme.Theme, width int) string {
	inner := width - 6 // border + padding
	if inner < 8 {
		inner = 8
	}
	if len(m.top) == 0 {
		return component.Card(t, "top sessions today (by cost)",
			t.Muted2.Render("(no sessions today)"), width)
	}
	var b strings.Builder
	for i, ts := range m.top {
		rd := component.SessionRowData{
			Index:       i + 1,
			Started:     time.Unix(0, ts.StartedAt).UTC(),
			ProjectName: ts.ProjectName,
			CostUSD:     ts.CostUSD,
			Prompts:     ts.Prompts,
			Live:        ts.Live,
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(component.SessionRow(t, rd, false, inner))
	}
	return component.Card(t, "top sessions today (by cost)", b.String(), width)
}

func (m *Model) renderRecentSessions(t *theme.Theme, width int) string {
	inner := width - 6 // border + padding
	if inner < 8 {
		inner = 8
	}
	if len(m.recent) == 0 {
		return component.Card(t, "recent sessions",
			t.Muted2.Render("(no sessions today)"), width)
	}
	var b strings.Builder
	for i, ts := range m.recent {
		rd := component.SessionRowData{
			Index:       i + 1,
			Started:     time.Unix(0, ts.StartedAt).UTC(),
			ProjectName: ts.ProjectName,
			CostUSD:     ts.CostUSD,
			Prompts:     ts.Prompts,
			Live:        ts.Live,
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(component.SessionRow(t, rd, i == m.recentCursor, inner))
	}
	return component.Card(t, "recent sessions", b.String(), width)
}

func (m *Model) renderHelpBar(t *theme.Theme, width int) string {
	hints := []component.KeyHint{
		{Key: "↑↓", Desc: "nav"},
		{Key: "⏎", Desc: "open"},
		{Key: "s", Desc: "sessions"},
		{Key: "r", Desc: "refresh"},
		{Key: "?", Desc: "help"},
		{Key: "q", Desc: "quit"},
	}
	return component.HelpBar(t, hints, width)
}
