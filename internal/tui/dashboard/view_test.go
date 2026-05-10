package dashboard

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

var update = flag.Bool("update", false, "update golden files")

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func clean(s string) string { return ansi.ReplaceAllString(s, "") }

func goldenPath(name string) string {
	return filepath.Join("testdata", name+".golden")
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	got = clean(got)
	path := goldenPath(name)
	if *update {
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
	if string(want) != got {
		t.Fatalf("%s mismatch:\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}

func TestView_Empty(t *testing.T) {
	m := New(nil)
	m.now = func() time.Time { return time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC) }
	out := m.View(80, 24)
	assertGolden(t, "empty", out)
}

func TestView_Happy(t *testing.T) {
	m := New(nil)
	m.now = func() time.Time { return time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC) }
	m.snap = readstore.Snapshot{
		Today:         readstore.WindowStats{CostUSD: 4.21, Prompts: 37, Tools: 152, Errors: 2},
		D7:            readstore.WindowStats{CostUSD: 28.40, Prompts: 214, Tools: 1100, Errors: 9},
		D30:           readstore.WindowStats{CostUSD: 112.05, Prompts: 892, Tools: 4400, Errors: 41},
		LatestEventTS: time.Date(2026, 5, 10, 11, 59, 0, 0, time.UTC).UnixNano(),
	}
	startedAt := time.Date(2026, 5, 10, 9, 14, 0, 0, time.UTC).UnixNano()
	m.top = []readstore.TopSession{
		{SessionID: "s1", ProjectName: "observer", StartedAt: startedAt, CostUSD: 1.92, Prompts: 14, Live: true},
	}
	out := m.View(80, 24)
	assertGolden(t, "happy", out)
}
