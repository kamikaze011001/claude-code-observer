# Phase 3 — TUI shell + M3.1 Dashboard

> Status: design
> Date: 2026-05-10
> Source: [docs/ROADMAP.md](../../ROADMAP.md) Phase 3
> Scope: TUI shell (navigation, theme, read pool, message protocol) + M3.1 Dashboard view. M3.2 (Sessions) and M3.3 (Prompt detail) get their own smaller specs once the shell is in.

## Goals

1. `cco` (no args) opens a Bubble Tea TUI showing today / 7-day / 30-day cost, prompts, tools, errors, plus top-3 most expensive sessions today.
2. Numbers match a direct SQL query on the rollup tables (truth check).
3. TUI never blocks the daemon's writes; daemon never blocks the TUI's reads.
4. Daemon-down is a soft state (banner + auto-recover), not a crash.
5. Shell scales to M3.2/M3.3 without refactor — adding a new view is one new package + one `PushViewMsg`.
6. Visual identity: neo-brutalist (thick borders, ALL CAPS labels, single hot-yellow accent, no AI-slop visual fluff).

## Non-goals

- M3.2 (Sessions list + detail) and M3.3 (Prompt detail) — separate specs.
- Auto-starting the daemon from the TUI (deferred to FUTURE.md / Phase 4).
- Live terminal capture testing (`vhs`/`expect`).
- Per-view refresh cadence — uniform 1 s tick is sufficient at our query volumes.
- IPC between daemon and TUI — the shared SQLite file with WAL is sufficient.

## Decisions (locked during brainstorming)

| # | Decision | Rationale |
|---|----------|-----------|
| 1 | Daemon-down: empty-state banner, ticker keeps retrying | Self-healing, less complex than spawning daemon, less jarring than hard exit |
| 2 | Read-only sqlite pool, separate from daemon's writer pool | WAL allows lock-free coexistence; `mode=ro` + `_query_only=1` enforce safety |
| 3 | Uniform 1 s ticker across all views | Rollup queries are sub-ms; per-view cadence is premature optimization |
| 4 | Page stack + global keymap (`q`/`b`/`esc`/`?`) | Matches the drill-down UX in the roadmap; simple to reason about |
| 5 | Architecture: root model + `View` interface (option A) | Each milestone adds one view package, no shell churn |
| 6 | Theme: neo-brutalist, single accent `#FFD400`, semantic red `#FF3B30` | Locked. Five primitives only (Border, Heading, Block, Pill, AccentText) |
| 7 | Dashboard layout: three KPI blocks + top sessions table | 1:1 with M3.1 demo criteria; three peer windows scan correctly |

---

## §1 — Architecture & package layout

```
internal/tui/
├── app/                  # Root Model: nav stack, ticker, theme, last error
│   ├── app.go            # type App struct (tea.Model)
│   ├── view.go           # type View interface
│   ├── keys.go           # global keymap (q, b, esc, ?, r)
│   └── messages.go       # TickMsg, ErrMsg, PushViewMsg, PopViewMsg
├── theme/                # neo-brutalist lipgloss styles (single source)
│   └── theme.go          # Border, Heading, Block, Pill, AccentText, palette
├── readstore/            # read-only DB pool, query funcs (no business logic)
│   ├── pool.go           # OpenRO(path) → *sql.DB (mode=ro, WAL, query_only)
│   └── queries.go        # DashboardSnapshot(ctx) returns rollups
└── dashboard/            # M3.1 view
    ├── model.go          # implements app.View
    └── view.go           # render via theme primitives
```

**Layering rules:**

- `app` depends on `theme` (for chrome) and the `View` interface only. No knowledge of any concrete view.
- `dashboard` depends on `app` (View interface, messages), `theme`, `readstore`. Cannot import other view packages.
- `theme` depends on nothing.
- `readstore` depends on `internal/domain` only — no `internal/repository`, no `internal/service`.

**Why `readstore` is separate from `internal/repository/`:** repository owns the writer pool with the daemon's lifecycle. `readstore` owns a reader pool with the TUI's lifecycle. Same DB file, different opening params, different consumers. Coupling them would mean TUI shutdown waits on daemon shutdown.

**Wiring (in `cmd/app/main.go` default subcommand):**

