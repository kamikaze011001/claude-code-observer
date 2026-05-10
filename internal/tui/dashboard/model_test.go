package dashboard

import (
	"errors"
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestModel_InitReturnsFetchCmd(t *testing.T) {
	m := New(nil)
	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("Init should return a fetch cmd")
	}
}

func TestModel_TickWhileInFlightSkipsFetch(t *testing.T) {
	m := New(nil)
	m.inFlight = true
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd != nil {
		t.Fatalf("tick during in-flight should not start a new fetch")
	}
}

func TestModel_DataMsgClearsInFlightAndStoresSnapshot(t *testing.T) {
	m := New(nil)
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
	m := New(nil)
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
	m := New(nil)
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
	m := New(nil)
	if got := m.Status(); got != theme.PillNoDaemon {
		t.Fatalf("status: got %v want PillNoDaemon", got)
	}
}

func TestModel_StatusStaleOnError(t *testing.T) {
	m := New(nil)
	m.stale = true
	m.lastOK = time.Now()
	if got := m.Status(); got != theme.PillStale {
		t.Fatalf("status: got %v want PillStale", got)
	}
}

func TestModel_StatusLiveWithRecentEvent(t *testing.T) {
	m := New(nil)
	fakeNow := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return fakeNow }
	m.lastOK = fakeNow
	m.snap.LatestEventTS = fakeNow.Add(-5 * time.Second).UnixNano()
	if got := m.Status(); got != theme.PillLive {
		t.Fatalf("status: got %v want PillLive", got)
	}
}
