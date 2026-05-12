package component

import (
	"strings"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// ModelBadge renders a colored badge labeled with the model family.
func ModelBadge(t *theme.Theme, model string) string {
	fam := familyFor(model)
	switch fam {
	case "opus":
		return t.BadgeOpus.Render(" opus ")
	case "sonnet":
		return t.BadgeSonnet.Render(" sonnet ")
	case "haiku":
		return t.BadgeHaiku.Render(" haiku ")
	}
	return t.Muted.Render(" model ")
}

func familyFor(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "opus"):
		return "opus"
	case strings.Contains(m, "sonnet"):
		return "sonnet"
	case strings.Contains(m, "haiku"):
		return "haiku"
	}
	return ""
}
