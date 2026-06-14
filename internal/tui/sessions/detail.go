package sessions

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/prompt"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

const (
	detailFetchTimeout = 500 * time.Millisecond
	detailPageSize     = 50
)

// timelineItem wraps a SessionItem with view state. For turns, children are
// loaded lazily on first expand (Task 8).
type timelineItem struct {
	readstore.SessionItem
	expanded bool
	children []readstore.TurnChild
	loaded   bool
}

type detailDataMsg struct {
	items   []readstore.SessionItem
	hasMore bool
	at      time.Time
}

// detailOlderMsg carries items older than the current tail. Unlike
// detailDataMsg it is appended to m.items, never replaces — preserves the
// user's cursor and scroll position while the list grows below them.
type detailOlderMsg struct {
	items   []readstore.SessionItem
	hasMore bool
	at      time.Time
}

// Detail is the session event timeline view model.
type Detail struct {
	pool         *sql.DB
	theme        *theme.Theme
	sessionID    string
	items        []timelineItem
	cursor       int
	offset       int  // index of first item rendered in the visible window
	viewport     int  // visible row count, written by View, read by Update for page-step sizing
	hasMore      bool
	loadingOlder bool // guards against double-fetch when pgdn is mashed at bottom
	inFlight     bool
	stale        bool
	lastOK       time.Time

	keys listKeys
}

// NewDetail constructs a Detail view for the given sessionID.
func NewDetail(pool *sql.DB, sessionID string, th *theme.Theme) app.View {
	return &Detail{pool: pool, theme: th, sessionID: sessionID, keys: defaultListKeys()}
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
		m.keys.PgUp,
		m.keys.PgDn,
		m.keys.Enter,
		key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "about")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

// Status reports the current connection state for the footer pill.
func (m *Detail) Status() component.Status {
	if m.lastOK.IsZero() && len(m.items) == 0 {
		return component.StatusNoDaemon
	}
	if m.stale {
		return component.StatusStale
	}
	return component.StatusLive
}

// applyItems replaces the item list (older=false) or appends (older=true),
// expanding the most-recent turn by default on a fresh load.
func (m *Detail) applyItems(items []readstore.SessionItem, older bool) {
	conv := make([]timelineItem, len(items))
	for i, it := range items {
		conv[i] = timelineItem{SessionItem: it}
	}
	if older {
		m.items = append(m.items, conv...)
		return
	}
	// Fresh load: expand only the first (most-recent) turn.
	for i := range conv {
		if conv[i].Kind == readstore.ItemTurn {
			conv[i].expanded = true
			break
		}
	}
	m.items = conv
}

// Update consumes a tea.Msg and returns an updated copy of itself plus any
// follow-on command.
func (m *Detail) Update(msg tea.Msg) (app.View, tea.Cmd) {
	switch v := msg.(type) {
	case app.TickMsg:
		if m.inFlight || m.loadingOlder || m.offset > 0 || len(m.items) > detailPageSize {
			return m, nil
		}
		m.inFlight = true
		return m, m.fetchCmd()

	case detailDataMsg:
		// Save the current item's identity for cursor restore after refresh.
		var curKind readstore.SessionItemKind
		var curTSNano int64
		hadCursor := false
		if m.cursor < len(m.items) {
			hadCursor = true
			cur := m.items[m.cursor]
			curKind = cur.Kind
			curTSNano = cur.TS.UnixNano()
		}
		m.applyItems(v.items, false)
		m.hasMore = v.hasMore
		m.lastOK = v.at
		m.inFlight = false
		m.loadingOlder = false
		m.stale = false
		m.cursor = 0
		m.offset = 0
		// Restore cursor: match by Kind + TS nanoseconds.
		if hadCursor {
			for i, it := range m.items {
				if it.Kind == curKind && it.TS.UnixNano() == curTSNano {
					m.cursor = i
					break
				}
			}
		}
		// Safety net: guards the empty/shrunk-list case where no item matched the restore.
		if m.cursor >= len(m.items) {
			m.cursor = max0(len(m.items) - 1)
		}
		return m, nil

	case detailOlderMsg:
		m.applyItems(v.items, true)
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
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case key.Matches(v, m.keys.Top):
			m.cursor = 0
		case key.Matches(v, m.keys.Bottom):
			m.cursor = max0(len(m.items) - 1)
		case key.Matches(v, m.keys.PgDn):
			step := m.viewport
			if step < 1 {
				step = 10
			}
			m.cursor += step
			if m.cursor > len(m.items)-1 {
				m.cursor = max0(len(m.items) - 1)
			}
			if m.cursor == len(m.items)-1 && m.hasMore && !m.loadingOlder {
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
			if len(m.items) == 0 {
				return m, nil
			}
			it := m.items[m.cursor]
			// Enter on an event row: open prompt if it's a user_prompt with a PromptID.
			// Enter on turn headers is implemented in Task 8.
			if it.Kind != readstore.ItemEvent {
				return m, nil
			}
			row := it.Event
			if row.EventName != domain.EventUserPrompt || row.PromptID == "" {
				return m, nil
			}
			pool := m.pool
			pid := row.PromptID
			th := m.theme
			return m, func() tea.Msg {
				return app.PushViewMsg{V: newPromptDetail(pool, pid, th)}
			}
		}
	}
	return m, nil
}

