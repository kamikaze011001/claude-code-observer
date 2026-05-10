package sessions

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestDetail_KeyJK(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.events = []readstore.EventRow{
		{TS: time.Now(), EventName: "claude_code.tool_result"},
		{TS: time.Now(), EventName: "claude_code.user_prompt", PromptID: "p1"},
	}
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if upd.(*Detail).cursor != 1 {
		t.Fatalf("cursor=%d want 1", upd.(*Detail).cursor)
	}
}

func TestDetail_EnterOnPromptPushes(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.events = []readstore.EventRow{
		{TS: time.Now(), EventName: "claude_code.tool_result", PromptID: "p1"},
		{TS: time.Now(), EventName: "claude_code.user_prompt", PromptID: "p1"},
	}
	m.cursor = 1
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected push cmd")
	}
	msg := cmd()
	if _, ok := msg.(app.PushViewMsg); !ok {
		t.Fatalf("msg=%T want PushViewMsg", msg)
	}
}

func TestDetail_EnterOnNonPromptDoesNothing(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.events = []readstore.EventRow{
		{TS: time.Now(), EventName: "claude_code.tool_result"},
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no cmd; got one")
	}
}

func TestDetail_StatusPill(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	if m.Status() != theme.PillNoDaemon {
		t.Fatal("empty status")
	}
	m.events = []readstore.EventRow{{}}
	if m.Status() != theme.PillLive {
		t.Fatal("with events")
	}
	m.stale = true
	if m.Status() != theme.PillStale {
		t.Fatal("stale")
	}
}

func TestDetail_Title(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "abcdef123456").(*Detail)
	if m.Title() != "SESSION abcdef12…" {
		t.Fatalf("title=%q", m.Title())
	}
}

var _ app.View = (*Detail)(nil)
