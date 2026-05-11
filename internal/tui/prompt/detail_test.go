package prompt

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

var updatePrompt = flag.Bool("update-prompt", false, "update prompt goldens")

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	got = stripANSI(got)
	if *updatePrompt {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
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
		t.Fatalf("golden %s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t.UTC()
}

func TestDetail_Title(t *testing.T) {
	t.Parallel()
	d := New(nil, "abcdef123456", nil).(*Detail)
	if d.Title() != "PROMPT abcdef12…" {
		t.Fatalf("title=%q", d.Title())
	}
}

func TestDetail_NotFoundSetsFlag(t *testing.T) {
	t.Parallel()
	d := New(nil, "p", nil).(*Detail)
	upd, _ := d.Update(app.ErrMsg{Err: readstore.ErrNotFound})
	if !upd.(*Detail).notFound {
		t.Fatal("expected notFound=true on ErrNotFound")
	}
}

func TestDetail_OtherErrorSetsStale(t *testing.T) {
	t.Parallel()
	d := New(nil, "p", nil).(*Detail)
	upd, _ := d.Update(app.ErrMsg{Err: errors.New("boom")})
	if !upd.(*Detail).stale {
		t.Fatal("expected stale=true on generic error")
	}
	if upd.(*Detail).notFound {
		t.Fatal("notFound should not be set for non-ErrNotFound")
	}
}

func TestDetail_DataPopulates(t *testing.T) {
	t.Parallel()
	d := New(nil, "p", nil).(*Detail)
	res := readstore.PromptDetailResult{
		Prompt: readstore.Prompt{PromptID: "p", CostUSD: 0.0042, InputTokens: 1240, OutputTokens: 312},
	}
	upd, _ := d.Update(detailDataMsg{result: res})
	if upd.(*Detail).result.Prompt.CostUSD != 0.0042 {
		t.Fatal("data not stored")
	}
}

func TestDetail_TickAfterDataRefetches(t *testing.T) {
	t.Parallel()
	d := New(nil, "p", nil).(*Detail)
	d.inFlight = false
	_, cmd := d.Update(app.TickMsg{})
	if cmd == nil {
		t.Fatal("expected fetchCmd from tick")
	}
}

func TestDetail_StatusPill(t *testing.T) {
	t.Parallel()
	d := New(nil, "p", nil).(*Detail)
	if d.Status() != theme.PillNoDaemon {
		t.Fatal("empty status")
	}
	d.notFound = true
	if d.Status() != theme.PillNoDaemon {
		t.Fatal("not-found should still be no-daemon-equivalent")
	}
}

func TestPromptDetailView_Golden(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	base := time.Date(2026, 5, 11, 9, 15, 42, 0, time.UTC)
	res := readstore.PromptDetailResult{
		Prompt: readstore.Prompt{
			PromptID:            "7b2e4d10-0000-0000-0000-000000000000",
			SessionID:           "a3f9c1b1-0000-0000-0000-000000000000",
			StartedAt:           base,
			EndedAt:             base.Add(32 * time.Second),
			PromptLength:        2341,
			CostUSD:             0.38,
			InputTokens:         12481,
			OutputTokens:        4012,
			CacheReadTokens:     88000,
			CacheCreationTokens: 2000,
			APIRequests:         3,
			ToolCalls:           5,
			HadError:            true,
		},
		APIRequests: []readstore.APIRequest{
			{TS: base.Add(1 * time.Second), Model: "claude-opus-4-7", CostUSD: 0.21, InputTokens: 8481, OutputTokens: 2140},
			{TS: base.Add(16 * time.Second), Model: "claude-opus-4-7", CostUSD: 0.09, InputTokens: 2000, OutputTokens: 872},
			{TS: base.Add(32 * time.Second), Model: "claude-opus-4-7", CostUSD: 0.08, InputTokens: 2000, OutputTokens: 1000},
		},
		ToolCalls: []readstore.ToolCall{
			{TS: base.Add(4 * time.Second), ToolName: "Write", Success: true, DurationMS: 112},
			{TS: base.Add(6 * time.Second), ToolName: "Bash", Success: false, DurationMS: 2104},
			{TS: base.Add(13 * time.Second), ToolName: "Bash", Success: true, DurationMS: 189},
			{TS: base.Add(20 * time.Second), ToolName: "Edit", Success: true, DurationMS: 76},
			{TS: base.Add(26 * time.Second), ToolName: "Read", Success: true, DurationMS: 31},
		},
	}
	d := &Detail{theme: &th, promptID: res.Prompt.PromptID, result: res, lastOK: time.Now()}
	got := d.View(90, 32)
	golden(t, "detail_populated", got)
}

func TestPromptDetailView_Golden_Empty(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	d := &Detail{theme: &th, promptID: "7b2e4d10-0000-0000-0000-000000000000"}
	got := d.View(90, 32)
	golden(t, "detail_empty", got)
}

func TestDetail_View_Found(t *testing.T) {
	t.Parallel()
	d := New(nil, "abcdef123456", nil).(*Detail)
	d.result = readstore.PromptDetailResult{
		Prompt: readstore.Prompt{
			PromptID: "abcdef123456", SessionID: "sess-x",
			StartedAt:   mustTime("2026-05-10T12:43:01Z"),
			EndedAt:     mustTime("2026-05-10T12:43:05Z"),
			CostUSD:     0.0042,
			InputTokens: 1240, OutputTokens: 312,
			APIRequests: 2, ToolCalls: 2,
		},
		APIRequests: []readstore.APIRequest{
			{TS: mustTime("2026-05-10T12:43:02Z"), Model: "claude-opus-4-7", CostUSD: 0.0021, InputTokens: 800, OutputTokens: 120},
			{TS: mustTime("2026-05-10T12:43:04Z"), Model: "claude-opus-4-7", CostUSD: 0.0021, InputTokens: 440, OutputTokens: 192},
		},
		ToolCalls: []readstore.ToolCall{
			{TS: mustTime("2026-05-10T12:43:02Z"), ToolName: "Read", DurationMS: 12, Success: true},
			{TS: mustTime("2026-05-10T12:43:03Z"), ToolName: "Bash", DurationMS: 1245, Success: false},
		},
	}
	d.lastOK = mustTime("2026-05-10T12:43:06Z")
	golden(t, "found", d.View(100, 30))
}

func TestDetail_View_NotFound(t *testing.T) {
	t.Parallel()
	d := New(nil, "abcdef123456", nil).(*Detail)
	d.notFound = true
	golden(t, "not_found", d.View(100, 30))
}

var _ app.View = (*Detail)(nil)
