# Productivity Metrics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface Claude Code productivity telemetry (lines of code, commits, PRs, active time, edit accept-rate) — already ingested into `metric_snapshots` but never displayed — across four TUI surfaces, backed by a new metric rollup.

**Architecture:** A new **metric rollup** mirrors the existing event rollup. Claude Code exports these counters with **DELTA** temporality, so aggregation is additive (`col = col + excluded.col`, equivalently `SUM`). Seven new columns on `sessions` hold per-session totals, fed inline at ingest in one transaction and rebuildable via `cco rebuild-rollups`. Reads land on the dashboard cards, sessions list, session detail card, and a new Productivity view.

**Tech Stack:** Go 1.25, SQLite (`modernc.org/sqlite`), Bubble Tea / lipgloss TUI. Table-driven tests throughout.

**Spec:** `docs/superpowers/specs/2026-06-15-productivity-metrics-design.md`

**Key facts (from the spec, grounded in the live DB):**
- Temporality is **delta** → aggregate with `SUM` / additive upsert, never `MAX`.
- `lines_of_code.count`: attr `type` ∈ {`added`,`removed`}; value = line count.
- `commit.count`, `pull_request.count`: plain counts; value = increment.
- `active_time.total`: attr `type` ∈ {`user`,`cli`} — **use `type=user` ONLY**; value is in seconds.
- `code_edit_tool.decision`: attr `decision` ∈ {`accept`,`reject`}; value = 1 per decision.
- Cost/token metrics are **not** rolled up here (events already carry them; double-count risk).

---

## File Structure

**Phase 1 — Foundation (no UI):**
- Create: `internal/repository/migrations/0003_sessions_productivity.sql` — 7 new columns.
- Modify: `internal/rollup/user_prompt.go` — extend `sessionCounters` struct, `sessionCounterUpsert`, `sessionCounterArgs`.
- Create: `internal/rollup/metric_rollup.go` — `MetricUpdater`, `metricUpdaters` registry, `ApplyMetric`.
- Create: `internal/rollup/metric_handlers.go` — the five metric handlers + `init()`.
- Create: `internal/rollup/metric_rollup_test.go`, `internal/rollup/metric_handlers_test.go`.
- Modify: `internal/repository/events.go` — `InsertMetricsAndApplyRollups`, `applyMetricRollupsTx`.
- Modify: `internal/service/service.go` — `IngestMetrics` calls the new combined path.
- Modify: `internal/repository/rollups.go` — `RebuildRollups` second replay pass over `metric_snapshots`.
- Modify: `cmd/app/rebuild.go` — report productivity totals.
- Modify: `internal/repository/migrate_test.go`, `internal/repository/repository_test.go` — schema_version 2 → 3.
- Modify: `docs/CLAUDE-CODE-OTEL.md` — delta-temporality correction.

**Phase 2 — Dashboard + List:**
- Modify: `internal/tui/readstore/queries.go` — `WindowStats`/`Snapshot` productivity fields + `DashboardSnapshot`; `SessionRow` + `SessionsPage`.
- Modify: `internal/tui/dashboard/view.go` — productivity rows in window cards.
- Modify: `internal/tui/sessions/list.go` — lines column.

**Phase 3 — Session detail card:**
- Modify: `internal/tui/readstore/queries.go` — `SessionHeader` query.
- Modify: `internal/tui/sessions/detail.go` — productivity card.

**Phase 4 — Productivity view (separable):**
- Modify: `internal/tui/readstore/queries.go` — `ProductivityByDay`.
- Create: `internal/tui/productivity/model.go`, `internal/tui/productivity/view.go`, `internal/tui/productivity/doc.go`.
- Modify: `internal/tui/dashboard/model.go` + `internal/tui/app/*` — keybinding to open the view.

---

# Phase 1 — Foundation

### Task 1: Migration — 7 productivity columns on `sessions`

**Files:**
- Create: `internal/repository/migrations/0003_sessions_productivity.sql`
- Modify: `internal/repository/migrate_test.go:175` (TestRunMigrations_EmbeddedInitial, version 2 → 3)
- Modify: `internal/repository/repository_test.go:33,57` (schema_version 2 → 3, rows 2 → 3)

- [ ] **Step 1: Update the embedded-migration test to expect version 3**

In `internal/repository/migrate_test.go`, in `TestRunMigrations_EmbeddedInitial`, change the version assertion:

```go
	if v != 3 {
		t.Errorf("version = %d, want 3", v)
	}
```

- [ ] **Step 2: Update repository_test.go version assertions**

In `internal/repository/repository_test.go`, `TestOpen_AppliesMigrations`:

```go
	if v != 3 {
		t.Errorf("schema_version = %d, want 3", v)
	}
```

And `TestOpen_Idempotent` (counts applied migrations):

```go
	if n != 3 {
		t.Errorf("schema_version rows = %d, want 3", n)
	}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/repository/ -run 'TestRunMigrations_EmbeddedInitial|TestOpen_AppliesMigrations|TestOpen_Idempotent' -v`
Expected: FAIL — `version = 2, want 3` (the migration file does not exist yet).

- [ ] **Step 4: Create the migration**

Create `internal/repository/migrations/0003_sessions_productivity.sql`:

```sql
-- 0003_sessions_productivity.sql — per-session productivity totals derived from
-- Claude Code metric datapoints (lines_of_code.count, commit.count,
-- pull_request.count, active_time.total[type=user], code_edit_tool.decision).
-- These counters arrive with DELTA temporality, so the metric rollup accumulates
-- them additively (col = col + excluded.col). See docs/CLAUDE-CODE-OTEL.md.

ALTER TABLE sessions ADD COLUMN lines_added     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN lines_removed   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN commits         INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN pull_requests   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN active_seconds  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN edits_accepted  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN edits_rejected  INTEGER NOT NULL DEFAULT 0;
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/repository/ -run 'TestRunMigrations_EmbeddedInitial|TestOpen_AppliesMigrations|TestOpen_Idempotent' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/repository/migrations/0003_sessions_productivity.sql internal/repository/migrate_test.go internal/repository/repository_test.go
git commit -m "feat(repository): add productivity columns to sessions (migration 0003)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 2: Extend the session counter upsert with productivity fields

The metric rollup reuses the existing `sessionCounterUpsert` plumbing so metric-derived rows go through the same additive upsert (and create the session row if absent). This task adds the 7 fields.

**Files:**
- Modify: `internal/rollup/user_prompt.go:15-53`
- Test: `internal/rollup/user_prompt_test.go` (existing args-shape test — update expected length)

- [ ] **Step 1: Update the existing sessionCounterArgs length test**

Open `internal/rollup/user_prompt_test.go`. Find the test that asserts the length/contents of `sessionCounterArgs(...)`. The args slice grows from 15 to 22 elements (7 new trailing counters). Update any hard-coded length check to `22` and append seven `int64(0)` values to any expected-args slice for the zero-counter case. (If the test only checks `Prompts: 1` positionally, extend the expected slice with `int64(0)` ×7 at the end.)

Run: `go test ./internal/rollup/ -run TestApplyUserPrompt -v`
Expected: FAIL (length mismatch) once Step 2 lands — but if the test currently only checks a prefix, it may still pass; that's fine, proceed.

- [ ] **Step 2: Extend the struct, upsert, and args builder**

In `internal/rollup/user_prompt.go`, replace the `sessionCounterUpsert` const, the `sessionCounters` struct, and `sessionCounterArgs` with:

```go
const sessionCounterUpsert = `INSERT INTO sessions (
    session_id, started_at, last_seen_at,
    input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens,
    cost_usd, api_requests, api_errors, subagent_requests, auxiliary_requests,
    tool_calls, tool_denied, prompts,
    lines_added, lines_removed, commits, pull_requests,
    active_seconds, edits_accepted, edits_rejected
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET
    started_at            = MIN(started_at, excluded.started_at),
    last_seen_at          = MAX(last_seen_at, excluded.last_seen_at),
    input_tokens          = input_tokens          + excluded.input_tokens,
    output_tokens         = output_tokens         + excluded.output_tokens,
    cache_read_tokens     = cache_read_tokens     + excluded.cache_read_tokens,
    cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
    cost_usd              = cost_usd              + excluded.cost_usd,
    api_requests          = api_requests          + excluded.api_requests,
    api_errors            = api_errors            + excluded.api_errors,
    subagent_requests     = subagent_requests     + excluded.subagent_requests,
    auxiliary_requests    = auxiliary_requests    + excluded.auxiliary_requests,
    tool_calls            = tool_calls            + excluded.tool_calls,
    tool_denied           = tool_denied           + excluded.tool_denied,
    prompts               = prompts               + excluded.prompts,
    lines_added           = lines_added           + excluded.lines_added,
    lines_removed         = lines_removed         + excluded.lines_removed,
    commits               = commits               + excluded.commits,
    pull_requests         = pull_requests         + excluded.pull_requests,
    active_seconds        = active_seconds        + excluded.active_seconds,
    edits_accepted        = edits_accepted        + excluded.edits_accepted,
    edits_rejected        = edits_rejected        + excluded.edits_rejected`

