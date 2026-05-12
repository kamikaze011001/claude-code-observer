package component

import (
	"strings"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// Card renders a rounded-bordered box of the given total cell width. The
// optional title is shown in muted style as the first body line. body may
// contain newlines; lines are not wrapped (caller is responsible).
//
// Width math: t.Card has Border(round)+Padding(0,2). Lipgloss Width(n) sets
// the width including padding but not the border chars; each side adds 1 char,
// so total outer width = Width(n) + 2. To hit outer==width: Width(width-2).
func Card(t *theme.Theme, title, body string, width int) string {
	// lipgloss: outer = Width(n) + 2 (border chars), so content = width - 2
	style := t.Card.Width(width - 2)
	var b strings.Builder
	if title != "" {
		b.WriteString(t.CardTitle.Render(title))
		b.WriteString("\n")
	}
	b.WriteString(body)
	return style.Render(b.String())
}
