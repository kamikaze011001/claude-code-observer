# Local Timezone Display Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render all TUI timestamps and day-window boundaries in the machine's local timezone (`time.Local`) instead of UTC, so a GMT+7 user sees correct wall-clock times and a "today" window aligned with local midnight.

**Architecture:** Storage and OTLP ingest remain unchanged (timestamps continue as UTC nanoseconds since epoch). The fix is confined to the display boundary: `internal/tui/readstore/queries.go` (8 sites) and `internal/tui/dashboard/view.go` (2 sites). `time.Local` honors the `TZ` env var, so users on servers can still override without a flag.

**Tech Stack:** Go 1.25 stdlib `time` package. Tests use existing `repository.Open` + `readstore.OpenRO` + SQLite fixture pattern from `internal/tui/readstore/queries_test.go`.

**Spec:** `docs/superpowers/specs/2026-05-12-local-timezone-display-design.md`

---

## Task 1: Failing test pinning local-midnight day-window semantics

This task locks in the *behavior* before changing any production code: when `now` is in a non-UTC zone, "today" must be the rows whose `started_at` is `>= local-midnight`, regardless of UTC midnight.

**Files:**
- Modify: `internal/tui/readstore/queries_test.go` (append new test at end of file)

- [ ] **Step 1: Append a new failing test**

Open `internal/tui/readstore/queries_test.go` and append at the end of the file:

```go
// TestRecentSessionsToday_LocalMidnight verifies that the "today" window
// is computed against local midnight, not UTC midnight. Regression guard
// for the timezone-display fix: a user in GMT+7 viewing the dashboard
// at 02:00 local on 2026-05-12 must see events between
// 2026-05-12T00:00+07:00 (= 2026-05-11T17:00Z) and now as "today".
func TestRecentSessionsToday_LocalMidnight(t *testing.T) {
	home := t.TempDir()
	repo, err := repository.Open(home)
	if err != nil {
		t.Fatalf("repository.Open: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	gmt7 := time.FixedZone("GMT+7", 7*3600)
	// 2026-05-12 02:00 local = 2026-05-11 19:00 UTC
	now := time.Date(2026, 5, 12, 2, 0, 0, 0, gmt7)
	// Local midnight 2026-05-12 = 2026-05-11 17:00 UTC
	localMidnight := time.Date(2026, 5, 12, 0, 0, 0, 0, gmt7)

	ins := func(id string, started time.Time) {
		_, err := repo.DB().ExecContext(context.Background(),
			`INSERT INTO sessions
			 (session_id, project_name, started_at, last_seen_at, ended_at,
			  cost_usd, prompts, tool_calls, api_errors,
			  input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens)
			 VALUES (?, ?, ?, ?, NULL, 0, 0, 0, 0, 0, 0, 0, 0)`,
			id, "obs", started.UnixNano(), started.UnixNano())
		if err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}

	// After local midnight — must be included.
	ins("after", localMidnight.Add(1*time.Hour))
	// Between UTC midnight (2026-05-12T00Z = 2026-05-12T07 local) and now
	// — not relevant for this case; skip.
	// Before local midnight (still UTC same day before noon UTC) — must be excluded.
	ins("before", localMidnight.Add(-2*time.Hour))

	pool, err := readstore.OpenRO(filepath.Join(home, "db.sqlite"))
	if err != nil {
		t.Fatalf("OpenRO: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	rows, err := readstore.RecentSessionsToday(context.Background(), pool, now, 10)
	if err != nil {
		t.Fatalf("RecentSessionsToday: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows: got %d want 1 (only the post-local-midnight row); rows=%+v", len(rows), rows)
	}
	if rows[0].SessionID != "after" {
		t.Errorf("session: got %q want %q", rows[0].SessionID, "after")
	}

	snap, _, err := readstore.DashboardSnapshot(context.Background(), pool, now)
	if err != nil {
		t.Fatalf("DashboardSnapshot: %v", err)
	}
	if got, want := snap.Today.Sessions, int64(1); got != want {
		t.Errorf("today sessions: got %d want %d", got, want)
	}
}
```

- [ ] **Step 2: Run the test and confirm it fails**

Run: `go test ./internal/tui/readstore/ -run TestRecentSessionsToday_LocalMidnight -v`

Expected: FAIL. Both assertions will fail because `startOfDay` is computed with `time.UTC`, so the "before" row (which is at `2026-05-11T15:00Z`, after `2026-05-11T00:00Z`) is incorrectly included.

- [ ] **Step 3: Commit the failing test**

```bash
git add internal/tui/readstore/queries_test.go
git commit -m "test(readstore): pin local-midnight semantics for today-window queries"
```

---

## Task 2: Switch day-window computation and timestamp display to local timezone

**Files:**
- Modify: `internal/tui/readstore/queries.go` (lines 70-73, 124, 240, 311, 395, 397, 421)
- Modify: `internal/tui/dashboard/view.go` (lines 155, 182)

- [ ] **Step 1: Update `internal/tui/readstore/queries.go` — `SessionsPage` scan loop (lines 70-73)**

Find:

```go
			r.StartedAt = time.Unix(0, started).UTC()
			r.LastSeenAt = time.Unix(0, lastSeen).UTC()
			if ended.Valid {
				r.EndedAt = time.Unix(0, ended.Int64).UTC()
```

Replace with:

