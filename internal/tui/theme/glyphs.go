package theme

import "github.com/charmbracelet/lipgloss"

// Glyphs is the set of unicode/nerd-font characters used in the UI. Two
// preset constructors are provided; consumers pick one at startup.
type Glyphs struct {
	Brand       string
	StatusOK    string
	StatusWarn  string
	StatusErr   string
	Cursor      string
	DeltaUp     string
	DeltaDown   string
	DeltaFlat   string
	Check       string
	Cross       string
	Spark       []rune
	Enter       string
	BorderRound lipgloss.Border
}

// UnicodeGlyphs is the default — renders everywhere.
func UnicodeGlyphs() Glyphs {
	return Glyphs{
		Brand:       "✦",
		StatusOK:    "●",
		StatusWarn:  "●",
		StatusErr:   "●",
		Cursor:      "▸",
		DeltaUp:     "▲",
		DeltaDown:   "▼",
		DeltaFlat:   "─",
		Check:       "✓",
		Cross:       "✗",
		Spark:       []rune("▁▂▃▄▅▆▇█"),
		Enter:       "⏎",
		BorderRound: lipgloss.RoundedBorder(),
	}
}

// NerdGlyphs uses Nerd Font private-use codepoints. Requires a patched font.
func NerdGlyphs() Glyphs {
	g := UnicodeGlyphs()
	g.Brand = ""     //   star
	g.StatusOK = ""  //   filled dot
	g.StatusWarn = ""
	g.StatusErr = ""
	g.Cursor = ""    //
	g.DeltaUp = ""   //   up arrow
	g.DeltaDown = "" //   down arrow
	g.Check = ""     //   check
	g.Cross = ""     //   cross
	return g
}
