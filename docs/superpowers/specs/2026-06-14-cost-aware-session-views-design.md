# Cost-Aware Session Views — Design

**Date:** 2026-06-14
**Branch:** `feat/cost-aware-session-views`
**Status:** Approved (design) — pending implementation plan

## Problem

The Session Detail timeline is a flat, monochrome `time · event · summary` stream. Every
row — a user prompt, a 12ms file edit, a 3-cent API call — is rendered in the same muted
gray with equal weight. Per-call cost technically exists (the `api_request` summary already
reads `claude-opus-4-8 $0.0042`) but it is invisible: buried in the summary column, in the
same gray as everything else. Users cannot *feel* where money goes inside a session.

Cost also exists on the Sessions List (per-session column) and Prompt Detail (per-call
list), but in both places it is flat single-color text that does not draw the eye or
support comparison.

**Goal:** Restructure the Session Detail timeline so cost is tied visibly to *what was
asked*, and apply one consistent visual "cost language" across all three surfaces.

## Scope

Three surfaces, one shared cost-color scale:

1. **Session Detail timeline** — the centerpiece. Reframe from a flat event stream into a
   **turn-grouped** view (chosen direction "B").
2. **Sessions List** — add tier-colored cost + a proportional spend bar.
3. **Prompt Detail** — tier-color each call's cost, add a cumulative column + turn total.

Out of scope: changes to ingestion, rollup, retention, or the OTLP receiver. This is a
read-side / TUI-only change. The waterfall view is untouched.

## Shared Cost Color Scale

A single tiering function maps a USD amount to a theme color, used everywhere a per-call or
per-turn cost is rendered:

| Tier    | Range          | Color (Mocha) |
|---------|----------------|---------------|
| cheap   | `< $0.01`      | green         |
| notable | `$0.01–$0.05`  | yellow        |
| heavy   | `> $0.05`      | peach         |

- **Absolute tiers** (not relative-to-session) so users build intuition for real cents over
  time. Thresholds are constants in one place, tunable later.
- Lives as a small helper in the `theme/component` package (e.g. `CostStyle(t, usd)`),
  consumed by the timeline, sessions list, and prompt detail renderers.
- Session-level (non-cost) events render in the existing muted style with an em-dash `—`
  in the cost slot.

## Surface 1 — Session Detail Timeline (turn-grouped)

### Behavior

- The timeline is organized into **turns**. A turn = one user prompt plus the events that
  belong to it (`prompt_id` match).
- Each turn renders a **header row** carrying its rollup: start time, command/label, prompt
  length, duration, call count, and **total cost** (tier-colored).
- **Default expand state:** the most-recent turn is expanded (children visible); all older
  turns are collapsed to their one-line header.
- **Expand/collapse:** a turn header toggles inline (e.g. space / left-right or the existing
  up/down + an expand key — exact binding decided in the plan). Collapsed shows `▸`,
  expanded shows `▾`.
- **Child rows** (when expanded): `api_request` rows show model, tokens (in/out),
  tier-colored cost; `tool_result` rows show tool, ✓/✗, duration; drawn with tree
  connectors (`├ … ╰`).
- **Enter on a turn header** still pushes the full **Prompt Detail** view (cards +
  waterfall). Inline expansion is the lightweight peek; Prompt Detail is the deep dive.

### Session-level events (no `prompt_id`)

Auth, MCP connection, compaction, permission-mode change, hooks, etc. have no prompt.
They are **interleaved chronologically as ungrouped rows**, visually de-emphasized (muted,
em-dash in the cost slot), preserving the full session story rather than hiding it.

### Data

The `prompts` rollup table already carries everything a turn header needs
(`cost_usd`, `input/output/cache tokens`, `api_requests`, `tool_calls`, `started_at`,
`ended_at`, `prompt_length`, `command_name`). The child events come from the existing
`events` table by `prompt_id`. Session-level events are `events` rows with a NULL/empty
`prompt_id`.

