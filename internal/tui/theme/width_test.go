package theme

import "testing"

func TestVisualWidth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"hello", 5},
		{"あ", 2},     // CJK wide
		{"🙂", 2},     // emoji wide
		{"a あ b", 6}, // mixed: a(1)+space(1)+あ(2)+space(1)+b(1)=6
	}
	for _, c := range cases {
		if got := VisualWidth(c.in); got != c.want {
			t.Errorf("VisualWidth(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
