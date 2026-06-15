package productivity

import (
	"errors"
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

// Compile-time assertion: Model satisfies app.View.
var _ app.View = (*Model)(nil)

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

func TestModel_DataMsgClearsInFlight(t *testing.T) {
	m := New(nil, nil)
	m.inFlight = true
	days := []readstore.ProductivityDay{{Day: "2026-06-14", LinesAdded: 30}}
	updated, _ := m.Update(dataMsg{days: days, at: time.Now()})
	got := updated.(*Model)
	if got.inFlight {
		t.Fatalf("dataMsg should clear inFlight")
	}
	if len(got.days) != 1 || got.days[0].Day != "2026-06-14" {
		t.Fatalf("days not stored: %+v", got.days)
	}
}

func TestModel_ErrMsgSetsStale(t *testing.T) {
	m := New(nil, nil)
	m.inFlight = true
	updated, _ := m.Update(app.ErrMsg{Err: errors.New("boom")})
	got := updated.(*Model)
	if got.inFlight {
		t.Fatalf("ErrMsg should clear inFlight")
	}
	if !got.stale {
		t.Fatalf("ErrMsg should set stale=true")
	}
}

func TestModel_StatusLiveByDefault(t *testing.T) {
	m := New(nil, nil)
	if got := m.Status(); got != component.StatusLive {
		t.Fatalf("status: got %v want StatusLive", got)
	}
}

func TestModel_StatusStaleOnError(t *testing.T) {
	m := New(nil, nil)
	m.stale = true
	if got := m.Status(); got != component.StatusStale {
		t.Fatalf("status: got %v want StatusStale", got)
	}
}

func TestModel_FetchCmdProducesMsg(t *testing.T) {
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
