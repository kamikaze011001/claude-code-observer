package component

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// costTier buckets a USD amount into a visual severity tier. Thresholds are
// absolute (not relative to a session) so users build intuition for real cents.
type costTier int

const (
	tierCheap   costTier = iota // < $0.01
	tierNotable                 // $0.01 – $0.05
	tierHeavy                   // > $0.05
)

// Tunable thresholds for the cost-color scale.
const (
	cheapMax   = 0.01 // strictly below => cheap
	notableMax = 0.05 // at or below => notable; above => heavy
)

func tierOf(usd float64) costTier {
	switch {
	case usd < cheapMax:
		return tierCheap
	case usd <= notableMax:
		return tierNotable
	default:
		return tierHeavy
	}
}

// CostColor returns the palette color for a USD amount's tier.
func CostColor(t *theme.Theme, usd float64) lipgloss.Color {
	switch tierOf(usd) {
	case tierHeavy:
		return t.Palette.Red
	case tierNotable:
		return t.Palette.Yellow
	default:
		return t.Palette.Green
	}
}

// CostText renders a USD amount as "$0.00" foreground-colored by its tier.
func CostText(t *theme.Theme, usd float64) string {
	return lipgloss.NewStyle().Foreground(CostColor(t, usd)).Render(fmt.Sprintf("$%.2f", usd))
}

// CostText4 is CostText with 4 decimal places, for sub-cent per-call amounts.
func CostText4(t *theme.Theme, usd float64) string {
	return lipgloss.NewStyle().Foreground(CostColor(t, usd)).Render(fmt.Sprintf("$%.4f", usd))
}
