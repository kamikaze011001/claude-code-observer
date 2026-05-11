package theme

import "github.com/charmbracelet/lipgloss"

// Palette holds the absolute colors a Theme uses. Values are lipgloss.Color
// hex literals so they bypass the terminal's 16-color overrides.
type Palette struct {
	Bg, BgAlt, Fg, FgMuted  lipgloss.Color
	Accent                   lipgloss.Color
	Blue, Green, Yellow, Red lipgloss.Color
	Teal, Mauve              lipgloss.Color
}

// MochaPalette is the flagship dark flavor.
func MochaPalette() Palette {
	return Palette{
		Bg:      "#1e1e2e",
		BgAlt:   "#313244",
		Fg:      "#cdd6f4",
		FgMuted: "#6c7086",
		Accent:  "#f5c2e7",
		Blue:    "#89b4fa",
		Green:   "#a6e3a1",
		Yellow:  "#f9e2af",
		Red:     "#f38ba8",
		Teal:    "#94e2d5",
		Mauve:   "#cba6f7",
	}
}

// MacchiatoPalette is the softer dark flavor.
func MacchiatoPalette() Palette {
	return Palette{
		Bg: "#24273a", BgAlt: "#363a4f", Fg: "#cad3f5", FgMuted: "#6e738d",
		Accent: "#f5bde6",
		Blue:   "#8aadf4", Green: "#a6da95", Yellow: "#eed49f", Red: "#ed8796",
		Teal: "#8bd5ca", Mauve: "#c6a0f6",
	}
}

// FrappePalette is the warmest dark flavor.
func FrappePalette() Palette {
	return Palette{
		Bg: "#303446", BgAlt: "#414559", Fg: "#c6d0f5", FgMuted: "#737994",
		Accent: "#f4b8e4",
		Blue:   "#8caaee", Green: "#a6d189", Yellow: "#e5c890", Red: "#e78284",
		Teal: "#81c8be", Mauve: "#ca9ee6",
	}
}

// LattePalette is the light flavor.
func LattePalette() Palette {
	return Palette{
		Bg: "#eff1f5", BgAlt: "#ccd0da", Fg: "#4c4f69", FgMuted: "#9ca0b0",
		Accent: "#ea76cb",
		Blue:   "#1e66f5", Green: "#40a02b", Yellow: "#df8e1d", Red: "#d20f39",
		Teal: "#179299", Mauve: "#8839ef",
	}
}
