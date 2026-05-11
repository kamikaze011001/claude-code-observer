# Session Detail: Viewport Scrolling + Keyset Pagination

**Status:** Design — awaiting implementation
**Date:** 2026-05-11
**Owner:** TUI (`internal/tui/sessions/detail.go`)

## Problem

The session-detail timeline view has two independent defects that together make events past the visible terminal area unreachable:

1. **No viewport.** `Detail.View` renders every event in `m.events` into a single string. Bubble Tea does not scroll long output, so when the rendered body exceeds terminal height, rows past the bottom are clipped and the cursor moves invisibly off-screen.
2. **No keyset pagination.** `Detail.fetchCmd` calls `readstore.SessionEvents(ctx, pool, sid, nil, detailPageSize)` with a `nil` cursor on every refresh. Events older than the most recent page are unreachable from the TUI — the footer literally says `"older events available — keyset cursor not yet wired (use SQL)"`.

The combined effect: for any session with more events than fit on screen, the user can neither scroll the visible window nor reach older events.

## Goals

- Let the user navigate every event in a session from the TUI, regardless of session size.
- Preserve scroll position deterministically — the view must not jump under the user's hands when live data ticks in.
- Keep the change contained to `internal/tui/sessions/detail.go` plus its tests/goldens. No new dependencies.

## Non-goals

- Fixing the cosmetic column-header misalignment in this view (same family as the sessions-list bug fixed earlier; tracked separately).
- Adding viewport scrolling to dashboard, sessions list, or prompt detail (none currently need it).
- Live tail-while-scrolled UX (incoming events merged into a scrolled window). Auto-refresh is paused while scrolled.
- Search, filtering, or jump-to-prompt navigation.

## Design

### Approach

Hand-rolled viewport in `Detail`. Rejected alternatives:

- **`bubbles/viewport` library** — scrolls a pre-rendered string. We need row-level cursor selection, so the cursor↔offset translation would have to be bolted on anyway. Net negative.
- **Reusable `scrolllist` component** — premature. Only `Detail` needs it today.

### State

Three new fields on `Detail`:

| Field | Type | Purpose |
|---|---|---|
| `offset` | `int` | Index of the first event rendered in the visible window. |
| `viewport` | `int` | Last computed visible-row count; written by `View`, read by `Update` for page-step sizing. |
| `loadingOlder` | `bool` | Guards against double-fetch when `pgdn` is mashed at the bottom. |

Page size is reduced from `200` to **`50`** (`detailPageSize = 50`). Smaller pages mean snappier first paint and quicker pagination feedback; the previous 200 was an arbitrary "high enough to avoid pagination" guess that no longer applies once pagination works.

### Messages

`detailDataMsg` (existing) keeps its replace-list semantics for initial load and tick refresh. On replace, `m.offset` is reset to `0` alongside the existing cursor re-anchoring logic so a shorter-than-before result can never leave the view stuck below the loaded range.

New: `detailOlderMsg{events []readstore.EventRow, hasMore bool, at time.Time}` — appended to `m.events`. Since events are stored newest-first, older rows naturally append at the tail.

### View pipeline

1. `chromeReserved := 7` (chrome header + column header + footer hints + spacing).
2. `m.viewport = max(5, height - chromeReserved)`. Fallback to `20` when `height == 0` (pre-`WindowSizeMsg`).
3. Clamp `offset` so cursor stays visible and offset stays within bounds:
   - `if m.cursor < m.offset { m.offset = m.cursor }`
   - `if m.cursor >= m.offset + m.viewport { m.offset = m.cursor - m.viewport + 1 }`
   - `if m.offset < 0 { m.offset = 0 }`
   - `if m.offset > max(0, len(events)-1) { m.offset = max(0, len(events)-1) }` — guards against a stale offset after a `detailDataMsg` replace shrinks the list.
4. Render `events[m.offset : min(m.offset+m.viewport, len(events))]`.
5. Footer hint state machine:
   - `hasMore && !loadingOlder` → `"press pgdn for older events"`
   - `loadingOlder` → `"loading older events…"`
   - otherwise → hint suppressed.

### Key handling

| Key | Behavior |
|---|---|
| `↑` / `k` | `cursor--` (clamped ≥0). |
| `↓` / `j` | `cursor++` (clamped ≤ `len(events)-1`). |
| `pgup` | `cursor -= viewport` (clamped ≥0). |
| `pgdn` | `cursor += viewport` (clamped ≤ `len(events)-1`). If cursor now equals `len(events)-1` and `hasMore && !loadingOlder`, set `loadingOlder=true` and return `fetchOlderCmd()`. |
| `g` | `cursor = 0`; `offset = 0`. |
| `G` | `cursor = len(events)-1`. Does **not** trigger a fetch — offset slides via the View clamp. |
| `enter` | Unchanged — opens prompt detail when cursor is on a `user_prompt` row. |

