package prompt

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

var defaultTheme = theme.Default()

const fetchTimeout = 500 * time.Millisecond

var errNoPool = errors.New("prompt: no read pool")

type detailDataMsg struct {
	result readstore.PromptDetailResult
	at     time.Time
}

// Detail is the Prompt Detail view.
type Detail struct {
	pool     *sql.DB
	promptID string
	result   readstore.PromptDetailResult
	notFound bool
	inFlight bool
	stale    bool
	lastOK   time.Time
}

// New constructs a Detail bound to a promptID.
func New(pool *sql.DB, promptID string) app.View {
	return &Detail{pool: pool, promptID: promptID}
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
	var b strings.Builder
	b.WriteString(defaultTheme.Heading.Render(d.Title()))
	if d.notFound {
		b.WriteString("\n\n")
		b.WriteString(defaultTheme.MutedText.Render("prompt not found — it may have been pruned"))
		return b.String()
	}
	if d.result.Prompt.PromptID == "" {
		b.WriteString("\n\n")
		b.WriteString(defaultTheme.MutedText.Render("loading…"))
		return b.String()
	}
	p := d.result.Prompt
	durSec := int64(0)
	if !p.EndedAt.IsZero() {
		durSec = int64(p.EndedAt.Sub(p.StartedAt).Seconds())
	}
	header := fmt.Sprintf("session %s   started %s   duration %ds",
		shortID(p.SessionID), p.StartedAt.Format("2006-01-02 15:04:05"), durSec)
	b.WriteString("\n")
	b.WriteString(defaultTheme.MutedText.Render(header))
	b.WriteString("\n\n")

	cost := fmt.Sprintf("$%.4f\n%d api requests", p.CostUSD, p.APIRequests)
	tokens := fmt.Sprintf("in %d / out %d\ncache r %d / w %d",
		p.InputTokens, p.OutputTokens, p.CacheReadTokens, p.CacheCreationTokens)
	b.WriteString(defaultTheme.Block(28).Render("Cost\n" + cost))
	b.WriteString("  ")
	b.WriteString(defaultTheme.Block(28).Render("Tokens\n" + tokens))
	b.WriteString("\n\n")

	b.WriteString(defaultTheme.Heading.Render("API REQUESTS"))
	b.WriteString("\n")
	if len(d.result.APIRequests) == 0 {
		b.WriteString(defaultTheme.MutedText.Render("  (none)\n"))
	} else {
		for _, r := range d.result.APIRequests {
			line := fmt.Sprintf("  %s  %-18s $%-7.4f  in %d out %d",
				r.TS.Format("15:04:05"), r.Model, r.CostUSD, r.InputTokens, r.OutputTokens)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(defaultTheme.Heading.Render("TOOL CALLS"))
	b.WriteString("\n")
	if len(d.result.ToolCalls) == 0 {
		b.WriteString(defaultTheme.MutedText.Render("  (none)\n"))
	} else {
		for _, tc := range d.result.ToolCalls {
			mark := ""
			if !tc.Success {
				mark = " ✗"
			}
			line := fmt.Sprintf("  %s  %-10s %dms%s",
				tc.TS.Format("15:04:05"), tc.ToolName, tc.DurationMS, mark)
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return b.String()
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
