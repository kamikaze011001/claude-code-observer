package waterfall

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

var _ app.View = (*Model)(nil)

func TestModel_Title(t *testing.T) {
	t.Parallel()
	m := New(nil, "abcdef123456", nil).(*Model)
	if m.Title() != "WATERFALL abcdef12…" {
		t.Fatalf("title = %q", m.Title())
	}
}

func TestModel_NotFoundSetsFlag(t *testing.T) {
	t.Parallel()
	m := New(nil, "p", nil).(*Model)
	upd, _ := m.Update(app.ErrMsg{Err: readstore.ErrNotFound})
	if !upd.(*Model).notFound {
		t.Fatal("expected notFound=true on ErrNotFound")
	}
}

func TestModel_GenericErrorSetsStale(t *testing.T) {
	t.Parallel()
	m := New(nil, "p", nil).(*Model)
	upd, _ := m.Update(app.ErrMsg{Err: errors.New("boom")})
	if !upd.(*Model).stale {
		t.Fatal("expected stale=true on generic error")
	}
}

func TestModel_DataPopulatesBars(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	m := New(nil, "p", nil).(*Model)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(time.Second), DurationMS: 1000, QuerySource: "main"},
		{TS: base.Add(3 * time.Second), DurationMS: 2000, QuerySource: "subagent"},
	}
	upd, _ := m.Update(waterfallDataMsg{reqs: reqs, at: time.Now()})
	got := upd.(*Model)
	if len(got.bars) != 2 {
		t.Fatalf("want 2 bars, got %d", len(got.bars))
	}
	if got.inFlight {
		t.Fatal("inFlight should be cleared after data")
	}
}

func TestModel_CursorMovesWithinBounds(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	m := New(nil, "p", nil).(*Model)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(time.Second), DurationMS: 1000, QuerySource: "main"},
		{TS: base.Add(3 * time.Second), DurationMS: 1000, QuerySource: "subagent"},
	}
	upd, _ := m.Update(waterfallDataMsg{reqs: reqs, at: time.Now()})
	m = upd.(*Model)

	upd, _ = m.Update(keyMsg("down"))
	m = upd.(*Model)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.cursor)
	}
	upd, _ = m.Update(keyMsg("down"))
	if upd.(*Model).cursor != 1 {
		t.Fatalf("cursor should clamp at 1")
	}
	upd, _ = m.Update(keyMsg("up"))
	m = upd.(*Model)
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.cursor)
	}
	upd, _ = m.Update(keyMsg("up"))
	if upd.(*Model).cursor != 0 {
		t.Fatalf("cursor should clamp at 0")
	}
}

func TestModel_TickAfterDataRefetches(t *testing.T) {
	t.Parallel()
	m := New(nil, "p", nil).(*Model)
	m.inFlight = false
	_, cmd := m.Update(app.TickMsg{})
	if cmd == nil {
		t.Fatal("expected fetch cmd from tick")
	}
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}
