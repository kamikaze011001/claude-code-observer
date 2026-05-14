package theme

import "github.com/charmbracelet/lipgloss"

// Theme is the style set for the TUI. All fields are derived from Palette and
// Glyphs by Build(). Use Build(MochaPalette(), UnicodeGlyphs()) or the package
// helper defaultTheme() to construct.
type Theme struct {
	Palette Palette
	Glyphs  Glyphs

	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Muted    lipgloss.Style // was Muted2 in PR 1-6
	Accent   lipgloss.Style
	Value    lipgloss.Style
	Label    lipgloss.Style

	Card      lipgloss.Style
	CardTitle lipgloss.Style
	Help      lipgloss.Style

	BadgeOpus   lipgloss.Style
	BadgeSonnet lipgloss.Style
	BadgeHaiku  lipgloss.Style

	PillLive     lipgloss.Style // was PillLiveS
	PillStale    lipgloss.Style // was PillStaleS
	PillNoDaemon lipgloss.Style
}

// Build constructs a Theme fully populated, derived from the given Palette and
// Glyphs. This is the single source of truth for all theme construction.
func Build(p Palette, g Glyphs) Theme {
	return Theme{
		Palette:  p,
		Glyphs:   g,
		Title:    lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		Subtitle: lipgloss.NewStyle().Foreground(p.FgMuted),
		Muted:    lipgloss.NewStyle().Foreground(p.FgMuted),
		Accent:   lipgloss.NewStyle().Foreground(p.Accent),
		Value:    lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		Label:    lipgloss.NewStyle().Foreground(p.FgMuted),

		Card:      lipgloss.NewStyle().Border(g.BorderRound).BorderForeground(p.FgMuted).Padding(0, 2),
		CardTitle: lipgloss.NewStyle().Foreground(p.FgMuted),
		Help:      lipgloss.NewStyle().Foreground(p.FgMuted),

		BadgeOpus:   lipgloss.NewStyle().Foreground(p.Bg).Background(p.Blue).Padding(0, 1),
		BadgeSonnet: lipgloss.NewStyle().Foreground(p.Bg).Background(p.Green).Padding(0, 1),
		BadgeHaiku:  lipgloss.NewStyle().Foreground(p.Bg).Background(p.Yellow).Padding(0, 1),

		PillLive:     lipgloss.NewStyle().Bold(true).Foreground(p.Bg).Background(p.Green).Padding(0, 1),
		PillStale:    lipgloss.NewStyle().Bold(true).Foreground(p.Fg).Background(p.FgMuted).Padding(0, 1),
		PillNoDaemon: lipgloss.NewStyle().Bold(true).Foreground(p.Fg).Background(p.Red).Padding(0, 1),
	}
}

// defaultTheme returns the standard theme using the Mocha palette and Unicode
// glyphs. Used internally for nil-safe fallbacks in view constructors.
func defaultTheme() Theme { return Build(MochaPalette(), UnicodeGlyphs()) }
