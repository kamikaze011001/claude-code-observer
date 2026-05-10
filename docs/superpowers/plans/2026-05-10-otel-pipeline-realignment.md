# OTel Pipeline Realignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Realign the receiver/eventparser/rollup/TUI code with the corrected `docs/CLAUDE-CODE-OTEL.md` so rollups reflect what Claude Code actually emits.

**Architecture:** Surgical fixes inside the existing pipeline (receiver → eventparser → repository → rollup → TUI readstore). Introduce typed event/metric name constants in `internal/domain/wire.go`. Fix one wrong literal (`"deny"` → `"reject"`). Extend the resource-attribute allowlist. Register no-op rollup handlers for 11 new event types so future Claude Code releases are visible in logs. No schema change.

**Tech Stack:** Go 1.25, `database/sql` + modernc SQLite, log/slog, table-driven tests.

---

## File Structure

**Create:**
- `internal/domain/wire.go` — typed string constants for event names and metric names.
- `internal/rollup/noop_events.go` — registers no-op handlers for the 11 newly-documented events.

**Modify:**
- `internal/eventparser/parser.go` — extend `resourceAttrAllowlist`.
- `internal/eventparser/parser_test.go` — fix `error_message`/`http_status_code` fixtures; add resource-allowlist test for `user.account_id`/`terminal.type`.
- `internal/rollup/rollup.go` — emit a debug log when an event has no handler.
- `internal/rollup/tool_decision.go` — change `"deny"` → `"reject"`; switch map key to const.
- `internal/rollup/tool_decision_test.go` — fix `"deny"`/`"allow"` fixtures.
- `internal/rollup/api_request.go` — add a clarifying comment about snake_case (log) vs camelCase (metric) cache token attributes; switch map key to const.
- `internal/rollup/api_error.go`, `internal/rollup/session_lifecycle.go`, `internal/rollup/tool_result.go`, `internal/rollup/user_prompt.go` — switch map keys to constants.
- `internal/rollup/registry_test.go` — assert every event constant in `domain/wire.go` is registered in `updaters`.
- `internal/repository/rollups_test.go` — fix `"deny"` fixture.
- `internal/tui/readstore/summarize.go` — add cases for `claude_code.code_edit_tool.decision` and the 11 new events; consume constants.
- `internal/tui/readstore/summarize_test.go` — fix `"deny"` → `"reject"`; cover new event cases.

---

## Task 1: Add wire-format constants

**Files:**
- Create: `internal/domain/wire.go`

- [ ] **Step 1: Create the constants file**

```go
package domain

// Event names — emitted by Claude Code as OTLP log records.
// Source of truth: docs/CLAUDE-CODE-OTEL.md §8.
const (
	EventUserPrompt            = "claude_code.user_prompt"
	EventAPIRequest            = "claude_code.api_request"
	EventAPIError              = "claude_code.api_error"
	EventToolResult            = "claude_code.tool_result"
	EventToolDecision          = "claude_code.tool_decision"
	EventCompaction            = "claude_code.compaction"
	EventPermissionModeChanged = "claude_code.permission_mode_changed"
	EventAuth                  = "claude_code.auth"
	EventMCPServerConnection   = "claude_code.mcp_server_connection"
	EventInternalError         = "claude_code.internal_error"
	EventPluginInstalled       = "claude_code.plugin_installed"
	EventSkillActivated        = "claude_code.skill_activated"
	EventAtMention             = "claude_code.at_mention"
	EventAPIRetriesExhausted   = "claude_code.api_retries_exhausted"
	EventHookExecutionStart    = "claude_code.hook_execution_start"
	EventHookExecutionComplete = "claude_code.hook_execution_complete"
	EventAPIRequestBody        = "claude_code.api_request_body"
	EventAPIResponseBody       = "claude_code.api_response_body"

	// Community-observed event names (not in official docs §8.8).
	// Retained because the existing rollup pipeline uses them.
	EventSessionStart = "claude_code.session_start"
	EventSessionEnd   = "claude_code.session_end"
)

// Metric names — emitted by Claude Code as OTLP metric datapoints.
// Source of truth: docs/CLAUDE-CODE-OTEL.md §7.1.
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

// AllEventNames is the canonical list of Claude Code event names this build
// recognises. The rollup registry test asserts every entry has a handler.
var AllEventNames = []string{
	EventUserPrompt, EventAPIRequest, EventAPIError, EventToolResult,
	EventToolDecision, EventCompaction, EventPermissionModeChanged, EventAuth,
	EventMCPServerConnection, EventInternalError, EventPluginInstalled,
	EventSkillActivated, EventAtMention, EventAPIRetriesExhausted,
	EventHookExecutionStart, EventHookExecutionComplete, EventAPIRequestBody,
	EventAPIResponseBody, EventSessionStart, EventSessionEnd,
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go vet ./internal/domain/...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/wire.go
git commit -m "feat(domain): add typed constants for OTel event and metric names"
```

