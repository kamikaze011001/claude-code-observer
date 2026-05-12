package prompt

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

const fetchTimeout = 500 * time.Millisecond

var errNoPool = errors.New("prompt: no read pool")

type detailDataMsg struct {
	result readstore.PromptDetailResult
	at     time.Time
}

// Detail is the Prompt Detail view.
type Detail struct {
	pool     *sql.DB
	theme    *theme.Theme
	promptID string
	result   readstore.PromptDetailResult
	notFound bool
	inFlight bool
	stale    bool
	lastOK   time.Time
}

// New constructs a Detail bound to a promptID.
func New(pool *sql.DB, promptID string, th *theme.Theme) app.View {
	return &Detail{pool: pool, theme: th, promptID: promptID}
}

func (d *Detail) th() *theme.Theme {
	if d.theme != nil {
		return d.theme
	}
	t := theme.Default()
	return &t
}

func (d *Detail) Init() tea.Cmd {
	d.inFlight = true
	return d.fetchCmd()
}

func (d *Detail) Title() string {
	id := d.promptID
	if len(id) > 8 {
		id = id[:8] + "…"
	}
	return "PROMPT " + id
}

func (d *Detail) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

func (d *Detail) Status() theme.PillState {
	if d.notFound || (d.lastOK.IsZero() && d.result.Prompt.PromptID == "") {
		return theme.PillNoDaemon
	}
	if d.stale {
		return theme.PillStale
	}
	return theme.PillLive
}

func (d *Detail) statusFor() component.Status {
	switch d.Status() {
	case theme.PillLive:
		return component.StatusLive
	case theme.PillStale:
		return component.StatusStale
	default:
		return component.StatusNoDaemon
	}
}

func (d *Detail) Update(msg tea.Msg) (app.View, tea.Cmd) {
	switch v := msg.(type) {
	case app.TickMsg:
		if d.inFlight {
			return d, nil
		}
		d.inFlight = true
		return d, d.fetchCmd()
	case detailDataMsg:
		d.result = v.result
		d.notFound = false
		d.stale = false
		d.lastOK = v.at
		d.inFlight = false
		return d, nil
	case app.ErrMsg:
		d.inFlight = false
		if errors.Is(v.Err, readstore.ErrNotFound) {
			d.notFound = true
			return d, nil
		}
		d.stale = true
		return d, nil
	}
	return d, nil
}

