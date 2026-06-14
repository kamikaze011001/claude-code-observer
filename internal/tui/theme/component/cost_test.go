package component

import (
	"strings"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestCostTier(t *testing.T) {
	cases := []struct {
		usd  float64
		want costTier
	}{
		{0, tierCheap},
		{0.0099, tierCheap},
		{0.01, tierNotable},
		{0.05, tierNotable},
		{0.0501, tierHeavy},
		{2.84, tierHeavy},
		{-1, tierCheap}, // guard: negative never panics, treated as cheap
	}
	for _, c := range cases {
		if got := tierOf(c.usd); got != c.want {
			t.Errorf("tierOf(%v)=%v want %v", c.usd, got, c.want)
		}
	}
}

func TestCostText_ColorsByTier(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	cheap := CostText(&th, 0.001)
	heavy := CostText(&th, 0.99)
	if cheap == heavy {
		t.Fatal("cheap and heavy cost text should differ in styling")
	}
	if want := "$0.00"; !strings.Contains(cheap, want) {
		t.Errorf("cheap=%q missing %q", cheap, want)
	}
	if want := "$0.99"; !strings.Contains(heavy, want) {
		t.Errorf("heavy=%q missing %q", heavy, want)
	}
}

func TestCostText4_FourDecimalPlaces(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	got := CostText4(&th, 0.001)
	if want := "$0.0010"; !strings.Contains(got, want) {
		t.Errorf("CostText4(0.001)=%q missing %q", got, want)
	}
}
