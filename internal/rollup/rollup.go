// Package rollup contains pure per-event-type updaters that produce SQL ops
// to maintain the sessions and prompts rollup tables. The repository layer
// executes the ops inside the same transaction as the corresponding event
// insert. See docs/superpowers/specs/2026-05-10-m2.1-rollup-engine-design.md.
package rollup

import "github.com/kamikaze011001/claude-code-observer/internal/domain"

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
// nil for unknown or empty event names.
func Apply(ev domain.Event) []Op {
	if ev.EventName == "" {
		return nil
	}
	u, ok := updaters[ev.EventName]
	if !ok {
		return nil
	}
	return u(ev)
}