// sessionCounterArgs builds the args slice for sessionCounterUpsert.
// Pass 0 for any counter that this updater does not bump.
type sessionCounters struct {
	InputTokens, OutputTokens, CacheReadTokens, CacheCreationTokens int64
	CostUSD                                                          float64
	APIRequests, APIErrors, SubagentRequests, AuxiliaryRequests     int64
	ToolCalls, ToolDenied, Prompts                                  int64
	LinesAdded, LinesRemoved, Commits, PullRequests                 int64
	ActiveSeconds, EditsAccepted, EditsRejected                     int64
}

func sessionCounterArgs(sessionID string, ts int64, c sessionCounters) []any {
	return []any{
		sessionID, ts, ts,
		c.InputTokens, c.OutputTokens, c.CacheReadTokens, c.CacheCreationTokens,
		c.CostUSD, c.APIRequests, c.APIErrors, c.SubagentRequests, c.AuxiliaryRequests,
		c.ToolCalls, c.ToolDenied, c.Prompts,
		c.LinesAdded, c.LinesRemoved, c.Commits, c.PullRequests,
		c.ActiveSeconds, c.EditsAccepted, c.EditsRejected,
	}
}
```

- [ ] **Step 3: Run rollup tests**

Run: `go test ./internal/rollup/ -v`
Expected: PASS (existing event handlers still pass 0 for the new counters via zero-value struct fields).

- [ ] **Step 4: Confirm it compiles end-to-end**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/rollup/user_prompt.go internal/rollup/user_prompt_test.go
git commit -m "feat(rollup): extend session counter upsert with productivity fields

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 3: Metric rollup registry — `MetricUpdater`, `metricUpdaters`, `ApplyMetric`

Parallel to the event `Apply`. Dispatches by metric name; unknown names are silently ignored so new upstream metrics never break ingest.

**Files:**
- Create: `internal/rollup/metric_rollup.go`
- Test: `internal/rollup/metric_rollup_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/rollup/metric_rollup_test.go`:

```go
package rollup

