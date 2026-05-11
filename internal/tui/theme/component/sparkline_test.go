package component

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestSparkline_Width(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8}
	out := Sparkline(&th, values, 8)
	if got := lipgloss.Width(out); got != 8 {
		t.Errorf("sparkline width: got %d want 8", got)
	}
}

func TestSparkline_Empty(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	out := Sparkline(&th, nil, 10)
	if got := lipgloss.Width(out); got != 10 {
		t.Errorf("empty sparkline width: got %d want 10", got)
	}
}
