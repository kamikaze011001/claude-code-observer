package prompt

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

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
	d := New(nil, "abcdef123456").(*Detail)
	if d.Title() != "PROMPT abcdef12…" {
		t.Fatalf("title=%q", d.Title())
	}
}

func TestDetail_NotFoundSetsFlag(t *testing.T) {
	t.Parallel()
	d := New(nil, "p").(*Detail)
	upd, _ := d.Update(app.ErrMsg{Err: readstore.ErrNotFound})
	if !upd.(*Detail).notFound {
		t.Fatal("expected notFound=true on ErrNotFound")
	}
}

func TestDetail_OtherErrorSetsStale(t *testing.T) {
	t.Parallel()
	d := New(nil, "p").(*Detail)
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
	d := New(nil, "p").(*Detail)
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
	d := New(nil, "p").(*Detail)
	d.inFlight = false
	_, cmd := d.Update(app.TickMsg{})
	if cmd == nil {
		t.Fatal("expected fetchCmd from tick")
	}
}

func TestDetail_StatusPill(t *testing.T) {
	t.Parallel()
	d := New(nil, "p").(*Detail)
	if d.Status() != theme.PillNoDaemon {
		t.Fatal("empty status")
	}
	d.notFound = true
	if d.Status() != theme.PillNoDaemon {
		t.Fatal("not-found should still be no-daemon-equivalent")
	}
}

func TestDetail_View_Found(t *testing.T) {
	t.Parallel()
	d := New(nil, "abcdef123456").(*Detail)
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
	d := New(nil, "abcdef123456").(*Detail)
	d.notFound = true
	golden(t, "not_found", d.View(100, 30))
}

var _ app.View = (*Detail)(nil)
