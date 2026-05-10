// Package rollup contains pure per-event-type updaters that produce SQL ops
// to maintain the sessions and prompts rollup tables. The repository layer
// executes the ops inside the same transaction as the corresponding event
// insert. See docs/superpowers/specs/2026-05-10-m2.1-rollup-engine-design.md.
package rollup
