package about

import (
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

func newModel(t *testing.T) *Model {
	t.Helper()
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	return New(&th, "v1.2.3", "abc1234")
}

func TestModel_Title(t *testing.T) {
	if got := newModel(t).Title(); got != "ABOUT" {
		t.Fatalf("Title() = %q, want %q", got, "ABOUT")
	}
}

func TestModel_Init_NoCmd(t *testing.T) {
	if cmd := newModel(t).Init(); cmd != nil {
		t.Fatalf("Init() should return nil cmd")
	}
}

func TestModel_BackKeyReturnsPopViewMsg(t *testing.T) {
	m := newModel(t)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd == nil {
		t.Fatalf("b should produce a tea.Cmd")
	}
	msg := cmd()
	if _, ok := msg.(app.PopViewMsg); !ok {
		t.Fatalf("b cmd should return PopViewMsg, got %T", msg)
	}
}

func TestModel_ShortHelp_ContainsBackAndQuit(t *testing.T) {
	help := newModel(t).ShortHelp()
	keys := make(map[string]bool)
	for _, k := range help {
		keys[k.Help().Key] = true
	}
	if !keys["b"] {
		t.Fatalf("ShortHelp missing [b] back; got %+v", help)
	}
	if !keys["q"] {
		t.Fatalf("ShortHelp missing [q] quit; got %+v", help)
	}
	var _ []key.Binding = help
}

func TestModel_Status_Live(t *testing.T) {
	if newModel(t).Status() != component.StatusLive {
		t.Fatalf("about view should report StatusLive (it has no daemon dependency)")
	}
}
