package waterfall

import (
	"sort"
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

// endMS returns the bar's end offset (start offset + duration).
func (b Bar) endMS() int64 { return b.OffsetMS + b.Req.DurationMS }

// packLane greedily packs bars into non-overlapping sub-rows. Bars are sorted
// by start offset; each bar is placed in the first sub-row whose last bar ends
// at or before this bar's start, otherwise a new sub-row is opened.
func packLane(bars []Bar) [][]Bar {
	if len(bars) == 0 {
		return nil
	}
	sorted := make([]Bar, len(bars))
	copy(sorted, bars)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].OffsetMS < sorted[j].OffsetMS
	})

	var rows [][]Bar
	rowEnd := []int64{} // last-bar end offset per row
	for _, b := range sorted {
		placed := false
		for i := range rows {
			if rowEnd[i] <= b.OffsetMS {
				rows[i] = append(rows[i], b)
				rowEnd[i] = b.endMS()
				placed = true
				break
			}
		}
		if !placed {
			rows = append(rows, []Bar{b})
			rowEnd = append(rowEnd, b.endMS())
		}
	}
	return rows
}

// scaleBar maps a bar's millisecond offset/duration onto terminal columns.
// Width is clamped to a minimum of 1 column. When totalSpanMS is 0 the bar
// renders as a single column at the start.
func scaleBar(offsetMS, durationMS, totalSpanMS int64, contentWidth int) (startCol, width int) {
	if totalSpanMS <= 0 || contentWidth <= 0 {
		return 0, 1
	}
	scale := float64(contentWidth) / float64(totalSpanMS)
	startCol = int(float64(offsetMS) * scale)
	width = int(float64(durationMS) * scale)
	if width < 1 {
		width = 1
	}
	if startCol < 0 {
		startCol = 0
	}
	if startCol+width > contentWidth {
		width = contentWidth - startCol
		if width < 1 {
			width = 1
		}
	}
	return startCol, width
}