New read-store query (name TBD, e.g. `SessionTurns`): returns, for a session, the ordered
list of turn headers (from `prompts`) merged with the session-level events (from `events`)
so the view can render one chronological sequence of {turn-header | ungrouped-event}. Child
events for an expanded turn are fetched via the existing per-prompt event query (lazy on
expand, or batched — decided in the plan based on row counts).

Keyset pagination (newest-first, "press d for older") is preserved at the **turn** level.

### Model changes

`internal/tui/sessions/detail.go` moves from a flat `[]EventRow` to a sequence of
renderable items: turn headers (collapsible, with an `expanded bool` + lazily-loaded
children) interleaved with ungrouped session events. Cursor navigation, the visible-window
offset logic, and stale/live status handling are preserved; only the item model and row
rendering change.

New renderers in `theme/component/row.go`: `TurnHeaderRow` (collapse glyph, label, rollup,
tier-colored total) and `TurnChildRow` (tree connector + api/tool detail). The existing
`EventRow` is reused for the ungrouped session-level rows (with the cost slot rendered as
`—`).

## Surface 2 — Sessions List

- Cost column becomes **tier-colored** via the shared scale.
- Add a `▏spend` **proportional bar** column, scaled to the most expensive session on the
  current page (relative within page, since the list is a comparison view). Cheapest
  sessions show an empty bar; the priciest shows a full bar.
- `internal/tui/theme/component/row.go::SessionRow` gains the bar; `formatColHeader` in
  `list.go` adds the `spend` column. Column-width math updated to keep
  `lipgloss.Width(out) == width`.
- The page max for bar scaling is computed in `list.go` from the fetched rows and passed
  into `SessionRowData`.

## Surface 3 — Prompt Detail

- The `api requests` card: each row's cost is **tier-colored**; add a **cumulative** column
  (running total down the list) and show the **turn total** in the card header.
- `APIRequestRow` / `APIRequestRowData` gain a cumulative field and use `CostStyle`.
- No layout restructure — the existing three summary cards + tool-calls card stay.

## Error / Edge Handling

- Zero-cost turns / calls render `$0.00` in the cheap (green) tier; never blank, so the
  column is always aligned.
- Session-level events always render `—` in the cost slot.
- A turn with no child events (e.g. prompt that errored before any api_request) expands to
  an empty/"(no calls)" body.
- Empty session (no prompts, only session-level events) renders the ungrouped rows alone.
- Narrow terminals: the spend bar and cumulative columns are the first to shrink/drop via
  the existing min-width clamps; cost text never truncates.
- Pruned/partial data (retention removed child events but kept the rollup): header still
  renders from the `prompts` row; expansion shows "(events pruned)".

## Testing

- **Cost tiering** (`CostStyle`): table-driven over boundary values
  ($0.0099, $0.01, $0.05, $0.0501, $0, negative-guard).
- **Renderers** (`TurnHeaderRow`, `TurnChildRow`, updated `SessionRow`, `APIRequestRow`):
  width-invariant assertions (`lipgloss.Width(out) == width`) at several widths, matching
  existing `row_test.go` patterns.
- **Read-store** (`SessionTurns` + cumulative computation): table-driven against an
  in-memory SQLite fixture, including interleaving of session-level events, collapsed-turn
  rollups, and keyset pagination boundaries — following `queries_test.go`.
- **Detail model**: expand/collapse toggling, default-latest-expanded, cursor preservation
  across refresh, Enter-opens-prompt-detail still fires.

## Decisions Captured

- Direction **B** (turn-grouped) over flat-cost-column (A) or spend-ledger (C). C may return
  later as an optional toggle.
- **Absolute** cost tiers, not share-of-session.
- Latest turn expanded by default; session events interleaved; Enter still opens full
  Prompt Detail.

## Non-Goals

- No new persisted columns or schema migration — everything derives from existing
  `prompts` and `events` tables.
- No spend-ledger (direction C) in this iteration.
- No changes to the dashboard, waterfall, or about views.
