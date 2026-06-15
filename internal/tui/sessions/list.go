package sessions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

const (
	listFetchTimeout = 500 * time.Millisecond
	listPageSize     = 50
)

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
	theme    *theme.Theme
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
	Up, Down, PgUp, PgDn, Top, Bottom, Enter, Toggle key.Binding
}

func defaultListKeys() listKeys {
	return listKeys{
		Up:     key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:   key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PgUp:   key.NewBinding(key.WithKeys("pgup", "u"), key.WithHelp("u", "prev page")),
		PgDn:   key.NewBinding(key.WithKeys("pgdown", "d"), key.WithHelp("d", "next page")),
		Top:    key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom: key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Enter:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Toggle: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "expand")),
	}
}

// NewList constructs a List bound to the given read pool and theme.
func NewList(pool *sql.DB, th *theme.Theme) *List {
	return &List{pool: pool, theme: th, keys: defaultListKeys()}
}

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
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "about")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

// Status reports the current connection state for the footer pill.
func (m *List) Status() component.Status {
	if m.lastOK.IsZero() && len(m.rows) == 0 {
		return component.StatusNoDaemon
	}
	if m.stale {
		return component.StatusStale
	}
	return component.StatusLive
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
			th := m.theme
			return m, func() tea.Msg {
				return app.PushViewMsg{V: NewDetail(pool, id, th)}
			}
		}
	}
	return m, nil
}

// View renders the sessions list.
func (m *List) View(width, height int) string {
	th := m.theme
	if th == nil {
		d := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
		th = &d
	}
	if width <= 0 {
		width = 90
	}

	// Header: brand · sessions · page · pill
	brand := th.Title.Render(th.Glyphs.Brand + " cco")
	bread := th.Muted.Render(fmt.Sprintf(" · sessions    page %d", len(m.prevCurs)+1))
	pill := component.StatusPill(th, m.Status())
	headerRight := lipgloss.NewStyle().Width(width - lipgloss.Width(brand) - lipgloss.Width(bread)).Align(lipgloss.Right).Render(pill)
	header := lipgloss.JoinHorizontal(lipgloss.Top, brand, bread, headerRight)

	// Body card
	if len(m.rows) == 0 {
		body := th.Muted.Render("no sessions yet — start using Claude Code with cco serve running")
		card := component.Card(th, "", body, width)
		help := component.HelpBar(th, m.helpHints(), width)
		return strings.Join([]string{header, "", card, "", help}, "\n")
	}

	// Column header strip + rows
	// inner = width - 6: border (2) + padding (4) consumed by Card.
	inner := width - 6
	if inner < 8 {
		inner = 8
	}
	columnHeader := th.Muted.Render(formatColHeader(inner))
	rows := []string{columnHeader}
	var maxCost float64
	for _, r := range m.rows {
		if r.CostUSD > maxCost {
			maxCost = r.CostUSD
		}
	}
	for i, r := range m.rows {
		rd := component.SessionRowData{
			Index:        i + 1,
			Started:      r.StartedAt,
			ProjectName:  defaultProject(r.ProjectName),
			DurationSec:  r.DurationSec,
			CostUSD:      r.CostUSD,
			MaxCostUSD:   maxCost,
			Prompts:      r.Prompts,
			Tokens:       r.Tokens,
			LinesAdded:   r.LinesAdded,
			LinesRemoved: r.LinesRemoved,
			Live:         r.Live,
		}
		rows = append(rows, component.SessionRow(th, rd, i == m.cursor, inner))
	}
	body := strings.Join(rows, "\n")
	card := component.Card(th, "", body, width)

	help := component.HelpBar(th, m.helpHints(), width)
	parts := []string{header, "", card}
	if m.nextCur != nil {
		parts = append(parts, th.Muted.Render("press d for next page"))
	}
	parts = append(parts, "", help)
	return strings.Join(parts, "\n")
}

func (m *List) helpHints() []component.KeyHint {
	return []component.KeyHint{
		{Key: "↑↓", Desc: "nav"}, {Key: "⏎", Desc: "open"}, {Key: "d", Desc: "next"}, {Key: "u", Desc: "prev"},
		{Key: "g/G", Desc: "top/bot"}, {Key: "b", Desc: "back"}, {Key: "?", Desc: "about"}, {Key: "q", Desc: "quit"},
	}
}

func defaultProject(s string) string {
	if s == "" {
		return "(unlabeled)"
	}
	return s
}

func formatColHeader(w int) string {
	// Column widths come from component package — single source of truth shared
	// with SessionRow so headers and rows always stay aligned.
	projW := w - component.ColIdxW - component.ColStartW - component.ColDurW - component.ColCostW - component.ColBarW - component.ColPrW - component.ColTokW - component.ColLinesW - component.ColLiveW - component.ColGutterCount
	effectiveBarW := component.ColBarW

	// Mirror SessionRow's bar-shrinks-first logic exactly.
	if projW < component.ProjMinW {
		deficit := component.ProjMinW - projW
		effectiveBarW = component.ColBarW - deficit
		if effectiveBarW < 0 {
			effectiveBarW = 0
		}
		projW = component.ProjMinW
	}
	// Safety clamp: keep content within w when bar is fully consumed.
	maxProjW := w - component.ColIdxW - component.ColStartW - component.ColDurW - component.ColCostW - effectiveBarW - component.ColPrW - component.ColTokW - component.ColLinesW - component.ColLiveW - component.ColGutterCount
	if maxProjW < projW {
		projW = maxProjW
		if projW < 0 {
			projW = 0
		}
	}

	// Use the same truncation helper as SessionRow for consistency.
	projLabel := component.TruncToWidth("project", projW)
	// When the bar shrinks to 0, pass "" so %-0s emits nothing (not "spend").
	spendLabel := "spend"
	if effectiveBarW == 0 {
		spendLabel = ""
	}
	return fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s",
		component.ColIdxW, "#",
		component.ColStartW, "started",
		projW, projLabel,
		component.ColDurW, "duration",
		component.ColCostW, "cost",
		effectiveBarW, spendLabel,
		component.ColPrW, "prompts",
		component.ColTokW, "tokens",
		component.ColLinesW, "lines",
		component.ColLiveW, "status",
	)
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