func (d *Detail) View(width, height int) string {
	th := d.th()
	if width <= 0 {
		width = 90
	}

	// Header
	brand := th.Title.Render(th.Glyphs.Brand + " cco")
	bread := th.Muted2.Render(" · prompt " + shortID(d.promptID))
	pill := component.StatusPill(th, d.statusFor())
	headerRight := lipgloss.NewStyle().Width(width - lipgloss.Width(brand) - lipgloss.Width(bread)).Align(lipgloss.Right).Render(pill)
	header := lipgloss.JoinHorizontal(lipgloss.Top, brand, bread, headerRight)

	if d.notFound {
		body := th.Muted2.Render("prompt not found — it may have been pruned")
		return strings.Join([]string{header, "", component.Card(th, "", body, width)}, "\n")
	}
	if d.result.Prompt.PromptID == "" {
		body := th.Muted2.Render("loading…")
		return strings.Join([]string{header, "", component.Card(th, "", body, width)}, "\n")
	}

	p := d.result.Prompt
	dur := int64(0)
	if !p.EndedAt.IsZero() {
		dur = int64(p.EndedAt.Sub(p.StartedAt).Seconds())
	}
	info := strings.Join([]string{
		th.Muted2.Render("session "), th.Accent.Render(shortID(p.SessionID)),
		th.Muted2.Render(" · started "), th.Accent.Render(p.StartedAt.Format("15:04:05")),
		th.Muted2.Render(" · duration "), th.Accent.Render(fmt.Sprintf("%ds", dur)),
		th.Muted2.Render(" · "), th.Accent.Render(fmt.Sprintf("%d chars", p.PromptLength)),
	}, "")

	// 3 summary cards
	// cardW = outer width of each card (including borders).
	// Card inner content width = cardW - 2 (borders) - 4 (padding) = cardW - 6.
	cardW := (width - 2) / 3
	cardContent := cardW - 6
	if cardContent < 8 {
		cardContent = 8
	}
	costBody := th.Value.Render(fmt.Sprintf("$%.2f", p.CostUSD)) + "\n\n" +
		th.Muted2.Render(fmt.Sprintf("%d api requests", p.APIRequests))
	tokensBody := strings.Join([]string{
		labelValue(th, "in", fmt.Sprintf("%d", p.InputTokens), cardContent),
		labelValue(th, "out", fmt.Sprintf("%d", p.OutputTokens), cardContent),
		labelValue(th, "cache r/w", fmt.Sprintf("%s / %s", component.HumanInt(p.CacheReadTokens), component.HumanInt(p.CacheCreationTokens)), cardContent),
	}, "\n")
	activityBody := strings.Join([]string{
		labelValue(th, "api reqs", fmt.Sprintf("%d", p.APIRequests), cardContent),
		labelValue(th, "tool calls", fmt.Sprintf("%d", p.ToolCalls), cardContent),
		labelValue(th, "errors", errorCountStyled(th, p), cardContent),
	}, "\n")
	summaryRow := lipgloss.JoinHorizontal(lipgloss.Top,
		component.Card(th, "cost", costBody, cardW), " ",
		component.Card(th, "tokens", tokensBody, cardW), " ",
		component.Card(th, "activity", activityBody, cardW),
	)

	// api requests card
	apiRows := []string{}
	for _, r := range d.result.APIRequests {
		apiRows = append(apiRows, component.APIRequestRow(th, component.APIRequestRowData{
			Time: r.TS, Model: r.Model, CostUSD: r.CostUSD,
			InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
		}, width-4))
	}
	apiCard := component.Card(th, "api requests", strings.Join(apiRows, "\n"), width)
	if len(apiRows) == 0 {
		apiCard = component.Card(th, "api requests", th.Muted2.Render("(none)"), width)
	}

	// tool calls card
	tcRows := []string{}
	for _, c := range d.result.ToolCalls {
		note := ""
		if !c.Success {
			note = "failed"
		}
		tcRows = append(tcRows, component.ToolCallRow(th, component.ToolCallRowData{
			Time: c.TS, ToolName: c.ToolName, Success: c.Success,
			DurationMS: c.DurationMS, Note: note,
		}, width-4))
	}
	tcCard := component.Card(th, "tool calls", strings.Join(tcRows, "\n"), width)
	if len(tcRows) == 0 {
		tcCard = component.Card(th, "tool calls", th.Muted2.Render("(none)"), width)
	}

	help := component.HelpBar(th, []component.KeyHint{
		{Key: "b", Desc: "back"},
		{Key: "r", Desc: "refresh"},
		{Key: "q", Desc: "quit"},
	}, width)
	return strings.Join([]string{header, "", info, "", summaryRow, "", apiCard, "", tcCard, "", help}, "\n")
}

// labelValue renders a "label   value" line padded to width.
func labelValue(th *theme.Theme, label, value string, width int) string {
	lbl := th.Label.Render(label)
	gap := width - lipgloss.Width(lbl) - lipgloss.Width(value)
	if gap < 1 {
		gap = 1
	}
	return lbl + strings.Repeat(" ", gap) + value
}

func errorCountStyled(th *theme.Theme, p readstore.Prompt) string {
	s := fmt.Sprintf("%d", boolInt(p.HadError))
	if p.HadError {
		return lipgloss.NewStyle().Foreground(th.Palette.Red).Render(s)
	}
	return s
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}

func (d *Detail) fetchCmd() tea.Cmd {
	pool := d.pool
	pid := d.promptID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		res, err := readstore.PromptDetail(ctx, pool, pid)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return detailDataMsg{result: res, at: time.Now()}
	}
}
