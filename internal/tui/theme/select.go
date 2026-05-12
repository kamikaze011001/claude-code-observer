package theme

import "strings"

// Resolve picks a Palette and Glyphs based on (in order of precedence):
//  1. flagTheme / flagIcons (CLI flag)
//  2. envTheme / envIcons (CCO_THEME / CCO_ICONS)
//  3. colorFGBG ($COLORFGBG; theme only — never affects icons)
//  4. defaults (mocha / unicode)
//
// The "auto" theme name means "use colorFGBG; fall back to mocha".
// Returns the built Theme plus the resolved palette + icons names (for
// logging or display).
func Resolve(flagTheme, flagIcons, envTheme, envIcons, colorFGBG string) (Theme, string, string) {
	paletteName := pickTheme(flagTheme, envTheme, colorFGBG)
	resolvedIcons := pickIcons(flagIcons, envIcons)

	var p Palette
	switch paletteName {
	case "macchiato":
		p = MacchiatoPalette()
	case "frappe":
		p = FrappePalette()
	case "latte":
		p = LattePalette()
	default:
		paletteName = "mocha"
		p = MochaPalette()
	}

	var g Glyphs
	if resolvedIcons == "nerd" {
		g = NerdGlyphs()
	} else {
		resolvedIcons = "unicode"
		g = UnicodeGlyphs()
	}
	return Build(p, g), paletteName, resolvedIcons
}

func pickTheme(flag, env, colorFGBG string) string {
	if flag != "" && flag != "auto" {
		return flag
	}
	if flag == "auto" {
		if isLightTerminal(colorFGBG) {
			return "latte"
		}
		return "mocha"
	}
	if env != "" {
		return env
	}
	if isLightTerminal(colorFGBG) {
		return "latte"
	}
	return "mocha"
}

func pickIcons(flag, env string) string {
	if flag != "" {
		return flag
	}
	if env != "" {
		return env
	}
	return "unicode"
}

// isLightTerminal parses $COLORFGBG (e.g. "0;15" — fg 0, bg 15). Background
// values >= 7 are considered "light." Empty / malformed values return false.
func isLightTerminal(colorFGBG string) bool {
	parts := strings.Split(colorFGBG, ";")
	if len(parts) < 2 {
		return false
	}
	bg := parts[len(parts)-1]
	switch bg {
	case "7", "8", "9", "10", "11", "12", "13", "14", "15":
		return true
	}
	return false
}
