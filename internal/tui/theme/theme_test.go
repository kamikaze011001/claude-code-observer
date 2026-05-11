package theme

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	// Force TrueColor profile so ANSI escape codes are emitted in tests
	// (by default lipgloss detects no TTY and strips colors).
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func TestDefault_HasAccentColor(t *testing.T) {
	th := Default()
	got := th.AccentText.Render("$4.21")
	if got == "$4.21" {
		t.Fatalf("AccentText should add ANSI styling, got plain %q", got)
	}
	if !strings.Contains(got, "$4.21") {
		t.Fatalf("AccentText should contain the original text, got %q", got)
	}
}

func TestDefault_BlockHasBorder(t *testing.T) {
	th := Default()
	got := th.Block(20).Render("hello")
	if !strings.ContainsAny(got, "╭╮╰╯─│┏┓┗┛━┃") {
		t.Fatalf("Block should render with a visible border, got %q", got)
	}
}

func TestDefault_PillStates(t *testing.T) {
	th := Default()
	cases := []PillState{PillLive, PillStale, PillNoDaemon}
	for _, s := range cases {
		got := th.Pill(s)
		if got == "" {
			t.Fatalf("Pill(%v) should not be empty", s)
		}
	}
}

func TestTheme_Build_PopulatesNewFields(t *testing.T) {
	th := Build(MochaPalette(), UnicodeGlyphs())
	if string(th.Palette.Bg) != "#1e1e2e" {
		t.Errorf("Palette not copied: %+v", th.Palette)
	}
	if th.Glyphs.Brand != "✦" {
		t.Errorf("Glyphs not copied: %+v", th.Glyphs)
	}
	// Derived styles are non-zero
	if th.Title.GetForeground() == nil {
		t.Errorf("Title style empty")
	}
	if th.Card.GetBorderStyle() == (lipgloss.Border{}) {
		t.Errorf("Card border not set")
	}
}

func TestTheme_LegacyAPI_StillWorks(t *testing.T) {
	th := Default()
	// The chrome in internal/tui/app/app.go uses these — they must keep working.
	if th.Heading.Render("X") == "" {
		t.Errorf("legacy Heading broken")
	}
	if th.Pill(PillLive) == "" {
		t.Errorf("legacy Pill broken")
	}
}
