package sessions

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/prompt"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

// eventItems wraps plain EventRows as timelineItems for direct injection into
// m.items in tests that don't need the full applyItems path.
func eventItems(rows ...readstore.EventRow) []timelineItem {
	items := make([]timelineItem, len(rows))
	for i, r := range rows {
		items[i] = timelineItem{SessionItem: readstore.SessionItem{
			Kind:  readstore.ItemEvent,
			Event: r,
			TS:    r.TS,
		}}
	}
	return items
}

func TestDetail_DefaultLatestExpanded(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.applyItems([]readstore.SessionItem{
		{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p2"}, TS: time.Unix(0, 3000)},
		{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p1"}, TS: time.Unix(0, 1000)},
	}, false)
	if !m.items[0].expanded {
		t.Error("latest turn (items[0]) should be expanded by default")
	}
	if m.items[1].expanded {
		t.Error("older turn should be collapsed by default")
	}
}

func TestDetail_KeyJK(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.applyItems([]readstore.SessionItem{
		{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p1"}, TS: time.Unix(0, 2000)},
		{Kind: readstore.ItemEvent, Event: readstore.EventRow{EventName: "auth"}, TS: time.Unix(0, 1000)},
	}, false)
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if upd.(*Detail).cursor != 1 {
		t.Fatalf("cursor=%d want 1", upd.(*Detail).cursor)
	}
}

func TestDetail_EnterOnPromptPushes(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.items = eventItems(
		readstore.EventRow{TS: time.Now(), EventName: "tool_result", PromptID: "p1"},
		readstore.EventRow{TS: time.Now(), EventName: "user_prompt", PromptID: "p1"},
	)
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
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.items = eventItems(readstore.EventRow{TS: time.Now(), EventName: "tool_result"})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no cmd; got one")
	}
}

func TestDetail_StatusPill(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	if m.Status() != component.StatusNoDaemon {
		t.Fatal("empty status")
	}
	m.items = eventItems(readstore.EventRow{})
	m.lastOK = time.Now()
	if m.Status() != component.StatusLive {
		t.Fatal("with events")
	}
	m.stale = true
	if m.Status() != component.StatusStale {
		t.Fatal("stale")
	}
}

func TestDetail_Title(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "abcdef123456", nil).(*Detail)
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
	m := NewDetail(nil, "abcdef123456", nil).(*Detail)
	out := m.View(80, 20)
	goldenDetail(t, "detail_empty", out)
}

func TestDetail_View_Scrolled(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "abcdef123456", nil).(*Detail)
	base := mustTime("2026-05-10T12:00:00Z")
	for i := 0; i < 30; i++ {
		m.items = append(m.items, timelineItem{SessionItem: readstore.SessionItem{
			Kind: readstore.ItemEvent,
			Event: readstore.EventRow{
				TS:        base.Add(time.Duration(i) * time.Second),
				EventName: "tool_result",
				Summary:   "Read 1ms",
			},
			TS: base.Add(time.Duration(i) * time.Second),
		}})
	}
	m.cursor = 20
	m.hasMore = true
	m.lastOK = base.Add(31 * time.Second)
	out := m.View(100, 12)
	goldenDetail(t, "detail_scrolled", out)
}

func TestDetail_View_Mixed(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "abcdef123456", nil).(*Detail)
	m.items = eventItems(
		readstore.EventRow{TS: mustTime("2026-05-10T12:43:01Z"), EventName: "user_prompt", PromptID: "p1", Summary: "prompt: 142ch /commit"},
		readstore.EventRow{TS: mustTime("2026-05-10T12:43:02Z"), EventName: "tool_result", Summary: "Read 12ms"},
		readstore.EventRow{TS: mustTime("2026-05-10T12:43:03Z"), EventName: "api_request", Summary: "claude-opus-4-7 $0.0021"},
		readstore.EventRow{TS: mustTime("2026-05-10T12:43:05Z"), EventName: "user_prompt", PromptID: "p2", Summary: "prompt: 88ch"},
	)
	m.cursor = 0
	m.lastOK = mustTime("2026-05-10T12:43:06Z")
	out := m.View(100, 20)
	goldenDetail(t, "detail_mixed", out)
}

func TestDetail_EnterPromptPushesPromptDetail(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.items = eventItems(readstore.EventRow{TS: time.Now(), EventName: "user_prompt", PromptID: "pX"})
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
	m := NewDetail(nil, "s1", nil).(*Detail)
	if len(m.ShortHelp()) == 0 {
		t.Fatal("ShortHelp empty")
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init nil")
	}
}

