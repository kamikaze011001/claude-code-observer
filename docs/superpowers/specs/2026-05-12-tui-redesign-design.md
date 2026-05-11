# TUI Redesign — Catppuccin / playful direction

**Status:** Approved · ready for plan
**Date:** 2026-05-12
**Replaces:** the neo-brutalist theme locked in `docs/superpowers/specs/2026-05-10-phase-3-tui-shell-m3.1-design.md` §4

## 1 · Problem

The current TUI uses a "neo-brutalist" theme — thick borders, hot-yellow accent (`#FFD400`), ALL CAPS labels, single accent color, semantic red, adaptive black/white foreground. The user finds it ugly and wants the polished Charm-ecosystem aesthetic (glow / soft-serve / gh-dash / superfile / catppuccin) while keeping the existing UX and data model.

This spec covers the full redesign of all three TUI screens — dashboard, sessions list, session/prompt detail — plus the minimal backend additions required to make the new dashboard meaningful.

## 2 · Locked design decisions

| Axis | Choice | Notes |
|---|---|---|
| Personality | **superfile** — rounded borders, icon-rich, playful | Decided after comparing glow / gh-dash / soft-serve / superfile mockups |
| Icons | **Unicode/emoji default**, **Nerd Font opt-in** via flag | Auto-detection is unreliable; default must work everywhere |
| Palette | **Catppuccin Mocha** default; **Latte** auto when terminal is light | Override via `--theme` |
| Density | **Dense / info-rich** | Observability tool; user wants what's-happening at a glance |
| Scope | **All 3 screens at once** | Cohesive result, no mixed-look period |
| Old theme | **Replace fully** (delete neo-brutalist) | Single source of truth |
| Backend scope | **Path 2** — minimal SQL-only additions, no new tables | See §5 |
| Implementation approach | **Approach 2** — theme abstraction first, then redesign | Buys `--theme` / `--icons` for free; consistent components across screens |

## 3 · Package layout

```
internal/tui/theme/
├── palette.go          // Mocha + Latte (+ future flavors) as data
├── glyphs.go           // Unicode set + NerdFont set
├── theme.go            // Theme struct = Palette + Glyphs + derived lipgloss.Styles
├── select.go           // Resolve --theme / --icons / $COLORFGBG / env → Theme
└── component/
    ├── card.go         // rounded-border container w/ optional title
    ├── kpi.go          // big value, label, optional delta arrow + color
    ├── sparkline.go    // ▁▂▃▄▅▆▇█ from []float64 (reserved for future)
    ├── badge.go        // model badge (opus/sonnet/haiku → color)
    ├── status.go       // live / stale / no-daemon pill
    ├── row.go          // session row, prompt row, event row variants
    └── help.go         // footer key hints
```

Each view (`dashboard`, `sessions`, `prompt`) keeps its current Bubble Tea model + `Update`; `View()` shrinks to composing `component.*` calls. `Theme` is built once in `internal/tui/app/app.go` and threaded into models as a field. No globals.

## 4 · Theme data model

```go
type Palette struct {
    Bg, BgAlt, Fg, FgMuted    lipgloss.Color
    Accent                     lipgloss.Color  // brand pink
    Blue, Green, Yellow, Red   lipgloss.Color  // semantic + model badges
    Teal, Mauve                lipgloss.Color  // secondary highlights
}

type Glyphs struct {
    Brand        string   // "✦"   (Nerd: "")
    StatusOK     string   // "●"
    StatusWarn   string   // "●"   (rendered yellow)
    StatusErr    string   // "●"   (rendered red)
    Cursor       string   // "▸"
    DeltaUp      string   // "▲"
    DeltaDown    string   // "▼"
    DeltaFlat    string   // "─"
    Check        string   // "✓"
    Cross        string   // "✗"
    Spark        []rune   // []rune("▁▂▃▄▅▆▇█")
    Enter        string   // "⏎"
    BorderRound  lipgloss.Border  // lipgloss.RoundedBorder()
}

type Theme struct {
    Palette
    Glyphs
    // Derived styles — built once at construction
    Title, Subtitle, Muted, Accent, Value, Label  lipgloss.Style
    Card, CardTitle                                lipgloss.Style
    Help                                           lipgloss.Style
    BadgeOpus, BadgeSonnet, BadgeHaiku             lipgloss.Style
    PillLive, PillStale, PillNoDaemon              lipgloss.Style
}
```

**Resolution order** for `--theme` and `--icons` (first wins): CLI flag → env var (`CCO_THEME`, `CCO_ICONS`) → `$COLORFGBG` heuristic (theme only) → defaults (`mocha`, `unicode`).

## 5 · Backend additions (Path 2)

All changes are SQL-only in `internal/tui/readstore/queries.go`. No new tables, no new rollup pass.

