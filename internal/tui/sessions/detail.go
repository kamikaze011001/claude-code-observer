package sessions

import (
	"context"
	"database/sql"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

const (
	detailFetchTimeout = 500 * time.Millisecond
	detailPageSize     = 200
)

type detailDataMsg struct {
	events  []readstore.EventRow
	hasMore bool
	at      time.Time
}

// Detail is the session event timeline view model.
type Detail struct {
	pool      *sql.DB
	sessionID string
	events    []readstore.EventRow
	cursor    int
	hasMore   bool
	inFlight  bool
	stale     bool
	lastOK    time.Time

	keys listKeys
}

// NewDetail constructs a Detail view for the given sessionID.
func NewDetail(pool *sql.DB, sessionID string) app.View {
	return &Detail{pool: pool, sessionID: sessionID, keys: defaultListKeys()}
}

// Init runs once when the view is pushed; starts the first fetch.
func (m *Detail) Init() tea.Cmd {
	m.inFlight = true
	return m.fetchCmd()
}

// Title appears in the top chrome.
func (m *Detail) Title() string {
	id := m.sessionID
	if len(id) > 8 {
		id = id[:8] + "…"
	}
	return "SESSION " + id
}

// ShortHelp lists the keys for the footer strip.
func (m *Detail) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keys.Up,
		m.keys.Down,
		m.keys.Enter,
		key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

// Status reports the current pill state for the footer.
func (m *Detail) Status() theme.PillState {
	if m.lastOK.IsZero() && len(m.events) == 0 {
		return theme.PillNoDaemon
	}
	if m.stale {
		return theme.PillStale
	}
	return theme.PillLive
}

// Update consumes a tea.Msg and returns an updated copy of itself plus any
// follow-on command.
func (m *Detail) Update(msg tea.Msg) (app.View, tea.Cmd) {
	switch v := msg.(type) {
	case app.TickMsg:
		if m.inFlight {
			return m, nil
		}
		m.inFlight = true
		return m, m.fetchCmd()

	case detailDataMsg:
		var cur readstore.EventRow
		if m.cursor < len(m.events) {
			cur = m.events[m.cursor]
		}
		m.events = v.events
		m.hasMore = v.hasMore
		m.lastOK = v.at
		m.inFlight = false
		m.stale = false
		m.cursor = 0
		for i, e := range m.events {
			if e.TS.Equal(cur.TS) && e.EventName == cur.EventName {
				m.cursor = i
				break
			}
		}
		return m, nil

	case app.ErrMsg:
		m.inFlight = false
		m.stale = true
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(v, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(v, m.keys.Down):
			if m.cursor < len(m.events)-1 {
				m.cursor++
			}
		case key.Matches(v, m.keys.Top):
			m.cursor = 0
		case key.Matches(v, m.keys.Bottom):
			m.cursor = max0(len(m.events) - 1)
		case key.Matches(v, m.keys.Enter):
			if len(m.events) == 0 {
				return m, nil
			}
			row := m.events[m.cursor]
			if row.EventName != "claude_code.user_prompt" || row.PromptID == "" {
				return m, nil
			}
			pool := m.pool
			pid := row.PromptID
			return m, func() tea.Msg {
				return app.PushViewMsg{V: newPromptDetail(pool, pid)}
			}
		}
	}
	return m, nil
}

// View renders the session event timeline.
// Implemented in Task 9.
func (m *Detail) View(width, height int) string {
	return ""
}

func (m *Detail) fetchCmd() tea.Cmd {
	pool := m.pool
	sid := m.sessionID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), detailFetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		rows, hasMore, err := readstore.SessionEvents(ctx, pool, sid, nil, detailPageSize)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return detailDataMsg{events: rows, hasMore: hasMore, at: time.Now()}
	}
}

// newPromptDetail is wired to internal/tui/prompt.New in Task 13.
var newPromptDetail = func(_ *sql.DB, _ string) app.View { return nil }
