package component

import (
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestModelBadge_FamilyMapping(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	cases := []struct {
		in, family string
	}{
		{"claude-opus-4-7", "opus"},
		{"opus-4-7", "opus"},
		{"claude-sonnet-4-6", "sonnet"},
		{"haiku-4-5-20251001", "haiku"},
		{"unknown-model", "model"}, // generic fallback
	}
	for _, c := range cases {
		got := ModelBadge(&th, c.in)
		if !containsCI(got, c.family) {
			t.Errorf("ModelBadge(%q) = %q; want family %q", c.in, got, c.family)
		}
	}
}

func containsCI(haystack, needle string) bool {
	h := []rune(haystack)
	n := []rune(needle)
	for i := 0; i+len(n) <= len(h); i++ {
		match := true
		for j := range n {
			if lower(h[i+j]) != lower(n[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
func lower(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + 32
	}
	return r
}