```go
			r.StartedAt = time.Unix(0, started).Local()
			r.LastSeenAt = time.Unix(0, lastSeen).Local()
			if ended.Valid {
				r.EndedAt = time.Unix(0, ended.Int64).Local()
```

- [ ] **Step 2: Update `DashboardSnapshot` startOfDay (line 124)**

Find:

```go
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
```

Replace with:

```go
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
```

Rationale: using `now.Location()` (not the global `time.Local`) keeps tests deterministic when they inject a `now` in a fixed zone.

- [ ] **Step 3: Update `RecentSessionsToday` startOfDay (line 240)**

Find:

```go
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).UnixNano()
```

Replace with:

```go
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UnixNano()
```

- [ ] **Step 4: Update remaining `.UTC()` sites in queries.go (lines 311, 395, 397, 421)**

The remaining occurrences are inside event-row, prompt-row, and prompt-detail scan loops. Open the file and use a search-and-replace mindset: every `time.Unix(0, <var>).UTC()` becomes `time.Unix(0, <var>).Local()`. After this step, `grep -n "\.UTC()" internal/tui/readstore/queries.go` must return zero matches.

Specifically:

Line ~311 (event row in `EventsPage` or similar):

```go
		r.TS = time.Unix(0, ts).UTC()
```

→

```go
		r.TS = time.Unix(0, ts).Local()
```

Line ~395 (prompt row started):

```go
	p.StartedAt = time.Unix(0, started).UTC()
```

→

```go
	p.StartedAt = time.Unix(0, started).Local()
```

Line ~397 (prompt row ended):

```go
		p.EndedAt = time.Unix(0, ended.Int64).UTC()
```

→

```go
		p.EndedAt = time.Unix(0, ended.Int64).Local()
```

Line ~421 (event timestamp in prompt-detail loop):

```go
		ev := time.Unix(0, ts).UTC()
```

→

```go
		ev := time.Unix(0, ts).Local()
```

- [ ] **Step 5: Update `internal/tui/dashboard/view.go` (lines 155, 182)**

Find both occurrences:

```go
			Started:     time.Unix(0, ts.StartedAt).UTC(),
```

Replace each with:

```go
			Started:     time.Unix(0, ts.StartedAt).Local(),
```

After this, `grep -n "\.UTC()" internal/tui/dashboard/view.go` must return zero matches.

- [ ] **Step 6: Run vet**

Run: `make vet`
Expected: no output, exit code 0.

- [ ] **Step 7: Run the new test — must now pass**

Run: `go test ./internal/tui/readstore/ -run TestRecentSessionsToday_LocalMidnight -v`
Expected: PASS.

- [ ] **Step 8: Run the full readstore suite — pre-existing tests must still pass**

Run: `go test ./internal/tui/readstore/ -v`
Expected: PASS for all tests. The pre-existing `TestRecentSessionsToday` and `TestDashboardSnapshot*` tests inject `now` in `time.UTC`, so `now.Location() == time.UTC` and their behavior is unchanged.

- [ ] **Step 9: Run the full test suite**

Run: `make test`
Expected: PASS across all packages.

- [ ] **Step 10: Build**

Run: `make build`
Expected: produces `bin/claude-code-observer` with no errors.

- [ ] **Step 11: Commit**

```bash
git add internal/tui/readstore/queries.go internal/tui/dashboard/view.go
git commit -m "fix(tui): render timestamps and day windows in local timezone

Display layer converted UTC nanos to time.Time with .UTC(), and the
dashboard's today/7d/30d windows used UTC midnight. Users outside
UTC saw shifted clocks and a 'today' that rolled over at the wrong
instant. Switch the readstore + dashboard view to .Local() and use
now.Location() for day boundaries. Storage and ingest unchanged."
```

---

## Task 3: Manual smoke verification

**Files:** none.

- [ ] **Step 1: Launch the TUI**

Run: `./bin/claude-code-observer`

Expected:
- Session list rows show times matching the user's wall clock (e.g., for a GMT+7 user, a session that started "10 minutes ago" displays a time within the last 10 minutes of `date +%H:%M`).
- Session detail event timestamps match the wall clock.
- Prompt detail "started" time matches the wall clock.
- TODAY card on the dashboard reflects sessions started since *local* midnight.

If the user's machine is already in UTC, set `TZ=Asia/Bangkok ./bin/claude-code-observer` to spot-check the +07:00 path.

- [ ] **Step 2: Quit the TUI**

Press `q`. No additional commit — Task 3 is verification only.

---

## Self-Review Notes

- **Spec coverage:**
  - Spec §"Changes" table → Task 2 steps 1-5 (all 8 `.UTC()` and 2 `time.Date` sites).
  - Spec §"Tests" → Task 1 (new `FixedZone` test) + Task 2 step 8 (existing tests).
  - Spec §"Verification" → Task 2 steps 6, 9, 10 + Task 3 (manual smoke).
- **Placeholders:** none — every code change shown in full.
- **Type consistency:** all changes are method swaps (`.UTC()` → `.Local()`) and a constant swap (`time.UTC` → `now.Location()`) on the same `time.Time` API; no new types or signatures introduced.
- **Departure from spec:** the spec said `time.Local`; the plan uses `now.Location()` inside the two `time.Date` calls. This is a strictly safer choice — it gives identical behavior on real runs (where `now = time.Now()` has `Location() == time.Local`) and keeps test injection working. Worth flagging to the user.
