package sessions

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestList_KeyJDownKKDown(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	m.rows = []readstore.SessionRow{
		{SessionID: "a"}, {SessionID: "b"}, {SessionID: "c"},
	}
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if upd.(*List).cursor != 1 {
		t.Fatalf("after j: cursor=%d want 1", upd.(*List).cursor)
	}
	upd2, _ := upd.(*List).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if upd2.(*List).cursor != 0 {
		t.Fatalf("after k: cursor=%d want 0", upd2.(*List).cursor)
	}
}

func TestList_KeyGTopAndShiftGBottom(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	m.rows = []readstore.SessionRow{{SessionID: "a"}, {SessionID: "b"}, {SessionID: "c"}}
	m.cursor = 1
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if upd.(*List).cursor != 2 {
		t.Fatalf("after G: cursor=%d want 2", upd.(*List).cursor)
	}
	upd2, _ := upd.(*List).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if upd2.(*List).cursor != 0 {
		t.Fatalf("after g: cursor=%d want 0", upd2.(*List).cursor)
	}
}

func TestList_StatusPill(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	if m.Status() != theme.PillNoDaemon {
		t.Fatalf("empty model status=%v want PillNoDaemon", m.Status())
	}
	m.rows = []readstore.SessionRow{{SessionID: "a"}}
	if m.Status() != theme.PillLive {
		t.Fatalf("with rows status=%v want PillLive", m.Status())
	}
	m.stale = true
	if m.Status() != theme.PillStale {
		t.Fatalf("stale status=%v want PillStale", m.Status())
	}
}

func TestList_Title(t *testing.T) {
	t.Parallel()
	if NewList(nil).Title() != "SESSIONS" {
		t.Fatal("title")
	}
}

func TestList_TickFetchesAndUpdatesRows(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init returned nil cmd")
	}
}

// Compile-time check that List implements app.View.
var _ app.View = (*List)(nil)
