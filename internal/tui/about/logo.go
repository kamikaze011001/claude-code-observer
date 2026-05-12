package about

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// Logo is the CCO figlet-block wordmark. 6 lines × 25 cells, fixed shape.
const Logo = ` ██████╗ ██████╗ ██████╗ ` + "\n" +
	`██╔════╝██╔════╝██╔═══██╗` + "\n" +
	`██║     ██║     ██║   ██║` + "\n" +
	`██║     ██║     ██║   ██║` + "\n" +
	`╚██████╗╚██████╗╚██████╔╝` + "\n" +
	` ╚═════╝ ╚═════╝ ╚═════╝ `

// Render returns Logo styled with the theme's accent color in bold.
func Render(t *theme.Theme) string {
	return lipgloss.NewStyle().Bold(true).Foreground(t.Palette.Accent).Render(Logo)
}