import (
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestApplyMetric_UnknownMetricReturnsNil(t *testing.T) {
	ops := ApplyMetric(domain.MetricSnapshot{MetricName: "claude_code.unknown.metric", Value: 5})
	if ops != nil {
		t.Fatalf("expected nil ops for unknown metric, got %d", len(ops))
	}
}

func TestApplyMetric_EmptyNameReturnsNil(t *testing.T) {
	if ops := ApplyMetric(domain.MetricSnapshot{}); ops != nil {
		t.Fatalf("expected nil for empty metric name")
	}
}

func TestApplyMetric_KnownMetricDispatches(t *testing.T) {
	ops := ApplyMetric(domain.MetricSnapshot{
		MetricName: domain.MetricCommit,
		SessionID:  "s1",
		TS:         1000,
		Value:      2,
	})
	if len(ops) != 1 {
		t.Fatalf("expected 1 op for commit metric, got %d", len(ops))
	}
}
```

- [ ] **Step 2: Run it to verify failure**

Run: `go test ./internal/rollup/ -run TestApplyMetric -v`
Expected: FAIL — `ApplyMetric` undefined.

- [ ] **Step 3: Implement the registry**

Create `internal/rollup/metric_rollup.go`:

```go
package rollup

import (
	"log/slog"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

// MetricUpdater turns one metric datapoint into zero or more SQL ops. Like
// Updater, it is a pure function over the snapshot and never touches the DB.
//
// Claude Code exports these counters with DELTA temporality: each datapoint is
// the increment for its interval, so handlers accumulate additively via
// sessionCounterUpsert (col = col + excluded.col).
type MetricUpdater func(snap domain.MetricSnapshot) []Op

// metricUpdaters maps fully-qualified Claude Code metric names to their updater.
// Unknown names get no entry — ApplyMetric silently ignores them so the ingest
// path never breaks when upstream adds a metric.
var metricUpdaters = map[string]MetricUpdater{}

// ApplyMetric looks up the updater for snap.MetricName and returns its ops.
// Returns nil for unknown or empty names, after a debug log.
func ApplyMetric(snap domain.MetricSnapshot) []Op {
	if snap.MetricName == "" {
		return nil
	}
	u, ok := metricUpdaters[snap.MetricName]
	if !ok || u == nil {
		slog.Debug("rollup: no handler for metric", "name", snap.MetricName)
		return nil
	}
	return u(snap)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/rollup/ -run TestApplyMetric -v`
Expected: PASS (the `commit` handler arrives in Task 4; `TestApplyMetric_KnownMetricDispatches` will fail until then — see note).

> Note: `TestApplyMetric_KnownMetricDispatches` depends on the `commit` handler registered in Task 4. If running Task 3 in isolation, expect that one subtest to fail with "expected 1 op, got 0"; the other two pass. It goes green at the end of Task 4. (Alternatively, move that subtest into Task 4 — keep it here for cohesion.)

- [ ] **Step 5: Commit**

```bash
git add internal/rollup/metric_rollup.go internal/rollup/metric_rollup_test.go
git commit -m "feat(rollup): metric rollup registry (ApplyMetric, delta-temporal)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 4: The five metric handlers

Each handler reads the snapshot's `Value` (a `float64` increment) and `Attrs`, and emits a `sessionCounterUpsert` op bumping the relevant column(s).

**Files:**
- Create: `internal/rollup/metric_handlers.go`
- Test: `internal/rollup/metric_handlers_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/rollup/metric_handlers_test.go`:

```go
package rollup

import (
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

// findCounter runs ApplyMetric and returns the sessionCounters that the single
// emitted op encodes, by reading back the positional args. Fails if not exactly
// one op or the op is not a sessionCounterUpsert.
func wantOneSessionOp(t *testing.T, snap domain.MetricSnapshot) []any {
	t.Helper()
	ops := ApplyMetric(snap)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Query != sessionCounterUpsert {
		t.Fatalf("expected sessionCounterUpsert, got different query")
	}
	return ops[0].Args
}

// arg indices into sessionCounterArgs (see Task 2 ordering):
// 0 session_id,1 started_at,2 last_seen_at,
// 3 input,4 output,5 cacheRead,6 cacheCreation,7 cost,8 apiReq,9 apiErr,
// 10 subagent,11 auxiliary,12 toolCalls,13 toolDenied,14 prompts,
// 15 lines_added,16 lines_removed,17 commits,18 pull_requests,
// 19 active_seconds,20 edits_accepted,21 edits_rejected
const (
	idxLinesAdded    = 15
	idxLinesRemoved  = 16
	idxCommits       = 17
	idxPullRequests  = 18
	idxActiveSeconds = 19
	idxEditsAccepted = 20
	idxEditsRejected = 21
)

func TestMetric_LinesOfCode(t *testing.T) {
	cases := []struct {
		name              string
		typ               string
		value             float64
		wantAdded, wantRm int64
	}{
		{"added", "added", 156, 156, 0},
		{"removed", "removed", 12, 0, 12},
		{"unknown type ignored", "weird", 99, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := wantOneSessionOp(t, domain.MetricSnapshot{
				MetricName: domain.MetricLinesOfCode, SessionID: "s1", TS: 1000,
				Value: c.value, Attrs: map[string]any{"type": c.typ},
			})
			if args[idxLinesAdded].(int64) != c.wantAdded {
				t.Errorf("lines_added = %v, want %d", args[idxLinesAdded], c.wantAdded)
			}
			if args[idxLinesRemoved].(int64) != c.wantRm {
				t.Errorf("lines_removed = %v, want %d", args[idxLinesRemoved], c.wantRm)
			}
		})
	}
}

func TestMetric_Commit(t *testing.T) {
	args := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricCommit, SessionID: "s1", TS: 1000, Value: 2,
	})
	if args[idxCommits].(int64) != 2 {
		t.Errorf("commits = %v, want 2", args[idxCommits])
	}
}

func TestMetric_PullRequest(t *testing.T) {
	args := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricPullRequest, SessionID: "s1", TS: 1000, Value: 1,
	})
	if args[idxPullRequests].(int64) != 1 {
		t.Errorf("pull_requests = %v, want 1", args[idxPullRequests])
	}
}

func TestMetric_ActiveTime_UserOnly(t *testing.T) {
	// type=user counts; type=cli is ignored (unreliable uptime).
	userArgs := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricActiveTime, SessionID: "s1", TS: 1000,
		Value: 42.6, Attrs: map[string]any{"type": "user"},
	})
	if userArgs[idxActiveSeconds].(int64) != 43 { // rounded
		t.Errorf("active_seconds = %v, want 43", userArgs[idxActiveSeconds])
	}
	cliArgs := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricActiveTime, SessionID: "s1", TS: 1000,
		Value: 9999, Attrs: map[string]any{"type": "cli"},
	})
	if cliArgs[idxActiveSeconds].(int64) != 0 {
		t.Errorf("active_seconds for cli = %v, want 0", cliArgs[idxActiveSeconds])
	}
}

func TestMetric_CodeEditDecision(t *testing.T) {
	acc := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricCodeEditToolDecision, SessionID: "s1", TS: 1000,
		Value: 1, Attrs: map[string]any{"decision": "accept"},
	})
	if acc[idxEditsAccepted].(int64) != 1 || acc[idxEditsRejected].(int64) != 0 {
		t.Errorf("accept: accepted=%v rejected=%v, want 1/0", acc[idxEditsAccepted], acc[idxEditsRejected])
	}
	rej := wantOneSessionOp(t, domain.MetricSnapshot{
		MetricName: domain.MetricCodeEditToolDecision, SessionID: "s1", TS: 1000,
		Value: 1, Attrs: map[string]any{"decision": "reject"},
	})
	if rej[idxEditsRejected].(int64) != 1 || rej[idxEditsAccepted].(int64) != 0 {
		t.Errorf("reject: accepted=%v rejected=%v, want 0/1", rej[idxEditsAccepted], rej[idxEditsRejected])
	}
}

func TestMetric_AllProductivityMetricsHaveHandler(t *testing.T) {
	for _, name := range []string{
		domain.MetricLinesOfCode, domain.MetricCommit, domain.MetricPullRequest,
		domain.MetricActiveTime, domain.MetricCodeEditToolDecision,
	} {
		if _, ok := metricUpdaters[name]; !ok {
			t.Errorf("metricUpdaters missing handler for %q", name)
		}
	}
}

func TestMetric_CostAndTokenNotRolledUp(t *testing.T) {
	// Cost/token metrics must NOT be handled here — events own those.
	for _, name := range []string{domain.MetricCostUsage, domain.MetricTokenUsage, domain.MetricSessionCount} {
		if _, ok := metricUpdaters[name]; ok {
			t.Errorf("metricUpdaters must NOT handle %q (double-count risk)", name)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/rollup/ -run TestMetric -v`
Expected: FAIL — handlers undefined.

- [ ] **Step 3: Implement the handlers**

Create `internal/rollup/metric_handlers.go`:

```go
package rollup

import (
	"math"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

// metricSessionOp wraps a sessionCounters into a single sessionCounterUpsert op
// keyed on the snapshot's session/ts. Returns nil when the session id is empty
// (cannot attribute the increment to any row).
func metricSessionOp(snap domain.MetricSnapshot, c sessionCounters) []Op {
	if snap.SessionID == "" {
		return nil
	}
	return []Op{{
		Query: sessionCounterUpsert,
		Args:  sessionCounterArgs(snap.SessionID, snap.TS, c),
	}}
}

// roundNonNeg rounds v to the nearest int64, clamping negatives to 0 (these are
// counts and durations; a negative delta is meaningless and ignored).
func roundNonNeg(v float64) int64 {
	if v <= 0 {
		return 0
	}
	return int64(math.Round(v))
}

func applyLinesOfCode(snap domain.MetricSnapshot) []Op {
	n := roundNonNeg(snap.Value)
	var c sessionCounters
	switch snapAttrString(snap.Attrs, "type") {
	case "added":
		c.LinesAdded = n
	case "removed":
		c.LinesRemoved = n
	default:
		return nil // unknown type — ignore
	}
	return metricSessionOp(snap, c)
}

func applyCommit(snap domain.MetricSnapshot) []Op {
	return metricSessionOp(snap, sessionCounters{Commits: roundNonNeg(snap.Value)})
}

func applyPullRequest(snap domain.MetricSnapshot) []Op {
	return metricSessionOp(snap, sessionCounters{PullRequests: roundNonNeg(snap.Value)})
}

func applyActiveTime(snap domain.MetricSnapshot) []Op {
	// Only the user-attributed series is reliable; cli reports process uptime.
	if snapAttrString(snap.Attrs, "type") != "user" {
		return nil
	}
	return metricSessionOp(snap, sessionCounters{ActiveSeconds: roundNonNeg(snap.Value)})
}

func applyCodeEditDecision(snap domain.MetricSnapshot) []Op {
	n := roundNonNeg(snap.Value)
	var c sessionCounters
	switch snapAttrString(snap.Attrs, "decision") {
	case "accept":
		c.EditsAccepted = n
	case "reject":
		c.EditsRejected = n
	default:
		return nil
	}
	return metricSessionOp(snap, c)
}

// snapAttrString reads a string attr from a metric snapshot's attrs map.
func snapAttrString(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	if s, ok := attrs[key].(string); ok {
		return s
	}
	return ""
}

func init() {
	metricUpdaters[domain.MetricLinesOfCode] = applyLinesOfCode
	metricUpdaters[domain.MetricCommit] = applyCommit
	metricUpdaters[domain.MetricPullRequest] = applyPullRequest
	metricUpdaters[domain.MetricActiveTime] = applyActiveTime
	metricUpdaters[domain.MetricCodeEditToolDecision] = applyCodeEditDecision
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/rollup/ -run 'TestMetric|TestApplyMetric' -v`
Expected: PASS (including `TestApplyMetric_KnownMetricDispatches` from Task 3).

- [ ] **Step 5: Commit**

```bash
git add internal/rollup/metric_handlers.go internal/rollup/metric_handlers_test.go
git commit -m "feat(rollup): productivity metric handlers (lines, commits, PRs, active time, edits)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 5: Repository — `InsertMetricsAndApplyRollups`

One transaction: insert raw snapshots, then apply metric rollup ops. Mirrors `InsertEventsAndApplyRollups`.

**Files:**
- Modify: `internal/repository/events.go` (add the combined method + `applyMetricRollupsTx`)
- Test: `internal/repository/metric_rollups_test.go` (create)

- [ ] **Step 1: Write the failing integration test**

Create `internal/repository/metric_rollups_test.go`:

```go
package repository

import (
	"context"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestInsertMetricsAndApplyRollups_SumsDeltaSnapshots(t *testing.T) {
	repo, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer repo.Close()
	ctx := context.Background()

	// Non-monotonic delta series for lines_added on one session: total = 156+201+11 = 368.
	snaps := []domain.MetricSnapshot{
		{TS: 100, SessionID: "s1", MetricName: domain.MetricLinesOfCode, Value: 156, Attrs: map[string]any{"type": "added"}},
		{TS: 200, SessionID: "s1", MetricName: domain.MetricLinesOfCode, Value: 201, Attrs: map[string]any{"type": "added"}},
		{TS: 300, SessionID: "s1", MetricName: domain.MetricLinesOfCode, Value: 11, Attrs: map[string]any{"type": "added"}},
		{TS: 300, SessionID: "s1", MetricName: domain.MetricLinesOfCode, Value: 30, Attrs: map[string]any{"type": "removed"}},
		{TS: 350, SessionID: "s1", MetricName: domain.MetricCommit, Value: 2},
		{TS: 360, SessionID: "s1", MetricName: domain.MetricActiveTime, Value: 45, Attrs: map[string]any{"type": "user"}},
		{TS: 360, SessionID: "s1", MetricName: domain.MetricActiveTime, Value: 9999, Attrs: map[string]any{"type": "cli"}},
		{TS: 370, SessionID: "s1", MetricName: domain.MetricCodeEditToolDecision, Value: 1, Attrs: map[string]any{"decision": "accept"}},
		{TS: 371, SessionID: "s1", MetricName: domain.MetricCodeEditToolDecision, Value: 1, Attrs: map[string]any{"decision": "reject"}},
	}
	if err := repo.InsertMetricsAndApplyRollups(ctx, snaps); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	var added, removed, commits, active, acc, rej int64
	row := repo.DB().QueryRowContext(ctx,
		`SELECT lines_added, lines_removed, commits, active_seconds, edits_accepted, edits_rejected
		 FROM sessions WHERE session_id = 's1'`)
	if err := row.Scan(&added, &removed, &commits, &active, &acc, &rej); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if added != 368 {
		t.Errorf("lines_added = %d, want 368 (SUM of deltas, not MAX=201)", added)
	}
	if removed != 30 || commits != 2 || active != 45 || acc != 1 || rej != 1 {
		t.Errorf("removed=%d commits=%d active=%d acc=%d rej=%d; want 30/2/45/1/1",
			removed, commits, active, acc, rej)
	}

	// Raw snapshots are still persisted.
	var n int
	if err := repo.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM metric_snapshots`).Scan(&n); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	if n != len(snaps) {
		t.Errorf("metric_snapshots rows = %d, want %d", n, len(snaps))
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/repository/ -run TestInsertMetricsAndApplyRollups -v`
Expected: FAIL — `InsertMetricsAndApplyRollups` undefined.

- [ ] **Step 3: Implement the combined method**

In `internal/repository/events.go`, add (after `InsertMetricSnapshots`):

```go
// InsertMetricsAndApplyRollups inserts a batch of metric snapshots and applies
// the metric rollup additively, in a single transaction. Either every snapshot
// + every rollup op lands, or nothing does. Mirrors InsertEventsAndApplyRollups.
func (r *Repository) InsertMetricsAndApplyRollups(ctx context.Context, snaps []domain.MetricSnapshot) error {
	if len(snaps) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertMetricSnapshotsTx(ctx, tx, snaps); err != nil {
		return err
	}
	if err := applyMetricRollupsTx(ctx, tx, snaps); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// applyMetricRollupsTx executes metric rollup ops for each snapshot on the tx.
// All ops target the sessions table, so execOpsOrdered's session-first pass
// handles them without FK ordering concerns.
func applyMetricRollupsTx(ctx context.Context, tx *sql.Tx, snaps []domain.MetricSnapshot) error {
	for i := range snaps {
		ops := rollup.ApplyMetric(snaps[i])
		if err := execOpsOrdered(ctx, tx, snaps[i].MetricName, ops); err != nil {
			return err
		}
	}
	return nil
}
```

The file `internal/repository/events.go` must import `"github.com/kamikaze011001/claude-code-observer/internal/rollup"`. Add it to the import block (it is currently imported only by `rollups.go`; both files share the `repository` package, but imports are per-file — add it here):

```go
import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
	"github.com/kamikaze011001/claude-code-observer/internal/rollup"
)
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/repository/ -run TestInsertMetricsAndApplyRollups -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/events.go internal/repository/metric_rollups_test.go
git commit -m "feat(repository): InsertMetricsAndApplyRollups (snapshots + metric rollup in one tx)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 6: Wire `service.IngestMetrics` to the rollup path

**Files:**
- Modify: `internal/service/service.go:73-75`

- [ ] **Step 1: Switch the ingest call**

In `internal/service/service.go`, in `IngestMetrics`, replace:

```go
	if err := s.repo.InsertMetricSnapshots(ctx, snaps); err != nil {
		return fmt.Errorf("insert metric snapshots: %w", err)
	}
```

with:

```go
	if err := s.repo.InsertMetricsAndApplyRollups(ctx, snaps); err != nil {
		return fmt.Errorf("insert metric snapshots: %w", err)
	}
```

- [ ] **Step 2: Verify the service tests still pass**

Run: `go test ./internal/service/ -v`
Expected: PASS. If a test asserts the old method was called via a fake repo, update the fake to implement `InsertMetricsAndApplyRollups` (search the test file for `InsertMetricSnapshots`).

- [ ] **Step 3: Full build + vet**

Run: `go vet ./... && go build ./...`
Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/service/service.go
git commit -m "feat(service): roll up productivity metrics on ingest

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 7: Rebuild — replay `metric_snapshots` after events

`cco rebuild-rollups` must reproduce productivity totals. Add a second replay pass and report the new totals.

**Files:**
- Modify: `internal/repository/rollups.go:91-137` (RebuildRollups)
- Modify: `cmd/app/rebuild.go:32-42` (report lines/commits)
- Test: `internal/repository/rebuild_test.go` (add idempotency assertion)

- [ ] **Step 1: Write the failing rebuild test**

Add to `internal/repository/rebuild_test.go`:

```go
func TestRebuildRollups_ReplaysProductivityMetrics(t *testing.T) {
	repo, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer repo.Close()
	ctx := context.Background()

	snaps := []domain.MetricSnapshot{
		{TS: 100, SessionID: "s1", MetricName: domain.MetricLinesOfCode, Value: 100, Attrs: map[string]any{"type": "added"}},
		{TS: 200, SessionID: "s1", MetricName: domain.MetricLinesOfCode, Value: 50, Attrs: map[string]any{"type": "added"}},
		{TS: 300, SessionID: "s1", MetricName: domain.MetricCommit, Value: 3},
	}
	if err := repo.InsertMetricsAndApplyRollups(ctx, snaps); err != nil {
		t.Fatalf("ingest: %v", err)
	}

	// Rebuild from raw snapshots; totals must be identical (idempotent).
	if err := repo.RebuildRollups(ctx); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	var added, commits int64
	if err := repo.DB().QueryRowContext(ctx,
		`SELECT lines_added, commits FROM sessions WHERE session_id='s1'`).Scan(&added, &commits); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if added != 150 || commits != 3 {
		t.Errorf("after rebuild lines_added=%d commits=%d, want 150/3", added, commits)
	}
}
```

(Ensure the test file imports `"context"` and `"github.com/kamikaze011001/claude-code-observer/internal/domain"`.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/repository/ -run TestRebuildRollups_ReplaysProductivityMetrics -v`
Expected: FAIL — `lines_added=0` (rebuild does not replay snapshots yet).

- [ ] **Step 3: Add the metric replay pass to RebuildRollups**

In `internal/repository/rollups.go`, inside `RebuildRollups`, after the events `rows` loop completes its `rows.Err()` check and **before** `tx.Commit()`, insert a second pass. Replace the tail of the function (from the events `rows.Err()` block through `tx.Commit()`) with:

```go
	if err = rows.Err(); err != nil {
		return fmt.Errorf("rows: %w", err)
	}

	// Second pass: replay metric snapshots so productivity columns are
	// reproduced. Delta values + additive upsert ⇒ replay re-sums correctly.
	mrows, err := tx.QueryContext(ctx, `
		SELECT id, ts, COALESCE(session_id, ''), metric_name, value, attrs
		FROM metric_snapshots
		ORDER BY ts ASC, id ASC`)
	if err != nil {
		return fmt.Errorf("select metric_snapshots: %w", err)
	}
	defer mrows.Close()

	for mrows.Next() {
		var snap domain.MetricSnapshot
		var attrs string
		if err = mrows.Scan(&snap.ID, &snap.TS, &snap.SessionID, &snap.MetricName, &snap.Value, &attrs); err != nil {
			return fmt.Errorf("scan metric: %w", err)
		}
		snap.Attrs = map[string]any{}
		if attrs != "" {
			if jerr := jsonUnmarshal(attrs, &snap.Attrs); jerr != nil {
				return fmt.Errorf("unmarshal metric attrs id=%d: %w", snap.ID, jerr)
			}
		}
		if err = execOpsOrdered(ctx, tx, snap.MetricName, rollup.ApplyMetric(snap)); err != nil {
			return fmt.Errorf("metric rollup id=%d: %w", snap.ID, err)
		}
	}
	if err = mrows.Err(); err != nil {
		return fmt.Errorf("metric rows: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
```

> Note: the events `rows` are read with `defer rows.Close()`, but the second query reuses the same tx. SQLite (modernc) allows a new query on the tx after the first `rows` is fully drained; `rows` is still open via defer but exhausted, which is safe. If a "database is locked" error appears, explicitly call `rows.Close()` before opening `mrows` instead of relying on defer.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/repository/ -run TestRebuildRollups -v`
Expected: PASS (all rebuild tests).

- [ ] **Step 5: Report productivity totals in the rebuild command**

In `cmd/app/rebuild.go`, after the existing counts, add lines/commits totals and extend the printed summary:

```go
			var events, sessions, prompts int64
			var linesAdded, linesRemoved, commits int64
			db := repo.DB()
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&events)
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&sessions)
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompts`).Scan(&prompts)
			_ = db.QueryRowContext(ctx,
				`SELECT COALESCE(SUM(lines_added),0), COALESCE(SUM(lines_removed),0), COALESCE(SUM(commits),0) FROM sessions`).
				Scan(&linesAdded, &linesRemoved, &commits)

			cmd.Printf("rebuilt: %d events → %d sessions, %d prompts | +%d/-%d lines, %d commits (elapsed: %s)\n",
				events, sessions, prompts, linesAdded, linesRemoved, commits, elapsed)
			logger.Info("rebuild-rollups complete",
				"events", events, "sessions", sessions, "prompts", prompts,
				"lines_added", linesAdded, "lines_removed", linesRemoved, "commits", commits,
				"elapsed_ms", elapsed.Milliseconds())
			return nil
```

(Update the `Long` description string to mention "Replays events and metric snapshots".)

- [ ] **Step 6: Build + vet**

Run: `go vet ./... && go build ./... && go test ./internal/repository/ ./cmd/... -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/repository/rollups.go internal/repository/rebuild_test.go cmd/app/rebuild.go
git commit -m "feat(rebuild): replay metric snapshots into productivity columns

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 8: Doc update — correct delta temporality in CLAUDE-CODE-OTEL.md

**Files:**
- Modify: `docs/CLAUDE-CODE-OTEL.md`

- [ ] **Step 1: Read the metric section**

Run: `grep -n -i 'temporal\|cumulative\|lines_of_code\|active_time\|MAX\|SUM' docs/CLAUDE-CODE-OTEL.md`
Read the surrounding sections (§7.1 metric names, and any aggregation note).

- [ ] **Step 2: Add/correct the temporality note**

In the metrics section, ensure the following is stated explicitly (add a subsection if none exists):

```markdown
### Metric temporality (verified empirically, 2026-06-15)

Claude Code exports counter metrics with **DELTA** temporality: each datapoint
is the increment for its export interval, **not** a cumulative running total.
Aggregate them with `SUM` (or an additive upsert `col = col + excluded.col`),
**never** `MAX`.

Verified by: per-session sums of `claude_code.cost.usage` equal the
event-derived `sessions.cost_usd` to the cent, and `claude_code.token.usage`
input sums equal `sessions.input_tokens` exactly. A `lines_of_code.count` series
within one session is non-monotonic (e.g. 156 → 633 → 201), confirming deltas.

Per-metric notes for the productivity rollup (`internal/rollup/metric_handlers.go`):
- `lines_of_code.count`: attr `type` ∈ {added, removed}. No `language`/`model`
  attr on CC ≤ 2.1.158 — per-language breakdown is a future enhancement.
- `active_time.total`: attr `type` ∈ {user, cli}. **Use `type=user` only**;
  `type=cli` reports process-uptime-like values (~50× larger) and is excluded.
- `code_edit_tool.decision`: attr `decision` ∈ {accept, reject}; value = 1.
- `cost.usage` / `token.usage` / `session.count`: NOT rolled up — events already
  derive cost/tokens; rolling these up would double-count.
```

- [ ] **Step 3: Commit**

```bash
git add docs/CLAUDE-CODE-OTEL.md
git commit -m "docs(otel): correct metric temporality to delta + productivity notes

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

**Phase 1 done.** At this point ingest and rebuild populate productivity columns; verify against the live DB if desired:
```bash
make build && ./bin/claude-code-observer rebuild-rollups --home ~/.claude-code-observer
sqlite3 ~/.claude-code-observer/db.sqlite \
  "SELECT session_id, lines_added, lines_removed, commits, active_seconds FROM sessions ORDER BY last_seen_at DESC LIMIT 5;"
```

---

# Phase 2 — Dashboard cards + Sessions list

### Task 9: readstore — productivity fields on window snapshot

**Files:**
- Modify: `internal/tui/readstore/queries.go:92-100` (WindowStats), `:121-195` (DashboardSnapshot)
- Test: `internal/tui/readstore/queries_test.go` (extend the dashboard test)

- [ ] **Step 1: Extend WindowStats**

In `internal/tui/readstore/queries.go`, add fields to `WindowStats`:

```go
// WindowStats is the rollup over a single time window.
type WindowStats struct {
	Sessions     int64
	CostUSD      float64
	Prompts      int64
	Tokens       int64
	Tools        int64
	Errors       int64
	LinesAdded   int64
	LinesRemoved int64
	Commits      int64
	PullRequests int64
	ActiveSec    int64
}
```

- [ ] **Step 2: Write/extend the failing test**

In `internal/tui/readstore/queries_test.go`, find the `DashboardSnapshot` test that seeds sessions. Add productivity columns to the seeded rows (via the existing insert helper — extend its column list and values to include `lines_added, lines_removed, commits, pull_requests, active_seconds`), then assert e.g.:

```go
	if snap.Today.LinesAdded != 150 {
		t.Errorf("Today.LinesAdded = %d, want 150", snap.Today.LinesAdded)
	}
	if snap.D7.Commits != 4 {
		t.Errorf("D7.Commits = %d, want 4", snap.D7.Commits)
	}
```

(Choose values matching whatever the test seeds. If the seed helper inserts via explicit column list, add the five columns there with chosen values; if it inserts a full row, append the values.)

Run: `go test ./internal/tui/readstore/ -run Dashboard -v`
Expected: FAIL — `LinesAdded = 0`.

- [ ] **Step 3: Extend the DashboardSnapshot query**

In `DashboardSnapshot`, the main 3-window query currently selects 6 aggregates per window (sessions, cost, prompts, tokens, tools, errors) for TODAY/7D/30D. Add five productivity aggregates per window. For each window's `CASE WHEN started_at >= ? THEN ... END` block, add:

```sql
  COALESCE(SUM(CASE WHEN started_at >= ? THEN lines_added    END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN lines_removed  END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN commits        END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN pull_requests  END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN active_seconds END), 0),
```

So each window block has 11 aggregates (6 existing + 5 new). Update the `?` argument list passed to `QueryRowContext` so each window threshold is repeated 11 times instead of 6 (i.e. `today` ×11, `d7` ×11, `d30` ×11, then the final `d30` for the `WHERE`). Update the `.Scan(...)` to read the five new fields per window into `&s.Today.LinesAdded, &s.Today.LinesRemoved, &s.Today.Commits, &s.Today.PullRequests, &s.Today.ActiveSec` (and likewise D7, D30).

> Keep column/scan order identical across the three windows. The `Scan` destination order must match the `SELECT` order exactly: for each window, the 6 existing then the 5 new.

For the **yesterday** sub-query (`yQ`), add the same five `COALESCE(SUM(...))` columns and scan them into `&s.Yesterday.LinesAdded, ...` if the dashboard delta strip will use them; otherwise leave yesterday as-is. (The delta strip in Task 10 does not require yesterday productivity — skip extending `yQ` unless adding a productivity delta row.)

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/tui/readstore/ -run Dashboard -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/readstore/queries.go internal/tui/readstore/queries_test.go
git commit -m "feat(readstore): productivity aggregates in DashboardSnapshot windows

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 10: Dashboard — productivity rows in window cards

**Files:**
- Modify: `internal/tui/dashboard/view.go:60-99` (renderWindowCard)

- [ ] **Step 1: Add the formatting helper**

In `internal/tui/dashboard/view.go`, add a helper near the top (after imports):

```go
// fmtLines renders an added/removed pair compactly, e.g. "+5,054 -324".
func fmtLines(added, removed int64) string {
	return fmt.Sprintf("+%s -%s", component.HumanInt(added), component.HumanInt(removed))
}

// fmtActive renders a duration in seconds as a compact human string, e.g.
// "23m", "4h12m". Zero renders as "0m".
func fmtActive(sec int64) string {
	if sec <= 0 {
		return "0m"
	}
	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
```

(`time` and `component` are already imported in this file.)

- [ ] **Step 2: Add productivity rows to renderWindowCard**

In `renderWindowCard`, after the existing `writeKV("cost", ...)` and the `errors` row, add three rows. Insert before the `errors` row so errors stays last:

```go
	writeKV("cost", fmt.Sprintf("$%.2f", ws.CostUSD))
	writeKV("lines", fmtLines(ws.LinesAdded, ws.LinesRemoved))
	writeKV("commits", fmt.Sprintf("%d", ws.Commits))
	writeKV("active", fmtActive(ws.ActiveSec))
```

> The `labelCol` width is 10; "commits" (7) and "active" (6) fit. If `pull_requests` should show, append `writeKV("PRs", fmt.Sprintf("%d", ws.PullRequests))` — optional; the spec's mock folds PRs next to commits. Keep it to commits-only for the first cut unless the card has vertical room.

- [ ] **Step 3: Verify build + visual smoke test**

Run: `go build ./... && go test ./internal/tui/... `
Expected: PASS. Then optionally `make run` and eyeball the dashboard cards for the new `lines / commits / active` rows.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/dashboard/view.go
git commit -m "feat(dashboard): productivity rows (lines, commits, active) in window cards

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 11: readstore — lines on `SessionRow`

**Files:**
- Modify: `internal/tui/readstore/queries.go:14-90` (SessionRow + SessionsPage)
- Test: `internal/tui/readstore/queries_test.go` (extend SessionsPage test)

- [ ] **Step 1: Add fields to SessionRow**

```go
type SessionRow struct {
	SessionID    string
	ProjectName  string
	StartedAt    time.Time
	LastSeenAt   time.Time
	EndedAt      time.Time
	DurationSec  int64
	CostUSD      float64
	Prompts      int64
	Tokens       int64
	LinesAdded   int64
	LinesRemoved int64
	Live         bool
}
```

- [ ] **Step 2: Extend the SessionsPage test**

In the `SessionsPage` test, seed `lines_added`/`lines_removed` on the inserted sessions and assert `out[0].LinesAdded`/`LinesRemoved` match. Run:

`go test ./internal/tui/readstore/ -run SessionsPage -v`
Expected: FAIL — fields are zero / scan mismatch.

- [ ] **Step 3: Extend the SessionsPage query + scan**

In `SessionsPage`, add the two columns to the `SELECT` (after `tokens`):

```sql
       input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens AS tokens,
       lines_added,
       lines_removed
```

And extend the `rows.Scan(...)` call to append `&r.LinesAdded, &r.LinesRemoved` at the end (matching SELECT order):

```go
		if err := rows.Scan(&r.SessionID, &r.ProjectName, &started, &lastSeen, &ended,
			&r.CostUSD, &r.Prompts, &r.Tokens, &r.LinesAdded, &r.LinesRemoved); err != nil {
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/tui/readstore/ -run SessionsPage -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/readstore/queries.go internal/tui/readstore/queries_test.go
git commit -m "feat(readstore): lines added/removed on SessionRow

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 12: Sessions list — lines column

**Files:**
- Modify: `internal/tui/sessions/list.go`

- [ ] **Step 1: Read the current row renderer**

Run: `grep -n 'CostUSD\|Prompts\|Tokens\|func.*render\|columns\|lipgloss' internal/tui/sessions/list.go`
Read the function that renders one `readstore.SessionRow` into a line (column layout: project, cost, prompts, tokens, time).

- [ ] **Step 2: Add a lines cell**

Following the existing column pattern, add a fixed-width cell rendering `+<added> -<removed>` using the theme's green/red. Add a helper in `list.go`:

```go
// linesCell renders "+1.2k -340" with added in green, removed in red.
func linesCell(t *theme.Theme, added, removed int64) string {
	add := lipgloss.NewStyle().Foreground(t.Palette.Green).Render("+" + component.HumanInt(added))
	rem := lipgloss.NewStyle().Foreground(t.Palette.Red).Render("-" + component.HumanInt(removed))
	return add + " " + rem
}
```

Insert the cell into the row composition next to tokens, with a fixed width consistent with the other columns (match the width treatment the file already uses — e.g. `lipgloss.NewStyle().Width(n).Render(...)`). If the list has a header row, add a `lines` header label aligned to the same width.

> Match the exact palette field names used elsewhere in this file (`t.Palette.Green`/`t.Palette.Red` per dashboard usage). If the file uses a different accessor (e.g. a `costColor` helper), follow that instead. `component` and `theme` imports already present; add them if not.

- [ ] **Step 3: Build + test + smoke**

Run: `go build ./... && go test ./internal/tui/...`
Expected: PASS. Optionally `make run`, press `s`, eyeball the new column.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/sessions/list.go
git commit -m "feat(sessions): lines added/removed column in the list view

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

# Phase 3 — Session detail card

### Task 13: readstore — `SessionHeader` productivity fields

The session detail view needs per-session productivity totals + edit accept-rate inputs. Check whether a session-header query already exists; if so extend it, else add `SessionHeader`.

**Files:**
- Modify: `internal/tui/readstore/queries.go`
- Test: `internal/tui/readstore/queries_test.go`

- [ ] **Step 1: Locate the existing session-header read**

Run: `grep -n 'func SessionHeader\|func SessionMeta\|FROM sessions WHERE session_id' internal/tui/readstore/queries.go internal/tui/sessions/detail.go`
Determine how `detail.go` currently loads the session's top-line metadata (project, cost, tokens). If a struct/query exists, extend it; otherwise add the new one below.

- [ ] **Step 2: Write the failing test**

Add to `queries_test.go`:

```go
func TestSessionHeader_Productivity(t *testing.T) {
	db := newTestDB(t) // use the package's existing test-DB helper
	mustExec(t, db, `INSERT INTO sessions
		(session_id, started_at, last_seen_at, lines_added, lines_removed, commits, pull_requests, active_seconds, edits_accepted, edits_rejected)
		VALUES ('s1', 1000, 2000, 500, 30, 3, 1, 600, 47, 3)`)
	h, err := SessionHeader(context.Background(), db, "s1")
	if err != nil {
		t.Fatalf("SessionHeader: %v", err)
	}
	if h.LinesAdded != 500 || h.Commits != 3 || h.EditsAccepted != 47 || h.EditsRejected != 3 {
		t.Errorf("got %+v", h)
	}
}
```

> Use whatever test-DB/seed helpers the file already provides (`newTestDB`, `mustExec`, or inline `Open(t.TempDir())`). Match the existing test style in this file.

Run: `go test ./internal/tui/readstore/ -run SessionHeader -v`
Expected: FAIL — undefined (or missing fields).

- [ ] **Step 3: Add/extend SessionHeader**

If none exists, add:

```go
// SessionHeader is the per-session summary shown atop the detail view.
type SessionHeader struct {
	SessionID     string
	ProjectName   string
	LinesAdded    int64
	LinesRemoved  int64
	Commits       int64
	PullRequests  int64
	ActiveSec     int64
	EditsAccepted int64
	EditsRejected int64
}

// SessionHeaderRow loads the productivity summary for one session. Returns
// ErrNotFound when the session does not exist.
func SessionHeaderRow(ctx context.Context, db *sql.DB, sessionID string) (SessionHeader, error) {
	const q = `
SELECT session_id, COALESCE(project_name, ''),
       lines_added, lines_removed, commits, pull_requests,
       active_seconds, edits_accepted, edits_rejected
FROM sessions WHERE session_id = ?`
	var h SessionHeader
	err := db.QueryRowContext(ctx, q, sessionID).Scan(
		&h.SessionID, &h.ProjectName,
		&h.LinesAdded, &h.LinesRemoved, &h.Commits, &h.PullRequests,
		&h.ActiveSec, &h.EditsAccepted, &h.EditsRejected,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return SessionHeader{}, ErrNotFound
	}
	if err != nil {
		return SessionHeader{}, fmt.Errorf("session header: %w", err)
	}
	return h, nil
}
```

> If a session-header read already exists, instead add the 7 productivity columns to that struct + query + scan, and name the test accordingly. Pick one approach and keep names consistent.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/tui/readstore/ -run SessionHeader -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/readstore/queries.go internal/tui/readstore/queries_test.go
git commit -m "feat(readstore): per-session productivity header

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 14: Session detail — productivity card

**Files:**
- Modify: `internal/tui/sessions/detail.go`

- [ ] **Step 1: Read the detail view's card rendering**

Run: `grep -n 'component.Card\|func.*render\|costUSD\|InputTokens\|fetchCmd\|dataMsg' internal/tui/sessions/detail.go`
Identify (a) where the model fetches session data, (b) where it renders the existing cost/token summary card(s).

- [ ] **Step 2: Fetch the header in the detail model**

In the detail model's fetch command, call `readstore.SessionHeaderRow(ctx, pool, sessionID)` and store the result on the model (add a `header readstore.SessionHeader` field and populate it in the success message handler). Follow the existing fetch/dataMsg pattern in the file.

- [ ] **Step 3: Add a productivity card renderer**

Add a render function alongside the existing card renderers:

```go
// renderProductivityCard shows lines, commits/PRs, active time, and edit
// accept-rate for the session.
func (m *Model) renderProductivityCard(t *theme.Theme, width int) string {
	h := m.header
	acceptRate := ""
	total := h.EditsAccepted + h.EditsRejected
	if total > 0 {
		pct := float64(h.EditsAccepted) / float64(total) * 100
		acceptRate = fmt.Sprintf("%.0f%% (%d/%d)", pct, h.EditsAccepted, total)
	} else {
		acceptRate = "—"
	}

	var b strings.Builder
	writeKV := func(label, value string) {
		b.WriteString(t.Label.Render(lipgloss.NewStyle().Width(12).Render(label)) + t.Value.Render(value) + "\n")
	}
	writeKV("lines", fmt.Sprintf("+%s -%s", component.HumanInt(h.LinesAdded), component.HumanInt(h.LinesRemoved)))
	writeKV("commits", fmt.Sprintf("%d", h.Commits))
	writeKV("pull reqs", fmt.Sprintf("%d", h.PullRequests))
	writeKV("active", fmtActiveDur(h.ActiveSec))
	writeKV("edit accept", acceptRate)

	return component.Card(t, "productivity", strings.TrimRight(b.String(), "\n"), width)
}

// fmtActiveDur renders seconds as "23m" / "4h12m" / "0m".
func fmtActiveDur(sec int64) string {
	if sec <= 0 {
		return "0m"
	}
	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	mn := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, mn)
	}
	return fmt.Sprintf("%dm", mn)
}
```

> Match the field/method names this file actually uses for theme (`t.Label`, `t.Value`, `component.Card`) — they mirror the dashboard. If `detail.go` builds cards differently, adapt to its pattern; keep the accept-rate math identical. Ensure `time`, `strings`, `component`, `lipgloss` are imported.

- [ ] **Step 4: Place the card in the view**

In the detail `View`, add `m.renderProductivityCard(t, width)` into the section list next to the existing cost/token card (same width handling).

- [ ] **Step 5: Build + test + smoke**

Run: `go build ./... && go test ./internal/tui/...`
Expected: PASS. Optionally `make run`, open a session, confirm the productivity card and accept-rate.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/sessions/detail.go
git commit -m "feat(sessions): productivity card with edit accept-rate in session detail

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

# Phase 4 — Dedicated Productivity view (separable)

> This phase overlaps the dashboard cards and is the largest piece. It can be split into its own plan if P1–P3 ship first. Kept here per the spec.

### Task 15: readstore — `ProductivityByDay`

**Files:**
- Modify: `internal/tui/readstore/queries.go`
- Test: `internal/tui/readstore/queries_test.go`

- [ ] **Step 1: Write the failing test**

Add to `queries_test.go`:

```go
func TestProductivityByDay(t *testing.T) {
	db := newTestDB(t)
	// Two sessions on the same UTC day, one the next day.
	day1 := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC).UnixNano()
	day1b := time.Date(2026, 6, 13, 18, 0, 0, 0, time.UTC).UnixNano()
	day2 := time.Date(2026, 6, 14, 9, 0, 0, 0, time.UTC).UnixNano()
	mustExec(t, db, `INSERT INTO sessions (session_id, started_at, last_seen_at, lines_added, commits, edits_accepted, edits_rejected) VALUES
		('a', ?, ?, 100, 1, 8, 2),
		('b', ?, ?, 50,  0, 4, 0),
		('c', ?, ?, 30,  2, 1, 1)`, day1, day1, day1b, day1b, day2, day2)

	rows, err := ProductivityByDay(context.Background(), db, 30, time.UTC)
	if err != nil {
		t.Fatalf("ProductivityByDay: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 day rows, got %d", len(rows))
	}
	// Newest day first.
	if rows[0].LinesAdded != 30 || rows[0].Commits != 2 {
		t.Errorf("day2 row = %+v", rows[0])
	}
	if rows[1].LinesAdded != 150 || rows[1].Commits != 1 {
		t.Errorf("day1 row = %+v", rows[1])
	}
}
```

Run: `go test ./internal/tui/readstore/ -run ProductivityByDay -v`
Expected: FAIL — undefined.

- [ ] **Step 2: Implement the query**

Add to `queries.go`:

```go
// ProductivityDay is one calendar day's productivity totals.
type ProductivityDay struct {
	Day           string // "2006-01-02" in the given location
	Sessions      int64
	LinesAdded    int64
	LinesRemoved  int64
	Commits       int64
	PullRequests  int64
	ActiveSec     int64
	EditsAccepted int64
	EditsRejected int64
}