| Change | Surface |
|---|---|
| `WindowStats` += `Sessions int64`, `Tokens int64` | Add `COUNT(*)` and `SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens)` to the snapshot query |
| `Snapshot` += `Yesterday WindowStats` | Same query, prior 24h window (`startOfDay - 24h` ≤ ts < `startOfDay`) |
| New `RecentSessionsToday(ctx, db, limit)` | Newest-first, today only — separate panel from existing top-3-by-cost |
| `SessionRow` += `Tokens int64` | `SELECT input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens` (already on `sessions` table) |

Out of scope (future milestone): per-session dominant model, 24h hourly activity histogram, model-mix card.

## 6 · Screens

### 6.1 Dashboard

Composition:
- **Header:** `✦ cco · dashboard` + breadcrumb + LIVE/STALE pill (from `LatestEventTS`)
- **Three side-by-side window cards** (today / 7d / 30d): `sessions`, `prompts`, `tokens`, `tools`, `cost`, `errors`
- **Delta strip:** today vs yesterday — sessions, prompts, tokens, cost — arrows colored by direction
- **Top sessions today (by cost) card:** existing top-3 — **read-only summary**, no cursor
- **Recent sessions card:** newest-first × 5 (new `RecentSessionsToday` query) — **cursor-selectable**, opens session detail on `⏎`
- **Help bar**

Only one cursor on the dashboard (in "recent sessions") — keeps the keymap simple and matches the existing single-cursor convention. The top-sessions card is a glanceable summary; users who want to act on it can press `s` for the full sessions list (already sorted by recency) and filter.

Responsive: at width < 80, the three window cards collapse to a vertical stack.

### 6.2 Sessions list (§5 in screen sequence)

Composition:
- **Header:** `✦ cco · sessions` + page indicator + LIVE pill
- **One full-width card** containing column header + `SessionRow` × page-size
- **Help bar**

Columns: `#`, `started`, `project`, `duration`, `cost`, `prompts`, `tokens`, `live`.

Selected row inverts background (`Style.Background(palette.BgAlt)`) instead of `▸` + bold. Page indicator in header. Keyset pagination, page size, and back-navigation behavior unchanged from current `internal/tui/sessions/list.go`.

### 6.3 Session detail — event timeline (§6 in screen sequence)

Composition:
- **Header:** `✦ cco · session <id…>` + one-line info strip (project · started · prompt count) + LIVE pill
- **One full-width card** containing column header + `EventRow` × visible window
- **Pagination hint** below card (`press pgdn for older events`)
- **Help bar**

Columns: `time`, `event`, `summary`.

User-prompt rows render with `Style.Background(palette.BgAlt)` — a subtle tint that gives clear visual scaffold for "where does each turn begin." Tool-result rows render `✓` (green) or `✗` (red) inline based on the `success` attr. API-request rows show model + cost + token shorthand in the summary. Cursor row inverts further on top of any tint.

