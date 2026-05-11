package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// Sparkline draws values as a row of block characters, scaled to fit width.
// If len(values) < width, values are right-padded with the lowest block.
// If len(values) > width, values are sampled.
func Sparkline(t *theme.Theme, values []float64, width int) string {
	if width <= 0 {
		return ""
	}
	blocks := t.Glyphs.Spark
	if len(values) == 0 {
		return strings.Repeat(string(blocks[0]), width)
	}
	// Sample / pad to `width` values.
	sampled := make([]float64, width)
	for i := 0; i < width; i++ {
		idx := i * len(values) / width
		if idx >= len(values) {
			idx = len(values) - 1
		}
		sampled[i] = values[idx]
	}
	var lo, hi = sampled[0], sampled[0]
	for _, v := range sampled {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	span := hi - lo
	var b strings.Builder
	for _, v := range sampled {
		bin := 0
		if span > 0 {
			bin = int((v - lo) / span * float64(len(blocks)-1))
			if bin < 0 {
				bin = 0
			}
			if bin >= len(blocks) {
				bin = len(blocks) - 1
			}
		}
		b.WriteRune(blocks[bin])
	}
	return lipgloss.NewStyle().Foreground(t.Palette.Blue).Render(b.String())
}
