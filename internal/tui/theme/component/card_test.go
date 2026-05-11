package component

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestCard_HasExpectedWidth(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	out := Card(&th, "label", "body text", 30)
	if got := lipgloss.Width(splitFirstLine(out)); got != 30 {
		t.Errorf("card top width: got %d want 30", got)
	}
}

func splitFirstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}
