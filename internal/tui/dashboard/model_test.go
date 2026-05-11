package dashboard

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/sessions"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestModel_InitReturnsFetchCmd(t *testing.T) {
	m := New(nil, nil)
	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("Init should return a fetch cmd")
	}
}

func TestModel_TickWhileInFlightSkipsFetch(t *testing.T) {
	m := New(nil, nil)
	m.inFlight = true
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd != nil {
		t.Fatalf("tick during in-flight should not start a new fetch")
	}
}

func TestModel_DataMsgClearsInFlightAndStoresSnapshot(t *testing.T) {
	m := New(nil, nil)
	m.inFlight = true
	snap := readstore.Snapshot{Today: readstore.WindowStats{CostUSD: 1.23}}
	top := []readstore.TopSession{{SessionID: "x"}}
	updated, _ := m.Update(dataMsg{snap: snap, top: top})
	got := updated.(*Model)
	if got.inFlight {
		t.Fatalf("dataMsg should clear inFlight")
	}
	if got.snap.Today.CostUSD != 1.23 {
		t.Fatalf("snapshot not stored: %+v", got.snap)
	}
	if len(got.top) != 1 || got.top[0].SessionID != "x" {
		t.Fatalf("top not stored: %+v", got.top)
	}
}

func TestModel_ErrMsgKeepsLastSnapshotAndSetsStale(t *testing.T) {
	m := New(nil, nil)
	m.snap.Today.CostUSD = 9.99
	m.inFlight = true
	updated, _ := m.Update(app.ErrMsg{Err: errors.New("boom")})
	got := updated.(*Model)
	if got.snap.Today.CostUSD != 9.99 {
		t.Fatalf("ErrMsg should preserve last snapshot")
	}
	if got.inFlight {
		t.Fatalf("ErrMsg should clear inFlight")
	}
	if !got.stale {
		t.Fatalf("ErrMsg should set stale=true")
	}
}

var _ app.View = (*Model)(nil)

func TestModel_InitCmdInvocable(t *testing.T) {
	m := New(nil, nil)
	cmd := m.Init()
	msg := cmd()
	if msg == nil {
		t.Fatalf("init cmd should produce a message even on failure")
	}
	if _, ok := msg.(app.ErrMsg); !ok {
		if _, ok := msg.(dataMsg); !ok {
			t.Fatalf("unexpected msg type %T", msg)
		}
	}
}

func TestModel_StatusNoDaemon(t *testing.T) {
	m := New(nil, nil)
	if got := m.Status(); got != theme.PillNoDaemon {
		t.Fatalf("status: got %v want PillNoDaemon", got)
	}
}

func TestModel_StatusStaleOnError(t *testing.T) {
	m := New(nil, nil)
	m.stale = true
	m.lastOK = time.Now()
	if got := m.Status(); got != theme.PillStale {
		t.Fatalf("status: got %v want PillStale", got)
	}
}

func TestModel_StatusLiveWithRecentEvent(t *testing.T) {
	m := New(nil, nil)
	fakeNow := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return fakeNow }
	m.lastOK = fakeNow
	m.snap.LatestEventTS = fakeNow.Add(-5 * time.Second).UnixNano()
	if got := m.Status(); got != theme.PillLive {
		t.Fatalf("status: got %v want PillLive", got)
	}
}

func TestModel_CursorClampsAtBounds(t *testing.T) {
	m := New(nil, nil)
	m.recent = []readstore.TopSession{
		{SessionID: "a"}, {SessionID: "b"}, {SessionID: "c"},
	}
	m.recentCursor = 0

	// Up at top should stay at 0
	m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.recentCursor != 0 {
		t.Fatalf("cursor should clamp at 0, got %d", m.recentCursor)
	}

	// Down twice → 2
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.recentCursor != 2 {
		t.Fatalf("cursor should be 2, got %d", m.recentCursor)
	}

	// Down again should clamp at 2 (len-1)
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.recentCursor != 2 {
		t.Fatalf("cursor should clamp at 2, got %d", m.recentCursor)
	}
}

func TestModel_EnterEmitsPushDetail(t *testing.T) {
	m := New(nil, nil)
	m.recent = []readstore.TopSession{
		{SessionID: "sess-xyz"},
	}
	m.recentCursor = 0
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter on non-empty recent should return a cmd")
	}
	msg := cmd()
	push, ok := msg.(app.PushViewMsg)
	if !ok {
		t.Fatalf("msg=%T want PushViewMsg", msg)
	}
	if push.V == nil {
		t.Fatal("pushed view is nil")
	}
}

func TestModel_EnterNoOpWhenEmpty(t *testing.T) {
	m := New(nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("Enter with empty recent should not return a cmd")
	}
}

func TestModel_KeySEmitsPushSessionsList(t *testing.T) {
	t.Parallel()
	m := New(nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("no cmd returned for 's'")
	}
	msg := cmd()
	push, ok := msg.(app.PushViewMsg)
	if !ok {
		t.Fatalf("msg=%T want PushViewMsg", msg)
	}
	if _, isList := push.V.(*sessions.List); !isList {
		t.Fatalf("pushed view is %T, want *sessions.List", push.V)
	}
}
