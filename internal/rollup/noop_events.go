package rollup

import "github.com/kamikaze011001/claude-code-observer/internal/domain"

// noopUpdater is a placeholder for newly-documented Claude Code events that we
// persist as raw rows but have not yet decided how to roll up. Registering them
// explicitly (rather than relying on the unknown-event fallthrough) means the
// registry test catches future drift, and the unhandled-event debug log in
// rollup.Apply only fires for events Claude Code adds *after* this build.
//
// TODO: design real rollups for these — see docs/CLAUDE-CODE-OTEL.md §8.6–§8.8.
func noopUpdater(_ domain.Event) []Op { return nil }

func init() {
	updaters[domain.EventCompaction] = noopUpdater
	updaters[domain.EventPermissionModeChanged] = noopUpdater
	updaters[domain.EventAuth] = noopUpdater
	updaters[domain.EventMCPServerConnection] = noopUpdater
	updaters[domain.EventInternalError] = noopUpdater
	updaters[domain.EventPluginInstalled] = noopUpdater
	updaters[domain.EventSkillActivated] = noopUpdater
	updaters[domain.EventAtMention] = noopUpdater
	updaters[domain.EventAPIRetriesExhausted] = noopUpdater
	updaters[domain.EventHookExecutionStart] = noopUpdater
	updaters[domain.EventHookExecutionComplete] = noopUpdater
	updaters[domain.EventAPIRequestBody] = noopUpdater
	updaters[domain.EventAPIResponseBody] = noopUpdater
}
