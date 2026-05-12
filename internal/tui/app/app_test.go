package app

import (
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

// fakeView is a no-op View used for stack assertions.
type fakeView struct {
	title      string
	lastMsg    tea.Msg
	initCalled bool
}

func (v *fakeView) Init() tea.Cmd                    { v.initCalled = true; return nil }
func (v *fakeView) Update(m tea.Msg) (View, tea.Cmd) { v.lastMsg = m; return v, nil }
func (v *fakeView) View(w, h int) string             { return v.title }
func (v *fakeView) Title() string                    { return v.title }
func (v *fakeView) ShortHelp() []key.Binding         { return nil }
func (v *fakeView) Status() component.Status { return component.StatusLive }

func newAppWith(views ...View) *App {
	a := New(theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs()))
	for _, v := range views {
		a.Push(v)
	}
	return a
}

func TestApp_QuitOnQ(t *testing.T) {
	a := newAppWith(&fakeView{title: "X"})
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("q should return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("q cmd should produce tea.QuitMsg, got %T", msg)
	}
}

func TestApp_BackPopsWhenStackDeep(t *testing.T) {
	a := newAppWith(&fakeView{title: "A"}, &fakeView{title: "B"})
	if got := a.StackDepth(); got != 2 {
		t.Fatalf("setup: depth %d", got)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if got := a.StackDepth(); got != 1 {
		t.Fatalf("after b: depth %d", got)
	}
}

func TestApp_BackNoOpAtRoot(t *testing.T) {
	a := newAppWith(&fakeView{title: "A"})
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if got := a.StackDepth(); got != 1 {
		t.Fatalf("root b should be no-op, depth=%d", got)
	}
}

func TestApp_RefreshForwardsTickToTopView(t *testing.T) {
	v := &fakeView{title: "A"}
	a := newAppWith(v)
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if _, ok := v.lastMsg.(TickMsg); !ok {
		t.Fatalf("r should forward TickMsg to top view, got %T", v.lastMsg)
	}
}

func TestApp_PushViewMsgPushesAndInits(t *testing.T) {
	a := newAppWith(&fakeView{title: "A"})
	child := &fakeView{title: "B"}
	a.Update(PushViewMsg{V: child})
	if got := a.StackDepth(); got != 2 {
		t.Fatalf("after push: depth %d", got)
	}
	if !child.initCalled {
		t.Fatalf("Init() should be called on push")
	}
}

func TestApp_ErrMsgIncrementsCounter(t *testing.T) {
	a := newAppWith(&fakeView{title: "A"})
	a.Update(ErrMsg{Err: errors.New("boom")})
	a.Update(ErrMsg{Err: errors.New("boom")})
	if got := a.ConsecutiveErrors(); got != 2 {
		t.Fatalf("consecutive errors: got %d want 2", got)
	}
}

func TestApp_TickForwardedToTop(t *testing.T) {
	v := &fakeView{title: "A"}
	a := newAppWith(v)
	a.Update(TickMsg{})
	if _, ok := v.lastMsg.(TickMsg); !ok {
		t.Fatalf("TickMsg should be forwarded, got %T", v.lastMsg)
	}
}

func TestApp_ConsecErrsResetOnSuccess(t *testing.T) {
	a := newAppWith(&fakeView{title: "A"})
	a.Update(ErrMsg{Err: errors.New("x")})
	a.Update(ErrMsg{Err: errors.New("x")})
	if a.ConsecutiveErrors() != 2 {
		t.Fatalf("setup: %d", a.ConsecutiveErrors())
	}
	// A non-Err message that goes through forwardTop should reset.
	a.Update(TickMsg{})
	if a.ConsecutiveErrors() != 0 {
		t.Fatalf("non-Err forward should reset, got %d", a.ConsecutiveErrors())
	}
}
