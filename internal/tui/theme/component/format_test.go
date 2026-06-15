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

func TestHumanActiveDuration(t *testing.T) {
	cases := []struct {
		sec  int64
		want string
	}{
		{0, "0m"}, {-1, "0m"},
		{30, "0m"}, {60, "1m"}, {1380, "23m"}, {3599, "59m"},
		{3600, "1h00m"}, {15120, "4h12m"},
	}
	for _, c := range cases {
		if got := HumanActiveDuration(c.sec); got != c.want {
			t.Errorf("HumanActiveDuration(%d) = %q want %q", c.sec, got, c.want)
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
		if got := HumanInt(c.n); got != c.want {
			t.Errorf("HumanInt(%d) = %q want %q", c.n, got, c.want)
		}
	}
}
