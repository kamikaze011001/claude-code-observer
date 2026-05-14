package waterfall

import (
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

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

func TestBuildBars_OffsetsAndSpan(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	// req A: starts at base+0ms, runs 1000ms  (TS = base+1000ms)
	// req B: starts at base+500ms, runs 2000ms (TS = base+2500ms)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(1000 * time.Millisecond), DurationMS: 1000, QuerySource: "main"},
		{TS: base.Add(2500 * time.Millisecond), DurationMS: 2000, QuerySource: "subagent"},
	}
	bars, totalSpanMS := buildBars(reqs)
	if len(bars) != 2 {
		t.Fatalf("want 2 bars, got %d", len(bars))
	}
	if bars[0].OffsetMS != 0 {
		t.Errorf("bar 0 offset = %d, want 0", bars[0].OffsetMS)
	}
	if bars[1].OffsetMS != 500 {
		t.Errorf("bar 1 offset = %d, want 500", bars[1].OffsetMS)
	}
	if bars[0].Lane != LaneMain || bars[1].Lane != LaneSubagent {
		t.Errorf("lanes wrong: %v %v", bars[0].Lane, bars[1].Lane)
	}
	// span = latest end (base+2500ms) - earliest start (base+0ms) = 2500ms
	if totalSpanMS != 2500 {
		t.Errorf("totalSpanMS = %d, want 2500", totalSpanMS)
	}
}

func TestBuildBars_Empty(t *testing.T) {
	t.Parallel()
	bars, span := buildBars(nil)
	if len(bars) != 0 || span != 0 {
		t.Fatalf("want empty, got %d bars span %d", len(bars), span)
	}
}

func TestBuildBars_ZeroDurationClamped(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	reqs := []readstore.WaterfallRequest{
		{TS: base, DurationMS: 0, QuerySource: "main"},
	}
	bars, span := buildBars(reqs)
	if len(bars) != 1 || bars[0].OffsetMS != 0 {
		t.Fatalf("unexpected bars: %+v", bars)
	}
	if span != 0 {
		t.Errorf("span = %d, want 0", span)
	}
}
