# Subagent Waterfall View — Design

> Date: 2026-05-14
> Status: Approved, ready for implementation planning
> Source idea: `docs/FUTURE.md` → "Subagent waterfall view"

## Summary

A new full-screen TUI page, reachable from Prompt Detail, that renders every
`api_request` (and `api_error`) under a single prompt as a horizontal timeline.
Bars are positioned by start time and sized by duration, grouped into three
lanes — `main` / `subagent` / `auxiliary` — so the user can see where time went
within a complex prompt and how subagent activity interleaves with the main
loop.

## Scope and constraints

### What the data supports

Per `api_request` / `api_error` log event we have, in the `events.attrs` JSON
blob (already persisted — see "Data flow" below):

- `prompt.id` — links to the parent prompt (a first-class column)
- `query_source` — **free-form string** on log events. Observed values include
  `repl_main_thread`, `compact`, and subagent names. (Note: the three-value
  `main`/`subagent`/`auxiliary` form documented for *metrics* does not match the
  *log-event* surface — see "OTel doc correction" below.)
- `duration_ms` — elapsed request time
- `model`, `cost_usd`, `input_tokens`, `output_tokens`
- event timestamp `ts` — fired at stream-end, so **start = ts − duration_ms**

### What the data does NOT support

- **No per-instance subagent identifier on log events.** Two parallel subagents
  of the same type are indistinguishable. A true per-subagent flame graph would
  require beta traces (`CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1`) and the
  `agent_id` / `parent_agent_id` span attributes. Out of scope.
- No `subagent_dispatch` event — subagent activity is inferred from
  `query_source` only.
- `tool_result` has no `query_source`, so tool calls cannot be attributed to a
  subagent vs. the main loop. Tool calls are therefore **not** rendered in this
  view.

Accepted consequence: this is a **query_source-banded request timeline**, not a
nested per-subagent flame graph. It still answers "where did the time go in this
prompt."

## Architecture

### New package: `internal/tui/waterfall/`

A `Model` implementing the existing `app.View` interface
(`internal/tui/app/view.go`): `Init`, `Update`, `View`, `Title`, `ShortHelp`,
`Status`. Constructed via `waterfall.New(pool *sql.DB, promptID string, th *theme.Theme) app.View`,
mirroring `internal/tui/prompt.New`.

Suggested file layout (follow the `prompt/` package shape):

- `model.go` — the `Model` struct, `Init`/`Update`, fetch command
- `view.go` — `View()` rendering: lanes, bars, time axis, detail panel
- `layout.go` — pure functions: relative-offset computation, bar scaling,
  greedy lane packing, `query_source` bucketing
- `*_test.go` — table-driven tests for `layout.go`; golden-file tests for `View()`

### Readstore: new query

Add to `internal/tui/readstore/queries.go`:

```go
type WaterfallRequest struct {
    TS           time.Time // event timestamp (stream-end)
    DurationMS   int64
    QuerySource  string    // raw, free-form
    Model        string
    CostUSD      float64
    InputTokens  int64
    OutputTokens int64
    IsError      bool      // true when sourced from an api_error event
}

func PromptWaterfall(ctx context.Context, db *sql.DB, promptID string) ([]WaterfallRequest, error)
```

Selects `ts, event_name, attrs` from `events` where
`prompt_id = ? AND event_name IN ('api_request','api_error')`, ordered by `ts`,
parsing the listed fields out of the `attrs` JSON. Mirrors the existing
event-parsing loop in `PromptDetail`. Returns an empty slice (not an error) when
the prompt has no API requests.

### Entry point: Prompt Detail

In `internal/tui/prompt/detail.go`:

- Add key `w` to `ShortHelp()` and the in-`View()` help bar: `w` → "waterfall".
- In `Update`, on `tea.KeyMsg` matching `w`, return an `app.PushViewMsg{V:
  waterfall.New(d.pool, d.promptID, d.theme)}` command.

The `app` shell already handles `PushViewMsg` (push onto the view stack) and
`Back` (`b` pops). No shell changes needed.

## Data flow

1. `cco serve` ingests OTLP logs; `eventparser.Parse` keeps **all** record-level
   attributes in `Event.Attrs`, persisted to `events.attrs` as JSON. No parser
   or schema change is required — `query_source` and `duration_ms` are already
   stored.
2. User opens Prompt Detail, presses `w`.
3. `waterfall.Model.Init` issues `PromptWaterfall` on the read pool with a
   500 ms timeout (same pattern as `prompt.Detail.fetchCmd`).
4. On the resulting data message, the model computes layout and renders.
5. On `app.TickMsg` the model re-fetches (live refresh), like other views.

## The view

### Layout

