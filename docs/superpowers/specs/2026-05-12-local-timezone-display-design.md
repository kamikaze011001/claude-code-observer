# Local Timezone Display

**Date:** 2026-05-12
**Status:** Approved, ready for implementation plan
**Scope:** Pre-ship bug fix

## Problem

OTLP timestamps are stored as UTC nanoseconds (correct). The TUI renders them in UTC, but users run `cco` on machines in their local timezone. A user in GMT+7 sees session times offset by 7 hours, and the "today / 7-day / 30-day" rollup windows roll over at UTC midnight (07:00 local), so "today" is wrong for the first 7 hours of each local day.

## Root cause

Storage layer is fine. Two display-layer issues:

1. **Formatting** — readstore and dashboard convert `time.Unix(0, ts)` to UTC before handing the `time.Time` to renderers:
   - `internal/tui/readstore/queries.go:70-73, 311, 395, 397, 421` — `.UTC()`
   - `internal/tui/dashboard/view.go:155, 182` — `.UTC()`
2. **Day windowing** — "today" computed in UTC:
   - `internal/tui/readstore/queries.go:124, 240` — `time.Date(..., time.UTC)`

Renderers (`internal/tui/theme/component/row.go`, `internal/tui/prompt/detail.go`) call `.Format("15:04:05")` etc. on whatever they receive; they're correct and untouched.

## Design

Use the machine's local timezone (`time.Local`) at the display boundary. Storage and ingest are unchanged.

### Changes

| File | Change |
|------|--------|
| `internal/tui/readstore/queries.go` | Replace 6 `.UTC()` calls with `.Local()` (lines 70-73, 311, 395, 397, 421). Replace `time.UTC` with `time.Local` in `time.Date(...)` at lines 124 and 240. |
| `internal/tui/dashboard/view.go` | Replace 2 `.UTC()` calls with `.Local()` (lines 155, 182). |

### Why `time.Local`

- `cco` is a single-user local CLI — the machine's clock is the right answer.
- Go's `time.Local` already honors the `TZ` env var, so any user who wants a different zone (e.g., on a server in a different region) can set `TZ=Asia/Bangkok cco` with no code change.
- No `--timezone` flag, no config field — YAGNI for ship.

### Day-boundary semantics

After the fix, `startOfDay` for a user in GMT+7 at 2026-05-12 02:00 local is `2026-05-12T00:00:00+07:00` = `2026-05-11T17:00:00Z`. Sessions started between 17:00Z yesterday and now count as "today". 7d and 30d windows shift accordingly.

### Tests

- Existing readstore tests inject `now` — they continue to work because `time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)` uses the injected calendar fields, not wall time.
- Add a focused test in `internal/tui/readstore/queries_test.go` (or extend an existing one) that:
  - Injects `now` in a non-UTC location (e.g., `time.FixedZone("GMT+7", 7*3600)`).
  - Inserts events spanning the local-midnight boundary.
  - Asserts the "today" count includes events after local midnight and excludes events before.
- No new tests needed for the `.UTC()` → `.Local()` formatting swap — it's a one-character behavioral change and visible in any manual run.

### Verification

Per CLAUDE.md, after the change run in order:

1. `make vet`
2. `make test`
3. `make build`
4. Manual smoke: `./bin/claude-code-observer` — confirm event timestamps in dashboard, sessions list, session detail, and prompt detail match local wall-clock time.

## Out of scope

- Persisted timezone preference / `--timezone` flag.
- Showing TZ abbreviation in row columns (rejected during brainstorm — column width cost not worth it for a local CLI).
- Migrating any stored data — storage is already correct.
- Ingest-side timezone handling — receiver continues to accept whatever the OTLP exporter sends and normalize to UTC nanos.
