package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// View is the contract every page in the TUI implements. The shell
// dispatches messages via Update and renders via View.
type View interface {
	// Init runs once when the view is pushed onto the stack.
	Init() tea.Cmd
	// Update consumes a tea.Msg and returns an updated copy of itself plus
	// any follow-on command.
	Update(msg tea.Msg) (View, tea.Cmd)
	// View renders the body content (chrome is rendered by the shell).
	View(width, height int) string
	// Title appears in the top chrome.
	Title() string
	// ShortHelp lists the keys for the footer strip.
	ShortHelp() []key.Binding
	// Status reports the current pill state for the footer.
	Status() theme.PillState
}
