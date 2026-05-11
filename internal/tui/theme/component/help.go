package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// KeyHint is one ("↑↓" — "nav") pair shown in the footer.
type KeyHint struct {
	Key  string
	Desc string
}

// HelpBar renders all hints in muted style, joined with two-space gutters,
// then trimmed/padded to width.
func HelpBar(t *theme.Theme, hints []KeyHint, width int) string {
	parts := make([]string, 0, len(hints)*2)
	for i, h := range hints {
		if i > 0 {
			parts = append(parts, "  ")
		}
		parts = append(parts, t.Muted2.Render(h.Key+" "+h.Desc))
	}
	line := strings.Join(parts, "")
	return lipgloss.NewStyle().Width(width).Render(line)
}
