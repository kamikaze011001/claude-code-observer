# Productivity Metrics — Design Spec

**Date:** 2026-06-15
**Status:** Approved (brainstorm) — pending implementation plan
**Branch:** `feat/productivity-metrics`

## Problem

Claude Code exports productivity telemetry that `cco` already ingests into the
`metric_snapshots` table but never surfaces:

| Metric | Meaning |
|---|---|
| `claude_code.lines_of_code.count` | Lines added / removed (`type` attr) |
| `claude_code.commit.count` | Git commits Claude made |
| `claude_code.pull_request.count` | PRs created |
| `claude_code.active_time.total` | Engaged time (`type=user`/`cli`) |
| `claude_code.code_edit_tool.decision` | Edit accept / reject (`decision`, `tool_name`, `language`) |

The TUI today is entirely about **cost + tokens + tool counts**. These five
signals add an **output / productivity** dimension — arguably the most
*interesting* per-user information ("Claude wrote 5,054 lines, made 8 commits and
2 PRs in 23 min of active time today"). Nothing in the app reads
`metric_snapshots`; it is write-only and pruned after retention.

## Validated facts (grounded in the live DB, 2026-06-15)

These were confirmed by querying the running daemon's database, not assumed:

1. **All 8 metrics are present**, and `session.id` is populated on **every** row
   (zero NULLs) — per-session attribution works for all of them.
2. **Temporality is DELTA, not cumulative.** Within one session, `lines_added`
   snapshots go `156 → 633 → 201 → 11 → 28 …` (non-monotonic ⇒ not cumulative).
   The correct aggregation is **`SUM(value)`**, never `MAX`.
3. **Cross-check proves it:** summing the `cost.usage` metric per session equals
   `sessions.cost_usd` (event-derived) *to the cent*, and `token.usage` input
   sum equals `sessions.input_tokens` *exactly*. Since the event-derived totals
   are known-correct, delta-`SUM` of the metrics is correct.
4. **`lines_of_code.count`** carries only `type` (added/removed) on the current
   Claude Code version (2.1.157/158) — **no `language` or `model` attr yet**
   (those are newer). So no per-language breakdown is available; do not design
   around it.
5. **`active_time.total`**: use **`type=user`** only. The `type=cli` variant
   reports implausibly large values (process-uptime-like) and is excluded.
6. **`code_edit_tool.decision`** carries `decision` (accept/reject), `tool_name`,
   `language`, `source` — enough for an overall accept-rate.

## Architecture — one foundation, four views

```
metric_snapshots (raw, already ingested, delta values)
        │  NEW: metric rollup (additive, mirrors event rollup)
        ▼
sessions table  ← +7 productivity columns
        │
        ├── Dashboard window cards  (SUM across sessions in TODAY / 7D / 30D)
        ├── Sessions list           (per-row column)
        ├── Session detail card     (per-session)
        └── Productivity view (new) (sessions grouped by day → trends)
```

The foundation mirrors the existing **event** rollup so it is `cco rebuild`-able
and survives retention pruning of raw snapshots (the `sessions` aggregate table
is never pruned).

## Data model — 7 new columns on `sessions`

New migration `0003_sessions_productivity.sql`:

```sql
ALTER TABLE sessions ADD COLUMN lines_added     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN lines_removed   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN commits         INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN pull_requests   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN active_seconds  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN edits_accepted  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN edits_rejected  INTEGER NOT NULL DEFAULT 0;
```

## Data flow — new metric rollup

Mirrors the event rollup (`internal/rollup/`), which uses an `updaters`
map (`event_name → Updater func(domain.Event) []Op`) applied inline at ingest
inside one transaction with additive `col = col + excluded.col` upserts.

1. **New parallel registry** in `internal/rollup/`:
   `metricUpdaters map[string]MetricUpdater`,
   `MetricUpdater func(domain.MetricSnapshot) []Op`, dispatched by metric name.

2. **Handlers** (additive upsert into the new columns):
   - `lines_of_code.count`: `type=added → lines_added += v`; `type=removed → lines_removed += v`
   - `commit.count → commits += v`
   - `pull_request.count → pull_requests += v`
   - `active_time.total`: **`type=user` only** `→ active_seconds += round(v)`
   - `code_edit_tool.decision`: `decision=accept → edits_accepted += v`; `decision=reject → edits_rejected += v`
   - Cost/token/session-count metrics: **no handler** (cost/tokens already come
     from events; rolling them up here would double-count).

3. **Ingest wiring:** add `InsertMetricsAndApplyRollups(ctx, snaps)` to the
   repository (one tx: insert snapshots + apply metric rollup ops), and have
   `service.IngestMetrics` call it instead of the insert-only path. The additive
   upsert creates the `sessions` row if absent (using the snapshot `ts` for
   `started_at`/`last_seen_at`), exactly as the event counter upsert does.

4. **Rebuild:** extend `RebuildRollups` with a second replay pass — after
   replaying `events`, stream `metric_snapshots ORDER BY ts ASC, id ASC` and
   apply metric rollups. Delta + additive ⇒ replay re-sums correctly. Update the
   rebuild summary output to include the new columns.

5. **Upsert plumbing:** add the 7 fields to the `sessionCounters` struct,
   `sessionCounterUpsert` INSERT column list + `ON CONFLICT DO UPDATE` clause,
   and `sessionCounterArgs` positional slice (or introduce a dedicated
   productivity upsert if cleaner — decide during planning).

## The four surfaces

### 1. Dashboard window cards
Add a productivity row to each TODAY / 7D / 30D card:

```
 TODAY            7 DAYS           30 DAYS
 $12.40 · 42 ⌁    $88 · 310 ⌁      $402 · 1.2k ⌁
 +5,054 −324 ln   +21k −3k ln      +90k −12k ln
 8 commits 2 PR   31 cmt 6 PR      120 cmt 22 PR
 ⏱ 23m active     ⏱ 4h12m          ⏱ 18h
```
Extend `DashboardSnapshot` window aggregation to `SUM` the new columns.

### 2. Sessions list
One compact column: `+5054/−324` (added green, removed red). Read the two
columns in the existing `SessionsPage` query (already selects from `sessions`).

### 3. Session detail card
A productivity block beside the existing cost/token cards: lines ±, commits,
PRs, active time, and **edit accept-rate** (`94% · 47/50`).

### 4. Productivity view (new top-level tab)
Per-day trend table (optionally sparklines): `date · lines ± · active · commits
· accept-rate`, grouped from `sessions` by start-day. Reuses window logic.
Largest piece; overlaps the dashboard cards. **Included in this spec but is the
last phase and may be split into its own spec/plan if P1–P3 land first.**

## Error handling
- Unknown metric name or missing/extra attrs ⇒ handler returns no ops (ignore,
  consistent with `noopUpdater`); never error the ingest path.
- A metric arriving before any event for its session creates a minimal session
  row via upsert; later events fill metadata (same as today's counter upserts).
- Non-numeric / negative deltas: clamp at parse (values are counts/seconds).

## Testing
- **Table-driven** tests per metric handler (snapshot attrs → expected Ops).
- **Rollup integration:** feed synthetic snapshots, assert session columns;
  explicit regression guard that aggregation is **`SUM`, not `MAX`** (feeds a
  non-monotonic delta series and asserts the total is the sum).
- **Rebuild:** ingest → `RebuildRollups` → identical productivity totals.
- **Readstore:** per-window and per-session productivity queries.
- **Cross-validation (optional, doc'd):** the cost/token metric-sum == event
  totals check used during design, as a sanity script.

## Phasing
- **P1 — Foundation:** migration + metric rollup + ingest wiring + rebuild +
  tests. No UI; verifiable via `cco rebuild-rollups` + SQL.
- **P2 — Dashboard cards + Sessions list column** (cheap reads on new columns).
- **P3 — Session detail card.**
- **P4 — Dedicated Productivity view** (largest; separable).

## Doc update (folded into P1)
Refresh `docs/CLAUDE-CODE-OTEL.md`:
- Correct the temporality note: these counters are **delta**; aggregate with
  `SUM`. (Prior assumptions implied cumulative.)
- Note the productivity metrics are now consumed by the rollup.
- Optionally note newer attrs not present on the current CC version
  (`language`/`model` on lines_of_code) as future enhancements.

## Out of scope
- Per-language / per-model line breakdowns (attrs not emitted by current CC).
- Spend attribution by skill/plugin/agent (separate direction).
- Quality signals (`api_refusal`, retries, compaction timeline markers).
- Any change to how cost/tokens are derived (still event-based).