// View renders the session event timeline. Renders only the visible window
// (items[offset:offset+viewport]) and slides offset to follow the cursor.
func (m *Detail) View(width, height int) string {
	th := m.theme
	if th == nil {
		d := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
		th = &d
	}
	if width <= 0 {
		width = 90
	}

	// Header
	brand := th.Title.Render(th.Glyphs.Brand + " cco")
	bread := th.Muted.Render(" · session " + shortID(m.sessionID))
	pill := component.StatusPill(th, m.Status())
	headerRight := lipgloss.NewStyle().Width(width - lipgloss.Width(brand) - lipgloss.Width(bread)).Align(lipgloss.Right).Render(pill)
	header := lipgloss.JoinHorizontal(lipgloss.Top, brand, bread, headerRight)

	if len(m.items) == 0 {
		body := th.Muted.Render("no events for this session")
		card := component.Card(th, "", body, width)
		help := component.HelpBar(th, m.helpHints(), width)
		return strings.Join([]string{header, "", card, "", help}, "\n")
	}

	m.viewport = visibleRows(height)
	clampOffset(m)

	// innerW: card outer = width, minus 2 border + 4 padding = content area.
	// Rows wider than the content area get wrap artifacts on background-styled rows.
	innerW := width - 6

	// Column header aligned with TurnHeaderRow columns:
	// glyph(2) + sep(1) + time(8) + sep(1) + label(labelHW) + sep(1) + cost(8) = innerW
	// So labelHW = innerW - 21, and the header string is 3 + 8 + 1 + labelHW + 1 + 8 = innerW.
	labelHW := innerW - 21
	if labelHW < 4 {
		labelHW = 4
	}
	// Header aligns with TurnHeaderRow; EventRow's time column sits 3 cols left of
	// the header label, by design (EventRow rows carry no glyph prefix).
	colHdr := fmt.Sprintf("   %-8s %-*s %-8s", "time", labelHW, "turn / event", "cost")
	rows := []string{th.Muted.Render(colHdr)}

	end := m.offset + m.viewport
	if end > len(m.items) {
		end = len(m.items)
	}
	for i := m.offset; i < end; i++ {
		it := m.items[i]
		switch it.Kind {
		case readstore.ItemTurn:
			t := it.Turn
			label := "/" + t.CommandName
			if t.CommandName == "" {
				label = "prompt"
			}
			rd := component.TurnHeaderRowData{
				Time:         t.StartedAt,
				Label:        label,
				PromptLength: t.PromptLength,
				DurationSec:  t.DurationSec,
				Calls:        t.APIRequests,
				CostUSD:      t.CostUSD,
				Expanded:     it.expanded,
			}
			rows = append(rows, component.TurnHeaderRow(th, rd, i == m.cursor, innerW))
			// Children render only when expanded; currently always empty (Task 8 loads them).
			for j, child := range it.children {
				crd := component.TurnChildRowData{
					Kind:         child.Kind,
					Model:        child.Model,
					CostUSD:      child.CostUSD,
					InputTokens:  child.InputTokens,
					OutputTokens: child.OutputTokens,
					ToolName:     child.ToolName,
					Success:      child.Success,
					DurationMS:   child.DurationMS,
					Last:         j == len(it.children)-1,
				}
				rows = append(rows, component.TurnChildRow(th, crd, innerW))
			}
		case readstore.ItemEvent:
			e := it.Event
			rd := component.EventRowData{
				Time:      e.TS,
				EventName: e.EventName,
				Summary:   e.Summary,
				IsPrompt:  e.EventName == domain.EventUserPrompt && e.PromptID != "",
			}
			rows = append(rows, component.EventRow(th, rd, i == m.cursor, innerW))
		}
	}
	card := component.Card(th, "", strings.Join(rows, "\n"), width)

	var hint string
	switch {
	case m.loadingOlder:
		hint = th.Muted.Render("loading older events…")
	case m.hasMore:
		hint = th.Muted.Render("press d for older events")
	}

	help := component.HelpBar(th, m.helpHints(), width)
	parts := []string{header, "", card}
	if hint != "" {
		parts = append(parts, hint)
	}
	parts = append(parts, "", help)
	return strings.Join(parts, "\n")
}

func (m *Detail) helpHints() []component.KeyHint {
	return []component.KeyHint{
		{Key: "↑↓", Desc: "nav"},
		{Key: "⏎", Desc: "open prompt"},
		{Key: "u/d", Desc: "scroll"},
		{Key: "b", Desc: "back"},
		{Key: "?", Desc: "about"},
		{Key: "q", Desc: "quit"},
	}
}

func shortID(s string) string {
	if len(s) > 8 {
		return s[:8] + "…"
	}
	return s
}

// fetchOlderCmd issues a keyset-paginated fetch for items strictly older
// than the current tail (items[len-1].TS). Result is delivered as
// detailOlderMsg and appended to m.items.
func (m *Detail) fetchOlderCmd() tea.Cmd {
	pool := m.pool
	sid := m.sessionID
	if len(m.items) == 0 {
		return nil
	}
	before := m.items[len(m.items)-1].TS.UnixNano()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), detailFetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		items, hasMore, err := readstore.SessionTurns(ctx, pool, sid, &before, detailPageSize)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return detailOlderMsg{items: items, hasMore: hasMore, at: time.Now()}
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
		items, hasMore, err := readstore.SessionTurns(ctx, pool, sid, nil, detailPageSize)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return detailDataMsg{items: items, hasMore: hasMore, at: time.Now()}
	}
}

var newPromptDetail = prompt.New

// visibleRows converts the terminal height passed to View into the number of
// item rows the body can show. Reserves chromeReserved lines for the chrome
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
// window itself stays within the loaded items range.
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
	maxOffset := len(m.items) - 1
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
}
