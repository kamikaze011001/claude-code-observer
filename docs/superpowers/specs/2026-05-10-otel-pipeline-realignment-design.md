# OTel pipeline realignment — fix wire-format drift in receiver/eventparser/rollup

**Date:** 2026-05-10
**Status:** Draft → ready for plan
**Driver:** Audit of `internal/eventparser/`, `internal/rollup/`, `internal/repository/`, and `internal/tui/readstore/` against the corrected `docs/CLAUDE-CODE-OTEL.md`. Code was built to a draft of that reference that did not match what Claude Code actually emits. The receiver runs without errors, but rollups are silently empty or wrong.

## Goal

Make the ingest pipeline match what Claude Code actually emits over OTLP, so the TUI shows correct cost/token/decision data and so future Claude Code releases that emit new event types are persisted (not silently dropped).

## Non-goals

- Designing real rollup logic for the 11 newly-documented events (compaction stats, hook timings, MCP server state, etc.). That is a follow-up driven by data captured after this fix lands.
- Schema changes. None are needed; existing `events` and rollup tables are sufficient.
- Switching the live data source from logs to metrics. Logs (5 s flush) remain the source of truth per OTel reference §12.5.

## Audit findings

| # | Severity | Location | Issue |
|---|----------|----------|-------|
| 1 | Critical | `internal/rollup/tool_decision.go:7` | Reads `decision == "deny"`. Spec values are `accept`/`reject`. Counter never increments. |
| 2 | Critical | `internal/rollup/tool_decision.go` registry | (Resolved by design choice) The log event `claude_code.tool_decision` carries all decisions; no separate metric handler is needed. Original audit suggested a missing metric handler; the log path is correct once values are fixed. |
| 3 | Critical | `internal/rollup/api_request.go:38–39` | (Resolved on inspection) Log event uses snake_case (`cache_read_tokens`, `cache_creation_tokens`). Only the metric `claude_code.token.usage` uses camelCase `cacheRead`/`cacheCreation`. The log read is correct; needs only a code comment to prevent future confusion. |
| 4 | High | `internal/eventparser/parser_test.go:148`, any consumer of `Event.Attrs` | Tests/code reference `error_message` and `http_status_code`; spec names are `error` and `status_code`. |
| 5 | High | `internal/tui/readstore/summarize_test.go:18` | Test fixture uses `"deny"` decision value. |
| 6 | High | `internal/eventparser/parser.go:21` | `resourceAttrAllowlist` missing `user.account_id` and `terminal.type`. |
| 7 | Medium | `internal/rollup/` updaters map | No handlers for 11 new events. They flow into the events table (raw store) but `rollup.Apply` returns nil with no log line, so unexpected new events are invisible. |
| 8 | Medium | `internal/tui/readstore/summarize.go:46` | No case for `claude_code.code_edit_tool.decision` and the 11 new events. |

## Architecture

No layer boundary changes. Pipeline stays:

```
OTLP gRPC (receiver)
  → eventparser.Parse → domain.Event
  → repository.events.Insert (raw row, always)
  → rollup.Apply (pure function over Event) → repository.rollups.Update
  → TUI readstore queries
```

The fix is surgical: correct attribute names where the code already touches them, extend the resource-attribute allowlist, replace string literals with typed constants, register no-op handlers for new events, and update tests.

## Components

### 1. `internal/domain/wire.go` (new file)

Typed string constants for every event name and metric name documented in the OTel reference. Two `const` blocks:

```go
// Event names (claude_code.*) — emitted as OTLP log records.
const (
    EventUserPrompt              = "claude_code.user_prompt"
    EventAPIRequest              = "claude_code.api_request"
    EventAPIError                = "claude_code.api_error"
    EventToolResult              = "claude_code.tool_result"
    EventToolDecision            = "claude_code.tool_decision"
    EventCompaction              = "claude_code.compaction"
    EventPermissionModeChanged   = "claude_code.permission_mode_changed"
    EventAuth                    = "claude_code.auth"
    EventMCPServerConnection     = "claude_code.mcp_server_connection"
    EventInternalError           = "claude_code.internal_error"
    EventPluginInstalled         = "claude_code.plugin_installed"
    EventSkillActivated          = "claude_code.skill_activated"
    EventAtMention               = "claude_code.at_mention"
    EventAPIRetriesExhausted     = "claude_code.api_retries_exhausted"
    EventHookExecutionStart      = "claude_code.hook_execution_start"
    EventHookExecutionComplete   = "claude_code.hook_execution_complete"
    EventAPIRequestBody          = "claude_code.api_request_body"
    EventAPIResponseBody         = "claude_code.api_response_body"
)

// Metric names (claude_code.*) — emitted as OTLP metric datapoints.
const (
    MetricSessionCount         = "claude_code.session.count"
    MetricLinesOfCode          = "claude_code.lines_of_code.count"
    MetricPullRequest          = "claude_code.pull_request.count"
    MetricCommit               = "claude_code.commit.count"
    MetricCostUsage            = "claude_code.cost.usage"
    MetricTokenUsage           = "claude_code.token.usage"
    MetricActiveTime           = "claude_code.active_time.total"
    MetricCodeEditToolDecision = "claude_code.code_edit_tool.decision"
)
```

