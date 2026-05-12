package component

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestStatusPill_RendersForEachState(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	cases := []struct {
		s    Status
		want string // substring expected inside the output
	}{
		{StatusLive, "LIVE"},
		{StatusStale, "STALE"},
		{StatusNoDaemon, "NO DAEMON"},
	}
	for _, c := range cases {
		got := StatusPill(&th, c.s)
		if !strings.Contains(lipgloss.NewStyle().Render(stripAnsi(got)), c.want) {
			t.Errorf("StatusPill(%v) = %q; want substring %q", c.s, got, c.want)
		}
	}
}

// stripAnsi for assertion — relies on lipgloss.Width which strips escapes.
func stripAnsi(s string) string {
	// simplest possible: filter to printable runes; full ANSI strip would use a regex
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			in = true
		case in && (r == 'm' || r == 'K'):
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
