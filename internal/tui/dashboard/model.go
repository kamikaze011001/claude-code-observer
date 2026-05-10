package dashboard

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
)

const fetchTimeout = 500 * time.Millisecond

var errNoPool = errors.New("dashboard: no read pool")

// dataMsg is the success result of a dashboard fetch. View-local; never
// crosses the shell.
type dataMsg struct {
	snap readstore.Snapshot
	top  []readstore.TopSession
	at   time.Time
}

// Model is the dashboard's tea model. Implements app.View.
type Model struct {
	pool     *sql.DB
	snap     readstore.Snapshot
	top      []readstore.TopSession
	lastOK   time.Time
	inFlight bool
	stale    bool
	now      func() time.Time
}

// New constructs a Model bound to the given read pool. pool may be nil in tests.
func New(pool *sql.DB) *Model {
	return &Model{pool: pool, now: time.Now}
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
		m.snap = v.snap
		m.top = v.top
		m.lastOK = v.at
		m.inFlight = false
		m.stale = false
		return m, nil
	case app.ErrMsg:
		m.inFlight = false
		m.stale = true
		return m, nil
	}
	return m, nil
}

func (m *Model) Title() string { return "DASHBOARD" }

func (m *Model) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sessions")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

const staleAfter = 30 * time.Second

// Status implements app.View. Returns the pill state the shell should show.
func (m *Model) Status() theme.PillState {
	if m.snap.LatestEventTS == 0 && len(m.top) == 0 && m.lastOK.IsZero() {
		return theme.PillNoDaemon
	}
	if m.stale {
		return theme.PillStale
	}
	if m.snap.LatestEventTS != 0 {
		latest := time.Unix(0, m.snap.LatestEventTS)
		if m.now().Sub(latest) > staleAfter {
			return theme.PillStale
		}
	}
	return theme.PillLive
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
		snap, top, err := readstore.DashboardSnapshot(ctx, pool, now())
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return dataMsg{snap: snap, top: top, at: now()}
	}
}
