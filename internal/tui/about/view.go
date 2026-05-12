package about

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

// Model is the about view. It has no state beyond the theme reference and
// the version metadata passed in at construction.
type Model struct {
	theme   *theme.Theme
	version string
	commit  string

	back key.Binding
	quit key.Binding
}

// New builds an about view. version and commit are rendered in the metadata
// line; pass them from internal/version at construction time.
func New(th *theme.Theme, version, commit string) *Model {
	return &Model{
		theme:   th,
		version: version,
		commit:  commit,
		back:    key.NewBinding(key.WithKeys("b", "esc"), key.WithHelp("b", "back")),
		quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (app.View, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(km, m.back) {
			return m, func() tea.Msg { return app.PopViewMsg{} }
		}
	}
	return m, nil
}

func (m *Model) Title() string { return "ABOUT" }

func (m *Model) ShortHelp() []key.Binding {
	return []key.Binding{m.back, m.quit}
}

func (m *Model) Status() component.Status { return component.StatusLive }

// View is implemented in render.go (added in Task 4). To keep this file
// self-contained and satisfy the app.View interface during incremental
// development, define a minimal stub here that Task 4 replaces.
func (m *Model) View(width, height int) string { return "" }

// Static interface compliance check.
var _ app.View = (*Model)(nil)