---

## Task 2: Fix the `"deny"` → `"reject"` bug in tool_decision rollup

**Files:**
- Test: `internal/rollup/tool_decision_test.go`
- Modify: `internal/rollup/tool_decision.go`

- [ ] **Step 1: Update test fixtures to use the corrected spec values**

Replace the file contents at `internal/rollup/tool_decision_test.go`:

```go
package rollup

import (
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApplyToolDecision_RejectBumpsToolDenied(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1",
		EventName: domain.EventToolDecision,
		Attrs:     map[string]any{"decision": "reject"},
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	// sessionCounterArgs index 13 = tool_denied
	if ops[0].Args[13] != int64(1) {
		t.Errorf("tool_denied = %v want 1", ops[0].Args[13])
	}
}

func TestApplyToolDecision_AcceptDoesNotBumpToolDenied(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1",
		EventName: domain.EventToolDecision,
		Attrs:     map[string]any{"decision": "accept"},
	}
	ops := Apply(ev)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Args[13] != int64(0) {
		t.Errorf("tool_denied = %v want 0", ops[0].Args[13])
	}
}

func TestApplyToolDecision_MissingDecisionDoesNotBump(t *testing.T) {
	ev := domain.Event{
		TS: 1000, SessionID: "s1",
		EventName: domain.EventToolDecision,
	}
	ops := Apply(ev)
	if ops[0].Args[13] != int64(0) {
		t.Errorf("tool_denied = %v want 0", ops[0].Args[13])
	}
}
```

- [ ] **Step 2: Run the tests, expect the first one to fail**

Run: `go test ./internal/rollup/ -run TestApplyToolDecision -v`
Expected: `TestApplyToolDecision_RejectBumpsToolDenied` FAILS with `tool_denied = 0 want 1` (because the implementation still compares against `"deny"`).

- [ ] **Step 3: Fix the implementation**

Replace the file contents at `internal/rollup/tool_decision.go`:

```go
package rollup

import "github.com/kamikaze011001/claude-code-observer/internal/domain"

// applyToolDecision bumps the per-session tool_denied counter when the user
// rejects a tool invocation. Spec values are accept|reject (see
// docs/CLAUDE-CODE-OTEL.md §8.5).
func applyToolDecision(ev domain.Event) []Op {
	var denied int64
	if attrString(ev.Attrs, "decision") == "reject" {
		denied = 1
	}
	return []Op{{
		Query: sessionCounterUpsert,
		Args:  sessionCounterArgs(ev.SessionID, ev.TS, sessionCounters{ToolDenied: denied}),
	}}
}

func init() {
	updaters[domain.EventToolDecision] = applyToolDecision
}
```

- [ ] **Step 4: Run the tests, expect them to pass**

