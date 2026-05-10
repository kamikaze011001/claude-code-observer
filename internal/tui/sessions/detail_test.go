package sessions

import (
	"flag"
	"os"
	"path/filepath"
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

var updateDetail = flag.Bool("update-detail", false, "update detail goldens")

func goldenDetail(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	got = stripANSI(got)
	if *updateDetail {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestDetail_View_Empty(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "abcdef123456").(*Detail)
	out := m.View(80, 20)
	goldenDetail(t, "detail_empty", out)
}

func TestDetail_View_Mixed(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "abcdef123456").(*Detail)
	m.events = []readstore.EventRow{
		{TS: mustTime("2026-05-10T12:43:01Z"), EventName: "claude_code.user_prompt", PromptID: "p1", Summary: "prompt: 142ch /commit"},
		{TS: mustTime("2026-05-10T12:43:02Z"), EventName: "claude_code.tool_result", Summary: "Read 12ms"},
		{TS: mustTime("2026-05-10T12:43:03Z"), EventName: "claude_code.api_request", Summary: "claude-opus-4-7 $0.0021"},
		{TS: mustTime("2026-05-10T12:43:05Z"), EventName: "claude_code.user_prompt", PromptID: "p2", Summary: "prompt: 88ch"},
	}
	m.cursor = 0
	m.lastOK = mustTime("2026-05-10T12:43:06Z")
	out := m.View(100, 20)
	goldenDetail(t, "detail_mixed", out)
}
