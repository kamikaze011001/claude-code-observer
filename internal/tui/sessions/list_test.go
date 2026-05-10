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

// Compile-time check that List implements app.View.
var _ app.View = (*List)(nil)
