package about

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

var update = flag.Bool("update", false, "rewrite testdata/*.golden files")

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func goldenPath(name string) string {
	return filepath.Join("testdata", name+".golden")
}

func renderForGolden(t *testing.T, width, height int) string {
	t.Helper()
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	m := New(&th, "v1.2.3", "abc1234")
	return stripANSI(m.View(width, height))
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	p := goldenPath(name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", p, err)
	}
	if got != string(want) {
		t.Fatalf("%s mismatch:\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}

func TestView_Wide_Golden(t *testing.T) {
	got := renderForGolden(t, 80, 24)
	if !strings.Contains(got, "claude code observer") {
		t.Fatalf("wide render missing tagline:\n%s", got)
	}
	if !strings.Contains(got, "v1.2.3") {
		t.Fatalf("wide render missing version:\n%s", got)
	}
	if !strings.Contains(got, "██") {
		t.Fatalf("wide render should include block logo glyphs:\n%s", got)
	}
	assertGolden(t, "wide", got)
}

func TestView_Narrow_Fallback(t *testing.T) {
	got := renderForGolden(t, 30, 10)
	if strings.Contains(got, "██") {
		t.Fatalf("narrow render should NOT contain block glyphs (fallback to compact wordmark):\n%s", got)
	}
	if !strings.Contains(got, "CCO") {
		t.Fatalf("narrow render should contain compact wordmark 'CCO':\n%s", got)
	}
	assertGolden(t, "narrow", got)
}