```go
pool, err := readstore.OpenRO(dbPath)
if err != nil { return fmt.Errorf("open read pool: %w", err) }
defer pool.Close()

shell := app.New(theme.Default())
shell.Push(dashboard.New(pool))

if _, err := tea.NewProgram(shell, tea.WithAltScreen()).Run(); err != nil {
    return err
}
```

---

## §2 — `View` interface & message protocol

```go
// internal/tui/app/view.go
type View interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (View, tea.Cmd)
    View(width, height int) string
    Title() string
    ShortHelp() []key.Binding
}
```

**Messages crossing the shell ↔ view boundary:**

```go
// internal/tui/app/messages.go
type TickMsg time.Time          // 1s ticker; root forwards to top view
type PushViewMsg struct{ V View }  // view returns to drill in
type PopViewMsg struct{}        // global b/esc, also returnable from a view
type ErrMsg struct{ Err error } // any view emits; shell shows in chrome
```

Every other concern (selection, filter, view-local refresh) is local to a view and never crosses the shell. Keeping the protocol minimal is what makes M3.2/M3.3 cheap to add.

**Drill-in flow (M3.2 preview):**

1. Sessions view's `Update(KeyEnter)` returns `PushViewMsg{V: prompt.New(pool, sessionID)}`.
2. Root `Update` intercepts, pushes view onto stack, calls new view's `Init`.
3. New view's `Init` returns a `tea.Cmd` that runs the SQLite query in a goroutine.
4. Query result arrives as a view-local message; view stores it, re-renders.

**Global key handling:** root intercepts `q`, `b`, `esc`, `?`, `r` *before* forwarding to top view. `r` is re-emitted as `TickMsg` so the view treats refresh identically to ticker fire — no separate refresh code path.

---

## §3 — Data flow & read pool

**Pool open params:**

```go
dsn := fmt.Sprintf(
    "file:%s?mode=ro&_journal_mode=WAL&_busy_timeout=2000&_query_only=1",
    path,
)
db, err := sql.Open("sqlite", dsn)
db.SetMaxOpenConns(2)
db.SetMaxIdleConns(2)
db.SetConnMaxLifetime(0)
```

`mode=ro` is enforced at the driver level. `_query_only=1` is belt-and-braces: any accidental write fails fast. WAL lets us read while the daemon writes without blocking either side.

**Every fetch is a `tea.Cmd` (goroutine), never inline in `Update`:**

```go
func fetchDashboard(pool *sql.DB) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
        defer cancel()
        snap, top, err := readstore.DashboardSnapshot(ctx, pool)
        if err != nil { return app.ErrMsg{Err: err} }
        return dashboardDataMsg{snap: snap, top: top}
    }
}
```

500 ms context timeout is a hard ceiling — if a query hangs, the shell never freezes. A timed-out fetch surfaces as `ErrMsg`; the view keeps showing the last successful snapshot with a `STALE` pill.

**Tick fan-out & in-flight de-dup:**

- Root model owns a single 1 s `tea.Tick`.
- On each tick it forwards `TickMsg` to the top view.
- Top view returns `fetchDashboard(...)` from its `Update(TickMsg)`.
- View tracks an `inFlight bool`; if `true` when the next tick arrives, view ignores the tick. Prevents query pileup.

**Daemon-down behavior:**

- `readstore.OpenRO` succeeds even if the file doesn't exist (sqlite errors only on first query). Wrapper detects file absence and dashboard renders empty-state.
- If file exists but `cco serve` is down: queries succeed against the static file. Banner shows when `now - max(events.ts) > 30s`, indicating writes have stopped.

**Two queries for M3.1, both rollup-table only (no `events` scans):**

```sql
-- snapshot: 3 windows in one query via CASE
SELECT
  SUM(CASE WHEN started_at >= :today  THEN cost_usd END) AS today_cost,
  SUM(CASE WHEN started_at >= :today  THEN prompts END)  AS today_prompts,
  SUM(CASE WHEN started_at >= :today  THEN tool_calls END) AS today_tools,
  SUM(CASE WHEN started_at >= :today  THEN api_errors END) AS today_errors,
  SUM(CASE WHEN started_at >= :d7     THEN cost_usd END) AS d7_cost,
  -- ...repeat for 7d and 30d windows...
FROM sessions;

-- top 3 today
SELECT id, project_name, started_at, cost_usd, prompts,
       (ended_at IS NULL) AS live
FROM sessions
WHERE started_at >= :today
ORDER BY cost_usd DESC
LIMIT 3;
```

