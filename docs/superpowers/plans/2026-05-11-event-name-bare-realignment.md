# Event-Name Bare Realignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Drop the `claude_code.` prefix from event-name constants and TUI hardcoded literals so the rollup engine matches what Claude Code actually emits, restoring the empty `sessions` / `prompts` tables and the TUI dashboard. Add a defensive `TrimPrefix` in the parser to survive future drift, and update `docs/CLAUDE-CODE-OTEL.md` §8 to match wire reality.

**Architecture:** Three-layer rename. (1) `internal/eventparser` normalizes inbound event names by stripping any leading `claude_code.` so the parser is forgiving. (2) `internal/domain/wire.go` constants drop the prefix, and every test/TUI literal that hardcoded the prefix follows. (3) Spec doc §8 is rewritten so future readers do not reintroduce the drift. Metric names (`claude_code.session.count`, etc.) and the metric snapshot path are out of scope.

**Tech Stack:** Go 1.25, `database/sql` + SQLite, OTLP/gRPC. Test framework: stdlib `testing`. Build: `make vet`, `make test`, `make build`.

**Spec:** `docs/superpowers/specs/2026-05-11-event-name-bare-realignment-design.md` (commit `e4ea91d`).

**Branch:** `cco-otel-audit-fix` (no worktree — direct changes on this branch).

---

## File Inventory

**Modify:**
- `internal/eventparser/parser.go` — add `strings.TrimPrefix("claude_code.")` in `eventNameOf`
- `internal/eventparser/parser_test.go` — add round-trip test, update existing assertions
- `internal/domain/wire.go` — bare event-name constants
- `internal/tui/readstore/queries.go` — `IN(...)` SQL list and two `switch` cases
- `internal/tui/sessions/detail.go` — two string literals
- `internal/tui/sessions/testdata/detail_mixed.golden` — golden file
- All test files using prefixed event-name string literals (see Task 3 list)
- `docs/CLAUDE-CODE-OTEL.md` — §8 (Events) headings, inline refs, callout

**Out of scope (do not touch):**
- `internal/tui/readstore/summarize.go:59` (`claude_code.code_edit_tool.decision` is a **metric** name, not an event — leave it)
- Any constant in the `Metric*` group in `wire.go`
- Frozen historical artifacts under `docs/specs/`, `docs/plans/`, `docs/superpowers/specs/`, `docs/superpowers/plans/`
- DB schema (no migration needed — stored event names are already bare)

---

## Task 1: Defensive prefix-strip in eventparser

**Files:**
- Modify: `internal/eventparser/parser.go:77-87` (`eventNameOf`)
- Modify: `internal/eventparser/parser.go:1-11` (add `strings` import)
- Test: `internal/eventparser/parser_test.go`

- [ ] **Step 1: Add a failing round-trip test**

Append to `internal/eventparser/parser_test.go`:

```go
func TestParse_StripsClaudeCodePrefixFromEventName(t *testing.T) {
	cases := []struct {
		name     string
		incoming string
		want     string
	}{
		{"prefixed user_prompt", "claude_code.user_prompt", "user_prompt"},
		{"prefixed api_request", "claude_code.api_request", "api_request"},
		{"already bare", "user_prompt", "user_prompt"},
		{"unknown bare passes through", "something_new", "something_new"},
		{"prefixed unknown stripped", "claude_code.something_new", "something_new"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &logspb.LogRecord{
				TimeUnixNano: 1,
				Attributes: []*commonpb.KeyValue{
					kvStr("event.name", tc.incoming),
					kvStr("session.id", "sess-1"),
				},
			}
			got, err := Parse(rec, nil)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got.EventName != tc.want {
				t.Fatalf("event_name = %q, want %q", got.EventName, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test and verify it fails**

Run: `go test ./internal/eventparser/ -run TestParse_StripsClaudeCodePrefix -v`
Expected: FAIL — at least one subtest reports `event_name = "claude_code.user_prompt", want "user_prompt"`.

- [ ] **Step 3: Add the `strings` import**

Edit `internal/eventparser/parser.go` import block to include `"strings"`. The import block currently looks like:

```go
import (
	"errors"
	"fmt"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)
```

Change to:

```go
import (
	"errors"
	"fmt"
	"strings"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)
