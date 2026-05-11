package sessions

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/prompt"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestDetail_KeyJK(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.events = []readstore.EventRow{
		{TS: time.Now(), EventName: "tool_result"},
		{TS: time.Now(), EventName: "user_prompt", PromptID: "p1"},
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
		{TS: time.Now(), EventName: "tool_result", PromptID: "p1"},
		{TS: time.Now(), EventName: "user_prompt", PromptID: "p1"},
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
		{TS: time.Now(), EventName: "tool_result"},
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

func TestDetail_View_Scrolled(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "abcdef123456").(*Detail)
	base := mustTime("2026-05-10T12:00:00Z")
	for i := 0; i < 30; i++ {
		m.events = append(m.events, readstore.EventRow{
			TS:        base.Add(time.Duration(i) * time.Second),
			EventName: "tool_result",
			Summary:   "Read 1ms",
		})
	}
	m.cursor = 20
	m.hasMore = true
	m.lastOK = base.Add(31 * time.Second)
	out := m.View(100, 12)
	goldenDetail(t, "detail_scrolled", out)
}

func TestDetail_View_Mixed(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "abcdef123456").(*Detail)
	m.events = []readstore.EventRow{
		{TS: mustTime("2026-05-10T12:43:01Z"), EventName: "user_prompt", PromptID: "p1", Summary: "prompt: 142ch /commit"},
		{TS: mustTime("2026-05-10T12:43:02Z"), EventName: "tool_result", Summary: "Read 12ms"},
		{TS: mustTime("2026-05-10T12:43:03Z"), EventName: "api_request", Summary: "claude-opus-4-7 $0.0021"},
		{TS: mustTime("2026-05-10T12:43:05Z"), EventName: "user_prompt", PromptID: "p2", Summary: "prompt: 88ch"},
	}
	m.cursor = 0
	m.lastOK = mustTime("2026-05-10T12:43:06Z")
	out := m.View(100, 20)
	goldenDetail(t, "detail_mixed", out)
}

func TestDetail_EnterPromptPushesPromptDetail(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.events = []readstore.EventRow{
		{TS: time.Now(), EventName: "user_prompt", PromptID: "pX"},
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("no cmd")
	}
	msg := cmd()
	push, ok := msg.(app.PushViewMsg)
	if !ok {
		t.Fatalf("msg type=%T", msg)
	}
	if _, isPrompt := push.V.(*prompt.Detail); !isPrompt {
		t.Fatalf("pushed=%T want *prompt.Detail", push.V)
	}
}

func TestDetail_ShortHelpAndInit(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	if len(m.ShortHelp()) == 0 {
		t.Fatal("ShortHelp empty")
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init nil")
	}
}

func TestDetail_FetchCmdNilPool(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	cmd := m.fetchCmd()
	if cmd == nil {
		t.Fatal("nil cmd")
	}
	if _, ok := cmd().(app.ErrMsg); !ok {
		t.Fatal("want ErrMsg")
	}
}

func TestDetail_TickInFlightNoOp(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.inFlight = true
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd != nil {
		t.Fatal("expected no cmd while in-flight")
	}
}

func TestDetail_ErrMsgSetsStale(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.Update(app.ErrMsg{Err: errSentinel("boom")})
	if !m.stale {
		t.Fatal("expected stale")
	}
}

func TestDetail_PgDn_StepsCursorByViewport(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	for i := 0; i < 30; i++ {
		m.events = append(m.events, readstore.EventRow{TS: time.Now(), EventName: "tool_result"})
	}
	m.viewport = 10
	m.cursor = 0
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if got := upd.(*Detail).cursor; got != 10 {
		t.Fatalf("cursor=%d want 10", got)
	}
}

func TestDetail_PgUp_StepsCursorByViewport(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	for i := 0; i < 30; i++ {
		m.events = append(m.events, readstore.EventRow{TS: time.Now(), EventName: "tool_result"})
	}
	m.viewport = 10
	m.cursor = 25
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if got := upd.(*Detail).cursor; got != 15 {
		t.Fatalf("cursor=%d want 15", got)
	}
}

func TestDetail_PgDn_ClampsAtLastEventWhenNoMore(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	for i := 0; i < 5; i++ {
		m.events = append(m.events, readstore.EventRow{TS: time.Now(), EventName: "tool_result"})
	}
	m.viewport = 10
	m.cursor = 0
	m.hasMore = false
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if got := upd.(*Detail).cursor; got != 4 {
		t.Fatalf("cursor=%d want 4 (last event)", got)
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd when hasMore=false; got one")
	}
}

func TestDetail_PgDn_AtBottomTriggersFetchOlder(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.events = []readstore.EventRow{
		{TS: mustTime("2026-05-10T12:00:01Z"), EventName: "tool_result"},
		{TS: mustTime("2026-05-10T12:00:00Z"), EventName: "tool_result"},
	}
	m.viewport = 10
	m.cursor = 1 // last loaded row
	m.hasMore = true
	upd, cmd := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if !upd.(*Detail).loadingOlder {
		t.Fatal("expected loadingOlder=true after pgdn at bottom with hasMore")
	}
	if cmd == nil {
		t.Fatal("expected fetch cmd")
	}
}

func TestDetail_PgDn_DoesNotDoubleFetchWhileLoading(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.events = []readstore.EventRow{
		{TS: mustTime("2026-05-10T12:00:00Z"), EventName: "tool_result"},
	}
	m.viewport = 10
	m.cursor = 0
	m.hasMore = true
	m.loadingOlder = true
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if cmd != nil {
		t.Fatal("expected nil cmd while loadingOlder=true")
	}
}

func TestDetail_DetailOlderMsg_AppendsAndClearsLoading(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.events = []readstore.EventRow{
		{TS: mustTime("2026-05-10T12:00:01Z"), EventName: "a"},
		{TS: mustTime("2026-05-10T12:00:00Z"), EventName: "b"},
	}
	m.cursor = 1
	m.offset = 0
	m.loadingOlder = true
	m.hasMore = true
	msg := detailOlderMsg{
		events: []readstore.EventRow{
			{TS: mustTime("2026-05-10T11:59:59Z"), EventName: "c"},
			{TS: mustTime("2026-05-10T11:59:58Z"), EventName: "d"},
		},
		hasMore: false,
		at:      mustTime("2026-05-10T12:00:02Z"),
	}
	upd, _ := m.Update(msg)
	d := upd.(*Detail)
	if len(d.events) != 4 {
		t.Fatalf("events len=%d want 4", len(d.events))
	}
	if d.events[3].EventName != "d" {
		t.Fatalf("tail event = %q want d", d.events[3].EventName)
	}
	if d.loadingOlder {
		t.Fatal("loadingOlder should be cleared")
	}
	if d.hasMore {
		t.Fatal("hasMore should reflect msg")
	}
	if d.cursor != 1 || d.offset != 0 {
		t.Fatalf("cursor/offset moved unexpectedly: cursor=%d offset=%d", d.cursor, d.offset)
	}
}

func TestDetail_FetchOlderCmd_NilPoolReturnsErrMsg(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1").(*Detail)
	m.events = []readstore.EventRow{
		{TS: mustTime("2026-05-10T12:00:00Z"), EventName: "tool_result"},
	}
	cmd := m.fetchOlderCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	if _, ok := cmd().(app.ErrMsg); !ok {
		t.Fatal("expected app.ErrMsg from nil-pool path")
	}
}
