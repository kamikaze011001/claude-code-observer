package theme

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPalettes_HaveAllRequiredColors(t *testing.T) {
	palettes := map[string]Palette{
		"Mocha":     MochaPalette(),
		"Macchiato": MacchiatoPalette(),
		"Frappe":    FrappePalette(),
		"Latte":     LattePalette(),
	}
	for name, p := range palettes {
		colors := map[string]lipgloss.Color{
			"Bg": p.Bg, "BgAlt": p.BgAlt, "Fg": p.Fg, "FgMuted": p.FgMuted,
			"Accent": p.Accent,
			"Blue":   p.Blue, "Green": p.Green, "Yellow": p.Yellow, "Red": p.Red,
			"Teal": p.Teal, "Mauve": p.Mauve,
		}
		for field, c := range colors {
			if string(c) == "" {
				t.Errorf("%s.%s is empty", name, field)
			}
		}
	}
}