// ProductivityByDay returns up to `days` of per-day productivity totals,
// newest day first, grouped by the local calendar day of started_at. loc
// determines day boundaries (pass time.Local in the app, time.UTC in tests).
func ProductivityByDay(ctx context.Context, db *sql.DB, days int, loc *time.Location) ([]ProductivityDay, error) {
	if days <= 0 {
		days = 30
	}
	// Offset (seconds) from UTC for the chosen location, applied to the
	// nanosecond ts so SQLite's date() groups on local calendar days.
	_, offset := time.Now().In(loc).Zone()
	const q = `
SELECT
  strftime('%Y-%m-%d', (started_at/1000000000) + ?, 'unixepoch') AS day,
  COUNT(*),
  COALESCE(SUM(lines_added), 0),
  COALESCE(SUM(lines_removed), 0),
  COALESCE(SUM(commits), 0),
  COALESCE(SUM(pull_requests), 0),
  COALESCE(SUM(active_seconds), 0),
  COALESCE(SUM(edits_accepted), 0),
  COALESCE(SUM(edits_rejected), 0)
FROM sessions
GROUP BY day
ORDER BY day DESC
LIMIT ?`
	rows, err := db.QueryContext(ctx, q, offset, days)
	if err != nil {
		return nil, fmt.Errorf("productivity by day: %w", err)
	}
	defer rows.Close()
	out := make([]ProductivityDay, 0, days)
	for rows.Next() {
		var d ProductivityDay
		if err := rows.Scan(&d.Day, &d.Sessions, &d.LinesAdded, &d.LinesRemoved,
			&d.Commits, &d.PullRequests, &d.ActiveSec, &d.EditsAccepted, &d.EditsRejected); err != nil {
			return nil, fmt.Errorf("productivity day scan: %w", err)
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("productivity day iter: %w", err)
	}
	return out, nil
}
```

> The `offset` shifts epoch seconds into local time before `strftime` extracts the day. This keeps grouping consistent with how the dashboard computes `startOfDay` in `now.Location()`. For fixed-offset zones this is exact; for zones with DST transitions within the window it can misattribute sessions near midnight on a transition day — acceptable for a per-day trend table.

- [ ] **Step 3: Run to verify pass**

Run: `go test ./internal/tui/readstore/ -run ProductivityByDay -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/readstore/queries.go internal/tui/readstore/queries_test.go
git commit -m "feat(readstore): ProductivityByDay per-day trend query

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 16: Productivity view — model + view

Build a new `app.View` mirroring the structure of `internal/tui/dashboard` (Model with pool/theme/now, fetch on Init/Tick, table render).

**Files:**
- Create: `internal/tui/productivity/doc.go`, `internal/tui/productivity/model.go`, `internal/tui/productivity/view.go`
- Test: `internal/tui/productivity/view_test.go`

- [ ] **Step 1: Read the dashboard model as the template**

Re-read `internal/tui/dashboard/model.go` (Model, New, Init, Update, fetchCmd, Title, ShortHelp, Status) and `internal/tui/app/messages.go` (TickMsg, ErrMsg, View interface) to copy the lifecycle exactly.

- [ ] **Step 2: Create doc.go**

```go
// Package productivity renders a per-day productivity trend table (lines,
// commits, PRs, active time, edit accept-rate) sourced from readstore.
package productivity
```

- [ ] **Step 3: Create model.go**

Mirror dashboard's Model. Key shape:

```go
package productivity

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

const fetchTimeout = 500 * time.Millisecond

var errNoPool = errors.New("productivity: no read pool")

type dataMsg struct {
	days []readstore.ProductivityDay
	at   time.Time
}

type Model struct {
	pool     *sql.DB
	theme    *theme.Theme
	days     []readstore.ProductivityDay
	cursor   int
	inFlight bool
	stale    bool
	now      func() time.Time
}

func New(pool *sql.DB, th *theme.Theme) *Model {
	return &Model{pool: pool, theme: th, now: time.Now}
}

func (m *Model) Init() tea.Cmd { m.inFlight = true; return m.fetchCmd() }

func (m *Model) Update(msg tea.Msg) (app.View, tea.Cmd) {
	switch v := msg.(type) {
	case app.TickMsg:
		if m.inFlight {
			return m, nil
		}
		m.inFlight = true
		return m, m.fetchCmd()
	case dataMsg:
		m.days = v.days
		if m.cursor >= len(m.days) {
			m.cursor = 0
		}
		m.inFlight = false
		m.stale = false
		return m, nil
	case app.ErrMsg:
		m.inFlight = false
		m.stale = true
		return m, nil
	case tea.KeyMsg:
		switch {
		case v.Type == tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
		case v.Type == tea.KeyDown:
			if m.cursor < len(m.days)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m *Model) Title() string { return "PRODUCTIVITY" }

func (m *Model) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

func (m *Model) Status() component.Status {
	if m.stale {
		return component.StatusStale
	}
	return component.StatusLive
}

func (m *Model) fetchCmd() tea.Cmd {
	pool := m.pool
	now := m.now
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		days, err := readstore.ProductivityByDay(ctx, pool, 30, time.Now().Location())
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return dataMsg{days: days, at: now()}
	}
}
```

> Confirm the exact `app.View` interface (method set) and `component.Status` constants by reading `internal/tui/app/messages.go` / `app.go`; adjust method signatures (especially `View(width, height int) string`) to match. The dashboard implements the same interface, so copy its method set verbatim.

- [ ] **Step 4: Create view.go**

```go
package productivity

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

var fallbackTheme = func() *theme.Theme { t := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs()); return &t }()

func (m *Model) th() *theme.Theme {
	if m.theme != nil {
		return m.theme
	}
	return fallbackTheme
}

func (m *Model) View(width, height int) string {
	if width <= 0 {
		width = 80
	}
	t := m.th()
	if len(m.days) == 0 {
		return component.Card(t, "productivity (last 30 days)", t.Muted.Render("(no data yet)"), width)
	}
	var b strings.Builder
	header := fmt.Sprintf("%-12s %-14s %-8s %-7s %s", "day", "lines", "commits", "active", "accept")
	b.WriteString(t.Label.Render(header) + "\n")
	for i, d := range m.days {
		line := fmt.Sprintf("%-12s %-14s %-8d %-7s %s",
			d.Day,
			fmt.Sprintf("+%s -%s", component.HumanInt(d.LinesAdded), component.HumanInt(d.LinesRemoved)),
			d.Commits,
			fmtActiveDur(d.ActiveSec),
			acceptRate(d.EditsAccepted, d.EditsRejected),
		)
		if i == m.cursor {
			line = lipgloss.NewStyle().Foreground(t.Palette.Base).Background(t.Palette.Text).Render(line)
		} else {
			line = t.Value.Render(line)
		}
		b.WriteString(line + "\n")
	}
	return component.Card(t, "productivity (last 30 days)", strings.TrimRight(b.String(), "\n"), width)
}

func fmtActiveDur(sec int64) string {
	if sec <= 0 {
		return "0m"
	}
	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	mn := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, mn)
	}
	return fmt.Sprintf("%dm", mn)
}

func acceptRate(acc, rej int64) string {
	total := acc + rej
	if total == 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f%%", float64(acc)/float64(total)*100)
}
```

> Verify `t.Palette.Base`/`t.Palette.Text` exist (read `internal/tui/theme/palette.go`); if the selection highlight in other views uses a specific style (e.g. `t.Selected`), use that instead for consistency.

- [ ] **Step 5: Write a render smoke test**

Create `internal/tui/productivity/view_test.go`:

```go
package productivity

import (
	"strings"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

func TestView_RendersDayRows(t *testing.T) {
	m := New(nil, nil)
	m.days = []readstore.ProductivityDay{
		{Day: "2026-06-14", LinesAdded: 1200, LinesRemoved: 340, Commits: 3, ActiveSec: 1500, EditsAccepted: 9, EditsRejected: 1},
	}
	out := m.View(100, 40)
	if !strings.Contains(out, "2026-06-14") {
		t.Errorf("expected day in output, got:\n%s", out)
	}
	if !strings.Contains(out, "90%") {
		t.Errorf("expected accept-rate 90%%, got:\n%s", out)
	}
}

func TestView_EmptyState(t *testing.T) {
	m := New(nil, nil)
	if out := m.View(100, 40); !strings.Contains(out, "no data") {
		t.Errorf("expected empty-state text, got:\n%s", out)
	}
}
```

- [ ] **Step 6: Build + test**

Run: `go build ./... && go test ./internal/tui/productivity/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/productivity/
git commit -m "feat(productivity): per-day trend view (model + view)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### Task 17: Wire the Productivity view into navigation

**Files:**
- Modify: `internal/tui/dashboard/model.go` (add a key to push the view)
- Modify: `internal/tui/dashboard/view.go:210-219` (help bar hint)

- [ ] **Step 1: Add a keybinding on the dashboard**

In `internal/tui/dashboard/model.go` `Update`, add a case (next to the `'s'` case) for `'p'` that pushes the productivity view:

```go
		case v.Type == tea.KeyRunes && len(v.Runes) == 1 && v.Runes[0] == 'p':
			pool := m.pool
			th := m.theme
			return m, func() tea.Msg {
				return app.PushViewMsg{V: productivity.New(pool, th)}
			}
```

Add the import `"github.com/kamikaze011001/claude-code-observer/internal/tui/productivity"`.

> Check for an import cycle: `productivity` imports `app` and `readstore` only (not `dashboard`), and `dashboard` already imports `sessions`, so importing `productivity` from `dashboard` is acyclic. Confirm `productivity` does NOT import `dashboard`.

- [ ] **Step 2: Add the help hint + ShortHelp binding**

In `renderHelpBar` (dashboard/view.go), add `{Key: "p", Desc: "productivity"}` to the hints slice. In `ShortHelp` (model.go), add `key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "productivity"))`.

- [ ] **Step 3: Build + test + smoke**

Run: `go vet ./... && go build ./... && go test ./...`
Expected: PASS. Then `make run`, press `p`, confirm the Productivity view opens and `esc` returns.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/dashboard/
git commit -m "feat(dashboard): 'p' opens the Productivity view

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Final verification (run after each phase, mandatory after Phase 4)

Per `CLAUDE.md`:

```bash
make vet     # fix vet issues first
make test    # fix failing tests
make build   # confirm it compiles
```

Then open a PR against `master` (do not merge locally without the user's say-so):

```bash
gh pr create --base master --title "feat: productivity metrics (lines, commits, PRs, active time, edit accept-rate)" \
  --body "Implements docs/superpowers/specs/2026-06-15-productivity-metrics-design.md. See plan docs/superpowers/plans/2026-06-15-productivity-metrics.md."
