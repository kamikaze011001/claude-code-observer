package waterfall

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

var updateWaterfall = flag.Bool("update-waterfall", false, "update waterfall goldens")

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	got = stripANSI(got)
	if *updateWaterfall {
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

func newModelWith(reqs []readstore.WaterfallRequest) *Model {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	m := New(nil, "7b2e4d10-0000-0000-0000-000000000000", &th).(*Model)
	m.bars, m.totalSpanMS = buildBars(reqs)
	m.reqs = reqs
	m.lastOK = time.Now()
	return m
}

func TestWaterfallView_Golden_Empty(t *testing.T) {
	m := newModelWith(nil)
	golden(t, "empty", m.View(90, 32))
}

func TestWaterfallView_Golden_MainOnly(t *testing.T) {
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(1 * time.Second), DurationMS: 900, QuerySource: "repl_main_thread", Model: "claude-opus-4-7", CostUSD: 0.21, InputTokens: 8000, OutputTokens: 2000},
		{TS: base.Add(4 * time.Second), DurationMS: 1500, QuerySource: "main", Model: "claude-opus-4-7", CostUSD: 0.18, InputTokens: 6000, OutputTokens: 1800},
	}
	golden(t, "main_only", newModelWith(reqs).View(90, 32))
}

func TestWaterfallView_Golden_AllLanesOverlap(t *testing.T) {
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(1 * time.Second), DurationMS: 1000, QuerySource: "repl_main_thread", Model: "claude-opus-4-7", CostUSD: 0.21, InputTokens: 8000, OutputTokens: 2000},
		{TS: base.Add(5 * time.Second), DurationMS: 3000, QuerySource: "general-purpose", Model: "claude-sonnet-4-6", CostUSD: 0.04, InputTokens: 1200, OutputTokens: 800},
		{TS: base.Add(6 * time.Second), DurationMS: 3500, QuerySource: "Explore", Model: "claude-sonnet-4-6", CostUSD: 0.05, InputTokens: 1400, OutputTokens: 900},
		{TS: base.Add(7 * time.Second), DurationMS: 400, QuerySource: "compact", Model: "claude-haiku-4-5", CostUSD: 0.002, InputTokens: 200, OutputTokens: 50},
		{TS: base.Add(12 * time.Second), DurationMS: 2000, QuerySource: "main", Model: "claude-opus-4-7", CostUSD: 0.19, InputTokens: 7000, OutputTokens: 2100},
	}
	golden(t, "all_lanes_overlap", newModelWith(reqs).View(90, 32))
}

func TestWaterfallView_Golden_Narrow(t *testing.T) {
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(1 * time.Second), DurationMS: 1000, QuerySource: "main", Model: "claude-opus-4-7", CostUSD: 0.21, InputTokens: 8000, OutputTokens: 2000},
		{TS: base.Add(3 * time.Second), DurationMS: 2000, QuerySource: "subagent", Model: "claude-sonnet-4-6", CostUSD: 0.04, InputTokens: 1200, OutputTokens: 800},
	}
	golden(t, "narrow", newModelWith(reqs).View(50, 24))
}

func TestWaterfallView_NotFound(t *testing.T) {
	m := newModelWith(nil)
	m.notFound = true
	golden(t, "not_found", m.View(90, 32))
}
