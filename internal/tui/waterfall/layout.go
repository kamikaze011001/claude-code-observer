package waterfall

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

// bucketLane maps a free-form query_source string (as seen on log events)
// to one of the three fixed lanes. Empty / unknown values fall through to
// the subagent lane, except the explicit main/auxiliary aliases.
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
