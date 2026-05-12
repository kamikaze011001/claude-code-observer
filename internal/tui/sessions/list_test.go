package sessions

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

// update controls both the legacy -update-list goldens and the new golden test.
var updateList = flag.Bool("update", false, "update golden files")

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
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch %s\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

// TestSessionsListView_Golden covers the new component-based View.
func TestSessionsListView_Golden(t *testing.T) {
	type testCase struct {
		name    string
		rows    []readstore.SessionRow
		cursor  int
		nextCur *int64
	}
	cases := []testCase{
		{
			name: "populated",
			rows: []readstore.SessionRow{
				{
					SessionID:   "s1",
					ProjectName: "claude-code-observer",
					StartedAt:   time.Date(2026, 5, 11, 9, 14, 0, 0, time.UTC),
					DurationSec: 4320,
					CostUSD:     1.12,
					Prompts:     12,
					Tokens:      38000,
					Live:        true,
				},
				{
					SessionID:   "s2",
					ProjectName: "cco-frontend",
					StartedAt:   time.Date(2026, 5, 11, 8, 2, 0, 0, time.UTC),
					DurationSec: 2732,
					CostUSD:     0.81,
					Prompts:     7,
					Tokens:      24000,
				},
				{
					SessionID:   "s3",
					ProjectName: "日本語プロジェクト",
					StartedAt:   time.Date(2026, 5, 11, 7, 30, 0, 0, time.UTC),
					DurationSec: 1338,
					CostUSD:     0.42,
					Prompts:     5,
					Tokens:      15000,
				},
			},
			cursor: 1,
		},
		{name: "empty"},
	}
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var lastOK time.Time
			if len(c.rows) > 0 {
				// Populated case: daemon was seen recently.
				lastOK = time.Date(2026, 5, 11, 9, 14, 0, 0, time.UTC)
			}
			m := &List{
				theme:   &th,
				rows:    c.rows,
				cursor:  c.cursor,
				nextCur: c.nextCur,
				lastOK:  lastOK,
				keys:    defaultListKeys(),
			}
			got := m.View(90, 32)
			goldenList(t, "list_"+c.name, got)
		})
	}
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
	m := NewList(nil, nil)
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
	m := NewList(nil, nil)
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
	m := NewList(nil, nil)
	if m.Status() != component.StatusNoDaemon {
		t.Fatalf("empty model status=%v want StatusNoDaemon", m.Status())
	}
	m.rows = []readstore.SessionRow{{SessionID: "a"}}
	m.lastOK = time.Now()
	if m.Status() != component.StatusLive {
		t.Fatalf("with rows status=%v want StatusLive", m.Status())
	}
	m.stale = true
	if m.Status() != component.StatusStale {
		t.Fatalf("stale status=%v want StatusStale", m.Status())
	}
}

func TestList_Title(t *testing.T) {
	t.Parallel()
	if NewList(nil, nil).Title() != "SESSIONS" {
		t.Fatal("title")
	}
}

func TestList_TickFetchesAndUpdatesRows(t *testing.T) {
	t.Parallel()
	m := NewList(nil, nil)
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init returned nil cmd")
	}
}

func TestList_EnterPushesDetail(t *testing.T) {
	t.Parallel()
	m := NewList(nil, nil)
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
	m := NewList(nil, nil)
	if len(m.ShortHelp()) == 0 {
		t.Fatal("ShortHelp empty")
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init nil")
	}
}

func TestList_FetchCmdNilPoolReturnsErr(t *testing.T) {
	t.Parallel()
	m := NewList(nil, nil)
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
	m := NewList(nil, nil)
	m.inFlight = true
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd != nil {
		t.Fatal("expected no cmd while in-flight")
	}
}

func TestList_ErrMsgSetsStale(t *testing.T) {
	t.Parallel()
	m := NewList(nil, nil)
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
	m := NewList(nil, nil)
	rows := []readstore.SessionRow{{SessionID: "x"}}
	m.cursor = 0
	m.Update(listDataMsg{rows: rows, next: nil, cursor: nil, at: time.Now()})
	if len(m.rows) != 1 {
		t.Fatalf("rows=%d", len(m.rows))
	}
}

func TestList_HumanDurationBranches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		secs int64
		want string
	}{
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
