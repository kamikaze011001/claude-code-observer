package theme

import "testing"

func TestResolve_OrderOfPrecedence(t *testing.T) {
	cases := []struct {
		name                                                    string
		flagTheme, flagIcons, envTheme, envIcons, colorFGBG string
		wantPalette, wantIcons                                  string
	}{
		{"flag wins over env", "latte", "nerd", "mocha", "unicode", "", "latte", "nerd"},
		{"env wins over colorfgbg", "", "", "frappe", "unicode", "15;0", "frappe", "unicode"},
		{"colorfgbg picks latte on light bg", "", "", "", "", "0;15", "latte", "unicode"},
		{"colorfgbg picks mocha on dark bg", "", "", "", "", "15;0", "mocha", "unicode"},
		{"all empty → defaults", "", "", "", "", "", "mocha", "unicode"},
		{"icons env", "", "", "", "nerd", "", "mocha", "nerd"},
		{"icons flag overrides icons env", "", "unicode", "", "nerd", "", "mocha", "unicode"},
		{"auto theme name + dark fgbg", "auto", "", "", "", "15;0", "mocha", "unicode"},
		{"auto theme name + light fgbg", "auto", "", "", "", "0;15", "latte", "unicode"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			th, name, icons := Resolve(c.flagTheme, c.flagIcons, c.envTheme, c.envIcons, c.colorFGBG)
			if name != c.wantPalette {
				t.Errorf("palette: got %q want %q", name, c.wantPalette)
			}
			if icons != c.wantIcons {
				t.Errorf("icons: got %q want %q", icons, c.wantIcons)
			}
			if string(th.Palette.Accent) == "" {
				t.Errorf("theme not built")
			}
		})
	}
}

func TestResolve_RejectsUnknownTheme(t *testing.T) {
	_, name, _ := Resolve("nonsense", "", "", "", "")
	if name != "mocha" {
		t.Errorf("unknown theme should fall back to mocha, got %q", name)
	}
}
