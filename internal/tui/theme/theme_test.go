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
	if !strings.ContainsAny(got, "┏┓┗┛━┃") {
		t.Fatalf("Block should render with thick border, got %q", got)
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
