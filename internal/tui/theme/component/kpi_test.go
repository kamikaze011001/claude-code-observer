package component

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestKPI_Width(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	out := KPI(&th, "cost", "$3.42", nil, 20)
	if got := lipgloss.Width(out); got != 20 {
		t.Errorf("kpi width: got %d want 20", got)
	}
}

func TestKPI_WithPositiveDelta(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	d := &Delta{Direction: DeltaUp, Text: "+12%"}
	out := KPI(&th, "tokens", "847k", d, 22)
	if got := lipgloss.Width(out); got != 22 {
		t.Errorf("kpi+delta width: got %d want 22", got)
	}
}
