package sessions

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/prompt"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

const (
	detailFetchTimeout = 500 * time.Millisecond
	detailPageSize     = 50
)

type detailDataMsg struct {
	events  []readstore.EventRow
	hasMore bool
	at      time.Time
}

// detailOlderMsg carries events older than the current tail. Unlike
// detailDataMsg it is appended to m.events, never replaces — preserves the
// user's cursor and scroll position while the list grows below them.
type detailOlderMsg struct {
	events  []readstore.EventRow
	hasMore bool
	at      time.Time
}

// Detail is the session event timeline view model.
type Detail struct {
	pool         *sql.DB
	sessionID    string
	events       []readstore.EventRow
	cursor       int
	offset       int  // index of first event rendered in the visible window
	viewport     int  // visible row count, written by View, read by Update for page-step sizing
	hasMore      bool
	loadingOlder bool // guards against double-fetch when pgdn is mashed at bottom
	inFlight     bool
	stale        bool
	lastOK       time.Time

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

	case detailOlderMsg:
		m.events = append(m.events, v.events...)
		m.hasMore = v.hasMore
		m.lastOK = v.at
		m.loadingOlder = false
		return m, nil

	case app.ErrMsg:
		m.inFlight = false
		m.loadingOlder = false
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
		case key.Matches(v, m.keys.PgDn):
			step := m.viewport
			if step < 1 {
				step = 10
			}
			m.cursor += step
			if m.cursor > len(m.events)-1 {
				m.cursor = max0(len(m.events) - 1)
			}
			if m.cursor == len(m.events)-1 && m.hasMore && !m.loadingOlder {
				m.loadingOlder = true
				return m, m.fetchOlderCmd()
			}
		case key.Matches(v, m.keys.PgUp):
			step := m.viewport
			if step < 1 {
				step = 10
			}
			m.cursor -= step
			if m.cursor < 0 {
				m.cursor = 0
			}
		case key.Matches(v, m.keys.Enter):
			if len(m.events) == 0 {
				return m, nil
			}
			row := m.events[m.cursor]
			if row.EventName != domain.EventUserPrompt || row.PromptID == "" {
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

// View renders the session event timeline. Renders only the visible window
// (events[offset:offset+viewport]) and slides offset to follow the cursor.
func (m *Detail) View(width, height int) string {
	var b strings.Builder
	b.WriteString(defaultTheme.Heading.Render(m.Title()))
	b.WriteString("\n\n")

	if len(m.events) == 0 {
		b.WriteString(defaultTheme.MutedText.Render("no events for this session"))
		return b.String()
	}

	m.viewport = visibleRows(height)
	clampOffset(m)

	header := fmt.Sprintf("%-19s %-26s %s", "TIME", "EVENT", "SUMMARY")
	b.WriteString(defaultTheme.MutedText.Render(header))
	b.WriteString("\n")

	end := m.offset + m.viewport
	if end > len(m.events) {
		end = len(m.events)
	}
	for i := m.offset; i < end; i++ {
		e := m.events[i]
		line := fmt.Sprintf("%-19s %-26s %s",
			e.TS.Format("2006-01-02 15:04:05"),
			e.EventName,
			e.Summary,
		)
		isPrompt := e.EventName == domain.EventUserPrompt && e.PromptID != ""
		switch {
		case i == m.cursor && isPrompt:
			line = defaultTheme.AccentText.Render("▶ " + line)
		case i == m.cursor:
			line = defaultTheme.AccentText.Render("▶ " + line)
		case isPrompt:
			line = "  " + defaultTheme.AccentText.Render(line)
		default:
			line = "  " + defaultTheme.MutedText.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	switch {
	case m.loadingOlder:
		b.WriteString("\n")
		b.WriteString(defaultTheme.MutedText.Render("loading older events…"))
	case m.hasMore:
		b.WriteString("\n")
		b.WriteString(defaultTheme.MutedText.Render("press pgdn for older events"))
	}
	b.WriteString("\n")
	b.WriteString(defaultTheme.MutedText.Render("enter on a bold prompt row opens prompt detail"))
	return b.String()
}

// fetchOlderCmd issues a keyset-paginated fetch for events strictly older
// than the current tail (events[len-1].TS). Result is delivered as
// detailOlderMsg and appended to m.events.
func (m *Detail) fetchOlderCmd() tea.Cmd {
	pool := m.pool
	sid := m.sessionID
	if len(m.events) == 0 {
		return nil
	}
	before := m.events[len(m.events)-1].TS.UnixNano()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), detailFetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		rows, hasMore, err := readstore.SessionEvents(ctx, pool, sid, &before, detailPageSize)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return detailOlderMsg{events: rows, hasMore: hasMore, at: time.Now()}
	}
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

var newPromptDetail = prompt.New

// visibleRows converts the terminal height passed to View into the number of
// event rows the body can show. Reserves chromeReserved lines for the chrome
// (title + footer), the view's own title block + column header, and the two
// hint lines at the bottom. Falls back to a sensible default before the
// first WindowSizeMsg.
func visibleRows(height int) int {
	const chromeReserved = 7
	if height <= 0 {
		return 20
	}
	v := height - chromeReserved
	if v < 5 {
		v = 5
	}
	return v
}

// clampOffset slides m.offset so the cursor is in the visible window and the
// window itself stays within the loaded events range.
func clampOffset(m *Detail) {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.viewport {
		m.offset = m.cursor - m.viewport + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
	maxOffset := len(m.events) - 1
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}