```
 cco · prompt 1a2b3c4d                                    [● live]

 0ms ──────────────────────────────────────────────── 12 480ms

 main       ▓▓▓▓▓        ▓▓▓▓▓▓▓▓▓▓▓
 subagent        ▓▓▓▓▓▓▓▓▓▓        ▓▓▓▓▓▓
                          ▓▓▓▓▓▓▓▓▓▓▓▓
 auxiliary  (none)

 ┌ selected ─────────────────────────────────────────────────┐
 │ model        claude-opus-4-7      query_source  subagent   │
 │ duration     3 210 ms             cost          $0.0421    │
 │ tokens       in 1 240 / out 890   started        15:04:07  │
 └────────────────────────────────────────────────────────────┘

 [↑↓] select  [b] back  [r] refresh  [?] about  [q] quit
```

- **Time axis**: relative — `0ms` at left, total span (last bar end − first bar
  start) at right. A tick header line spans the content width.
- **Three fixed lanes**, always rendered top-to-bottom: `main`, `subagent`,
  `auxiliary`. An empty lane renders a single thin muted `(none)` row.
- **`query_source` bucketing** (pure function in `layout.go`):
  - `main`, `repl_main_thread` → **main** lane
  - `auxiliary`, `compact` → **auxiliary** lane
  - anything else (including any subagent name) → **subagent** lane
  - empty/missing `query_source` → **main** lane
  - The raw `query_source` value is preserved and shown in the detail panel.
- **Bars within a lane**: greedy interval-packed into sub-rows so overlapping
  requests (parallel subagents) don't collide. Packing is a pure function:
  sort by start, place each bar in the first sub-row whose last bar ends before
  this bar starts, else open a new sub-row.
- **Bar geometry**: `startCol = round(offsetMS × scale)`,
  `width = max(1, round(durationMS × scale))`, where
  `scale = contentWidth / totalSpanMS`. Bars use a lane color from the theme;
  `IsError` bars use the red accent.

### Interaction

- A flat slice of all bars, ordered by start time, backs a cursor index.
- `↑` / `↓` move the cursor; the selected bar is highlighted (bold / inverted).
- A `component.Card`-style "selected" panel below the lanes shows the selected
  request: `model`, raw `query_source`, `duration_ms`, `cost_usd`,
  input/output tokens, and wall-clock start time.
- Footer keys: `↑↓ select · b back · r refresh · ? about · q quit`.
- `Status()` returns live/stale/no-daemon using the same logic shape as
  `prompt.Detail` (live once data has loaded; stale on fetch error).

### Edge cases

| Case | Behavior |
|------|----------|
| No `api_request`/`api_error` events | Render a card: "no api requests for this prompt" |
| Missing/empty `query_source` | Bucket into `main` lane |
| Missing or zero `duration_ms` | Render a 1-column marker bar |
| All requests share a start time (clock resolution) | Packing still works; minimum 1-col widths apply |
| `totalSpanMS == 0` (single instant request) | `scale` guarded; render single 1-col bar at offset 0 |
| Very narrow terminal width | Lanes and axis clamp to a minimum content width (reuse the `width <= 0 → 90` guard pattern from `prompt.Detail`) |
| Prompt not found / pruned | "prompt not found — it may have been pruned" card |

## OTel doc correction (included in this work)

`docs/CLAUDE-CODE-OTEL.md` §8.2 (`api_request`) and §8.3 (`api_error`)
currently describe `query_source` as one of `main` / `subagent` / `auxiliary`.
That is the **metrics** cardinality bucketing. On **log events** — the surface
this tool consumes — `query_source` is a free-form string (`repl_main_thread`,
`compact`, or a subagent name). The implementation plan includes a step to
correct both sections so the doc matches the data the receiver actually sees.

## Testing

- **`layout.go` pure functions** — table-driven tests:
  - `query_source` bucketing (each known value, unknown value, empty string)
  - relative-offset computation (start = ts − duration)
  - bar scaling (normal span, zero span, sub-pixel durations clamped to 1 col)
  - greedy lane packing (no overlap, full overlap, partial overlap, identical
    starts)
- **`View()` golden-file tests** — matching the existing `*.golden` pattern in
  `prompt/` and `dashboard/`:
  - empty (no requests)
  - single lane only (`main` only)
  - all three lanes populated, with overlapping subagent bars
  - narrow terminal width
- **`PromptWaterfall` readstore test** — against a temp SQLite with seeded
  `api_request` + `api_error` events; assert ordering, error flag, and correct
  attribute parsing. Follows the existing `readstore` test setup.

## Out of scope

- Per-instance subagent separation (needs beta traces).
- Rendering `tool_result` events in the timeline (no `query_source` to attribute
  them).
- A session-level waterfall (this view is per-prompt only).
- Absolute clock-time axis.
