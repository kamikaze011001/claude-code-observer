package rollup

import (
	"log/slog"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

// Op is one SQL statement plus its positional arguments. The repository
// executes Ops on the active transaction in the order they appear.
type Op struct {
	Query string
	Args  []any
}

// Updater turns one event into zero or more SQL ops. Updaters are pure
// functions over the event; they never touch the DB directly.
type Updater func(ev domain.Event) []Op

// updaters maps fully-qualified Claude Code event names to their updater.
// Unknown event names get a nil entry — Apply silently ignores them so the
// ingest path never breaks just because a new event type was added upstream.
var updaters = map[string]Updater{}

// Apply looks up the updater for ev.EventName and returns its ops. Returns
// nil for unknown or empty event names, after emitting a debug log so future
// Claude Code releases that introduce new event types are visible.
//
// For non-session_start events that carry project.name in attrs, Apply
// prepends a sessionMetadataUpsert so the session row gets labelled even when
// Claude Code never emits session_start (observed in practice — some clients
// emit only user_prompt + api_request streams). COALESCE makes the op
// idempotent when metadata is already set.
func Apply(ev domain.Event) []Op {
	if ev.EventName == "" {
		return nil
	}
	u, ok := updaters[ev.EventName]
	if !ok || u == nil {
		slog.Debug("rollup: no handler for event", "name", ev.EventName)
		return nil
	}
	ops := u(ev)
	if ev.EventName != domain.EventSessionStart && attrString(ev.Attrs, "project.name") != "" {
		meta := Op{
			Query: sessionMetadataUpsert,
			Args: []any{
				ev.SessionID,
				ev.TS, ev.TS,
				attrString(ev.Attrs, "project.name"),
				attrString(ev.Attrs, "project.cwd"),
				attrString(ev.Attrs, "app.version"),
				attrString(ev.Attrs, "os.type"),
				attrString(ev.Attrs, "user.id"),
			},
		}
		ops = append([]Op{meta}, ops...)
	}
	return ops
}