Attribute-level constants are deliberately **not** introduced. The drift bit at the name layer; attribute strings are local to each handler and refactor cost outweighs the benefit.

### 2. `internal/eventparser/parser.go`

Extend `resourceAttrAllowlist` with `user.account_id` and `terminal.type`. No other change.

### 3. `internal/rollup/tool_decision.go`

Single substitution: `"deny"` → `"reject"` (line 7). The map key `claude_code.tool_decision` stays — but switch the literal to `domain.EventToolDecision`. Update the value comparison to use a const if a clear naming convention exists; otherwise leave the literal `"reject"` with a comment.

### 4. `internal/rollup/api_request.go`

Add a one-line comment above the cache-token reads:

```go
// Log-event attrs use snake_case (cache_read_tokens). The claude_code.token.usage
// metric uses attribute `type` with camelCase values (cacheRead/cacheCreation).
// Do not unify these without checking docs/CLAUDE-CODE-OTEL.md §7.1.
```

No code change.

### 5. `internal/rollup/` — 11 new no-op handlers

For each new event, register a handler in the updaters map that returns `nil, nil`. Goal: **make event-name registration explicit and complete**, so when Claude Code adds a 12th event, the missing-handler debug log fires and we notice.

```go
// internal/rollup/noop_events.go (new)
func init() {
    register(domain.EventCompaction, noop)
    register(domain.EventPermissionModeChanged, noop)
    // ... 9 more
}
func noop(_ *Event) ([]Op, error) { return nil, nil }
```

Each line gets a `// TODO: real rollup — see docs/CLAUDE-CODE-OTEL.md §8.6` comment pointing at the spec section.

### 6. `internal/rollup/rollup.go` (Apply function)

Add a debug-level log line when an event name is not in the registry:

```go
slog.Debug("rollup: no handler for event", "name", ev.Name)
```

Currently it returns nil silently — that's fine for performance, but combined with the explicit registry above, this turns "Claude Code added a new event" from invisible into a line in the daemon log.

### 7. Test fixtures

Update these files to use corrected spec values:

- `internal/eventparser/parser_test.go:148` — `error_message` → `error`, `http_status_code` → `status_code`.
- `internal/tui/readstore/summarize_test.go:18` — `"deny"` → `"reject"`.
- `internal/repository/rollups_test.go:27` — `"deny"` → `"reject"`.
- `internal/rollup/tool_decision_test.go:13` — `"deny"` → `"reject"`.

Add two new tests:

- `internal/rollup/registry_test.go` — table-driven test asserting every constant in `domain.wire.go` either has a real handler or a registered no-op (compile-time-ish: missing entries fail the test).
- `internal/eventparser/parser_test.go` — round-trip test that an event with `user.account_id` and `terminal.type` resource attributes preserves them in `Event.Attrs`.

### 8. `internal/tui/readstore/summarize.go`

Add `switch` cases for `claude_code.code_edit_tool.decision` (already-spec'd metric) and the 11 new events. One-line human-readable formatting, e.g. `"compaction: 12.3k → 4.1k tokens"`. Use the `domain.Event*` constants in the case labels.

## Data flow

Unchanged. The parser still produces `domain.Event`; rollup still consumes it; TUI still queries the rollups view. The fix is in *what* string values pass through, not the topology.

## Error handling

- Parser: extending the resource allowlist is additive; nothing fails harder.
- Rollup: no-op handlers cannot error. Real handlers (tool_decision, etc.) keep their existing error paths.
- The new debug log for unhandled events does not block ingestion.

## Testing

Run order after changes:

1. `make vet`
2. `make test` — all updated fixtures and the two new tests must pass.
3. `make build` — verify the const refactor compiles cleanly.

Manual smoke test:

1. Start daemon: `./bin/claude-code-observer serve --log-level debug`.
2. Run a `claude` command in a project wired with `cco init`.
3. Confirm in `cco.log` that no `"rollup: no handler for event"` debug lines appear for events listed in `domain.wire.go`.
4. Open `cco`, perform an Edit that you reject. Confirm the `code_edit_tool.decision` reject count increments in the TUI.

## Out of scope

- Designing rollups for the 11 new events. Defer to a follow-up spec once we have captured real data.
- Schema migration. Current `events` table already stores attrs as JSON; new event types fit without schema change.
- Metric-side ingestion of token/cost. Log path is sufficient for the live TUI.
- Refactoring attribute-level strings into constants. YAGNI.

## Estimated diff

~15 files, ~300 additions, ~50 modifications. Single PR.
