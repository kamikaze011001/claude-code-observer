package waterfall

import "testing"

func TestBucketLane(t *testing.T) {
	t.Parallel()
	cases := []struct {
		querySource string
		want        LaneKind
	}{
		{"main", LaneMain},
		{"repl_main_thread", LaneMain},
		{"", LaneMain},
		{"auxiliary", LaneAuxiliary},
		{"compact", LaneAuxiliary},
		{"subagent", LaneSubagent},
		{"general-purpose", LaneSubagent},
		{"Explore", LaneSubagent},
	}
	for _, c := range cases {
		if got := bucketLane(c.querySource); got != c.want {
			t.Errorf("bucketLane(%q) = %v, want %v", c.querySource, got, c.want)
		}
	}
}

func TestLaneKindString(t *testing.T) {
	t.Parallel()
	cases := map[LaneKind]string{
		LaneMain:      "main",
		LaneSubagent:  "subagent",
		LaneAuxiliary: "auxiliary",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
}