### Tick refresh (auto-refresh policy)

Tick is suppressed when the user is scrolled or paginated. In `Update` on `app.TickMsg`:

```go
if m.inFlight || m.offset > 0 || len(m.events) > detailPageSize || m.loadingOlder {
    return m, nil
}
m.inFlight = true
return m, m.fetchCmd()
```

Refresh resumes once the user is back at offset 0 with no older pages loaded.

### Keyset pagination

`fetchOlderCmd`:

```go
func (m *Detail) fetchOlderCmd() tea.Cmd {
    pool, sid := m.pool, m.sessionID
    if len(m.events) == 0 {
        return nil
    }
    before := m.events[len(m.events)-1].TS.UnixNano()
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), detailFetchTimeout)
        defer cancel()
        if pool == nil {
            return app.ErrMsg{Err: errNoPool}
        }
        rows, hasMore, err := readstore.SessionEvents(ctx, pool, sid, &before, detailPageSize)
        if err != nil {
            return app.ErrMsg{Err: err}
        }
        return detailOlderMsg{events: rows, hasMore: hasMore, at: time.Now()}
    }
}
```

On `detailOlderMsg`: `events = append(events, v.events...)`; update `hasMore`; clear `loadingOlder`. Cursor and offset unchanged — user's selection stays put while the list grows below them.

### `ShortHelp` update

Add `pgdn`/`pgup` bindings so the footer strip surfaces them:

```go
return []key.Binding{
    m.keys.Up, m.keys.Down,
    m.keys.PgUp, m.keys.PgDn,
    m.keys.Enter,
    backKey, quitKey,
}
```

## Edge cases

| Situation | Behavior |
|---|---|
| `height == 0` (pre-`WindowSizeMsg`) | Viewport defaults to 20. |
| Empty session | "no events for this session" path unchanged. |
| Repeated `pgdn` while `loadingOlder` | Cursor moves within already-loaded rows; second fetch suppressed by flag. |
| `pgdn` at last loaded row, `!hasMore` | No fetch; cursor stays at bottom. |
| Tick fires at offset 0 with no pagination | Refresh runs as today; cursor re-anchors by `(TS, EventName)`. |
| `detailOlderMsg` arrives after the user pressed `g` | Older events appended; cursor stays at 0 (top) — no jump. |
| `app.ErrMsg` during pagination | `loadingOlder` cleared, `stale=true`. User can retry by pressing `pgdn` again. |

## Test plan

### Unit (`detail_test.go`)

- `TestDetail_PgDn_ScrollsCursorByViewport` — set `viewport=10`, `cursor=0`; pgdn → cursor=10.
- `TestDetail_PgDn_AtBottomTriggersFetchOlder` — `hasMore=true`, cursor at last row; pgdn returns a cmd whose msg type is `detailOlderMsg`; `loadingOlder=true` after the call.
- `TestDetail_PgDn_AtBottomNoFetchWhenLoading` — `loadingOlder=true`; pgdn returns nil cmd.
- `TestDetail_PgDn_AtBottomNoFetchWhenHasMoreFalse` — `hasMore=false`; pgdn returns nil cmd.
- `TestDetail_DetailOlderMsg_Appends` — initial events `[A,B,C]`, message with `[D,E]` → events `[A,B,C,D,E]`; `loadingOlder=false`; cursor unchanged.
- `TestDetail_Tick_SuppressedWhenScrolled` — `offset=5`; tick → nil cmd.
- `TestDetail_Tick_SuppressedWhenPaginated` — `len(events) > detailPageSize`; tick → nil cmd.
- `TestDetail_Tick_RunsAtTopWithOnePage` — `offset=0`, one page loaded; tick → fetch cmd.

### Golden (`testdata/`)

- `detail_mixed.golden` — unchanged (4 rows fit any height ≥ 12).
- `detail_scrolled.golden` — **new**. 30 generated events, `View(100, 12)`, `cursor=20`. Verifies that:
  - only the viewport-sized window is rendered,
  - the cursor row is the bold one,
  - footer reads `"press pgdn for older events"` when `hasMore=true`.

## File touch list

- `internal/tui/sessions/detail.go` — viewport state, key handlers, pagination cmd, message, tick guard, page size constant change.
- `internal/tui/sessions/detail_test.go` — unit tests listed above.
- `internal/tui/sessions/testdata/detail_scrolled.golden` — new golden.

No other files change. No new dependencies. No migrations.

## Out of scope (explicit)

- Header column alignment in the timeline view (cosmetic; deferred).
- Viewport in other views (dashboard, sessions list, prompt detail).
- Search / filter / jump-to-prompt navigation.
- Merging live-tailing events into a scrolled window.
