package about

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// View renders the about screen. On terminals ≥ 32 cells wide it shows the
// full block logo + metadata; narrower terminals get a compact wordmark.
func (m *Model) View(width, height int) string {
	if width < 32 {
		return m.renderNarrow(width, height)
	}
	return m.renderWide(width, height)
}

func (m *Model) renderWide(width, height int) string {
	logo := Render(m.theme)
	tagline := m.theme.Subtitle.Render("claude code observer")
	meta := m.theme.Muted.Render(
		"v" + strings.TrimPrefix(m.version, "v") +
			" · commit " + m.commit +
			" · local OTLP receiver",
	)
	help := m.theme.Help.Render("[b] back   [q] quit")

	body := strings.Join([]string{logo, "", tagline, meta, "", help}, "\n")
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}

func (m *Model) renderNarrow(width, height int) string {
	brand := m.theme.Accent.Render(m.theme.Glyphs.Brand) + " " + m.theme.Title.Render("CCO")
	tagline := m.theme.Subtitle.Render("claude code observer")
	meta := m.theme.Muted.Render("v" + strings.TrimPrefix(m.version, "v"))
	help := m.theme.Help.Render("[b] back  [q] quit")

	body := strings.Join([]string{brand, tagline, meta, "", help}, "\n")
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}

// Static interface compliance check.
var _ app.View = (*Model)(nil)
