package component

import "testing"

func TestHumanDuration(t *testing.T) {
	cases := []struct {
		sec  int64
		want string
	}{
		{0, ""}, {-1, ""},
		{1, "1s"}, {59, "59s"},
		{60, "1m00s"}, {125, "2m05s"},
		{3600, "1h00m"}, {3725, "1h02m"},
	}
	for _, c := range cases {
		if got := humanDuration(c.sec); got != c.want {
			t.Errorf("humanDuration(%d) = %q want %q", c.sec, got, c.want)
		}
	}
}

func TestHumanInt(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"}, {1, "1"}, {999, "999"},
		{1000, "1k"}, {38000, "38k"},
		{1_000_000, "1.0M"}, {1_500_000, "1.5M"},
	}
	for _, c := range cases {
		if got := humanInt(c.n); got != c.want {
			t.Errorf("humanInt(%d) = %q want %q", c.n, got, c.want)
		}
	}
}
