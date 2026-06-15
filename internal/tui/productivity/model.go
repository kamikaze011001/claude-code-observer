package productivity

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

const fetchTimeout = 500 * time.Millisecond

var errNoPool = errors.New("productivity: no read pool")

// dataMsg is the success result of a productivity fetch. View-local; never
// crosses the shell.
type dataMsg struct {
	days []readstore.ProductivityDay
	at   time.Time
}

// Model is the productivity view's tea model. Implements app.View.
type Model struct {
	pool     *sql.DB
	theme    *theme.Theme
	days     []readstore.ProductivityDay
	cursor   int
	inFlight bool
	stale    bool
	now      func() time.Time
}

// New constructs a Model bound to the given read pool and theme. pool may be nil in tests.
func New(pool *sql.DB, th *theme.Theme) *Model {
	return &Model{pool: pool, theme: th, now: time.Now}
}

func (m *Model) Init() tea.Cmd {
	m.inFlight = true
	return m.fetchCmd()
}

func (m *Model) Update(msg tea.Msg) (app.View, tea.Cmd) {
	switch v := msg.(type) {
	case app.TickMsg:
		if m.inFlight {
			return m, nil
		}
		m.inFlight = true
		return m, m.fetchCmd()
	case dataMsg:
		m.days = v.days
		if m.cursor >= len(m.days) {
			m.cursor = 0
		}
		m.inFlight = false
		m.stale = false
		return m, nil
	case app.ErrMsg:
		m.inFlight = false
		m.stale = true
		return m, nil
	case tea.KeyMsg:
		switch {
		case v.Type == tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case v.Type == tea.KeyDown:
			if m.cursor < len(m.days)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m *Model) Title() string { return "PRODUCTIVITY" }

func (m *Model) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

// Status implements app.View.
func (m *Model) Status() component.Status {
	if m.stale {
		return component.StatusStale
	}
	return component.StatusLive
}

func (m *Model) fetchCmd() tea.Cmd {
	pool := m.pool
	now := m.now
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		days, err := readstore.ProductivityByDay(ctx, pool, 30, time.Now().Location())
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return dataMsg{days: days, at: now()}
	}
}
