package about

import (
	"regexp"
	"strings"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func visibleWidth(s string) int {
	// All logo glyphs are width-1 cells in a monospace terminal; runes count.
	return len([]rune(stripANSI(s)))
}

func TestLogo_HasFixedShape(t *testing.T) {
	if c := strings.Count(Logo, "\n"); c != 5 {
		t.Fatalf("Logo should have 5 newlines (6 visual lines), got %d", c)
	}
	for i, line := range strings.Split(Logo, "\n") {
		if w := visibleWidth(line); w != 25 {
			t.Fatalf("Logo line %d width = %d, want 25", i, w)
		}
	}
}

func TestRender_NonEmpty(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	out := Render(&th)
	if out == "" {
		t.Fatalf("Render returned empty string")
	}
	if !strings.Contains(stripANSI(out), "██") {
		t.Fatalf("Render output should contain block glyphs; got:\n%s", out)
	}
}
