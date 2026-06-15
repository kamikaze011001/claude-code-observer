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
	header  readstore.SessionHeader
}

// detailOlderMsg carries items older than the current tail. Unlike
// detailDataMsg it is appended to m.items, never replaces — preserves the
// user's cursor and scroll position while the list grows below them.
type detailOlderMsg struct {
	items   []readstore.SessionItem
	hasMore bool
	at      time.Time
}

// detailChildrenMsg delivers the lazily-loaded children for a specific turn.
type detailChildrenMsg struct {
	promptID string
	children []readstore.TurnChild
}

// Detail is the session event timeline view model.
type Detail struct {
	pool         *sql.DB
	theme        *theme.Theme
	sessionID    string
	header       readstore.SessionHeader
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
		m.keys.Toggle,
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
		m.header = v.header
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

	case detailChildrenMsg:
		for i := range m.items {
			if m.items[i].Kind == readstore.ItemTurn && m.items[i].Turn.PromptID == v.promptID {
				m.items[i].children = v.children
				m.items[i].loaded = true
				break
			}
		}
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
		case key.Matches(v, m.keys.Toggle):
			if len(m.items) == 0 || m.items[m.cursor].Kind != readstore.ItemTurn {
				return m, nil
			}
			it := &m.items[m.cursor]
			it.expanded = !it.expanded
			if it.expanded && !it.loaded {
				return m, m.fetchChildrenCmd(it.Turn.PromptID)
			}
			return m, nil
		case key.Matches(v, m.keys.Enter):
			if len(m.items) == 0 || m.items[m.cursor].Kind != readstore.ItemTurn {
				return m, nil
			}
			pid := m.items[m.cursor].Turn.PromptID
			if pid == "" {
				return m, nil
			}
			pool, th := m.pool, m.theme
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
		emptyParts := []string{header, ""}
		if m.header.SessionID != "" {
			emptyParts = append(emptyParts, m.renderProductivityCard(th, width), "")
		}
		emptyParts = append(emptyParts, card, "", help)
		return strings.Join(emptyParts, "\n")
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

	// Row-budget loop: tracks rendered rows (1 per turn header + 1 per child +
	// 1 per event) and stops once the budget (m.viewport) is consumed.  This
	// prevents an expanded turn with many children from overflowing the card
	// height, while leaving cursor navigation item-based (clampOffset /
	// visibleRows / PgDn step are unchanged).
	budget := m.viewport
	for i := m.offset; budget > 0 && i < len(m.items); i++ {
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
			budget-- // one row consumed for the turn header
			// Children render only when expanded; each child consumes one budget row.
			for j, child := range it.children {
				if budget <= 0 {
					break
				}
				// budget==1 means this is the last row we can render: treat it
				// as the visual tail so TurnChildRow draws the ╰ connector even
				// when the real child list continues beyond the budget.
				isLast := j == len(it.children)-1 || budget == 1
				crd := component.TurnChildRowData{
					Kind:         child.Kind,
					Model:        child.Model,
					CostUSD:      child.CostUSD,
					InputTokens:  child.InputTokens,
					OutputTokens: child.OutputTokens,
					ToolName:     child.ToolName,
					Success:      child.Success,
					DurationMS:   child.DurationMS,
					Last:         isLast,
				}
				rows = append(rows, component.TurnChildRow(th, crd, innerW))
				budget--
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
			budget--
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
	parts := []string{header, ""}
	if m.header.SessionID != "" {
		parts = append(parts, m.renderProductivityCard(th, width), "")
	}
	parts = append(parts, card)
	if hint != "" {
		parts = append(parts, hint)
	}
	parts = append(parts, "", help)
	return strings.Join(parts, "\n")
}

func (m *Detail) helpHints() []component.KeyHint {
	return []component.KeyHint{
		{Key: "↑↓", Desc: "nav"},
		{Key: "space", Desc: "expand"},
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

// fetchChildrenCmd issues a fetch for the child events (api_request /
// tool_result) belonging to promptID. The result is delivered as
// detailChildrenMsg and merged into the matching timelineItem.
func (m *Detail) fetchChildrenCmd(promptID string) tea.Cmd {
	pool := m.pool
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), detailFetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		ch, err := readstore.SessionTurnChildren(ctx, pool, promptID)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return detailChildrenMsg{promptID: promptID, children: ch}
	}
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
		// Best-effort: a missing or erroring header is treated as zero-value (card hidden).
		hdr, _ := readstore.SessionHeaderRow(ctx, pool, sid)
		return detailDataMsg{items: items, hasMore: hasMore, at: time.Now(), header: hdr}
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

// itemRows reports how many rendered rows an item occupies: a turn header plus
// its children when expanded (children are empty until lazily loaded), otherwise
// a single row. Mirrors the row-budget accounting in View.
func itemRows(it timelineItem) int {
	if it.Kind == readstore.ItemTurn && it.expanded {
		return 1 + len(it.children)
	}
	return 1
}

// renderProductivityCard shows lines, commits/PRs, active time, and edit
// accept-rate for the session. It is only rendered when m.header.SessionID is
// non-empty (i.e. the header was successfully loaded from the DB).
func (m *Detail) renderProductivityCard(t *theme.Theme, width int) string {
	h := m.header
	acceptRate := "—"
	total := h.EditsAccepted + h.EditsRejected
	if total > 0 {
		pct := float64(h.EditsAccepted) / float64(total) * 100
		acceptRate = fmt.Sprintf("%.0f%% (%d/%d)", pct, h.EditsAccepted, total)
	}

	var b strings.Builder
	writeKV := func(label, value string) {
		b.WriteString(t.Label.Render(lipgloss.NewStyle().Width(12).Render(label)) + t.Value.Render(value) + "\n")
	}
	writeKV("lines", fmt.Sprintf("+%s -%s", component.HumanInt(h.LinesAdded), component.HumanInt(h.LinesRemoved)))
	writeKV("commits", fmt.Sprintf("%d", h.Commits))
	writeKV("pull reqs", fmt.Sprintf("%d", h.PullRequests))
	writeKV("active", component.HumanActiveDuration(h.ActiveSec))
	writeKV("edit accept", acceptRate)

	return component.Card(t, "productivity", strings.TrimRight(b.String(), "\n"), width)
}

// clampOffset slides m.offset so the cursor is in the visible window and the
// window itself stays within the loaded items range.
//
// Note: clampOffset is only called from View after the early-return for empty
// items, so len(m.items) > 0 is guaranteed here.
func clampOffset(m *Detail) {
	// Scrolling up: ensure cursor is not above the visible window.
	if m.cursor < m.offset {
		m.offset = m.cursor
	}

	// Scrolling down: walk up from the cursor accumulating rendered rows until
	// the budget (m.viewport) is reached, so the cursor item's header is
	// guaranteed to render even when an expanded turn above it would otherwise
	// consume the entire budget.
	if m.cursor >= m.offset {
		rowsUsed := 1 // the cursor item's own header row
		newOffset := m.cursor
		for i := m.cursor - 1; i >= m.offset; i-- {
			r := itemRows(m.items[i])
			if rowsUsed+r > m.viewport {
				break
			}
			rowsUsed += r
			newOffset = i
		}
		if newOffset > m.offset {
			m.offset = newOffset
		}
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