Run: `go test ./internal/rollup/ -run TestApplyToolDecision -v`
Expected: all three PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/rollup/tool_decision.go internal/rollup/tool_decision_test.go
git commit -m "fix(rollup): tool_decision uses reject (not deny) per OTel spec"
```

---

## Task 3: Switch remaining rollup registrations to constants

**Files:**
- Modify: `internal/rollup/api_error.go`, `internal/rollup/api_request.go`, `internal/rollup/session_lifecycle.go`, `internal/rollup/tool_result.go`, `internal/rollup/user_prompt.go`

- [ ] **Step 1: Replace each registration string literal with the matching constant**

In `internal/rollup/api_error.go` line 28:

```go
updaters[domain.EventAPIError] = applyAPIError
```

In `internal/rollup/api_request.go` line 75:

```go
updaters[domain.EventAPIRequest] = applyAPIRequest
```

In `internal/rollup/session_lifecycle.go` lines 48–49:

```go
updaters[domain.EventSessionStart] = applySessionStart
updaters[domain.EventSessionEnd] = applySessionEnd
```

In `internal/rollup/tool_result.go` line 21:

```go
updaters[domain.EventToolResult] = applyToolResult
```

In `internal/rollup/user_prompt.go` line 78:

```go
updaters[domain.EventUserPrompt] = applyUserPrompt
```

If any of these files don't already import `github.com/kamikaze011001/claude-code-observer/internal/domain`, add the import.

- [ ] **Step 2: Build and run the existing rollup test suite**

Run: `go vet ./internal/rollup/... && go test ./internal/rollup/...`
Expected: PASS (no behaviour change — these are name swaps).

- [ ] **Step 3: Commit**

```bash
git add internal/rollup/api_error.go internal/rollup/api_request.go internal/rollup/session_lifecycle.go internal/rollup/tool_result.go internal/rollup/user_prompt.go
git commit -m "refactor(rollup): use domain event constants for registry keys"
```

---

## Task 4: Add cache-token attribute clarifying comment

**Files:**
- Modify: `internal/rollup/api_request.go`

- [ ] **Step 1: Locate the cache-token reads (lines around 38–39)**

Run: `grep -n "cache_read_tokens\|cache_creation_tokens" internal/rollup/api_request.go`
Expected: two lines reading these attributes.

- [ ] **Step 2: Add a comment immediately above the first cache_* read**

Insert these three comment lines directly above the `cacheRead := attrInt64(ev.Attrs, "cache_read_tokens")` line (or whatever the exact local-variable name is):

```go
// Log-event attrs use snake_case (cache_read_tokens, cache_creation_tokens).
// The claude_code.token.usage metric uses attribute `type` with camelCase
// values (cacheRead, cacheCreation). Do not unify — see docs/CLAUDE-CODE-OTEL.md §7.1 / §8.2.
```

- [ ] **Step 3: Verify it builds and tests still pass**

Run: `go vet ./internal/rollup/... && go test ./internal/rollup/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/rollup/api_request.go
git commit -m "docs(rollup): clarify snake_case (log) vs camelCase (metric) cache attrs"
```

---

## Task 5: Extend resource-attribute allowlist

**Files:**
- Test: `internal/eventparser/parser_test.go`
- Modify: `internal/eventparser/parser.go`

- [ ] **Step 1: Add a failing test asserting the new resource attrs propagate**

Append the following test to `internal/eventparser/parser_test.go` (use whatever helper functions the existing tests use to build a `resourcepb.Resource` and `logspb.LogRecord`; check the file for existing helpers like `kvStr`, `mkRecord`, `mkResource`):

```go
func TestParse_PropagatesNewResourceAttrs(t *testing.T) {
	rec := mkRecord(
		kvStr("session.id", "s1"),
		kvStr("event.name", "claude_code.user_prompt"),
	)
	res := mkResource(
		kvStr("user.account_id", "user_01ABC"),
		kvStr("terminal.type", "iTerm.app"),
	)
	ev, err := Parse(rec, res)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if ev.Attrs["user.account_id"] != "user_01ABC" {
		t.Errorf("user.account_id = %v", ev.Attrs["user.account_id"])
	}
	if ev.Attrs["terminal.type"] != "iTerm.app" {
		t.Errorf("terminal.type = %v", ev.Attrs["terminal.type"])
	}
}
```

If `mkRecord` / `mkResource` / `kvStr` are not the exact helper names, read `internal/eventparser/parser_test.go` and `internal/eventparser/fixture_test.go` and use the actual helpers. The test must build a record with a session ID and a resource carrying the two new attributes.

- [ ] **Step 2: Run the test, expect it to fail**

Run: `go test ./internal/eventparser/ -run TestParse_PropagatesNewResourceAttrs -v`
Expected: FAIL — both attrs come back nil because the allowlist filters them out.

- [ ] **Step 3: Extend the allowlist**

In `internal/eventparser/parser.go`, modify the `resourceAttrAllowlist` declaration (currently lines 21–32) to add two entries:

```go
var resourceAttrAllowlist = map[string]struct{}{
	"project.name":    {},
	"project.cwd":     {},
	"app.version":     {},
	"service.version": {},
	"os.type":         {},
	"os.version":      {},
	"host.arch":       {},
	"user.id":         {},
	"user.email":      {},
	"organization.id": {},
	"user.account_id": {}, // §9 of docs/CLAUDE-CODE-OTEL.md
	"terminal.type":   {}, // §9 of docs/CLAUDE-CODE-OTEL.md
}
```

- [ ] **Step 4: Run the test, expect it to pass**

Run: `go test ./internal/eventparser/ -run TestParse_PropagatesNewResourceAttrs -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/eventparser/parser.go internal/eventparser/parser_test.go
git commit -m "feat(eventparser): allowlist user.account_id and terminal.type resource attrs"
```

---

## Task 6: Fix `error_message` / `http_status_code` test fixtures

**Files:**
- Modify: `internal/eventparser/parser_test.go`

- [ ] **Step 1: Update the api_error fixture to corrected attribute names**

Locate the `api_error`-related test in `internal/eventparser/parser_test.go` near lines 148–161. Change:

- `kvStr("error_message", "rate limit")` → `kvStr("error", "rate limit")`
- `kvInt("http_status_code", 429)` → `kvInt("status_code", 429)`
- `if ev.Attrs["error_message"] != "rate limit"` → `if ev.Attrs["error"] != "rate limit"`
- `t.Errorf("error_message = %v", ev.Attrs["error_message"])` → `t.Errorf("error = %v", ev.Attrs["error"])`
- `if ev.Attrs["http_status_code"] != int64(429)` → `if ev.Attrs["status_code"] != int64(429)`
- `t.Errorf("http_status_code = %v", ev.Attrs["http_status_code"])` → `t.Errorf("status_code = %v", ev.Attrs["status_code"])`

- [ ] **Step 2: Run the test, expect it to pass**

Run: `go test ./internal/eventparser/ -v`
Expected: all PASS (the parser is attribute-name-agnostic — it just propagates).

- [ ] **Step 3: Commit**

```bash
git add internal/eventparser/parser_test.go
git commit -m "test(eventparser): use corrected api_error attr names (error, status_code)"
```

---

## Task 7: Fix `"deny"` literals in repository and TUI tests

**Files:**
- Modify: `internal/repository/rollups_test.go`
- Modify: `internal/tui/readstore/summarize_test.go`
- Modify: `internal/tui/readstore/summarize.go`

- [ ] **Step 1: Replace the rollups_test.go fixture**

In `internal/repository/rollups_test.go` line 27, change:

```go
Attrs: map[string]any{"decision": "deny"}},
```

to:

```go
Attrs: map[string]any{"decision": "reject"}},
```

- [ ] **Step 2: Replace the summarize_test.go fixture**

In `internal/tui/readstore/summarize_test.go` line 18, change:

```go
{"tool_decision", "claude_code.tool_decision", `{"decision":"deny","tool_name":"Bash"}`, "deny Bash"},
```

to:

```go
{"tool_decision", "claude_code.tool_decision", `{"decision":"reject","tool_name":"Bash"}`, "reject Bash"},
```

- [ ] **Step 3: Run both packages' tests**

Run: `go test ./internal/repository/... ./internal/tui/readstore/...`
Expected: PASS. (`summarize.go` already prints `decision tool_name` verbatim — no code change needed for summarize itself yet.)

- [ ] **Step 4: Commit**

```bash
git add internal/repository/rollups_test.go internal/tui/readstore/summarize_test.go
git commit -m "test: use reject (not deny) tool_decision value per OTel spec"
```

---

## Task 8: Register no-op handlers for the 11 new event types

**Files:**
- Create: `internal/rollup/noop_events.go`

- [ ] **Step 1: Create the no-op registration file**

Write `internal/rollup/noop_events.go`:

```go
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
```

- [ ] **Step 2: Build and test**

Run: `go vet ./internal/rollup/... && go test ./internal/rollup/...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/rollup/noop_events.go
git commit -m "feat(rollup): register no-op handlers for new Claude Code events"
```

---

## Task 9: Registry coverage test

**Files:**
- Modify: `internal/rollup/registry_test.go`

- [ ] **Step 1: Add a coverage assertion**

Append to `internal/rollup/registry_test.go`:

```go
func TestApply_AllDomainEventsHaveAHandler(t *testing.T) {
	for _, name := range domain.AllEventNames {
		if _, ok := updaters[name]; !ok {
			t.Errorf("rollup.updaters has no entry for %q (declared in domain.AllEventNames)", name)
		}
	}
}
```

- [ ] **Step 2: Run it, expect PASS**

Run: `go test ./internal/rollup/ -run TestApply_AllDomainEventsHaveAHandler -v`
Expected: PASS — every constant from Task 1 is registered (real handler or no-op).

- [ ] **Step 3: Commit**

```bash
git add internal/rollup/registry_test.go
git commit -m "test(rollup): assert every domain event constant has a registered handler"
```

---

## Task 10: Debug log on unhandled events

**Files:**
- Modify: `internal/rollup/rollup.go`

- [ ] **Step 1: Add the debug log**

Replace the `Apply` function body in `internal/rollup/rollup.go`:

```go
// Apply looks up the updater for ev.EventName and returns its ops. Returns
// nil for unknown or empty event names, after emitting a debug log so future
// Claude Code releases that introduce new event types are visible.
func Apply(ev domain.Event) []Op {
	if ev.EventName == "" {
		return nil
	}
	u, ok := updaters[ev.EventName]
	if !ok || u == nil {
		slog.Debug("rollup: no handler for event", "name", ev.EventName)
		return nil
	}
	return u(ev)
}
```

Add `"log/slog"` to the imports in the same file.

- [ ] **Step 2: Run rollup tests**

Run: `go test ./internal/rollup/...`
Expected: PASS. (Existing `TestApply_UnknownEventNameReturnsNil` and `TestApply_EmptyEventNameReturnsNil` still pass; the debug log is non-observable to the test.)

- [ ] **Step 3: Commit**

```bash
git add internal/rollup/rollup.go
git commit -m "feat(rollup): debug-log unhandled event names to surface upstream drift"
```

---

## Task 11: Extend summarize() with new event cases

**Files:**
- Modify: `internal/tui/readstore/summarize.go`
- Modify: `internal/tui/readstore/summarize_test.go`

- [ ] **Step 1: Add table-driven tests for the new cases**

Append the following entries to the table inside `internal/tui/readstore/summarize_test.go` (preserve the existing structure of the test table — read the file first to see the exact `[]struct{}` field names):

```go
{"compaction", "claude_code.compaction", `{"pre_tokens":12300,"post_tokens":4100,"trigger":"auto"}`, "compaction: 12300→4100 tok"},
{"code_edit_decision", "claude_code.code_edit_tool.decision", `{"decision":"reject","tool_name":"Edit"}`, "Edit reject"},
{"permission_mode", "claude_code.permission_mode_changed", `{"to":"acceptEdits"}`, "permission_mode → acceptEdits"},
{"auth", "claude_code.auth", `{"event":"login"}`, "auth: login"},
{"mcp_conn", "claude_code.mcp_server_connection", `{"server_name":"github","state":"connected"}`, "mcp github: connected"},
{"internal_err", "claude_code.internal_error", `{"error":"oops"}`, "internal_error: oops"},
{"plugin_installed", "claude_code.plugin_installed", `{"name":"foo"}`, "plugin installed: foo"},
{"skill_activated", "claude_code.skill_activated", `{"name":"brainstorm"}`, "skill: brainstorm"},
{"at_mention", "claude_code.at_mention", `{"target":"file"}`, "@mention: file"},
{"retries_exhausted", "claude_code.api_retries_exhausted", `{"attempt":4}`, "api retries exhausted: 4"},
{"hook_start", "claude_code.hook_execution_start", `{"hook":"PreToolUse"}`, "hook start: PreToolUse"},
{"hook_complete", "claude_code.hook_execution_complete", `{"hook":"PreToolUse","duration_ms":12}`, "hook done: PreToolUse 12ms"},
```

(`api_request_body` and `api_response_body` are off-by-default and not user-visible in the timeline; let them fall through to the default eventName rendering.)

- [ ] **Step 2: Run tests, expect them to FAIL**

Run: `go test ./internal/tui/readstore/ -run TestSummarize -v`
Expected: each new row FAILS with the default-case output (raw event name).

- [ ] **Step 3: Add summarize() cases**

Modify `internal/tui/readstore/summarize.go`. Replace its entire `switch` body with one that handles the existing cases plus the new ones. Use `domain.Event*` constants in case labels — add the import. Show the full file:

```go
package readstore

