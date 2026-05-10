package theme

import "github.com/charmbracelet/lipgloss"

// PillState identifies which footer pill to render.
type PillState int

const (
	PillLive PillState = iota
	PillStale
	PillNoDaemon
)

// Theme is the neo-brutalist style set. Single accent color, thick borders,
// ALL CAPS labels. See docs/superpowers/specs/2026-05-10-phase-3-tui-shell-m3.1-design.md §4.
type Theme struct {
	AccentColor lipgloss.Color
	ErrorColor  lipgloss.Color
	Fg          lipgloss.AdaptiveColor
	Muted       lipgloss.AdaptiveColor

	Heading    lipgloss.Style
	AccentText lipgloss.Style
	ErrorText  lipgloss.Style
	MutedText  lipgloss.Style

	border lipgloss.Border
}

// Default returns the locked v1 theme: hot yellow accent, semantic red,
// adaptive black/white foreground.
func Default() Theme {
	accent := lipgloss.Color("#FFD400")
	errCol := lipgloss.Color("#FF3B30")
	fg := lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
	muted := lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}

	return Theme{
		AccentColor: accent,
		ErrorColor:  errCol,
		Fg:          fg,
		Muted:       muted,

		Heading:    lipgloss.NewStyle().Bold(true).Foreground(fg).Padding(0, 1),
		AccentText: lipgloss.NewStyle().Bold(true).Foreground(accent),
		ErrorText:  lipgloss.NewStyle().Bold(true).Foreground(errCol),
		MutedText:  lipgloss.NewStyle().Foreground(muted),

		border: lipgloss.ThickBorder(),
	}
}

// Block returns a thick-bordered style sized to minWidth (cells).
func (t Theme) Block(minWidth int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(t.border).
		BorderForeground(t.Fg).
		Padding(1, 2).
		Width(minWidth)
}

// Pill renders the footer state pill.
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
