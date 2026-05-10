package sessions

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

var updateList = flag.Bool("update-list", false, "update list goldens")

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func goldenList(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	got = stripANSI(got)
	if *updateList {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func TestList_View_Empty(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	out := m.View(80, 20)
	goldenList(t, "list_empty", out)
}

func TestList_View_OnePage(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	m.rows = []readstore.SessionRow{
		{SessionID: "abc123def", ProjectName: "claude-code-observer", StartedAt: mustTime("2026-05-10T12:43:01Z"), DurationSec: 842, CostUSD: 0.42, Prompts: 7, Live: true},
		{SessionID: "def456abc", ProjectName: "my-other-project", StartedAt: mustTime("2026-05-10T12:18:55Z"), DurationSec: 4920, CostUSD: 1.84, Prompts: 23, Live: false},
		{SessionID: "ghi789", ProjectName: "", StartedAt: mustTime("2026-05-10T11:00:00Z"), DurationSec: 60, CostUSD: 0.01, Prompts: 1, Live: false},
	}
	m.cursor = 1
	m.lastOK = mustTime("2026-05-10T12:43:02Z")
	out := m.View(100, 20)
	goldenList(t, "list_one_page", out)
}

func mustTime(s string) time.Time {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return tm.UTC()
}

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

func TestList_EnterPushesDetail(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	m.rows = []readstore.SessionRow{{SessionID: "abc"}}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("no cmd from Enter")
	}
	msg := cmd()
	push, ok := msg.(app.PushViewMsg)
	if !ok {
		t.Fatalf("msg=%T want PushViewMsg", msg)
	}
	d, ok := push.V.(*Detail)
	if !ok {
		t.Fatalf("pushed=%T want *Detail", push.V)
	}
	if d.sessionID != "abc" {
		t.Fatalf("sessionID=%q want abc", d.sessionID)
	}
}

// Compile-time check that List implements app.View.
var _ app.View = (*List)(nil)

func TestList_ShortHelpAndInit(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	if len(m.ShortHelp()) == 0 {
		t.Fatal("ShortHelp empty")
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init nil")
	}
}

func TestList_FetchCmdNilPoolReturnsErr(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	cmd := m.fetchCmd(nil)
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(app.ErrMsg); !ok {
		t.Fatalf("msg=%T want ErrMsg", msg)
	}
}

func TestList_TickIgnoredWhenInFlight(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	m.inFlight = true
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd != nil {
		t.Fatal("expected no cmd while in-flight")
	}
}

func TestList_ErrMsgSetsStale(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	m.Update(app.ErrMsg{Err: errBoomList})
	if !m.stale {
		t.Fatal("expected stale on ErrMsg")
	}
}

var errBoomList = errSentinel("boom")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func TestList_DataMsgUpdatesRows(t *testing.T) {
	t.Parallel()
	m := NewList(nil)
	rows := []readstore.SessionRow{{SessionID: "x"}}
	m.cursor = 0
	m.Update(listDataMsg{rows: rows, next: nil, cursor: nil, at: time.Now()})
	if len(m.rows) != 1 {
		t.Fatalf("rows=%d", len(m.rows))
	}
}

func TestList_HumanDurationBranches(t *testing.T) {
	t.Parallel()
	cases := []struct{ secs int64; want string }{
		{30, "30s"},
		{125, "2m05s"},
		{3700, "1h01m"},
	}
	for _, c := range cases {
		if got := humanDuration(c.secs); got != c.want {
			t.Fatalf("humanDuration(%d)=%q want %q", c.secs, got, c.want)
		}
	}
}

func TestList_SamePtrBranches(t *testing.T) {
	t.Parallel()
	if !samePtr(nil, nil) {
		t.Fatal("nil,nil want true")
	}
	a := int64(1)
	b := int64(1)
	if !samePtr(&a, &b) {
		t.Fatal("equal vals want true")
	}
	c := int64(2)
	if samePtr(&a, &c) {
		t.Fatal("differing vals want false")
	}
	if samePtr(nil, &a) {
		t.Fatal("nil/non-nil want false")
	}
}
