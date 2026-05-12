package theme

import "github.com/charmbracelet/lipgloss"

// PillState identifies which footer pill to render.
type PillState int

const (
	PillLive PillState = iota
	PillStale
	PillNoDaemon
)

// Theme is the style set for the TUI. It carries both the legacy shape
// (AccentColor, Heading, Pill method, etc.) used by the existing chrome in
// internal/tui/app/app.go and the new Catppuccin-based shape (Card, badges,
// pills as styles) used by the redesigned views. Both shapes are always
// populated by Build() so all consumers work regardless of which path
// constructed the Theme.
type Theme struct {
	// --- legacy shape (removed in PR 7) ---
	AccentColor lipgloss.Color
	ErrorColor  lipgloss.Color
	Fg          lipgloss.AdaptiveColor
	Muted       lipgloss.AdaptiveColor

	Heading    lipgloss.Style
	AccentText lipgloss.Style
	ErrorText  lipgloss.Style
	MutedText  lipgloss.Style

	border lipgloss.Border

	// --- new shape ---
	Palette Palette
	Glyphs  Glyphs

	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Muted2   lipgloss.Style // "Muted" name is taken by legacy AdaptiveColor above
	Accent   lipgloss.Style
	Value    lipgloss.Style
	Label    lipgloss.Style

	Card      lipgloss.Style
	CardTitle lipgloss.Style
	Help      lipgloss.Style

	BadgeOpus   lipgloss.Style
	BadgeSonnet lipgloss.Style
	BadgeHaiku  lipgloss.Style

	PillLiveS    lipgloss.Style // suffixed to avoid colliding with the PillLive const
	PillStaleS   lipgloss.Style
	PillNoDaemon lipgloss.Style // struct field; the const PillNoDaemon is a PillState — no collision
}

// Build constructs a Theme fully populated with both the legacy and new shapes,
// derived from the given Palette and Glyphs. This is the single source of
// truth for all theme construction; Default() and Resolve() both delegate here.
func Build(p Palette, g Glyphs) Theme {
	fg := lipgloss.AdaptiveColor{Light: string(p.Fg), Dark: string(p.Fg)}
	muted := lipgloss.AdaptiveColor{Light: string(p.FgMuted), Dark: string(p.FgMuted)}
	return Theme{
		// legacy shape — kept populated for app.go chrome and existing views
		AccentColor: p.Accent,
		ErrorColor:  p.Red,
		Fg:          fg,
		Muted:       muted,
		Heading:     lipgloss.NewStyle().Bold(true).Foreground(p.Accent).Padding(0, 1),
		AccentText:  lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		ErrorText:   lipgloss.NewStyle().Bold(true).Foreground(p.Red),
		MutedText:   lipgloss.NewStyle().Foreground(p.FgMuted),
		border:      g.BorderRound,

		// new shape
		Palette:  p,
		Glyphs:   g,
		Title:    lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		Subtitle: lipgloss.NewStyle().Foreground(p.FgMuted),
		Muted2:   lipgloss.NewStyle().Foreground(p.FgMuted),
		Accent:   lipgloss.NewStyle().Foreground(p.Accent),
		Value:    lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
		Label:    lipgloss.NewStyle().Foreground(p.FgMuted),

		Card:      lipgloss.NewStyle().Border(g.BorderRound).BorderForeground(p.FgMuted).Padding(0, 2),
		CardTitle: lipgloss.NewStyle().Foreground(p.FgMuted),
		Help:      lipgloss.NewStyle().Foreground(p.FgMuted),

		BadgeOpus:   lipgloss.NewStyle().Foreground(p.Bg).Background(p.Blue).Padding(0, 1),
		BadgeSonnet: lipgloss.NewStyle().Foreground(p.Bg).Background(p.Green).Padding(0, 1),
		BadgeHaiku:  lipgloss.NewStyle().Foreground(p.Bg).Background(p.Yellow).Padding(0, 1),

		PillLiveS:    lipgloss.NewStyle().Bold(true).Foreground(p.Bg).Background(p.Green).Padding(0, 1),
		PillStaleS:   lipgloss.NewStyle().Bold(true).Foreground(p.Fg).Background(p.FgMuted).Padding(0, 1),
		PillNoDaemon: lipgloss.NewStyle().Bold(true).Foreground(p.Fg).Background(p.Red).Padding(0, 1),
	}
}

// Default returns the standard theme using the Mocha palette and Unicode glyphs.
func Default() Theme { return Build(MochaPalette(), UnicodeGlyphs()) }

// Block returns a bordered style sized to minWidth (cells).
func (t Theme) Block(minWidth int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(t.border).
		BorderForeground(t.Fg).
		Padding(1, 2).
		Width(minWidth)
}

// Pill renders the footer state pill using the legacy API.
func (t Theme) Pill(s PillState) string {
	switch s {
	case PillLive:
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(t.AccentColor).
			Padding(0, 1).
			Render("● LIVE")
	case PillStale:
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(t.Muted).
			Padding(0, 1).
			Render("STALE")
	case PillNoDaemon:
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(t.ErrorColor).
			Padding(0, 1).
			Render("⚠ NO DAEMON")
	}
	return ""
}
