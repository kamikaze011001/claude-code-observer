package sessions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

const (
	listFetchTimeout = 500 * time.Millisecond
	listPageSize     = 50
)

// defaultTheme is memoized to avoid allocating a new theme on every frame.
var defaultTheme = theme.Default()

var errNoPool = errors.New("sessions: no read pool")

type listDataMsg struct {
	rows   []readstore.SessionRow
	next   *int64
	cursor *int64
	at     time.Time
}

// List is the sessions list view model.
type List struct {
	pool     *sql.DB
	rows     []readstore.SessionRow
	cursor   int
	pageCur  *int64
	nextCur  *int64
	prevCurs []*int64
	inFlight bool
	stale    bool
	lastOK   time.Time

	keys listKeys
}

type listKeys struct {
	Up, Down, PgUp, PgDn, Top, Bottom, Enter key.Binding
}

func defaultListKeys() listKeys {
	return listKeys{
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PgUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "prev page")),
		PgDn:   key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "next page")),
		Top:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	}
}

// NewList constructs a List bound to the given read pool.
func NewList(pool *sql.DB) *List { return &List{pool: pool, keys: defaultListKeys()} }

// Init runs once when the view is pushed; starts the first fetch.
func (m *List) Init() tea.Cmd {
	m.inFlight = true
	return m.fetchCmd(nil)
}

// Title appears in the top chrome.
func (m *List) Title() string { return "SESSIONS" }

// ShortHelp lists the keys for the footer strip.
func (m *List) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keys.Up,
		m.keys.Down,
		m.keys.Enter,
		m.keys.PgDn,
		key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

// Status reports the current pill state for the footer.
func (m *List) Status() theme.PillState {
	if m.lastOK.IsZero() && len(m.rows) == 0 {
		return theme.PillNoDaemon
	}
	if m.stale {
		return theme.PillStale
	}
	return theme.PillLive
}

// Update consumes a tea.Msg and returns an updated copy of itself plus any
// follow-on command.
func (m *List) Update(msg tea.Msg) (app.View, tea.Cmd) {
	switch v := msg.(type) {
	case app.TickMsg:
		if m.inFlight || m.pageCur != nil {
			return m, nil
		}
		m.inFlight = true
		return m, m.fetchCmd(nil)

	case listDataMsg:
		if !samePtr(v.cursor, m.pageCur) {
			m.inFlight = false
			return m, nil
		}
		m.rows = v.rows
		m.nextCur = v.next
		m.lastOK = v.at
		m.inFlight = false
		m.stale = false
		if m.cursor >= len(m.rows) {
			m.cursor = max0(len(m.rows) - 1)
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
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case key.Matches(v, m.keys.Top):
			m.cursor = 0
		case key.Matches(v, m.keys.Bottom):
			m.cursor = max0(len(m.rows) - 1)
		case key.Matches(v, m.keys.PgDn):
			if m.nextCur != nil && !m.inFlight {
				m.prevCurs = append(m.prevCurs, m.pageCur)
				m.pageCur = m.nextCur
				m.cursor = 0
				m.inFlight = true
				return m, m.fetchCmd(m.pageCur)
			}
		case key.Matches(v, m.keys.PgUp):
			if len(m.prevCurs) > 0 && !m.inFlight {
				back := m.prevCurs[len(m.prevCurs)-1]
				m.prevCurs = m.prevCurs[:len(m.prevCurs)-1]
				m.pageCur = back
				m.cursor = 0
				m.inFlight = true
				return m, m.fetchCmd(m.pageCur)
			}
		case key.Matches(v, m.keys.Enter):
			if len(m.rows) == 0 {
				return m, nil
			}
			id := m.rows[m.cursor].SessionID
			pool := m.pool
			return m, func() tea.Msg {
				return app.PushViewMsg{V: NewDetail(pool, id)}
			}
		}
	}
	return m, nil
}

// View renders the sessions list.
func (m *List) View(width, height int) string {
	var b strings.Builder
	b.WriteString(defaultTheme.Heading.Render("SESSIONS"))
	b.WriteString("    page ")
	b.WriteString(fmt.Sprintf("%d", len(m.prevCurs)+1))
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		b.WriteString(defaultTheme.MutedText.Render("no sessions yet — start using Claude Code with cco serve running"))
		return b.String()
	}

	const projW = 20
	header := fmt.Sprintf("  %-3s %-16s %-*s %-9s %-7s %-7s %s",
		"#", "STARTED", projW, "PROJECT", "DURATION", "COST", "PROMPTS", "STATUS")
	b.WriteString(defaultTheme.MutedText.Render(header))
	b.WriteString("\n")

	for i, r := range m.rows {
		project := r.ProjectName
		if project == "" {
			project = "(unlabeled)"
		}
		row := fmt.Sprintf("%-3d %-16s %-*s %-9s $%-6.2f %-7d %s",
			i+1,
			r.StartedAt.Format("2006-01-02 15:04"),
			projW, truncRunesView(project, projW),
			humanDuration(r.DurationSec),
			r.CostUSD,
			r.Prompts,
			liveBadge(r.Live),
		)
		if i == m.cursor {
			row = defaultTheme.AccentText.Render("▶ " + row)
		} else {
			row = "  " + row
		}
		b.WriteString(row)
		b.WriteString("\n")
	}
	if m.nextCur != nil {
		b.WriteString("\n")
		b.WriteString(defaultTheme.MutedText.Render("press pgdn for next page"))
	}
	return b.String()
}

func liveBadge(live bool) string {
	if !live {
		return ""
	}
	return defaultTheme.Pill(theme.PillLive)
}

func humanDuration(sec int64) string {
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%dm%02ds", sec/60, sec%60)
	}
	return fmt.Sprintf("%dh%02dm", sec/3600, (sec%3600)/60)
}

func truncRunesView(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func (m *List) fetchCmd(cursor *int64) tea.Cmd {
	pool := m.pool
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), listFetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		rows, next, err := readstore.SessionsPage(ctx, pool, cursor, listPageSize)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return listDataMsg{rows: rows, next: next, cursor: cursor, at: time.Now()}
	}
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func samePtr(a, b *int64) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}
