package component

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// Direction is the sign of a Delta.
type Direction int

const (
	DeltaFlat Direction = iota
	DeltaUp
	DeltaDown
)

// Delta annotates a KPI with a small change indicator.
type Delta struct {
	Direction Direction
	Text      string // e.g. "+12%" or "+$0.41"
}

// RenderDeltaInline renders a one-line delta indicator: glyph + space + text,
// coloured green (up), red (down), or muted (flat).
func RenderDeltaInline(t *theme.Theme, dir Direction, text string) string {
	var glyph string
	var style lipgloss.Style
	switch dir {
	case DeltaUp:
		glyph = t.Glyphs.DeltaUp
		style = lipgloss.NewStyle().Foreground(t.Palette.Green)
	case DeltaDown:
		glyph = t.Glyphs.DeltaDown
		style = lipgloss.NewStyle().Foreground(t.Palette.Red)
	default:
		glyph = t.Glyphs.DeltaFlat
		style = t.Muted2
	}
	return style.Render(glyph + " " + text)
}

// KPI renders one row: "<label>  <value>  <delta?>" padded to width.
func KPI(t *theme.Theme, label, value string, d *Delta, width int) string {
	lbl := t.Label.Render(label)
	val := t.Value.Render(value)
	parts := []string{lbl, "  ", val}
	if d != nil {
		var glyph, styled string
		switch d.Direction {
		case DeltaUp:
			glyph = t.Glyphs.DeltaUp
			styled = lipgloss.NewStyle().Foreground(t.Palette.Green).Render(glyph + " " + d.Text)
		case DeltaDown:
			glyph = t.Glyphs.DeltaDown
			styled = lipgloss.NewStyle().Foreground(t.Palette.Red).Render(glyph + " " + d.Text)
		default:
			styled = t.Muted2.Render(t.Glyphs.DeltaFlat + " " + d.Text)
		}
		parts = append(parts, "  ", styled)
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return lipgloss.NewStyle().Width(width).Render(line)
}