```

- [ ] **Step 4: Implement TrimPrefix in `eventNameOf`**

Replace `internal/eventparser/parser.go:77-87` with:

```go
// eventNameOf returns the event name from (in order) LogRecord.EventName,
// then the "event.name" attribute. If neither is set, returns "".
//
// Claude Code currently emits bare event names (e.g. "user_prompt"). This
// function strips a leading "claude_code." defensively so the receiver keeps
// working if a future release re-introduces the prefix. See
// docs/CLAUDE-CODE-OTEL.md §8.
func eventNameOf(rec *logspb.LogRecord, flat map[string]any) string {
	var name string
	if n := rec.GetEventName(); n != "" {
		name = n
	} else if s, ok := flat["event.name"].(string); ok {
		name = s
	}
	return strings.TrimPrefix(name, "claude_code.")
}
```

- [ ] **Step 5: Update existing parser tests that assert prefixed event names**

In `internal/eventparser/parser_test.go`:

- Line 55: replace `got.EventName != "claude_code.user_prompt"` with `got.EventName != "user_prompt"`.
- Line 85: replace `got.EventName != "claude_code.something_new"` with `got.EventName != "something_new"`.

Other occurrences in this file (e.g. inputs at lines 26, 38, 76, 121, 144, 169, 192, 211) that pass prefixed names as **input** are valid as-is — they exercise the strip path. Do not change inputs.

If a test has the structure `EventName: "claude_code.from_field"` set on the `LogRecord` proto field directly (line 103) and then asserts `ev.EventName != "claude_code.from_field"` at line 112: keep the input as-is, change the assertion to `ev.EventName != "from_field"`.

- [ ] **Step 6: Run full parser test suite and verify all pass**

Run: `go test ./internal/eventparser/ -v`
Expected: PASS for every test, including the new `TestParse_StripsClaudeCodePrefixFromEventName`.

- [ ] **Step 7: Commit**

```bash
git add internal/eventparser/parser.go internal/eventparser/parser_test.go
git commit -m "feat(eventparser): defensively strip claude_code. prefix from event names

Claude Code emits events with bare event_name (e.g. 'user_prompt').
TrimPrefix keeps the parser tolerant of a future release that re-adds
the 'claude_code.' prefix without requiring a code patch."
```

---

## Task 2: Bare event-name constants in `internal/domain/wire.go`

**Files:**
- Modify: `internal/domain/wire.go:5-29` (event-name constants)

- [ ] **Step 1: Replace the event-name constant block**

Replace `internal/domain/wire.go:1-29` with:

```go
package domain

// Event names — emitted by Claude Code as OTLP log records.
// Source of truth: docs/CLAUDE-CODE-OTEL.md §8.
//
// Claude Code emits these as bare strings on LogRecord.event_name (no
// "claude_code." prefix). The eventparser also strips a leading
// "claude_code." defensively (see internal/eventparser/parser.go), so
// updaters keyed on these constants match either form on the wire.
const (
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

	// Community-observed event names (not in official docs §8.8).
	// Retained because the existing rollup pipeline uses them.
	EventSessionStart = "session_start"
	EventSessionEnd   = "session_end"
)
```

Leave the `Metric*` constants (lines 33–42) and `AllEventNames` (lines 46–53) untouched — they reference the renamed constants by identifier, so values cascade automatically.

- [ ] **Step 2: Run vet and the rollup registry test to confirm constants compile**

Run: `go vet ./... && go test ./internal/rollup/ -run TestRegistryHasHandlerForEveryKnownEvent -v`
Expected: vet clean. Registry test may fail because tests in other packages still hardcode prefixed literals — that is expected and Task 3 fixes it. The registry test itself should pass because it iterates `domain.AllEventNames`, which now resolves to bare values that the updaters map will be keyed under after Task 3 (or already is keyed under, since updaters use the same constants — confirm PASS).

If the registry test fails with "no handler for X", investigate before proceeding — it means an updater file references a string literal instead of the constant.

- [ ] **Step 3: Commit**

```bash
git add internal/domain/wire.go
git commit -m "refactor(domain): drop claude_code. prefix from event-name constants

