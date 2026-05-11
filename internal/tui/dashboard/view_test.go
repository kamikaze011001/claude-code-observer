package dashboard

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

func buildTheme() *theme.Theme {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	return &th
}

func TestDashboardView_Golden(t *testing.T) {
	fakeNow := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, 5, 10, 9, 14, 0, 0, time.UTC).UnixNano()
	latestEventTS := time.Date(2026, 5, 10, 11, 59, 0, 0, time.UTC).UnixNano()

	cases := []struct {
		name   string
		model  func() *Model
	}{
		{
			name: "populated",
			model: func() *Model {
				m := &Model{
					theme: buildTheme(),
					now:   func() time.Time { return fakeNow },
					snap: readstore.Snapshot{
						Today:     readstore.WindowStats{Sessions: 3, CostUSD: 4.21, Prompts: 37, Tokens: 12500, Tools: 152, Errors: 2},
						Yesterday: readstore.WindowStats{Sessions: 2, CostUSD: 3.10, Prompts: 28, Tokens: 9800, Tools: 110, Errors: 0},
						D7:        readstore.WindowStats{Sessions: 18, CostUSD: 28.40, Prompts: 214, Tokens: 87000, Tools: 1100, Errors: 9},
						D30:       readstore.WindowStats{Sessions: 71, CostUSD: 112.05, Prompts: 892, Tokens: 340000, Tools: 4400, Errors: 41},
						LatestEventTS: latestEventTS,
					},
					top: []readstore.TopSession{
						{SessionID: "s1", ProjectName: "observer", StartedAt: startedAt, CostUSD: 1.92, Prompts: 14, Live: true},
					},
					recent: []readstore.TopSession{
						{SessionID: "s2", ProjectName: "observer", StartedAt: startedAt, CostUSD: 0.80, Prompts: 8, Live: false},
						{SessionID: "s3", ProjectName: "api-test", StartedAt: startedAt, CostUSD: 0.50, Prompts: 5, Live: false},
					},
					recentCursor: 0,
				}
				return m
			},
		},
		{
			name: "empty",
			model: func() *Model {
				return &Model{
					theme: buildTheme(),
					now:   func() time.Time { return fakeNow },
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.model()
			out := m.View(90, 32)
			assertGolden(t, tc.name, out)
		})
	}
}
