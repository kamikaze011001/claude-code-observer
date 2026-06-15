package productivity

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

// fallbackTheme is used when the model's theme pointer is nil (e.g. in tests).
var fallbackTheme = func() *theme.Theme {
	t := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	return &t
}()

func (m *Model) th() *theme.Theme {
	if m.theme != nil {
		return m.theme
	}
	return fallbackTheme
}

// View renders the productivity body at the given terminal dimensions.
func (m *Model) View(width, height int) string {
	if width <= 0 {
		width = 80
	}
	t := m.th()

	if len(m.days) == 0 {
		return component.Card(t, "productivity (last 30 days)", t.Muted.Render("(no data yet)"), width)
	}

	var b strings.Builder
	header := fmt.Sprintf("%-12s %-14s %-8s %-7s %s", "day", "lines", "commits", "active", "accept")
	b.WriteString(t.Label.Render(header) + "\n")

	for i, d := range m.days {
		line := fmt.Sprintf("%-12s %-14s %-8d %-7s %s",
			d.Day,
			fmt.Sprintf("+%s -%s", component.HumanInt(d.LinesAdded), component.HumanInt(d.LinesRemoved)),
			d.Commits,
			component.HumanActiveDuration(d.ActiveSec),
			acceptRate(d.EditsAccepted, d.EditsRejected),
		)
		if i == m.cursor {
			line = lipgloss.NewStyle().
				Foreground(t.Palette.Bg).
				Background(t.Palette.Fg).
				Render(line)
		} else {
			line = t.Value.Render(line)
		}
		b.WriteString(line + "\n")
	}

	return component.Card(t, "productivity (last 30 days)", strings.TrimRight(b.String(), "\n"), width)
}

// acceptRate formats the edit accept percentage, e.g. "90%" or "—" when no edits.
func acceptRate(acc, rej int64) string {
	total := acc + rej
	if total == 0 {
		return "—" // em dash
	}
	return fmt.Sprintf("%.0f%%", float64(acc)/float64(total)*100)
}

