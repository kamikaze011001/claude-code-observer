package component

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestHelpBar_Width(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	hints := []KeyHint{{"↑↓", "nav"}, {"⏎", "open"}, {"q", "quit"}}
	out := HelpBar(&th, hints, 60)
	if got := lipgloss.Width(out); got != 60 {
		t.Errorf("help width: got %d want 60", got)
	}
}