Match the wire reality: Claude Code emits bare event names. The
eventparser strips a leading 'claude_code.' defensively, so this rename
restores the rollup-engine name lookup that was silently missing on
every incoming event."
```

---

## Task 3: Cascade rename to TUI prod code, tests, and golden

**Files:**
- Modify: `internal/tui/readstore/queries.go:325, 346, 356`
- Modify: `internal/tui/sessions/detail.go:140, 174`
- Modify: `internal/tui/sessions/testdata/detail_mixed.golden`
- Modify: every `_test.go` file under `internal/` containing the literal `"claude_code."` followed by an event name

The goal of this task is `make test` green. Do all changes in one task because they are mechanically the same and only land consistent together.

- [ ] **Step 1: List every remaining hit**

Run: `grep -rn '"claude_code\.' internal/`
Expected: a list of files containing prefixed event-name string literals. Capture this list — every line is a target. Skip any line that resolves to a **metric** name (`session.count`, `lines_of_code.count`, `pull_request.count`, `commit.count`, `cost.usage`, `token.usage`, `active_time.total`, `code_edit_tool.decision`); those are out of scope.

- [ ] **Step 2: Update `internal/tui/readstore/queries.go`**

Line 325, replace:

```go
WHERE prompt_id = ? AND event_name IN ('claude_code.tool_result','claude_code.api_request')
```

with:

```go
WHERE prompt_id = ? AND event_name IN ('tool_result','api_request')
```

Line 346, replace:

```go
case "claude_code.tool_result":
```

with:

```go
case domain.EventToolResult:
```

Line 356, replace:

```go
case "claude_code.api_request":
```

with:

```go
case domain.EventAPIRequest:
```

If `domain` is not yet imported in this file, add `"github.com/kamikaze011001/claude-code-observer/internal/domain"` to the import block.

- [ ] **Step 3: Update `internal/tui/sessions/detail.go`**

Line 140, replace:

```go
if row.EventName != "claude_code.user_prompt" || row.PromptID == "" {
```

with:

```go
if row.EventName != domain.EventUserPrompt || row.PromptID == "" {
```

Line 174, replace:

```go
isPrompt := e.EventName == "claude_code.user_prompt" && e.PromptID != ""
```

with:

```go
isPrompt := e.EventName == domain.EventUserPrompt && e.PromptID != ""
```

If `domain` is not imported in this file, add `"github.com/kamikaze011001/claude-code-observer/internal/domain"` to the import block.

- [ ] **Step 4: Update the golden**

Run: `grep -n "claude_code\." internal/tui/sessions/testdata/detail_mixed.golden`
For every match, drop the `claude_code.` prefix from that line. Save the file.

- [ ] **Step 5: Update every `_test.go` file under `internal/`**

For each file from Step 1's list (excluding the parser test, which Task 1 already updated), replace every occurrence of `"claude_code.<event>"` with `"<event>"` where `<event>` is an event name from `internal/domain/wire.go`.

Concretely, run this `sed` chain — review the diff before staging, since `sed` will not distinguish event from metric strings if both appear:

```bash
for f in $(grep -rl '"claude_code\.' internal/ --include='*_test.go'); do
  # Event names only — leave metrics (session.count, token.usage, etc.) alone.
  sed -i.bak \
    -e 's/"claude_code\.user_prompt"/"user_prompt"/g' \
    -e 's/"claude_code\.api_request"/"api_request"/g' \
    -e 's/"claude_code\.api_error"/"api_error"/g' \
    -e 's/"claude_code\.tool_result"/"tool_result"/g' \
    -e 's/"claude_code\.tool_decision"/"tool_decision"/g' \
    -e 's/"claude_code\.compaction"/"compaction"/g' \
    -e 's/"claude_code\.permission_mode_changed"/"permission_mode_changed"/g' \
    -e 's/"claude_code\.auth"/"auth"/g' \
    -e 's/"claude_code\.mcp_server_connection"/"mcp_server_connection"/g' \
    -e 's/"claude_code\.internal_error"/"internal_error"/g' \
    -e 's/"claude_code\.plugin_installed"/"plugin_installed"/g' \
    -e 's/"claude_code\.skill_activated"/"skill_activated"/g' \
    -e 's/"claude_code\.at_mention"/"at_mention"/g' \
    -e 's/"claude_code\.api_retries_exhausted"/"api_retries_exhausted"/g' \
    -e 's/"claude_code\.hook_execution_start"/"hook_execution_start"/g' \
    -e 's/"claude_code\.hook_execution_complete"/"hook_execution_complete"/g' \
    -e 's/"claude_code\.api_request_body"/"api_request_body"/g' \
    -e 's/"claude_code\.api_response_body"/"api_response_body"/g' \
    -e 's/"claude_code\.session_start"/"session_start"/g' \
    -e 's/"claude_code\.session_end"/"session_end"/g' \
    "$f"
  rm -f "$f.bak"
done
```

After running, `grep -rn '"claude_code\.' internal/ --include='*_test.go'` should return only metric-name hits (or nothing). Inspect each remaining hit and confirm it is a metric, not an event.

The `internal/rollup/registry_test.go:10` literal `"claude_code.something_we_dont_handle"` is testing the unknown-event path; the strip would reduce it to `"something_we_dont_handle"` which is still unknown, so the test passes either way. Update it for consistency:

```go
ops := Apply(domain.Event{EventName: "something_we_dont_handle"})
```

- [ ] **Step 6: Run full test suite**

Run: `make vet && make test`
Expected: vet clean, all tests PASS.

If a test fails with a stale prefixed literal, grep for it and remove the prefix.

- [ ] **Step 7: Run build to confirm everything compiles**

Run: `make build`
Expected: produces `bin/claude-code-observer` cleanly.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/ internal/repository/ internal/rollup/ internal/retention/ internal/receiver/ internal/service/
git commit -m "refactor: drop claude_code. prefix from TUI literals, tests, golden

Cascading update after the wire.go constant rename. TUI queries and
session-detail code now switch on domain.EventX constants; tests and
golden file use the same bare names that arrive on the wire."
```

---

## Task 4: Spec doc — `docs/CLAUDE-CODE-OTEL.md` §8

**Files:**
- Modify: `docs/CLAUDE-CODE-OTEL.md` — §8 only

- [ ] **Step 1: Locate §8 boundaries**

Run: `grep -n "^##\|^###" docs/CLAUDE-CODE-OTEL.md`
Identify the line where §8 begins and where §9 begins. All edits in this task are between those two markers.

- [ ] **Step 2: Add the wire-format callout near the top of §8**

Immediately after the §8 heading line, insert:

```markdown
> **Wire format note:** Claude Code emits these events with **bare** names on `LogRecord.event_name` — e.g. `user_prompt`, not `claude_code.user_prompt`. The `claude_code.` namespace is reserved for metric names (§7). Receivers should match on bare names; reference implementations may also strip a leading `claude_code.` defensively to survive a future re-prefix.
```

- [ ] **Step 3: Rewrite subsection headings and inline references**

Within §8, rename every heading and inline event-name reference. Suggested mapping:

| Before | After |
|---|---|
| `### 8.1 \`claude_code.user_prompt\`` | `### 8.1 \`user_prompt\`` |
| `### 8.2 \`claude_code.api_request\`` | `### 8.2 \`api_request\`` |
| `### 8.3 \`claude_code.api_error\`` | `### 8.3 \`api_error\`` |
| `### 8.4 \`claude_code.tool_result\`` | `### 8.4 \`tool_result\`` |
| `### 8.5 \`claude_code.tool_decision\`` | `### 8.5 \`tool_decision\`` |
| `### 8.6 \`claude_code.compaction\`` | `### 8.6 \`compaction\`` |
| `\`claude_code.permission_mode_changed\`` (§8.8 list) | `\`permission_mode_changed\`` |
| `\`claude_code.auth\`` (§8.8 list) | `\`auth\`` |
| `\`claude_code.mcp_server_connection\`` (§8.8 list) | `\`mcp_server_connection\`` |
| `\`claude_code.internal_error\`` (§8.8 list) | `\`internal_error\`` |
| Any other prefixed event-name reference inside §8 | bare equivalent |

Keep references to **§8 link targets** consistent (anchors derived from heading text will change — that's fine, no other doc cross-links into §8 by anchor).

Also leave inline references to **environment variables** (`OTEL_LOG_USER_PROMPTS`, `OTEL_LOG_TOOL_DETAILS`, `OTEL_LOG_RAW_API_BODIES`) and metric names as-is. The lines at §6 line 160-163 that mention `claude_code.user_prompt` / `claude_code.tool_result` / `claude_code.api_request_body` / `claude_code.api_response_body` are **describing what the env var emits** — update them too, since they describe events:

| Before | After |
|---|---|
| `Include the full text of user prompts in \`claude_code.user_prompt\` log records.` | `Include the full text of user prompts in \`user_prompt\` log records.` |
| `Include \`tool_parameters\` in \`claude_code.tool_result\` records:` | `Include \`tool_parameters\` in \`tool_result\` records:` |
| `as \`claude_code.api_request_body\` / \`claude_code.api_response_body\` log events` | `as \`api_request_body\` / \`api_response_body\` log events` |

- [ ] **Step 4: Verify §7 (metrics) is unchanged**

Run: `grep -n "claude_code\." docs/CLAUDE-CODE-OTEL.md`
Expected: every remaining hit is either a **metric** name in §7 or in §6 environment-variable descriptions that legitimately reference metric/legacy naming. Confirm no event-name hits remain inside §8.

- [ ] **Step 5: Commit**

```bash
git add docs/CLAUDE-CODE-OTEL.md
git commit -m "docs(otel): align §8 with bare event names on the wire

Claude Code emits events with bare names (no claude_code. prefix). §8
now matches; §7 metric names keep the prefix. Adds a wire-format
callout so future readers do not reintroduce the drift."
```

---

## Task 5: Operational fix-up and end-to-end verification

This task does not modify code. It exercises the binary against the live database to confirm the bug is fixed for the user.

- [ ] **Step 1: Confirm any running daemon is stopped**

Run: `ps aux | grep -E "claude-code-observer|cco serve" | grep -v grep`
Expected: no rows. If a daemon is running, stop it with `kill <PID>` and re-check.

- [ ] **Step 2: Build the new binary**

Run: `make build`
Expected: `bin/claude-code-observer` rebuilt, no errors.

- [ ] **Step 3: Rebuild rollups against the existing DB**

Run: `./bin/claude-code-observer rebuild-rollups`
Expected output (counts will reflect whatever has accumulated):

```
rebuilt: <N> events → <M> sessions, <P> prompts (elapsed: ...)
```

`<M>` and `<P>` MUST be greater than 0 if the events table has any `user_prompt` or `api_request` rows. If both are still 0, stop — the rename did not land or there is another issue.

- [ ] **Step 4: Spot-check sessions and prompts**

Run:

```bash
sqlite3 ~/.claude-code-observer/db.sqlite \
  "SELECT 'sessions',COUNT(*) FROM sessions UNION ALL SELECT 'prompts',COUNT(*) FROM prompts;"
```

Expected: both rows show non-zero counts.

- [ ] **Step 5: Start the daemon and open the TUI**

In one terminal: `./bin/claude-code-observer serve --log-level debug`
In another: `./bin/claude-code-observer`

Expected: TUI dashboard renders sessions, costs, and tool usage. Confirm the previously-blank views now have content. Press `q` to exit.

- [ ] **Step 6: No commit needed**

This task is operational only.

---

## Self-Review Notes

**Spec coverage:**
- Spec §1 (constants) → Task 2 ✓
- Spec §2 (parser TrimPrefix) → Task 1 ✓
- Spec §3 (TUI queries.go) → Task 3 ✓
- Spec §4 (TUI detail.go) → Task 3 ✓
- Spec §5 (tests + goldens) → Task 1 (parser tests), Task 3 (everything else) ✓
- Spec §6 (docs/CLAUDE-CODE-OTEL.md) → Task 4 ✓
- Spec §7 (operational fix-up) → Task 5 ✓
- Spec verification list → Task 5 ✓

**Out-of-scope items confirmed:**
- Metric constants and metric tables — not modified
- DB schema — no migration

**Type consistency:**
- `domain.EventToolResult`, `domain.EventAPIRequest`, `domain.EventUserPrompt` referenced in Task 3 are the same identifiers redefined in Task 2. Values cascade.