func TestDetail_FetchCmdNilPool(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
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
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.inFlight = true
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd != nil {
		t.Fatal("expected no cmd while in-flight")
	}
}

func TestDetail_ErrMsgSetsStale(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.Update(app.ErrMsg{Err: errSentinel("boom")})
	if !m.stale {
		t.Fatal("expected stale")
	}
}

func TestDetail_PgDn_StepsCursorByViewport(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	for i := 0; i < 30; i++ {
		m.items = append(m.items, timelineItem{SessionItem: readstore.SessionItem{
			Kind: readstore.ItemEvent, TS: time.Now(),
		}})
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
	m := NewDetail(nil, "s1", nil).(*Detail)
	for i := 0; i < 30; i++ {
		m.items = append(m.items, timelineItem{SessionItem: readstore.SessionItem{
			Kind: readstore.ItemEvent, TS: time.Now(),
		}})
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
	m := NewDetail(nil, "s1", nil).(*Detail)
	for i := 0; i < 5; i++ {
		m.items = append(m.items, timelineItem{SessionItem: readstore.SessionItem{
			Kind: readstore.ItemEvent, TS: time.Now(),
		}})
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
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.items = eventItems(
		readstore.EventRow{TS: mustTime("2026-05-10T12:00:01Z"), EventName: "tool_result"},
		readstore.EventRow{TS: mustTime("2026-05-10T12:00:00Z"), EventName: "tool_result"},
	)
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
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.items = eventItems(readstore.EventRow{TS: mustTime("2026-05-10T12:00:00Z"), EventName: "tool_result"})
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
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.items = eventItems(
		readstore.EventRow{TS: mustTime("2026-05-10T12:00:01Z"), EventName: "a"},
		readstore.EventRow{TS: mustTime("2026-05-10T12:00:00Z"), EventName: "b"},
	)
	m.cursor = 1
	m.offset = 0
	m.loadingOlder = true
	m.hasMore = true
	msg := detailOlderMsg{
		items: []readstore.SessionItem{
			{Kind: readstore.ItemEvent, Event: readstore.EventRow{TS: mustTime("2026-05-10T11:59:59Z"), EventName: "c"}, TS: mustTime("2026-05-10T11:59:59Z")},
			{Kind: readstore.ItemEvent, Event: readstore.EventRow{TS: mustTime("2026-05-10T11:59:58Z"), EventName: "d"}, TS: mustTime("2026-05-10T11:59:58Z")},
		},
		hasMore: false,
		at:      mustTime("2026-05-10T12:00:02Z"),
	}
	upd, _ := m.Update(msg)
	d := upd.(*Detail)
	if len(d.items) != 4 {
		t.Fatalf("items len=%d want 4", len(d.items))
	}
	if d.items[3].Event.EventName != "d" {
		t.Fatalf("tail item event name = %q want d", d.items[3].Event.EventName)
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
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.items = eventItems(readstore.EventRow{TS: mustTime("2026-05-10T12:00:00Z"), EventName: "tool_result"})
	cmd := m.fetchOlderCmd()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}
	if _, ok := cmd().(app.ErrMsg); !ok {
		t.Fatal("expected app.ErrMsg from nil-pool path")
	}
}

func TestDetail_Tick_SuppressedWhenScrolled(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.items = eventItems(readstore.EventRow{TS: time.Now(), EventName: "tool_result"})
	m.offset = 3
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd != nil {
		t.Fatal("expected nil cmd while offset>0")
	}
}

func TestDetail_Tick_SuppressedWhenPaginated(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	for i := 0; i < detailPageSize+1; i++ {
		m.items = append(m.items, timelineItem{SessionItem: readstore.SessionItem{
			Kind: readstore.ItemEvent, TS: time.Now(),
		}})
	}
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd != nil {
		t.Fatal("expected nil cmd when older pages have been loaded")
	}
}

func TestDetail_Tick_SuppressedWhileLoadingOlder(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.items = eventItems(readstore.EventRow{TS: time.Now(), EventName: "tool_result"})
	m.loadingOlder = true
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd != nil {
		t.Fatal("expected nil cmd while loadingOlder=true")
	}
}

func TestDetail_Tick_RunsAtTopWithOnePage(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.items = eventItems(readstore.EventRow{TS: time.Now(), EventName: "tool_result"})
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd == nil {
		t.Fatal("expected fetch cmd at top with one page loaded")
	}
}

func TestDetail_View_NoBlankRowsBetweenEvents(t *testing.T) {
	t.Parallel()
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	base := mustTime("2026-05-10T12:00:00Z")
	m := &Detail{theme: &th, sessionID: "s1", lastOK: base}
	m.items = eventItems(
		readstore.EventRow{TS: base, EventName: "session_lifecycle", Summary: "started"},
		readstore.EventRow{TS: base.Add(time.Second), EventName: "user_prompt", PromptID: "p1", Summary: "prompt: 88ch"},
		readstore.EventRow{TS: base.Add(2 * time.Second), EventName: "tool_result", Summary: "Read 12ms"},
	)
	m.cursor = 0 // first row selected; selected rows are background-styled
	out := stripANSI(m.View(90, 32))

	// Every card body line ("│ … │") must carry content. A border-only line
	// with blank interior is a lipgloss wrap artifact from rows that overflow
	// the card's content width.
	var bodyRows int
	for _, ln := range strings.Split(out, "\n") {
		s := strings.TrimSpace(ln)
		if !strings.HasPrefix(s, "│") || !strings.HasSuffix(s, "│") {
			continue
		}
		bodyRows++
		if strings.TrimSpace(strings.Trim(s, "│")) == "" {
			t.Fatalf("blank card body line (wrap artifact):\n%s", out)
		}
	}
	if bodyRows != 4 { // 1 column header + 3 events
		t.Fatalf("card body rows = %d; want 4\n%s", bodyRows, out)
	}
}

func TestSessionDetailView_Golden(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	base := time.Date(2026, 5, 11, 9, 14, 0, 0, time.UTC)
	m := &Detail{theme: &th, sessionID: "a3f9c1b1-0000-0000-0000-000000000000"}
	m.items = eventItems(
		readstore.EventRow{TS: base.Add(2 * time.Second), EventName: "session_lifecycle", Summary: "started"},
		readstore.EventRow{TS: base.Add(8 * time.Second), EventName: "user_prompt", PromptID: "p1", Summary: `"refactor receiver pipeline"`},
		readstore.EventRow{TS: base.Add(9 * time.Second), EventName: "api_request", Summary: "opus-4-7  $0.12 · 8k/3k"},
		readstore.EventRow{TS: base.Add(11 * time.Second), EventName: "tool_decision", Summary: "Read · approved"},
		readstore.EventRow{TS: base.Add(11 * time.Second), EventName: "tool_result", Summary: "Read ✓ 42ms"},
	)
	got := m.View(90, 32)
	goldenDetail(t, "detail_populated", got)
}

func TestSessionDetailView_Golden_Empty(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	m := &Detail{theme: &th, sessionID: "a3f9c1b1-0000-0000-0000-000000000000"}
	got := m.View(90, 32)
	goldenDetail(t, "detail_golden_empty", got)
}

func TestDetail_CursorRestoreAcrossRefresh(t *testing.T) {
	t.Parallel()
	// Two deterministic turn items; avoid TS=0 so they are realistic values.
	baseItems := []readstore.SessionItem{
		{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p1"}, TS: time.Unix(0, 1000)},
		{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p2"}, TS: time.Unix(0, 2000)},
	}

	t.Run("case_A_same_items_restores_cursor", func(t *testing.T) {
		t.Parallel()
		m := NewDetail(nil, "s1", nil).(*Detail)
		m.applyItems(baseItems, false)
		m.cursor = 1 // cursor on p2 (TS=2000)

		upd, _ := m.Update(detailDataMsg{items: baseItems, at: time.Now()})
		d := upd.(*Detail)
		if d.cursor != 1 {
			t.Errorf("cursor=%d want 1 (same Kind+TS survived refresh)", d.cursor)
		}
	})

	t.Run("case_B_missing_item_falls_back", func(t *testing.T) {
		t.Parallel()
		m := NewDetail(nil, "s1", nil).(*Detail)
		m.applyItems(baseItems, false)
		m.cursor = 1 // cursor on p2 (TS=2000)

		// New list: index-0 item survives (TS=1000), index-1 item replaced by
		// brand-new p3 (TS=3000). TS=2000 is gone — cursor must not stay stale.
		newItems := []readstore.SessionItem{
			{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p1"}, TS: time.Unix(0, 1000)},
			{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p3"}, TS: time.Unix(0, 3000)},
		}
		upd, _ := m.Update(detailDataMsg{items: newItems, at: time.Now()})
		d := upd.(*Detail)
		if d.cursor < 0 || d.cursor >= len(d.items) {
			t.Errorf("cursor=%d out of valid range [0, %d)", d.cursor, len(d.items))
		}
		// With TS=2000 gone the restore loop finds no match; cursor stays at 0.
		if d.cursor != 0 {
			t.Errorf("cursor=%d want 0 (stale item absent, fell back to reset position)", d.cursor)
		}
	})
}
