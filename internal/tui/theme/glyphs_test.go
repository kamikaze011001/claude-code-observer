package theme

import "testing"

func TestGlyphs_AllSetsCoverRequiredKeys(t *testing.T) {
	cases := map[string]Glyphs{"unicode": UnicodeGlyphs(), "nerd": NerdGlyphs()}
	for name, g := range cases {
		if g.Brand == "" || g.StatusOK == "" || g.StatusWarn == "" || g.StatusErr == "" ||
			g.Cursor == "" || g.DeltaUp == "" || g.DeltaDown == "" || g.DeltaFlat == "" ||
			g.Check == "" || g.Cross == "" || g.Enter == "" || len(g.Spark) == 0 {
			t.Errorf("%s: missing required glyph", name)
		}
	}
}
