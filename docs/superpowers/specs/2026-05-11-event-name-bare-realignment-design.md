# Event-Name Bare Realignment

**Date:** 2026-05-11
**Status:** Approved — ready for implementation plan
**Branch:** `cco-otel-audit-fix`

## Problem

The TUI dashboard renders no sessions or prompts even though OTLP events are flowing into the receiver. SQLite shows 28+ rows in `events` and 29+ rows in `metric_snapshots`, but `sessions` and `prompts` are empty.

Root cause: a name-format mismatch between what Claude Code emits and what the rollup engine looks up.

- Claude Code emits log records whose `event_name` field is **bare** (`user_prompt`, `api_request`, `tool_result`, ...).
- The eventparser stores the name verbatim, so `events.event_name` rows are bare.
- `internal/domain/wire.go` registers updaters under **prefixed** names (`claude_code.user_prompt`, ...).
- `rollup.Apply` looks up `updaters[ev.EventName]`, misses on every event, returns no ops, and `sessions`/`prompts` stay empty.

The same mismatch breaks two TUI code paths that filter or switch on prefixed event names (`internal/tui/readstore/queries.go`, `internal/tui/sessions/detail.go`).

The current spec doc (`docs/CLAUDE-CODE-OTEL.md` §8) documents prefixed event names, which contradicts the wire reality.

## Goal

Restore the rollup pipeline so events flow into `sessions` / `prompts` and the TUI shows data. Align constants, code, and docs with the bare event names Claude Code actually emits, while keeping the parser defensive against future drift.

## Non-Goals

- Renaming any **metric** constants. Metric names (`claude_code.session.count`, `claude_code.token.usage`, `claude_code.code_edit_tool.decision`, ...) are correctly prefixed on the wire and stay as-is.
- Schema changes. Existing `events.event_name` rows are already bare — no migration needed.
- Refactoring the rollup engine, parser structure, or TUI architecture.

## Design

### 1. Constants — `internal/domain/wire.go`

Drop the `claude_code.` prefix from every `Event*` constant. After the change:

```go
EventUserPrompt            = "user_prompt"
EventAPIRequest            = "api_request"
EventAPIError              = "api_error"
EventToolResult            = "tool_result"
EventToolDecision          = "tool_decision"
EventCompaction            = "compaction"
EventPermissionModeChanged = "permission_mode_changed"
EventAuth                  = "auth"
EventMCPServerConnection   = "mcp_server_connection"
EventInternalError         = "internal_error"
EventPluginInstalled       = "plugin_installed"
EventSkillActivated        = "skill_activated"
EventAtMention             = "at_mention"
EventAPIRetriesExhausted   = "api_retries_exhausted"
EventHookExecutionStart    = "hook_execution_start"
EventHookExecutionComplete = "hook_execution_complete"
EventAPIRequestBody        = "api_request_body"
EventAPIResponseBody       = "api_response_body"
EventSessionStart          = "session_start"
EventSessionEnd            = "session_end"
```

The `KnownEvents` slice keeps the same shape; only the values change.

### 2. Defensive normalization — `internal/eventparser/parser.go`

In `eventNameOf` (currently lines 79–87), strip a leading `claude_code.` if present. This makes the parser tolerant of a future Claude Code release that re-introduces the prefix without requiring a code patch.

```go
func eventNameOf(rec *logspb.LogRecord, flat map[string]any) string {
    var name string
    if n := rec.GetEventName(); n != "" {
        name = n
    } else if s, ok := flat["event.name"].(string); ok {
        name = s
    }
    // Claude Code currently emits bare event names (e.g. "user_prompt").
    // Strip a "claude_code." prefix defensively so we keep working if a
    // future release re-introduces it (per docs/CLAUDE-CODE-OTEL.md §8).
    return strings.TrimPrefix(name, "claude_code.")
}
```

### 3. TUI read paths — `internal/tui/readstore/queries.go`

Update the `event_name IN (...)` SQL list at line 325 and the two `switch` cases at lines 346 and 356 to use bare names. Prefer importing `domain.EventToolResult` / `domain.EventAPIRequest` so the lookup stays in sync with the source of truth.

### 4. TUI session detail — `internal/tui/sessions/detail.go`

Replace the two `"claude_code.user_prompt"` string literals at lines 140 and 174 with `domain.EventUserPrompt` (bare value).

### 5. Tests and goldens

- Update parser tests in `internal/eventparser/parser_test.go` to assert that input `claude_code.user_prompt` (and friends) round-trips to bare `user_prompt`. Keep at least one test that supplies a bare name to lock in the no-op path.
- Update other test files that hard-code prefixed event names (`internal/repository/*_test.go`, `internal/rollup/*_test.go`, `internal/tui/**/_test.go`, `internal/retention/pruner_test.go`).
- Refresh `internal/tui/sessions/testdata/detail_mixed.golden` and any other goldens that print event names.

### 6. Documentation — `docs/CLAUDE-CODE-OTEL.md`

Rewrite §8 (Events) so each subsection header and inline reference uses the bare event name (`user_prompt`, `api_request`, ...).

Add a short callout near the top of §8:

> Claude Code emits events with **bare** names on `LogRecord.event_name` (e.g. `user_prompt`, not `claude_code.user_prompt`). The `claude_code.` namespace is reserved for **metric** names (§7). Receivers should match on bare names; the eventparser strips a leading `claude_code.` defensively if a future release re-adds it.

Metric tables in §7 stay unchanged.

Audit other docs for stale references (informational only — fix in the same change if trivial):

- `docs/ARCHITECTURE.md`
- `docs/DATA-MODELS.md`
- `docs/CONTEXT.md`
- `docs/decisions/ADR-003-logs-as-primary-signal.md`
- `docs/ROADMAP.md`, `docs/FUTURE.md`
- Historical specs/plans under `docs/superpowers/` and `docs/specs/`, `docs/plans/` are frozen artifacts — leave them alone.

### 7. Operational fix-up

After the code change ships:

1. Stop the running daemon.
2. Run `./bin/claude-code-observer rebuild-rollups` to backfill `sessions` and `prompts` from the existing `events` rows.
3. Start the new binary with `serve`.
4. Open the TUI and confirm sessions and prompts render.

This is a one-time operator step, not part of the code change.

## Verification

Run after each implementation step, in order:

1. `make vet`
2. `make test`
3. `make build`

End-to-end check:

4. `./bin/claude-code-observer rebuild-rollups` against the existing `~/.claude-code-observer/db.sqlite` — expect non-zero `sessions` and `prompts` counts in the output.
5. Restart `serve`, open the TUI, confirm sessions list is populated.

## Risks

- **Drift from spec doc reality.** Mitigated by updating §8 in the same change.
- **Missed prefixed string literal somewhere in code or tests** → silent rollup miss for that event class. Mitigated by `git grep "claude_code\."` after the rename and verifying every remaining hit is intentional (metrics, docs, frozen artifacts).
- **Future re-introduction of prefix by Claude Code.** Mitigated by the defensive `TrimPrefix` in the parser plus a test that exercises both inputs.
