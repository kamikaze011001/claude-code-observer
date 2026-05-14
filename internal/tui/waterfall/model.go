package waterfall

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

var errNoPool = errors.New("waterfall: no read pool")

// waterfallDataMsg carries a completed PromptWaterfall fetch.
type waterfallDataMsg struct {
	reqs []readstore.WaterfallRequest
	at   time.Time
}

// Model is the waterfall view: a per-prompt timeline of api_request /
// api_error events banded into main/subagent/auxiliary lanes.
type Model struct {
	pool        *sql.DB
	theme       *theme.Theme
	promptID    string
	reqs        []readstore.WaterfallRequest
	bars        []Bar // ordered by ts (fetch order); cursor indexes this
	totalSpanMS int64
	cursor      int
	notFound    bool
	inFlight    bool
	stale       bool
	lastOK      time.Time
}

// New constructs a waterfall Model bound to a promptID.
func New(pool *sql.DB, promptID string, th *theme.Theme) app.View {
	return &Model{pool: pool, theme: th, promptID: promptID}
}

func (m *Model) th() *theme.Theme {
	if m.theme != nil {
		return m.theme
	}
	t := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	return &t
}

func (m *Model) Init() tea.Cmd {
	m.inFlight = true
	return m.fetchCmd()
}

func (m *Model) Title() string {
	return "WATERFALL " + shortID(m.promptID)
}

func (m *Model) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "select")),
		key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "about")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

func (m *Model) Status() component.Status {
	if m.notFound || (m.lastOK.IsZero() && len(m.bars) == 0) {
		return component.StatusNoDaemon
	}
	if m.stale {
		return component.StatusStale
	}
	return component.StatusLive
}

func (m *Model) Update(msg tea.Msg) (app.View, tea.Cmd) {
	switch v := msg.(type) {
	case app.TickMsg:
		if m.inFlight {
			return m, nil
		}
		m.inFlight = true
		return m, m.fetchCmd()
	case waterfallDataMsg:
		m.reqs = v.reqs
		m.bars, m.totalSpanMS = buildBars(v.reqs)
		if m.cursor > len(m.bars)-1 {
			m.cursor = max0(len(m.bars) - 1)
		}
		m.notFound = false
		m.stale = false
		m.lastOK = v.at
		m.inFlight = false
		return m, nil
	case app.ErrMsg:
		m.inFlight = false
		if errors.Is(v.Err, readstore.ErrNotFound) {
			m.notFound = true
			return m, nil
		}
		m.stale = true
		return m, nil
	case tea.KeyMsg:
		switch v.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.bars)-1 {
				m.cursor++
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) fetchCmd() tea.Cmd {
	pool := m.pool
	pid := m.promptID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		reqs, err := readstore.PromptWaterfall(ctx, pool, pid)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return waterfallDataMsg{reqs: reqs, at: time.Now()}
	}
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}

// View is a placeholder — Task 6 (view.go) will replace this with the real
// rendering implementation. It exists here only so Model satisfies the
// app.View interface in isolation.
// TODO(task6): remove this method when view.go defines the real View().
func (m *Model) View(width, height int) string { return "" }