```

---

## Self-Review (completed by plan author)

**Spec coverage:** ✅ 7 columns (Task 1), metric rollup + 5 handlers w/ delta-SUM (Tasks 3–4), ingest wiring (Tasks 5–6), rebuild replay (Task 7), doc correction (Task 8), dashboard cards (Tasks 9–10), sessions list column (Tasks 11–12), session detail card + accept-rate (Tasks 13–14), dedicated view (Tasks 15–17). Cost/token explicitly NOT rolled up — guarded by `TestMetric_CostAndTokenNotRolledUp`. SUM-not-MAX guarded by `TestInsertMetricsAndApplyRollups_SumsDeltaSnapshots` and the non-monotonic series. `active_time` user-only guarded by `TestMetric_ActiveTime_UserOnly`.

**Placeholder scan:** No TBD/TODO. View tasks (12, 14, 16, 17) instruct reading the current file first because lipgloss layout is pattern-bound; they still ship concrete, complete helper code and exact data wiring.

**Type consistency:** `sessionCounters` field names (`LinesAdded`, `LinesRemoved`, `Commits`, `PullRequests`, `ActiveSeconds`, `EditsAccepted`, `EditsRejected`) used identically in Tasks 2 and 4. `WindowStats`/`SessionRow`/`SessionHeader`/`ProductivityDay` field names consistent across readstore + view tasks. `ApplyMetric`/`metricUpdaters`/`MetricUpdater` consistent across Tasks 3–7. Column names match the migration in Task 1 throughout.

**Known soft spots (flagged inline for the implementer):**
- `active_time.total` unit assumed to be **seconds**; confirm against a live datapoint during Task 4 — if it is milliseconds, divide by 1000 in `applyActiveTime`.
- `code_edit_tool.decision` attr value assumed `accept`/`reject`; if the live data uses `accepted`/`rejected`, adjust the `switch` in `applyCodeEditDecision` (and its test).
- Exact theme accessor names (`t.Palette.Green/Red/Base/Text`, `t.Selected`) must be confirmed against `internal/tui/theme/palette.go` during view tasks.
