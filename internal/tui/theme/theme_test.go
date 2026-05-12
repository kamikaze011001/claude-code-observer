package theme

import (
	"os"
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
	if th.Muted.GetForeground() == nil {
		t.Errorf("Muted style empty")
	}
	if th.PillLive.GetBackground() == nil {
		t.Errorf("PillLive style empty")
	}
	if th.PillStale.GetBackground() == nil {
		t.Errorf("PillStale style empty")
	}
	if th.PillNoDaemon.GetBackground() == nil {
		t.Errorf("PillNoDaemon style empty")
	}
}
