package waterfall

import (
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

// LaneKind is one of the three fixed waterfall lanes.
type LaneKind int

const (
	LaneMain LaneKind = iota
	LaneSubagent
	LaneAuxiliary
)

func (l LaneKind) String() string {
	switch l {
	case LaneSubagent:
		return "subagent"
	case LaneAuxiliary:
		return "auxiliary"
	default:
		return "main"
	}
}

// Bar is a single request positioned on the timeline.
type Bar struct {
	Req      readstore.WaterfallRequest
	Lane     LaneKind
	OffsetMS int64 // start offset from the earliest bar start
}

// startOf returns the request start time: TS is stream-end, so start = TS - duration.
func startOf(r readstore.WaterfallRequest) time.Time {
	return r.TS.Add(-time.Duration(r.DurationMS) * time.Millisecond)
}

// buildBars converts raw requests (assumed ts-ascending) into Bars with a
// computed lane and a start offset relative to the earliest start. It also
// returns the total timeline span in milliseconds (latest end - earliest start).
func buildBars(reqs []readstore.WaterfallRequest) (bars []Bar, totalSpanMS int64) {
	if len(reqs) == 0 {
		return nil, 0
	}
	earliest := startOf(reqs[0])
	for _, r := range reqs[1:] {
		if s := startOf(r); s.Before(earliest) {
			earliest = s
		}
	}
	var latestEnd time.Time
	for _, r := range reqs {
		offset := startOf(r).Sub(earliest).Milliseconds()
		bars = append(bars, Bar{
			Req:      r,
			Lane:     bucketLane(r.QuerySource),
			OffsetMS: offset,
		})
		if r.TS.After(latestEnd) {
			latestEnd = r.TS
		}
	}
	totalSpanMS = latestEnd.Sub(earliest).Milliseconds()
	if totalSpanMS < 0 {
		totalSpanMS = 0
	}
	return bars, totalSpanMS
}

// bucketLane maps a free-form query_source string (as seen on log events)
// to one of the three fixed lanes. The empty string and the explicit main
// aliases map to the main lane; "auxiliary"/"compact" to the auxiliary lane;
// every other value (including any subagent name) to the subagent lane.
func bucketLane(querySource string) LaneKind {
	switch querySource {
	case "", "main", "repl_main_thread":
		return LaneMain
	case "auxiliary", "compact":
		return LaneAuxiliary
	default:
		return LaneSubagent
	}
}