Existing viewport/scroll logic (`internal/tui/sessions/detail.go`'s `clampOffset` + `visibleRows`) is preserved.

### 6.4 Prompt detail (§7 in screen sequence)

Composition:
- **Header:** `✦ cco · prompt <id…>` + LIVE pill
- **Info strip:** `session · started · duration · length` — one line, muted
- **Three side-by-side summary cards:** `cost` / `tokens` / `activity`
  - `cost`: `$X.XX` value + `N api requests` muted
  - `tokens`: `in / out / cache r / cache w`
  - `activity`: `api reqs`, `tool calls`, `errors` (error count colored red if > 0)
- **api requests card:** full-width list of `APIRequest` rows (time · model badge · cost · in/out tokens)
- **tool calls card:** full-width list of `ToolCall` rows (time · tool name · ✓/✗ · duration)
- **Help bar**

All values from existing `PromptDetailResult`. The "activity" card is the only new view — counts already exist on `Prompt` (`APIRequests`, `ToolCalls`, `HadError`).

## 7 · Alignment discipline (load-bearing)

Bordered cards + multi-column data make alignment failure modes more visible than the current flat layout. Three rules:

**7.1 Width is measured in visible cells, never bytes.**
- `lipgloss.Width(s)` for measurement (strips ANSI; `len(s)` does not)
- `github.com/mattn/go-runewidth.StringWidth` when truncating user content (project name, prompt text, summary) that may contain CJK or emoji
- `Style.Width(n).MaxWidth(n)` on every fixed-width column
- Rows composed via `lipgloss.JoinHorizontal(lipgloss.Top, …)` of pre-padded columns. **No `fmt.Sprintf("%-20s …")`** on multi-column lines — it counts bytes.

**7.2 Width budgeting is explicit at the view layer.**
View receives `(width, height)`, subtracts chrome, divides remainder among columns per a documented budget table, passes each component its exact cell width. Components never guess.

Example — sessions-list row at 90 cols:

```
total            90
borders + pad   - 4   (rounded card)
# col           - 4
started col     -18
project col     -22  (truncated w/ runewidth + ellipsis)
duration col    -10
cost col        - 8
prompts col     - 8
tokens col      - 7
live col        - 8
gutters (8 × 1) - 8
================ ───
                 = -1  → trip a build-time check
```

A failing budget is a test-time error, not a runtime mis-render.

**7.3 Golden-file tests at row + view level.**
- Every component test asserts `lipgloss.Width(out) == expectedWidth` for every input case including CJK + emoji
- Byte-exact golden files in `testdata/` per component × representative inputs
- One view-level golden per screen × state (empty / populated / loading / stale / no-daemon), rendered at fixed `(90, 32)`
- `go test ./internal/tui/... -update` regenerates; PR diff makes changes deliberate

This is the technique `gh-dash` and `soft-serve` use — the only sustainable way to keep multi-column TUIs honest as the code drifts.

## 8 · CLI surface

Two new persistent cobra flags on the root command:

```
cco [--theme mocha|macchiato|frappe|latte|auto]
    [--icons unicode|nerd]
```

Env equivalents: `CCO_THEME`, `CCO_ICONS`. Resolution order per §4.

No config file in v1. Adding one later under `$CCO_HOME/config.toml` is a separate spec.

## 9 · Testing approach

| Layer | What | Tool |
|---|---|---|
| Palette / glyphs selection | Deterministic resolution from `(args, env)` across all 4 themes × 2 icon sets | Table-driven `_test.go` |
| Components | (a) `lipgloss.Width(out) == expected` per input incl. CJK/emoji; (b) byte-exact goldens; (c) glyph swap renders different bytes | Table-driven + `testdata/` goldens |
| Width budgeter | Per-view column budget sums ≤ width — test fails on overflow | `_test.go` |
| Views | One golden per view × state at fixed `(90, 32)` | Goldens in `internal/tui/{dashboard,sessions,prompt}/testdata/` |
| Readstore additions | SQL tests with fixture rows | Existing `readstore/queries_test.go` pattern |
| Bubble Tea models | Unchanged — only `View()` is rewritten | Existing tests |

## 10 · Migration plan

Each step is independently shippable; each PR ≤ ~400 LOC diff target.

1. **PR 1 — Readstore additions.** `WindowStats.{Sessions, Tokens}`, `Snapshot.Yesterday`, `RecentSessionsToday`, `SessionRow.Tokens`. Existing callers ignore new fields. Unit tests added.
2. **PR 2 — Theme foundation.** Add new files (`palette.go` / `glyphs.go` / `select.go`) alongside the existing `theme.go`. The new `Theme` struct and the legacy `theme.Default()` (returning the old brutalist styles) coexist; existing views keep importing the legacy API and continue to compile and render unchanged. Wire `--theme` / `--icons` flags through cobra; resolution happens but doesn't affect the visible UI yet. Hide nothing — both shapes live in the package simultaneously.
3. **PR 3 — Component primitives.** Build `internal/tui/theme/component/{card,kpi,sparkline,badge,status,row,help}.go` with golden tests. No view changes yet.
4. **PR 4 — Dashboard rewrite.** Compose components; use new readstore fields; view-level golden.
5. **PR 5 — Sessions list rewrite.** Same pattern.
6. **PR 6 — Session detail + Prompt detail rewrite.** Together — they share `EventRow`/`APIRequestRow`/`ToolCallRow` styling.
7. **PR 7 — Cleanup.** Delete the legacy `theme.go` shape (thick-border `Block`, `Heading`, `Pill`, `AccentText`, `MutedText`, `ErrorText`) now that no view imports them. Remove `theme.Default()`; consumers receive a `*Theme` via the app context. Delete unused styles.

Total: ~7 PRs over ~1 week of focused work.

## 11 · Out of scope (deferred)

- 24-hour activity sparkline / hourly event histogram (needs new time-bucketed query)
- Model-mix card (needs per-session dominant-model materialization or events join)
- Per-row model badge on sessions list (same reason)
- Config file at `$CCO_HOME/config.toml`
- Light/dark theme switching at runtime (only at startup)
- Macchiato / Frappé flavors as runtime choices (`--theme` will accept them but they're aliases of Mocha until styled — flag-level parity, palette-level deferred)

## 12 · Risks

| Risk | Mitigation |
|---|---|
| Multi-column alignment regresses | §7 — golden tests + width budget test + `lipgloss.Width` discipline |
| User without Nerd Font sets `--icons nerd` and gets tofu | Documented in `cco --help`; default is `unicode` |
| Catppuccin colors clash on terminals with custom color schemes | Palette values are absolute (`lipgloss.Color("#…")`), bypassing terminal's 16-color overrides; cannot be misrendered by user theme |
| Light-mode detection via `$COLORFGBG` is heuristic | Documented; explicit `--theme` always wins; default to Mocha when env var is missing |
| Risk of feature creep around model mix / activity sparkline | Explicitly deferred in §11; this spec stops at Path 2 |
| PR 7 leaves no fallback if a view regresses | Each prior PR independently passes view-level goldens; PR 7 is a delete-only diff with no runtime change |
