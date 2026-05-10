package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// defaultTheme is memoized to avoid allocating a new theme on every frame.
var defaultTheme = theme.Default()

// View renders the dashboard body.
func (m *Model) View(width, height int) string {
	th := defaultTheme
	if width <= 0 {
		width = 80
	}
	blockW := (width - 4) / 3
	if blockW < 16 {
		blockW = 16
	}

	blocks := []string{
		m.renderBlock(th, blockW, "TODAY", m.snap.Today),
		m.renderBlock(th, blockW, "7 DAYS", m.snap.D7),
		m.renderBlock(th, blockW, "30 DAYS", m.snap.D30),
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, blocks...)

	var banner string
	if m.snap.LatestEventTS == 0 && len(m.top) == 0 {
		banner = th.ErrorText.Render("⚠ NO DATA — IS `cco serve` RUNNING?")
	}

	tableW := blockW*3 + 4
	table := m.renderTopSessions(th, tableW)

	parts := []string{}
	if banner != "" {
		parts = append(parts, banner, "")
	}
	parts = append(parts, row, "", table)
	return strings.Join(parts, "\n")
}

func (m *Model) renderBlock(th theme.Theme, w int, label string, ws readstore.WindowStats) string {
	cost := th.AccentText.Render(fmt.Sprintf("$%.2f", ws.CostUSD))
	prompts := fmt.Sprintf("%d PROMPTS", ws.Prompts)
	tools := fmt.Sprintf("%s TOOLS", humanInt(ws.Tools))

	errLine := th.MutedText.Render(fmt.Sprintf("%d ERRORS", ws.Errors))
	if ws.Errors > 0 {
		errLine = th.ErrorText.Render(fmt.Sprintf("⚠ %d ERRORS", ws.Errors))
	}

	body := strings.Join([]string{
		th.Heading.Render(label),
		cost,
		prompts,
		tools,
		errLine,
	}, "\n")
	return th.Block(w).Render(body)
}

func (m *Model) renderTopSessions(th theme.Theme, w int) string {
	header := th.Heading.Render("TOP SESSIONS TODAY")
	if len(m.top) == 0 {
		return th.Block(w).Render(header + "\n" + th.MutedText.Render("(no sessions today)"))
	}
	rows := []string{header, "#  PROJECT          STARTED  COST    PROMPTS  STATUS"}
	for i, ts := range m.top {
		started := time.Unix(0, ts.StartedAt).UTC().Format("15:04")
		project := truncate(ts.ProjectName, 16)
		status := ""
		if ts.Live {
			status = th.AccentText.Render("● LIVE")
		}
		costPlain := fmt.Sprintf("$%-6.2f", ts.CostUSD) // 7-char fixed width including $ sign
		costStyled := th.AccentText.Render(costPlain)
		rows = append(rows, fmt.Sprintf("%d  %-16s %-7s %s  %-7d %s",
			i+1, project, started,
			costStyled,
			ts.Prompts, status))
	}
	return th.Block(w).Render(strings.Join(rows, "\n"))
}

func humanInt(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