Both hit the existing `sessions(started_at)` index from M0.2.

---

## §4 — Neo-brutalist theme

Single source: `internal/tui/theme/theme.go`. View packages **only** read from theme; no inline `lipgloss.NewStyle()` outside `theme`.

**Palette:**

```go
var (
    Accent  = lipgloss.Color("#FFD400")  // hot yellow — locked
    Error   = lipgloss.Color("#FF3B30")  // semantic red — only on errors
    Fg      = lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
    Muted   = lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}
)
```

**Five primitives (the only styles allowed):**

- `Border` — thick `┏━┓` lipgloss border.
- `Heading` — bold, uppercase, `Fg`, padding `0 1`.
- `Block(width)` — `Border` + padding `1 2` + min width. Used for KPI tiles.
- `Pill(state)` — bold, inverse fg/bg, padding `0 1`. Used for `● LIVE`, `STALE`, `⚠ NO DAEMON`.
- `AccentText` — `Foreground(Accent)`, `Bold`. Used for cost figures and the focused row.

**Anti-slop rules (enforced in code review):**

1. **One accent.** Yellow appears on cost figures and the focused row only. Never on borders, never on labels.
2. **No rounded corners, no gradients, no drop shadows.** Thick border or no border.
3. **ALL CAPS labels, sentence-case data.** `TODAY $4.21`, not `Today: $4.21`.
4. **No emoji except `⚠` for errors and `●` for live.** No 🚀, no ✨.
5. **No abbreviation creativity.** `7 DAYS` not `WK`. Brutalism is honest signage.

**Chrome (rendered by root, not views):**

```
███████████████████████████████████████████████████████████████████████
█ CCO │ DASHBOARD                                              v0.1.0 █
███████████████████████████████████████████████████████████████████████
... view content ...
[s] sessions  [r] refresh  [?] help  [q] quit                    ● LIVE
```

The footer right-side pill has three states:

- `● LIVE` — last successful query within 30 s of `now`, daemon is writing.
- `STALE` — last query succeeded but data hasn't advanced in 30 s, or last query errored.
- `⚠ NO DAEMON` — DB file missing.

**Dashboard layout (M3.1):**

```
█ CCO │ DASHBOARD                                              v0.1.0 █
┏━━━━ TODAY ━━━━┓ ┏━━━ 7 DAYS ━━━┓ ┏━━━ 30 DAYS ━━━┓
┃  $4.21        ┃ ┃  $28.40       ┃ ┃  $112.05       ┃
┃  37 PROMPTS   ┃ ┃  214 PROMPTS  ┃ ┃  892 PROMPTS   ┃
┃  152 TOOLS    ┃ ┃  1.1k TOOLS   ┃ ┃  4.4k TOOLS    ┃
┃  ⚠ 2 ERRORS   ┃ ┃  ⚠ 9 ERRORS   ┃ ┃  ⚠ 41 ERRORS   ┃
┗━━━━━━━━━━━━━━━┛ ┗━━━━━━━━━━━━━━┛ ┗━━━━━━━━━━━━━━━━┛
┏━━━ TOP SESSIONS TODAY ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ #  PROJECT          STARTED  COST    PROMPTS  STATUS ┃
┃ 1  observer         09:14    $1.92   14       ● LIVE ┃
┃ 2  scratch          08:03    $1.40   11              ┃
┃ 3  notes-ingest     11:22    $0.89    7              ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
[s] sessions  [r] refresh  [?] help  [q] quit            ● LIVE
```

Cost figures rendered in `AccentText`. Error counts in `Error` color when > 0, `Muted` when 0. `LIVE` pill on the row whose session is open.

---

## §5 — Error handling

Three failure classes. Each has one defined surface; never a panic, never a stack trace on screen.

**1. DB unreachable (file missing or `sql.Open` fails).** Shell catches at startup. Dashboard mounts in empty-state mode: zero numbers, `⚠ NO DAEMON` pill in footer, banner row above the KPI blocks: `NO DATABASE — IS \`cco serve\` RUNNING?`. Ticker keeps firing fetches every 1 s; first success dismisses banner + pill.