import (
	"encoding/json"
	"fmt"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

const maxSummaryRunes = 59

func summarize(eventName string, attrs []byte) string {
	var a map[string]any
	if err := json.Unmarshal(attrs, &a); err != nil {
		return truncRunes(eventName, maxSummaryRunes)
	}
	switch eventName {
	case domain.EventUserPrompt:
		if length, ok := a["prompt_length"]; ok {
			var lenStr string
			if f, isFloat := length.(float64); isFloat {
				lenStr = fmt.Sprintf("%dch", int(f))
			} else {
				lenStr = fmt.Sprintf("%vch", length)
			}
			if cmd, hasCmd := a["command_name"].(string); hasCmd && cmd != "" {
				return truncRunes("prompt: "+lenStr+" /"+cmd, maxSummaryRunes)
			}
			return truncRunes("prompt: "+lenStr, maxSummaryRunes)
		}
		return "prompt"
	case domain.EventToolResult:
		tool, _ := a["tool_name"].(string)
		durStr := "?ms"
		if d, ok := a["duration_ms"].(float64); ok {
			durStr = fmt.Sprintf("%dms", int(d))
		}
		mark := ""
		if ok, isBool := a["success"].(bool); isBool && !ok {
			mark = " ✗"
		}
		return truncRunes(fmt.Sprintf("%s %s%s", tool, durStr, mark), maxSummaryRunes)
	case domain.EventToolDecision:
		dec, _ := a["decision"].(string)
		tool, _ := a["tool_name"].(string)
		return truncRunes(fmt.Sprintf("%s %s", dec, tool), maxSummaryRunes)
	case domain.EventAPIRequest:
		model, _ := a["model"].(string)
		cost, _ := a["cost_usd"].(float64)
		return truncRunes(fmt.Sprintf("%s $%.4f", model, cost), maxSummaryRunes)
	case domain.EventAPIError:
		if msg, ok := a["error"].(string); ok && msg != "" {
			return truncRunes("error: "+msg, maxSummaryRunes)
		}
		if code := a["status_code"]; code != nil {
			return truncRunes(fmt.Sprintf("error: %v", code), maxSummaryRunes)
		}
		return "error"
	case "claude_code.code_edit_tool.decision":
		dec, _ := a["decision"].(string)
		tool, _ := a["tool_name"].(string)
		return truncRunes(fmt.Sprintf("%s %s", tool, dec), maxSummaryRunes)
	case domain.EventCompaction:
		pre, _ := a["pre_tokens"].(float64)
		post, _ := a["post_tokens"].(float64)
		return truncRunes(fmt.Sprintf("compaction: %d→%d tok", int(pre), int(post)), maxSummaryRunes)
	case domain.EventPermissionModeChanged:
		to, _ := a["to"].(string)
		return truncRunes("permission_mode → "+to, maxSummaryRunes)
	case domain.EventAuth:
		evt, _ := a["event"].(string)
		return truncRunes("auth: "+evt, maxSummaryRunes)
	case domain.EventMCPServerConnection:
		name, _ := a["server_name"].(string)
		state, _ := a["state"].(string)
		return truncRunes(fmt.Sprintf("mcp %s: %s", name, state), maxSummaryRunes)
	case domain.EventInternalError:
		msg, _ := a["error"].(string)
		return truncRunes("internal_error: "+msg, maxSummaryRunes)
	case domain.EventPluginInstalled:
		name, _ := a["name"].(string)
		return truncRunes("plugin installed: "+name, maxSummaryRunes)
	case domain.EventSkillActivated:
		name, _ := a["name"].(string)
		return truncRunes("skill: "+name, maxSummaryRunes)
	case domain.EventAtMention:
		target, _ := a["target"].(string)
		return truncRunes("@mention: "+target, maxSummaryRunes)
	case domain.EventAPIRetriesExhausted:
		var attempt int
		if f, ok := a["attempt"].(float64); ok {
			attempt = int(f)
		}
		return truncRunes(fmt.Sprintf("api retries exhausted: %d", attempt), maxSummaryRunes)
	case domain.EventHookExecutionStart:
		hook, _ := a["hook"].(string)
		return truncRunes("hook start: "+hook, maxSummaryRunes)
	case domain.EventHookExecutionComplete:
		hook, _ := a["hook"].(string)
		dur := ""
		if d, ok := a["duration_ms"].(float64); ok {
			dur = fmt.Sprintf(" %dms", int(d))
		}
		return truncRunes("hook done: "+hook+dur, maxSummaryRunes)
	default:
		return truncRunes(eventName, maxSummaryRunes)
	}
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
```

- [ ] **Step 4: Run tests, expect PASS**

Run: `go test ./internal/tui/readstore/ -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/readstore/summarize.go internal/tui/readstore/summarize_test.go
git commit -m "feat(tui): summarize new Claude Code event types in timeline"
```

---

## Task 12: Full verification sweep

- [ ] **Step 1: Run vet across the repo**

Run: `make vet`
Expected: no output, exit 0.

- [ ] **Step 2: Run the full test suite**

Run: `make test`
Expected: all packages PASS.

- [ ] **Step 3: Build the binary**

Run: `make build`
Expected: `bin/claude-code-observer` produced, no errors.

- [ ] **Step 4: Manual smoke check (optional, requires a wired project)**

```bash
./bin/claude-code-observer serve --log-level debug &
SERVE_PID=$!
sleep 1
# In a project that has run `cco init`, run any claude command, then:
sleep 25  # wait one OTel flush cycle
grep "rollup: no handler for event" ~/.claude-code-observer/logs/cco.log || echo "no unhandled events — clean"
kill $SERVE_PID
```

Expected: the grep prints `"no unhandled events — clean"`. If it prints log lines, those are events Claude Code emits that aren't yet in `domain.AllEventNames` — file as a follow-up.

- [ ] **Step 5: Final commit (if anything was tweaked) or push**

If you fixed anything during the sweep, commit it. Otherwise stop here and request review.

---

## Self-Review

**Spec coverage:** All eight audit findings have a task — issue 1 (Task 2), issues 2 & 3 (Task 4, comment + Task 2 value fix), issue 4 (Task 6), issue 5 (Task 7), issue 6 (Task 5), issue 7 (Task 8 + Task 10), issue 8 (Task 11). Spec component 1 (`wire.go`) → Task 1. Component 6 (Apply debug log) → Task 10. Component 7 (test fixtures) → Tasks 2/6/7. Component 8 (summarize) → Task 11.

**Placeholders:** None — every step shows the actual code or command.

**Type consistency:** `noopUpdater` signature matches the existing `Updater` type alias `func(domain.Event) []Op` (no error return — confirmed in `internal/rollup/rollup.go:14`). Constants in Task 1 are referenced verbatim by every later task.

**Spec note resolved:** The original spec used `register()` and an error-returning handler; the actual codebase uses direct `updaters[name] = fn` assignment with `func(ev domain.Event) []Op` — plan reflects the codebase, not the spec's pseudocode.