**2. Query error (timeout, transient lock, bad row).** `fetchDashboard` cmd returns `ErrMsg{Err}`. Shell stores it on root model as `lastErr` with timestamp. Footer pill flips to `STALE`. View keeps rendering the last-known-good snapshot — never blanks out on a single bad fetch. After 3 consecutive errors the banner upgrades to `⚠ READ ERROR — q TO QUIT, ? FOR DETAILS`. `?` opens a help overlay that includes the last error string verbatim.

**3. Render-side panic.** Root wraps each delegated `View(...)` call in a `defer recover()`. On recover: log to stderr, render fallback frame (`⚠ VIEW ERROR — b TO RETURN`), pop broken view on next `b`/`esc`. Safety net only — every recover is treated as a bug to fix.

**Explicitly not implemented:**

- No retry-with-backoff. The ticker is the retry.
- No toast queue. One pill, one banner, one help overlay.
- No "report this error" button.

**Logging:** TUI writes to `~/.claude-code-observer/tui.log` via `log/slog`. Empty by default; populated only on errors. Separate file from daemon log so post-hoc debugging doesn't require splitting one log.

---

## §6 — Testing strategy

Coverage target: ≥ 60% on `internal/tui/dashboard/` (per roadmap M3.1 gate).

**Unit tests — Bubble Tea models (table-driven):**

- `app/app_test.go` — root model:
  - global `q` → returns `tea.Quit`
  - `b`/`esc` with stack depth ≥ 2 → pops; depth 1 → no-op
  - `r` → emits `TickMsg` to top view
  - `PushViewMsg` → pushes view, calls `Init`
  - `ErrMsg` → stores `lastErr`, increments error counter
  - `TickMsg` → forwarded to top view
- `dashboard/model_test.go`:
  - `Init` returns one fetch cmd
  - `Update(dashboardDataMsg)` → state updated, `inFlight=false`
  - `Update(TickMsg)` while `inFlight=true` → no new cmd
  - `Update(ErrMsg)` → keeps last good snapshot, sets stale flag

**Golden-file tests — render output:**

- `dashboard/view_test.go` snapshots `View(80, 24)` for: happy path, empty-state, stale, narrow (60 col).
- Stored under `dashboard/testdata/*.golden`. Updated with `go test -update`. ANSI escapes stripped for diff readability.

**Integration test — readstore against temp sqlite:**

- `readstore/queries_test.go`: applies M0.2 migration to a temp db, inserts known sessions/prompts via writer pool, opens separate `OpenRO` pool, asserts `DashboardSnapshot` returns expected aggregates.
- `EXPLAIN QUERY PLAN` assertion confirms snapshot query hits `sessions(started_at)` index.

**Explicitly not tested:**

- Live terminal capture (`vhs`, `expect`).
- `cco serve` + `cco` end-to-end in CI — manual demo step in roadmap.
- Model fuzz testing — finite state space.

**`docs/MANUAL-VERIFICATION.md`** gets a Phase 3 section before tagging M3.1: 6-step checklist (start daemon → run cco → verify numbers match `sqlite3` query → kill daemon → verify STALE pill → verify NO DAEMON banner on fresh db).

---

## Acceptance criteria (M3.1 demo)

Per roadmap:

- [ ] `cco serve` running in one terminal, `cco` in another.
- [ ] Use Claude Code in a third terminal — Dashboard updates within ~2 s.
- [ ] Stop using Claude Code — Dashboard stable, no flicker.
- [ ] Numbers match `sqlite3 ... "SELECT SUM(cost_usd) FROM sessions WHERE started_at >= unixepoch('now', 'start of day')*1e9"`.
- [ ] `go test ./internal/tui/...` ≥ 60% coverage on `dashboard/`.
- [ ] `go vet ./...`, `golangci-lint run`, `go test ./...` all clean.

## Open questions resolved

All clarifying questions resolved during brainstorming. None deferred.

## Follow-ups (separate specs)

- **M3.2** — Sessions list + Session detail view. Will reuse `app.View` interface, `theme`, `readstore`. New query: paged sessions list, paged events for a session.
- **M3.3** — Prompt detail view. New query: prompt rollup + tool calls + api_requests. First place `json_extract` shows up in a hot path (acceptable — only fires on drill-in, not on dashboard tick).
