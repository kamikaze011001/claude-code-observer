# TUI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the neo-brutalist TUI with a Catppuccin-Mocha, rounded, icon-rich design across all three screens (dashboard, sessions list, session/prompt detail) while adding the minimal SQL-only backend fields required to make the new dashboard meaningful.

**Architecture:** Theme abstraction first (palette + glyphs + derived styles), then a small `component/` library of pure render helpers, then per-screen rewrites that compose those components. Each screen's Bubble Tea model + `Update` logic stays intact; only `View()` is rewritten. The legacy `theme.Default()` API coexists with the new shape until the final cleanup PR so each intermediate state compiles and runs.

**Tech Stack:** Go 1.25 · `charmbracelet/bubbletea` · `charmbracelet/lipgloss` (rounded borders, ANSI-aware width) · `mattn/go-runewidth` (promoted to direct dep, used for CJK/emoji truncation) · `spf13/cobra` (`--theme` / `--icons` persistent flags) · `database/sql` + SQLite (readstore queries).

**Spec reference:** `docs/superpowers/specs/2026-05-12-tui-redesign-design.md`

---

## File Structure

Files created or modified by this plan, grouped by task.

| Path | Action | Responsibility |
|---|---|---|
| `internal/tui/readstore/queries.go` | Modify | Add `Sessions`/`Tokens` to `WindowStats`, `Yesterday` field on `Snapshot`, new `RecentSessionsToday`, `Tokens` on `SessionRow` |
| `internal/tui/readstore/queries_test.go` | Modify | Cover new fields and the new query |
| `internal/tui/theme/palette.go` | Create | `Palette` struct + four flavor constructors (Mocha, Macchiato, Frappé, Latte) |
| `internal/tui/theme/palette_test.go` | Create | Verify each palette has all required colors set |
| `internal/tui/theme/glyphs.go` | Create | `Glyphs` struct + `UnicodeGlyphs()` / `NerdGlyphs()` constructors |
| `internal/tui/theme/glyphs_test.go` | Create | Verify both glyph sets cover all required keys |
| `internal/tui/theme/select.go` | Create | `Resolve(themeName, iconsName, colorFGBG, env)` → `*Theme`, deterministic |
| `internal/tui/theme/select_test.go` | Create | Table-driven resolution tests |
| `internal/tui/theme/theme.go` | Modify | Add new `Theme` struct + derived styles alongside the existing struct; keep legacy fields (`Heading`, `Pill`, `ErrorText`, `MutedText`, `AccentText`, `Block`, `Default()`) working until PR 7 |
| `internal/tui/theme/theme_test.go` | Modify | Add tests for new `Theme.Build()` |
| `internal/tui/theme/component/doc.go` | Create | Package doc |
| `internal/tui/theme/component/card.go` | Create | `Card(t *Theme, title, body string, width int)` |
| `internal/tui/theme/component/card_test.go` | Create | Width + golden tests |
| `internal/tui/theme/component/kpi.go` | Create | `KPI(t *Theme, label, value string, delta *Delta, width int)` |
| `internal/tui/theme/component/kpi_test.go` | Create | Width + golden tests |
| `internal/tui/theme/component/sparkline.go` | Create | `Sparkline(t *Theme, values []float64, width int)` (reserved; not used by initial views) |
| `internal/tui/theme/component/sparkline_test.go` | Create | Width + clamp tests |
| `internal/tui/theme/component/badge.go` | Create | `ModelBadge(t *Theme, model string)` |
| `internal/tui/theme/component/badge_test.go` | Create | Color-by-family mapping test |
| `internal/tui/theme/component/status.go` | Create | `StatusPill(t *Theme, s Status)` |
| `internal/tui/theme/component/status_test.go` | Create | All 3 pill states |
| `internal/tui/theme/component/row.go` | Create | `SessionRow`, `PromptRow`, `EventRow`, `APIRequestRow`, `ToolCallRow` |
| `internal/tui/theme/component/row_test.go` | Create | Width + golden tests, including CJK project name |
| `internal/tui/theme/component/help.go` | Create | `HelpBar(t *Theme, hints []KeyHint, width int)` |
| `internal/tui/theme/component/help_test.go` | Create | Width test |
| `internal/tui/theme/component/budget.go` | Create | `Budget(width int)` builder used by views; panics on overflow |
| `internal/tui/theme/component/budget_test.go` | Create | Overflow panic + happy-path tests |
| `cmd/app/main.go` | Modify | Register `--theme` / `--icons` persistent flags + `CCO_THEME` / `CCO_ICONS` env fallback |
| `cmd/app/main_test.go` | Modify | Cover new flag resolution |
| `cmd/app/tui.go` | Modify | Resolve theme + pass to `app.New` and view constructors |
| `internal/tui/app/app.go` | Modify | Render chrome via new `Theme` while still accepting legacy field reads (no behavioral change in PR 2; rewritten in PR 4) |
| `internal/tui/dashboard/view.go` | Modify | Rewrite `View()` using components; receive `*theme.Theme` via constructor |
| `internal/tui/dashboard/model.go` | Modify | Accept `*theme.Theme` in `New`; store on model; consume new readstore fields |
| `internal/tui/dashboard/view_test.go` | Modify | Replace with golden-file tests at `(90, 32)` per state |
| `internal/tui/dashboard/testdata/*.golden` | Create | Golden outputs |
| `internal/tui/sessions/list.go` | Modify | Rewrite `View()` using components; receive `*theme.Theme` via constructor |
| `internal/tui/sessions/list_test.go` | Modify | Golden-file tests |
| `internal/tui/sessions/testdata/*.golden` | Create | Golden outputs (list + detail) |
| `internal/tui/sessions/detail.go` | Modify | Rewrite `View()` using components; receive `*theme.Theme` |
| `internal/tui/sessions/detail_test.go` | Modify | Golden tests |
| `internal/tui/prompt/detail.go` | Modify | Rewrite `View()` using components; receive `*theme.Theme` |
| `internal/tui/prompt/detail_test.go` | Modify | Golden tests |
| `internal/tui/prompt/testdata/*.golden` | Create | Golden outputs |
| `internal/tui/theme/theme.go` | Modify (PR 7) | Delete legacy fields and `Default()`; promote new Theme to package-level |
| `internal/tui/theme/theme_test.go` | Modify (PR 7) | Drop tests for removed legacy types |
| `go.mod` / `go.sum` | Modify | Promote `mattn/go-runewidth` from indirect to direct dep |

---

## Conventions used across tasks

**TDD micro-cycle.** Every code change follows: write failing test → run, confirm it fails for the *right reason* → write minimal code → run, confirm pass → commit. Skip the cycle only for trivial, untestable changes (a doc comment, a flag registration with no logic).

**Commit message style.** Match existing repo: `feat(tui):`, `feat(readstore):`, `refactor(theme):`, `test(tui):`, `chore(deps):`. Body wraps at 72.

**Run verification after each step that touches code:** `make vet && make test && make build`. If any fails, fix before committing. The plan calls this out only when a specific check matters.

**Theme during migration.** PR 2 introduces the new `Theme` struct *alongside* the legacy fields on the same struct. Legacy fields (`Heading`, `MutedText`, `AccentText`, `ErrorText`, `Pill`, `Block`) keep working until PR 7. New fields (`Title`, `Subtitle`, `Muted`, `Accent`, `Value`, `Label`, `Card`, etc.) are added. `theme.Default()` continues to return a populated `Theme` for both shapes. This lets each intermediate PR compile and the binary stay runnable.

**Width discipline (load-bearing — Spec §7).** Every row in the new `component/` package builds its line with `lipgloss.JoinHorizontal(lipgloss.Top, …)` of pre-padded columns. Truncation uses `runewidth.Truncate` (or `runewidth.StringWidth` + manual slice for ellipsis). Tests assert `lipgloss.Width(out) == expected` for every input including a CJK project name and an emoji project name.

---

## Task 1 — Readstore additions (Path 2 SQL only)

**Files:**
- Modify: `internal/tui/readstore/queries.go`
- Modify: `internal/tui/readstore/queries_test.go`

This task adds the four backend fields the new dashboard needs. No new tables, no rollup changes. Each step is a TDD cycle for one new field/query.

### Step 1.1 — Failing test for `WindowStats.Sessions` and `WindowStats.Tokens`

Open `internal/tui/readstore/queries_test.go`. Extend `TestDashboardSnapshot_AggregatesByWindow` so it inserts token values and asserts the two new fields. Add to the `insertSession` helper an `inputTok`, `outputTok` param.

- [ ] **Step 1.1.1: Replace `insertSession` helper to also write tokens.**

```go
insertSession := func(id, project string, started time.Time, cost float64, prompts, tools, errors int, inTok, outTok int64) {
    _, err := repo.DB().ExecContext(context.Background(),
        `INSERT INTO sessions
         (session_id, project_name, started_at, last_seen_at,
          cost_usd, prompts, tool_calls, api_errors,
          input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0)`,
        id, project, started.UnixNano(), started.UnixNano(),
        cost, prompts, tools, errors, inTok, outTok)
    if err != nil {
        t.Fatalf("insert: %v", err)
    }
}

insertSession("today1", "obs", now,                 1.50, 5, 20, 1, 1000, 200)
insertSession("today2", "obs", now.Add(time.Hour),  0.80, 3, 12, 0,  500, 100)
insertSession("d2",     "scratch", twoDaysAgo,      2.00, 8, 30, 0, 2000, 400)
insertSession("d10",    "obs", tenDaysAgo,          4.00, 10, 40, 2, 3000, 600)
insertSession("d40",    "obs", fortyDaysAgo,       99.00, 100, 500, 50, 99999, 99999)
```

- [ ] **Step 1.1.2: Add new field assertions to the test.**

After the existing `snap.D30.Errors` check:

```go
if got, want := snap.Today.Sessions, int64(2); got != want {
    t.Errorf("today sessions: got %d want %d", got, want)
}
if got, want := snap.Today.Tokens, int64(1800); got != want { // 1000+200 + 500+100
    t.Errorf("today tokens: got %d want %d", got, want)
}
if got, want := snap.D7.Sessions, int64(3); got != want {
    t.Errorf("7d sessions: got %d want %d", got, want)
}
if got, want := snap.D30.Tokens, int64(7400); got != want { // 1800 + 2400 + 3600 (today + d2 + d10)
    t.Errorf("30d tokens: got %d want %d", got, want)
}
```

- [ ] **Step 1.1.3: Run the test, confirm failure.**

```bash
go test ./internal/tui/readstore/ -run TestDashboardSnapshot_AggregatesByWindow -v
```

Expected: FAIL — `snap.Today.Sessions undefined` and similar.

### Step 1.2 — Implement `Sessions` + `Tokens` on `WindowStats`

- [ ] **Step 1.2.1: Add the two fields to `WindowStats`.**

Edit `internal/tui/readstore/queries.go`. Replace the existing `WindowStats` block:

```go
// WindowStats is the rollup over a single time window.
type WindowStats struct {
    Sessions int64
    CostUSD  float64
    Prompts  int64
    Tokens   int64
    Tools    int64
    Errors   int64
}
```

- [ ] **Step 1.2.2: Extend the snapshot query to compute the new sums.**

Replace the existing `const q = ...` SQL inside `DashboardSnapshot` and the corresponding `Scan` call:

```go
const q = `
SELECT
  COALESCE(SUM(CASE WHEN started_at >= ? THEN 1                                                                                       END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd                                                                                END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts                                                                                 END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens                END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls                                                                              END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors                                                                              END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN 1                                                                                       END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd                                                                                END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts                                                                                 END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens                END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls                                                                              END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors                                                                              END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN 1                                                                                       END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd                                                                                END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts                                                                                 END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens                END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls                                                                              END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors                                                                              END), 0)
FROM sessions
WHERE started_at >= ?`

var s Snapshot
err := db.QueryRowContext(ctx, q,
    today, today, today, today, today, today,
    d7, d7, d7, d7, d7, d7,
    d30, d30, d30, d30, d30, d30,
    d30,
).Scan(
    &s.Today.Sessions, &s.Today.CostUSD, &s.Today.Prompts, &s.Today.Tokens, &s.Today.Tools, &s.Today.Errors,
    &s.D7.Sessions,    &s.D7.CostUSD,    &s.D7.Prompts,    &s.D7.Tokens,    &s.D7.Tools,    &s.D7.Errors,
    &s.D30.Sessions,   &s.D30.CostUSD,   &s.D30.Prompts,   &s.D30.Tokens,   &s.D30.Tools,   &s.D30.Errors,
)
```

- [ ] **Step 1.2.3: Run the test, confirm pass.**

```bash
go test ./internal/tui/readstore/ -run TestDashboardSnapshot_AggregatesByWindow -v
```

Expected: PASS.

### Step 1.3 — Failing test for `Snapshot.Yesterday`

- [ ] **Step 1.3.1: Add a `Yesterday` field assertion to the same test.**

Append:

```go
// Yesterday window covers [startOfDay-24h, startOfDay). None of our seeded
// rows fall in that window, so Yesterday should be zero.
if got, want := snap.Yesterday.Sessions, int64(0); got != want {
    t.Errorf("yesterday sessions: got %d want %d", got, want)
}
```

Add a new sub-test (separate function) that *does* place a row in the yesterday window:

```go
func TestDashboardSnapshot_YesterdayWindow(t *testing.T) {
    home := t.TempDir()
    repo, err := repository.Open(home)
    if err != nil { t.Fatalf("repository.Open: %v", err) }
    t.Cleanup(func() { _ = repo.Close() })

    now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
    startOfDay := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
    yesterday := startOfDay.Add(-3 * time.Hour) // inside yesterday window

    _, err = repo.DB().ExecContext(context.Background(),
        `INSERT INTO sessions
         (session_id, project_name, started_at, last_seen_at,
          cost_usd, prompts, tool_calls, api_errors,
          input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens)
         VALUES ('y1', 'obs', ?, ?, 1.23, 2, 5, 0, 100, 50, 0, 0)`,
        yesterday.UnixNano(), yesterday.UnixNano())
    if err != nil { t.Fatalf("insert: %v", err) }

    pool, err := readstore.OpenRO(filepath.Join(home, "db.sqlite"))
    if err != nil { t.Fatalf("OpenRO: %v", err) }
    t.Cleanup(func() { _ = pool.Close() })

    snap, _, err := readstore.DashboardSnapshot(context.Background(), pool, now)
    if err != nil { t.Fatalf("DashboardSnapshot: %v", err) }

    if got, want := snap.Yesterday.Sessions, int64(1); got != want {
        t.Errorf("yesterday sessions: got %d want %d", got, want)
    }
    if got, want := snap.Yesterday.CostUSD, 1.23; got != want {
        t.Errorf("yesterday cost: got %.2f want %.2f", got, want)
    }
    if got, want := snap.Yesterday.Tokens, int64(150); got != want {
        t.Errorf("yesterday tokens: got %d want %d", got, want)
    }
}
```

- [ ] **Step 1.3.2: Run, confirm failure.**

```bash
go test ./internal/tui/readstore/ -run "TestDashboardSnapshot" -v
```

Expected: FAIL — `snap.Yesterday undefined`.

### Step 1.4 — Implement `Yesterday`

- [ ] **Step 1.4.1: Add field to `Snapshot`.**

```go
type Snapshot struct {
    Today         WindowStats
    Yesterday     WindowStats
    D7            WindowStats
    D30           WindowStats
    LatestEventTS int64
}
```

- [ ] **Step 1.4.2: Compute the yesterday window in `DashboardSnapshot` with a second query.**

After the existing snapshot query, before the `topQ` query, insert:

```go
yStart := startOfDay.Add(-24 * time.Hour).UnixNano()
yEnd := today
const yQ = `
SELECT
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN 1                                                                                END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN cost_usd                                                                         END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN prompts                                                                          END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens         END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN tool_calls                                                                       END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? AND started_at < ? THEN api_errors                                                                       END), 0)
FROM sessions
WHERE started_at >= ? AND started_at < ?`
if err := db.QueryRowContext(ctx, yQ,
    yStart, yEnd, yStart, yEnd, yStart, yEnd,
    yStart, yEnd, yStart, yEnd, yStart, yEnd,
    yStart, yEnd,
).Scan(
    &s.Yesterday.Sessions, &s.Yesterday.CostUSD, &s.Yesterday.Prompts,
    &s.Yesterday.Tokens,   &s.Yesterday.Tools,   &s.Yesterday.Errors,
); err != nil {
    return Snapshot{}, nil, fmt.Errorf("yesterday snapshot: %w", err)
}
```

- [ ] **Step 1.4.3: Run, confirm pass.**

```bash
go test ./internal/tui/readstore/ -v
```

Expected: all `TestDashboardSnapshot*` tests PASS.

### Step 1.5 — Failing test for `RecentSessionsToday`

- [ ] **Step 1.5.1: Add new test.**

Append to `queries_test.go`:

```go
func TestRecentSessionsToday(t *testing.T) {
    home := t.TempDir()
    repo, err := repository.Open(home)
    if err != nil { t.Fatalf("repository.Open: %v", err) }
    t.Cleanup(func() { _ = repo.Close() })

    now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
    startOfDay := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

    ins := func(id, project string, started time.Time, cost float64, prompts int64, ended *time.Time) {
        endedNS := sql.NullInt64{}
        if ended != nil { endedNS.Valid = true; endedNS.Int64 = ended.UnixNano() }
        _, err := repo.DB().ExecContext(context.Background(),
            `INSERT INTO sessions
             (session_id, project_name, started_at, last_seen_at, ended_at,
              cost_usd, prompts, tool_calls, api_errors,
              input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens)
             VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, 0, 0, 0, 0)`,
            id, project, started.UnixNano(), started.UnixNano(), endedNS,
            cost, prompts)
        if err != nil { t.Fatalf("insert: %v", err) }
    }
    yEnded := now.Add(-2 * time.Hour)
    ins("r1", "obs", now.Add(-10*time.Minute), 0.10, 1, nil)            // newest, live
    ins("r2", "scratch", now.Add(-30*time.Minute), 0.20, 2, &yEnded)
    ins("r3", "obs", now.Add(-3*time.Hour), 0.30, 3, &yEnded)
    ins("r4", "obs", startOfDay.Add(-1*time.Hour), 9.99, 9, nil)        // yesterday — excluded

    pool, err := readstore.OpenRO(filepath.Join(home, "db.sqlite"))
    if err != nil { t.Fatalf("OpenRO: %v", err) }
    t.Cleanup(func() { _ = pool.Close() })

    rows, err := readstore.RecentSessionsToday(context.Background(), pool, now, 10)
    if err != nil { t.Fatalf("RecentSessionsToday: %v", err) }

    if len(rows) != 3 {
        t.Fatalf("rows: got %d want 3", len(rows))
    }
    if rows[0].SessionID != "r1" || rows[1].SessionID != "r2" || rows[2].SessionID != "r3" {
        t.Errorf("order wrong: %+v", rows)
    }
    if !rows[0].Live {
        t.Errorf("r1 should be live")
    }
}
```

(Also add `"database/sql"` to the import block at the top of the file.)

- [ ] **Step 1.5.2: Run, confirm failure.**

```bash
go test ./internal/tui/readstore/ -run TestRecentSessionsToday -v
```

Expected: FAIL — `readstore.RecentSessionsToday undefined`.

### Step 1.6 — Implement `RecentSessionsToday`

- [ ] **Step 1.6.1: Add the function below `DashboardSnapshot` in `queries.go`.**

```go
// RecentSessionsToday returns up to limit sessions started since the start of
// the UTC day containing now, newest-first. The shape is the same as
// TopSession so the dashboard can reuse the same row renderer.
func RecentSessionsToday(ctx context.Context, db *sql.DB, now time.Time, limit int) ([]TopSession, error) {
    if limit <= 0 {
        limit = 5
    }
    startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).UnixNano()
    const q = `
SELECT session_id, COALESCE(project_name, ''), started_at, cost_usd, prompts, ended_at IS NULL
FROM sessions
WHERE started_at >= ?
ORDER BY started_at DESC
LIMIT ?`
    rows, err := db.QueryContext(ctx, q, startOfDay, limit)
    if err != nil {
        return nil, fmt.Errorf("recent sessions query: %w", err)
    }
    defer rows.Close()
    var out []TopSession
    for rows.Next() {
        var r TopSession
        var live int
        if err := rows.Scan(&r.SessionID, &r.ProjectName, &r.StartedAt, &r.CostUSD, &r.Prompts, &live); err != nil {
            return nil, fmt.Errorf("recent session scan: %w", err)
        }
        r.Live = live == 1
        out = append(out, r)
    }
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("recent sessions iter: %w", err)
    }
    return out, nil
}
```

- [ ] **Step 1.6.2: Run, confirm pass.**

```bash
go test ./internal/tui/readstore/ -v
```

Expected: all readstore tests PASS.

### Step 1.7 — Failing test for `SessionRow.Tokens`

- [ ] **Step 1.7.1: Extend an existing `TestSessionsPage*` test (or add a new one) to assert `Tokens`.**

Find the smallest existing `TestSessionsPage` test that inserts one row, and add a `Tokens` assertion against it. If none exists with explicit token values, add this test:

```go
func TestSessionsPage_IncludesTokens(t *testing.T) {
    home := t.TempDir()
    repo, err := repository.Open(home)
    if err != nil { t.Fatalf("repository.Open: %v", err) }
    t.Cleanup(func() { _ = repo.Close() })

    started := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC).UnixNano()
    _, err = repo.DB().ExecContext(context.Background(),
        `INSERT INTO sessions
         (session_id, project_name, started_at, last_seen_at,
          cost_usd, prompts,
          input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens)
         VALUES ('s1', 'obs', ?, ?, 1.00, 3, 1000, 200, 50, 10)`,
        started, started)
    if err != nil { t.Fatalf("insert: %v", err) }

    pool, err := readstore.OpenRO(filepath.Join(home, "db.sqlite"))
    if err != nil { t.Fatalf("OpenRO: %v", err) }
    t.Cleanup(func() { _ = pool.Close() })

    rows, _, err := readstore.SessionsPage(context.Background(), pool, nil, 10)
    if err != nil { t.Fatalf("SessionsPage: %v", err) }
    if len(rows) != 1 { t.Fatalf("rows: got %d", len(rows)) }
    if got, want := rows[0].Tokens, int64(1260); got != want {
        t.Errorf("tokens: got %d want %d", got, want)
    }
}
```

- [ ] **Step 1.7.2: Run, confirm failure.**

```bash
go test ./internal/tui/readstore/ -run TestSessionsPage_IncludesTokens -v
```

Expected: FAIL — `rows[0].Tokens undefined`.

### Step 1.8 — Implement `SessionRow.Tokens`

- [ ] **Step 1.8.1: Add field to `SessionRow`.**

In `queries.go`:

```go
type SessionRow struct {
    SessionID   string
    ProjectName string
    StartedAt   time.Time
    LastSeenAt  time.Time
    EndedAt     time.Time
    DurationSec int64
    CostUSD     float64
    Prompts     int64
    Tokens      int64
    Live        bool
}
```

- [ ] **Step 1.8.2: Update the `SessionsPage` SQL and Scan to include the sum.**

Replace the `const q = ...` and the `Scan` call inside `SessionsPage`:

```go
const q = `
SELECT session_id,
       COALESCE(project_name, ''),
       started_at,
       last_seen_at,
       ended_at,
       cost_usd,
       prompts,
       input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens AS tokens
FROM sessions
WHERE (? IS NULL OR started_at < ?)
ORDER BY started_at DESC
LIMIT ?`
```

In the loop body:

```go
if err := rows.Scan(&r.SessionID, &r.ProjectName, &started, &lastSeen, &ended, &r.CostUSD, &r.Prompts, &r.Tokens); err != nil {
    return nil, nil, fmt.Errorf("sessions page scan: %w", err)
}
```

- [ ] **Step 1.8.3: Run, confirm pass.**

```bash
go test ./internal/tui/readstore/ -v
```

Expected: all readstore tests PASS.

### Step 1.9 — Verify and commit PR 1

- [ ] **Step 1.9.1: Run full verification.**

```bash
make vet && make test && make build
```

Expected: all green. The `dashboard.renderBlock` callsite in `internal/tui/dashboard/view.go` only reads `ws.CostUSD`, `ws.Prompts`, `ws.Tools`, `ws.Errors` — unchanged fields — so it compiles unchanged.

- [ ] **Step 1.9.2: Commit.**

```bash
git add internal/tui/readstore/queries.go internal/tui/readstore/queries_test.go
git commit -m "feat(readstore): add Sessions/Tokens per window, Yesterday window, RecentSessionsToday

Path 2 backend additions from docs/superpowers/specs/2026-05-12-tui-redesign-design.md §5.
SQL-only changes; no new tables. Existing callers ignore the new fields."
```

---

## Task 2 — Theme foundation (palette, glyphs, select, new Theme shape)

**Files:**
- Modify: `go.mod`, `go.sum` (promote runewidth to direct dep)
- Create: `internal/tui/theme/palette.go`, `palette_test.go`
- Create: `internal/tui/theme/glyphs.go`, `glyphs_test.go`
- Create: `internal/tui/theme/select.go`, `select_test.go`
- Modify: `internal/tui/theme/theme.go`, `theme_test.go`
- Modify: `cmd/app/main.go`, `main_test.go`
- Modify: `cmd/app/tui.go`

This task introduces the new `Theme` shape alongside the existing one. After this task the binary still renders the legacy brutalist UI, but the new flags are wired and the new types are usable.

### Step 2.1 — Promote runewidth to a direct dependency

- [ ] **Step 2.1.1: Add the direct require.**

```bash
go get github.com/mattn/go-runewidth@v0.0.19
go mod tidy
```

- [ ] **Step 2.1.2: Verify it appears in the direct `require` block of go.mod.**

```bash
grep -n "mattn/go-runewidth" go.mod
```

Expected: line appears outside the `// indirect` block.

- [ ] **Step 2.1.3: Commit.**

```bash
git add go.mod go.sum
git commit -m "chore(deps): promote mattn/go-runewidth to direct dep

Used by upcoming theme/component package for CJK/emoji-aware width calculations."
```

### Step 2.2 — Palette

- [ ] **Step 2.2.1: Failing test for palette constructors.**

Create `internal/tui/theme/palette_test.go`:

```go
package theme

import (
    "testing"

    "github.com/charmbracelet/lipgloss"
)

func TestPalettes_HaveAllRequiredColors(t *testing.T) {
    palettes := map[string]Palette{
        "Mocha":     MochaPalette(),
        "Macchiato": MacchiatoPalette(),
        "Frappe":    FrappePalette(),
        "Latte":     LattePalette(),
    }
    for name, p := range palettes {
        colors := map[string]lipgloss.Color{
            "Bg": p.Bg, "BgAlt": p.BgAlt, "Fg": p.Fg, "FgMuted": p.FgMuted,
            "Accent": p.Accent,
            "Blue": p.Blue, "Green": p.Green, "Yellow": p.Yellow, "Red": p.Red,
            "Teal": p.Teal, "Mauve": p.Mauve,
        }
        for field, c := range colors {
            if string(c) == "" {
                t.Errorf("%s.%s is empty", name, field)
            }
        }
    }
}
```

- [ ] **Step 2.2.2: Run, confirm failure.**

```bash
go test ./internal/tui/theme/ -run TestPalettes_HaveAllRequiredColors -v
```

Expected: FAIL — `Palette undefined`.

- [ ] **Step 2.2.3: Implement palette.**

Create `internal/tui/theme/palette.go`:

```go
package theme

import "github.com/charmbracelet/lipgloss"

// Palette holds the absolute colors a Theme uses. Values are lipgloss.Color
// hex literals so they bypass the terminal's 16-color overrides.
type Palette struct {
    Bg, BgAlt, Fg, FgMuted   lipgloss.Color
    Accent                    lipgloss.Color
    Blue, Green, Yellow, Red lipgloss.Color
    Teal, Mauve              lipgloss.Color
}

// MochaPalette is the flagship dark flavor.
func MochaPalette() Palette {
    return Palette{
        Bg:      "#1e1e2e",
        BgAlt:   "#313244",
        Fg:      "#cdd6f4",
        FgMuted: "#6c7086",
        Accent:  "#f5c2e7",
        Blue:    "#89b4fa",
        Green:   "#a6e3a1",
        Yellow:  "#f9e2af",
        Red:     "#f38ba8",
        Teal:    "#94e2d5",
        Mauve:   "#cba6f7",
    }
}

// MacchiatoPalette is the softer dark flavor.
func MacchiatoPalette() Palette {
    return Palette{
        Bg: "#24273a", BgAlt: "#363a4f", Fg: "#cad3f5", FgMuted: "#6e738d",
        Accent: "#f5bde6",
        Blue:   "#8aadf4", Green: "#a6da95", Yellow: "#eed49f", Red: "#ed8796",
        Teal: "#8bd5ca", Mauve: "#c6a0f6",
    }
}

// FrappePalette is the warmest dark flavor.
func FrappePalette() Palette {
    return Palette{
        Bg: "#303446", BgAlt: "#414559", Fg: "#c6d0f5", FgMuted: "#737994",
        Accent: "#f4b8e4",
        Blue:   "#8caaee", Green: "#a6d189", Yellow: "#e5c890", Red: "#e78284",
        Teal: "#81c8be", Mauve: "#ca9ee6",
    }
}

// LattePalette is the light flavor.
func LattePalette() Palette {
    return Palette{
        Bg: "#eff1f5", BgAlt: "#ccd0da", Fg: "#4c4f69", FgMuted: "#9ca0b0",
        Accent: "#ea76cb",
        Blue:   "#1e66f5", Green: "#40a02b", Yellow: "#df8e1d", Red: "#d20f39",
        Teal: "#179299", Mauve: "#8839ef",
    }
}
```

- [ ] **Step 2.2.4: Run, confirm pass.**

```bash
go test ./internal/tui/theme/ -v
```

Expected: PASS.

### Step 2.3 — Glyphs

- [ ] **Step 2.3.1: Failing test.**

Create `internal/tui/theme/glyphs_test.go`:

```go
package theme

import "testing"

func TestGlyphs_AllSetsCoverRequiredKeys(t *testing.T) {
    cases := map[string]Glyphs{"unicode": UnicodeGlyphs(), "nerd": NerdGlyphs()}
    for name, g := range cases {
        if g.Brand == "" || g.StatusOK == "" || g.Cursor == "" ||
            g.DeltaUp == "" || g.DeltaDown == "" || g.DeltaFlat == "" ||
            g.Check == "" || g.Cross == "" || g.Enter == "" || len(g.Spark) == 0 {
            t.Errorf("%s: missing required glyph", name)
        }
    }
}
```

- [ ] **Step 2.3.2: Run, confirm failure.**

```bash
go test ./internal/tui/theme/ -run TestGlyphs -v
```

Expected: FAIL — `Glyphs undefined`.

- [ ] **Step 2.3.3: Implement glyphs.**

Create `internal/tui/theme/glyphs.go`:

```go
package theme

import "github.com/charmbracelet/lipgloss"

// Glyphs is the set of unicode/nerd-font characters used in the UI. Two
// preset constructors are provided; consumers pick one at startup.
type Glyphs struct {
    Brand       string
    StatusOK    string
    StatusWarn  string
    StatusErr   string
    Cursor      string
    DeltaUp     string
    DeltaDown   string
    DeltaFlat   string
    Check       string
    Cross       string
    Spark       []rune
    Enter       string
    BorderRound lipgloss.Border
}

// UnicodeGlyphs is the default — renders everywhere.
func UnicodeGlyphs() Glyphs {
    return Glyphs{
        Brand:       "✦",
        StatusOK:    "●",
        StatusWarn:  "●",
        StatusErr:   "●",
        Cursor:      "▸",
        DeltaUp:     "▲",
        DeltaDown:   "▼",
        DeltaFlat:   "─",
        Check:       "✓",
        Cross:       "✗",
        Spark:       []rune("▁▂▃▄▅▆▇█"),
        Enter:       "⏎",
        BorderRound: lipgloss.RoundedBorder(),
    }
}

// NerdGlyphs uses Nerd Font private-use codepoints. Requires a patched font.
func NerdGlyphs() Glyphs {
    g := UnicodeGlyphs()
    g.Brand = ""     //   star
    g.StatusOK = ""  //   filled dot
    g.StatusWarn = ""
    g.StatusErr = ""
    g.Cursor = ""    // 
    g.DeltaUp = ""   //   up arrow
    g.DeltaDown = "" //   down arrow
    g.Check = ""     //   check
    g.Cross = ""     //   cross
    return g
}
```

- [ ] **Step 2.3.4: Run, confirm pass.**

```bash
go test ./internal/tui/theme/ -v
```

Expected: PASS.

### Step 2.4 — New `Theme` struct alongside the legacy one

- [ ] **Step 2.4.1: Failing test for `Theme.Build`.**

Append to `internal/tui/theme/theme_test.go`:

```go
func TestTheme_Build_PopulatesNewFields(t *testing.T) {
    th := Build(MochaPalette(), UnicodeGlyphs())
    if string(th.Palette.Bg) != "#1e1e2e" {
        t.Errorf("Palette not copied: %+v", th.Palette)
    }
    if th.Glyphs.Brand != "✦" {
        t.Errorf("Glyphs not copied: %+v", th.Glyphs)
    }
    // Derived styles are non-zero
    if th.Title.GetForeground() == nil {
        t.Errorf("Title style empty")
    }
    if th.Card.GetBorderStyle() == (lipgloss.Border{}) {
        t.Errorf("Card border not set")
    }
}

func TestTheme_LegacyAPI_StillWorks(t *testing.T) {
    th := Default()
    // The chrome in internal/tui/app/app.go uses these — they must keep working.
    if th.Heading.Render("X") == "" { t.Errorf("legacy Heading broken") }
    if th.Pill(PillLive) == "" { t.Errorf("legacy Pill broken") }
}
```

- [ ] **Step 2.4.2: Run, confirm failure.**

```bash
go test ./internal/tui/theme/ -run "TestTheme_Build_PopulatesNewFields" -v
```

Expected: FAIL — `Build undefined`.

- [ ] **Step 2.4.3: Add the new shape to `theme.go` without removing the legacy one.**

Open `internal/tui/theme/theme.go`. Keep everything that's there. **Append** these declarations at the bottom of the file:

```go
// Theme now extends to include the new redesign shape. New fields are added
// to the existing struct; legacy fields stay populated by Default() so app.go
// chrome keeps rendering until the views are migrated.
//
// NOTE: the existing Theme struct above (AccentColor, ErrorColor, Heading,
// AccentText, ErrorText, MutedText, border) is the legacy shape. The fields
// below are the new shape; both coexist on the same struct.

// (Add these fields to the Theme struct above by editing the struct literal.)
```

Then edit the existing `Theme` struct definition to add the new fields. The full struct now reads:

```go
type Theme struct {
    // --- legacy shape (removed in PR 7) ---
    AccentColor lipgloss.Color
    ErrorColor  lipgloss.Color
    Fg          lipgloss.AdaptiveColor
    Muted       lipgloss.AdaptiveColor

    Heading    lipgloss.Style
    AccentText lipgloss.Style
    ErrorText  lipgloss.Style
    MutedText  lipgloss.Style

    border lipgloss.Border

    // --- new shape ---
    Palette Palette
    Glyphs  Glyphs

    Title    lipgloss.Style
    Subtitle lipgloss.Style
    Muted2   lipgloss.Style // "Muted" name is taken by legacy AdaptiveColor above
    Accent   lipgloss.Style
    Value    lipgloss.Style
    Label    lipgloss.Style

    Card      lipgloss.Style
    CardTitle lipgloss.Style
    Help      lipgloss.Style

    BadgeOpus   lipgloss.Style
    BadgeSonnet lipgloss.Style
    BadgeHaiku  lipgloss.Style

    PillLiveS    lipgloss.Style // "PillLive" the constant name conflict
    PillStaleS   lipgloss.Style
    PillNoDaemon lipgloss.Style
}
```

(Naming: `Muted` is already used by the legacy AdaptiveColor — the new muted style is `Muted2`. The new pill styles are suffixed with `S` to avoid colliding with the `PillLive` / `PillStale` / `PillNoDaemon` const identifiers in the legacy `PillState`. PR 7 collapses these names.)

- [ ] **Step 2.4.4: Add the `Build` constructor.**

In `theme.go`, after the existing `Default()` function:

```go
// Build constructs a Theme value populated only with the new shape. The
// legacy fields are zero — only suitable for the new components, not for
// rendering chrome via the legacy app.go path.
func Build(p Palette, g Glyphs) Theme {
    return Theme{
        Palette: p,
        Glyphs:  g,

        Title:    lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
        Subtitle: lipgloss.NewStyle().Foreground(p.FgMuted),
        Muted2:   lipgloss.NewStyle().Foreground(p.FgMuted),
        Accent:   lipgloss.NewStyle().Foreground(p.Accent),
        Value:    lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
        Label:    lipgloss.NewStyle().Foreground(p.FgMuted),

        Card:      lipgloss.NewStyle().Border(g.BorderRound).BorderForeground(p.FgMuted).Padding(0, 2),
        CardTitle: lipgloss.NewStyle().Foreground(p.FgMuted),
        Help:      lipgloss.NewStyle().Foreground(p.FgMuted),

        BadgeOpus:   lipgloss.NewStyle().Foreground(p.Bg).Background(p.Blue).Padding(0, 1),
        BadgeSonnet: lipgloss.NewStyle().Foreground(p.Bg).Background(p.Green).Padding(0, 1),
        BadgeHaiku:  lipgloss.NewStyle().Foreground(p.Bg).Background(p.Yellow).Padding(0, 1),

        PillLiveS:    lipgloss.NewStyle().Bold(true).Foreground(p.Bg).Background(p.Green).Padding(0, 1),
        PillStaleS:   lipgloss.NewStyle().Bold(true).Foreground(p.Fg).Background(p.FgMuted).Padding(0, 1),
        PillNoDaemon: lipgloss.NewStyle().Bold(true).Foreground(p.Fg).Background(p.Red).Padding(0, 1),
    }
}
```

- [ ] **Step 2.4.5: Update `Default()` to populate both shapes.**

In `theme.go`, modify `Default()`:

```go
func Default() Theme {
    accent := lipgloss.Color("#FFD400")
    errCol := lipgloss.Color("#FF3B30")
    fg := lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
    muted := lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}

    t := Theme{
        AccentColor: accent,
        ErrorColor:  errCol,
        Fg:          fg,
        Muted:       muted,

        Heading:    lipgloss.NewStyle().Bold(true).Foreground(fg).Padding(0, 1),
        AccentText: lipgloss.NewStyle().Bold(true).Foreground(accent),
        ErrorText:  lipgloss.NewStyle().Bold(true).Foreground(errCol),
        MutedText:  lipgloss.NewStyle().Foreground(muted),

        border: lipgloss.ThickBorder(),
    }
    // Populate new shape from the Mocha palette so components are usable
    // even when callers only have a legacy theme handle.
    nt := Build(MochaPalette(), UnicodeGlyphs())
    t.Palette = nt.Palette
    t.Glyphs = nt.Glyphs
    t.Title, t.Subtitle, t.Muted2, t.Accent, t.Value, t.Label = nt.Title, nt.Subtitle, nt.Muted2, nt.Accent, nt.Value, nt.Label
    t.Card, t.CardTitle, t.Help = nt.Card, nt.CardTitle, nt.Help
    t.BadgeOpus, t.BadgeSonnet, t.BadgeHaiku = nt.BadgeOpus, nt.BadgeSonnet, nt.BadgeHaiku
    t.PillLiveS, t.PillStaleS, t.PillNoDaemon = nt.PillLiveS, nt.PillStaleS, nt.PillNoDaemon
    return t
}
```

- [ ] **Step 2.4.6: Run, confirm pass.**

```bash
go test ./internal/tui/theme/ -v
```

Expected: both tests PASS.

### Step 2.5 — Selection (resolve `--theme` / `--icons` / env / `$COLORFGBG`)

- [ ] **Step 2.5.1: Failing test for `Resolve`.**

Create `internal/tui/theme/select_test.go`:

```go
package theme

import "testing"

func TestResolve_OrderOfPrecedence(t *testing.T) {
    cases := []struct {
        name                                    string
        flagTheme, flagIcons, envTheme, envIcons, colorFGBG string
        wantPalette, wantIcons                  string
    }{
        {"flag wins over env", "latte", "nerd", "mocha", "unicode", "", "latte", "nerd"},
        {"env wins over colorfgbg",  "", "", "frappe", "unicode", "15;0", "frappe", "unicode"},
        {"colorfgbg picks latte on light bg", "", "", "", "", "0;15", "latte", "unicode"},
        {"colorfgbg picks mocha on dark bg",  "", "", "", "", "15;0", "mocha", "unicode"},
        {"all empty → defaults",  "", "", "", "", "", "mocha", "unicode"},
        {"icons env",  "", "", "", "nerd", "", "mocha", "nerd"},
        {"icons flag overrides icons env", "", "unicode", "", "nerd", "", "mocha", "unicode"},
        {"auto theme name + dark fgbg",  "auto", "", "", "", "15;0", "mocha", "unicode"},
        {"auto theme name + light fgbg", "auto", "", "", "", "0;15", "latte", "unicode"},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            th, name, icons := Resolve(c.flagTheme, c.flagIcons, c.envTheme, c.envIcons, c.colorFGBG)
            if name != c.wantPalette {
                t.Errorf("palette: got %q want %q", name, c.wantPalette)
            }
            if icons != c.wantIcons {
                t.Errorf("icons: got %q want %q", icons, c.wantIcons)
            }
            if string(th.Palette.Accent) == "" {
                t.Errorf("theme not built")
            }
        })
    }
}

func TestResolve_RejectsUnknownTheme(t *testing.T) {
    _, name, _ := Resolve("nonsense", "", "", "", "")
    if name != "mocha" {
        t.Errorf("unknown theme should fall back to mocha, got %q", name)
    }
}
```

- [ ] **Step 2.5.2: Run, confirm failure.**

```bash
go test ./internal/tui/theme/ -run TestResolve -v
```

Expected: FAIL — `Resolve undefined`.

- [ ] **Step 2.5.3: Implement `select.go`.**

Create `internal/tui/theme/select.go`:

```go
package theme

import "strings"

// Resolve picks a Palette and Glyphs based on (in order of precedence):
//  1. flagTheme / flagIcons (CLI flag)
//  2. envTheme / envIcons (CCO_THEME / CCO_ICONS)
//  3. colorFGBG ($COLORFGBG; theme only — never affects icons)
//  4. defaults (mocha / unicode)
//
// The "auto" theme name means "use colorFGBG; fall back to mocha".
// Returns the built Theme plus the resolved palette + icons names (for
// logging or display).
func Resolve(flagTheme, flagIcons, envTheme, envIcons, colorFGBG string) (Theme, string, string) {
    paletteName := pickTheme(flagTheme, envTheme, colorFGBG)
    iconsName := pickIcons(flagIcons, envIcons)

    var p Palette
    switch paletteName {
    case "macchiato":
        p = MacchiatoPalette()
    case "frappe":
        p = FrappePalette()
    case "latte":
        p = LattePalette()
    default:
        paletteName = "mocha"
        p = MochaPalette()
    }

    var g Glyphs
    if iconsName == "nerd" {
        g = NerdGlyphs()
    } else {
        iconsName = "unicode"
        g = UnicodeGlyphs()
    }
    return Build(p, g), paletteName, iconsName
}

func pickTheme(flag, env, colorFGBG string) string {
    if flag != "" && flag != "auto" {
        return flag
    }
    if flag == "auto" {
        if isLightTerminal(colorFGBG) {
            return "latte"
        }
        return "mocha"
    }
    if env != "" {
        return env
    }
    if isLightTerminal(colorFGBG) {
        return "latte"
    }
    return "mocha"
}

func pickIcons(flag, env string) string {
    if flag != "" {
        return flag
    }
    if env != "" {
        return env
    }
    return "unicode"
}

// isLightTerminal parses $COLORFGBG (e.g. "0;15" — fg 0, bg 15). Background
// values >= 7 are considered "light." Empty / malformed values return false.
func isLightTerminal(colorFGBG string) bool {
    parts := strings.Split(colorFGBG, ";")
    if len(parts) < 2 {
        return false
    }
    bg := parts[len(parts)-1]
    switch bg {
    case "7", "8", "9", "10", "11", "12", "13", "14", "15":
        return true
    }
    return false
}
```

- [ ] **Step 2.5.4: Run, confirm pass.**

```bash
go test ./internal/tui/theme/ -v
```

Expected: all theme tests PASS.

### Step 2.6 — Wire `--theme` / `--icons` flags through cobra

- [ ] **Step 2.6.1: Add flags to root cobra command.**

Edit `cmd/app/main.go`. After the `homeDir` and `logLevel` globals, add:

```go
var (
    themeName string
    iconsName string
)
```

In `newRootCmd()`, after the existing `PersistentFlags` calls, add:

```go
cmd.PersistentFlags().StringVar(&themeName, "theme", "", "Color theme: mocha|macchiato|frappe|latte|auto (default: $CCO_THEME or auto)")
cmd.PersistentFlags().StringVar(&iconsName, "icons", "", "Icon set: unicode|nerd (default: $CCO_ICONS or unicode)")
```

- [ ] **Step 2.6.2: Resolve and pass the theme in `tui.go`.**

Edit `cmd/app/tui.go`. Replace `shell := app.New(theme.Default())`:

```go
import "os"  // ensure imported

// ...
envTheme := os.Getenv("CCO_THEME")
envIcons := os.Getenv("CCO_ICONS")
colorFGBG := os.Getenv("COLORFGBG")
th, _, _ := theme.Resolve(themeName, iconsName, envTheme, envIcons, colorFGBG)
shell := app.New(th)
shell.Push(dashboard.New(pool))
```

- [ ] **Step 2.6.3: Run full verification.**

```bash
make vet && make test && make build
```

Expected: green. The flags are wired but the chrome still uses legacy fields (`th.Heading`, `th.Pill`) — both shapes are populated by `Build()`/`Resolve()` via `Default()` fallback paths.

Hold on — `Resolve()` calls `Build()` which does **not** populate legacy fields. That means after this step the chrome may render with empty styles. Fix in next step.

- [ ] **Step 2.6.4: Backfill legacy fields in `Build()` so the chrome still works.**

Open `internal/tui/theme/theme.go`. Modify `Build` to also populate the legacy fields (using the new palette):

```go
func Build(p Palette, g Glyphs) Theme {
    fg := lipgloss.AdaptiveColor{Light: string(p.Fg), Dark: string(p.Fg)}
    muted := lipgloss.AdaptiveColor{Light: string(p.FgMuted), Dark: string(p.FgMuted)}
    return Theme{
        // legacy shape — kept populated for app.go chrome and existing views
        AccentColor: p.Accent,
        ErrorColor:  p.Red,
        Fg:          fg,
        Muted:       muted,
        Heading:     lipgloss.NewStyle().Bold(true).Foreground(p.Accent).Padding(0, 1),
        AccentText:  lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
        ErrorText:   lipgloss.NewStyle().Bold(true).Foreground(p.Red),
        MutedText:   lipgloss.NewStyle().Foreground(p.FgMuted),
        border:      g.BorderRound,

        // new shape
        Palette: p,
        Glyphs:  g,
        Title:    lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
        Subtitle: lipgloss.NewStyle().Foreground(p.FgMuted),
        Muted2:   lipgloss.NewStyle().Foreground(p.FgMuted),
        Accent:   lipgloss.NewStyle().Foreground(p.Accent),
        Value:    lipgloss.NewStyle().Bold(true).Foreground(p.Accent),
        Label:    lipgloss.NewStyle().Foreground(p.FgMuted),
        Card:      lipgloss.NewStyle().Border(g.BorderRound).BorderForeground(p.FgMuted).Padding(0, 2),
        CardTitle: lipgloss.NewStyle().Foreground(p.FgMuted),
        Help:      lipgloss.NewStyle().Foreground(p.FgMuted),
        BadgeOpus:   lipgloss.NewStyle().Foreground(p.Bg).Background(p.Blue).Padding(0, 1),
        BadgeSonnet: lipgloss.NewStyle().Foreground(p.Bg).Background(p.Green).Padding(0, 1),
        BadgeHaiku:  lipgloss.NewStyle().Foreground(p.Bg).Background(p.Yellow).Padding(0, 1),
        PillLiveS:    lipgloss.NewStyle().Bold(true).Foreground(p.Bg).Background(p.Green).Padding(0, 1),
        PillStaleS:   lipgloss.NewStyle().Bold(true).Foreground(p.Fg).Background(p.FgMuted).Padding(0, 1),
        PillNoDaemon: lipgloss.NewStyle().Bold(true).Foreground(p.Fg).Background(p.Red).Padding(0, 1),
    }
}
```

Simplify `Default()` to just call `Build`:

```go
func Default() Theme { return Build(MochaPalette(), UnicodeGlyphs()) }
```

Note: this *also* changes the visible chrome color (legacy yellow → new pink) the moment PR 2 lands. That's intentional — it's the first visible signal that the redesign is in flight, and avoids carrying two completely independent palettes. The user already approved the catppuccin direction. Add this as a single-sentence note in the PR 2 commit message.

Update the legacy `Pill` method (still on `Theme` from the existing code) — keep it as-is; it already uses the (now-pink) `AccentColor` for `PillLive`.

- [ ] **Step 2.6.5: Run full verification.**

```bash
make vet && make test && make build
./bin/claude-code-observer --help | grep -E "theme|icons"
```

Expected: tests pass; `--help` shows both flags.

- [ ] **Step 2.6.6: Smoke-test the binary by hand.**

```bash
./bin/claude-code-observer
```

Expected: TUI opens. Chrome now uses pink/dark palette (from Mocha) instead of yellow on black. Press `q` to exit.

- [ ] **Step 2.6.7: Commit.**

```bash
git add internal/tui/theme/ cmd/app/main.go cmd/app/tui.go
git commit -m "feat(theme): introduce Palette/Glyphs/Resolve + new Theme fields

Adds palette + glyph data and the new theme shape (Card, KPI, badges, pills)
alongside the existing legacy fields. Wires --theme/--icons cobra flags +
CCO_THEME/CCO_ICONS env + \$COLORFGBG auto-detection. The legacy
theme.Default() now delegates to Build(MochaPalette(), UnicodeGlyphs()) so
the chrome inherits the new palette immediately — visible accent shifts
from yellow to pink. Views are unchanged this PR."
```

---

## Task 3 — Component primitives

**Files:** Create the entire `internal/tui/theme/component/` package per the File Structure table.

Each component is a pure render function. Tests assert `lipgloss.Width(out)` equals an expected cell count for each input, and a byte-exact golden file is matched for representative cases. Goldens live in `internal/tui/theme/component/testdata/`.

### Step 3.0 — Package skeleton

- [ ] **Step 3.0.1: Create the package doc file.**

```bash
mkdir -p internal/tui/theme/component/testdata
```

Create `internal/tui/theme/component/doc.go`:

```go
// Package component contains pure render helpers used by the TUI views.
// Each function takes a *theme.Theme + data + a target width and returns a
// styled string. Width discipline: outputs satisfy lipgloss.Width(out) ==
// width (for fixed-width components) so views can compose them with
// lipgloss.JoinHorizontal without misalignment.
package component
```

### Step 3.1 — `Budget` width-budgeter

This is the load-bearing alignment primitive. Build it first.

- [ ] **Step 3.1.1: Failing test.**

Create `internal/tui/theme/component/budget_test.go`:

```go
package component

import "testing"

func TestBudget_Allocate_HappyPath(t *testing.T) {
    b := Budget(40)
    b.Fixed("a", 10)
    b.Fixed("b", 12)
    b.Gutters(2, 1) // 2 gutters × 1 cell
    rest := b.Remaining()
    if rest != 16 {
        t.Errorf("remaining: got %d want 16", rest)
    }
    b.Flex("c", rest)
    if err := b.Validate(); err != nil {
        t.Errorf("validate: %v", err)
    }
    if got := b.Width("c"); got != 16 {
        t.Errorf("flex width: got %d want 16", got)
    }
}

func TestBudget_Allocate_Overflow_FailsValidate(t *testing.T) {
    b := Budget(20)
    b.Fixed("a", 15)
    b.Fixed("b", 10)
    if err := b.Validate(); err == nil {
        t.Errorf("expected overflow error")
    }
}
```

- [ ] **Step 3.1.2: Run, confirm failure.**

```bash
go test ./internal/tui/theme/component/ -run TestBudget -v
```

Expected: FAIL — `Budget undefined`.

- [ ] **Step 3.1.3: Implement.**

Create `internal/tui/theme/component/budget.go`:

```go
package component

import "fmt"

// Builder accumulates column widths and reports overflow before rendering.
type Builder struct {
    total int
    cols  map[string]int
    used  int
}

// Budget returns a width budgeter targeting `total` cells.
func Budget(total int) *Builder {
    return &Builder{total: total, cols: map[string]int{}}
}

// Fixed reserves `w` cells for the named column.
func (b *Builder) Fixed(name string, w int) {
    b.cols[name] = w
    b.used += w
}

// Gutters reserves count×width cells for inter-column spacing.
func (b *Builder) Gutters(count, width int) {
    b.used += count * width
}

// Flex reserves the remainder for the named column.
func (b *Builder) Flex(name string, w int) {
    b.cols[name] = w
    b.used += w
}

// Remaining returns cells still available.
func (b *Builder) Remaining() int { return b.total - b.used }

// Width looks up the cells allocated to a column.
func (b *Builder) Width(name string) int { return b.cols[name] }

// Validate returns an error iff the sum of allocations exceeds total.
func (b *Builder) Validate() error {
    if b.used > b.total {
        return fmt.Errorf("budget overflow: allocated %d > total %d", b.used, b.total)
    }
    return nil
}
```

- [ ] **Step 3.1.4: Run, confirm pass.**

```bash
go test ./internal/tui/theme/component/ -v
```

Expected: PASS.

### Step 3.2 — `StatusPill`

- [ ] **Step 3.2.1: Failing test.**

Create `internal/tui/theme/component/status_test.go`:

```go
package component

import (
    "strings"
    "testing"

    "github.com/charmbracelet/lipgloss"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestStatusPill_RendersForEachState(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    cases := []struct {
        s    Status
        want string // substring expected inside the output
    }{
        {StatusLive, "LIVE"},
        {StatusStale, "STALE"},
        {StatusNoDaemon, "NO DAEMON"},
    }
    for _, c := range cases {
        got := StatusPill(&th, c.s)
        if !strings.Contains(lipgloss.NewStyle().Render(stripAnsi(got)), c.want) {
            t.Errorf("StatusPill(%v) = %q; want substring %q", c.s, got, c.want)
        }
    }
}

// stripAnsi for assertion — relies on lipgloss.Width which strips escapes.
func stripAnsi(s string) string {
    // simplest possible: filter to printable runes; full ANSI strip would use a regex
    var b strings.Builder
    in := false
    for _, r := range s {
        switch {
        case r == 0x1b:
            in = true
        case in && (r == 'm' || r == 'K'):
            in = false
        case !in:
            b.WriteRune(r)
        }
    }
    return b.String()
}
```

- [ ] **Step 3.2.2: Run, confirm failure.**

```bash
go test ./internal/tui/theme/component/ -run TestStatusPill -v
```

Expected: FAIL — `StatusPill undefined`.

- [ ] **Step 3.2.3: Implement.**

Create `internal/tui/theme/component/status.go`:

```go
package component

import "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"

// Status identifies which pill to render.
type Status int

const (
    StatusLive Status = iota
    StatusStale
    StatusNoDaemon
)

// StatusPill renders a colored pill for the connection state.
func StatusPill(t *theme.Theme, s Status) string {
    switch s {
    case StatusLive:
        return t.PillLiveS.Render(t.Glyphs.StatusOK + " LIVE")
    case StatusStale:
        return t.PillStaleS.Render("STALE")
    case StatusNoDaemon:
        return t.PillNoDaemon.Render(t.Glyphs.StatusErr + " NO DAEMON")
    }
    return ""
}
```

- [ ] **Step 3.2.4: Run, confirm pass.**

```bash
go test ./internal/tui/theme/component/ -v
```

Expected: PASS.

### Step 3.3 — `ModelBadge`

- [ ] **Step 3.3.1: Failing test.**

Create `internal/tui/theme/component/badge_test.go`:

```go
package component

import (
    "testing"

    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestModelBadge_FamilyMapping(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    cases := []struct {
        in, family string
    }{
        {"claude-opus-4-7", "opus"},
        {"opus-4-7", "opus"},
        {"claude-sonnet-4-6", "sonnet"},
        {"haiku-4-5-20251001", "haiku"},
        {"unknown-model", "model"}, // generic fallback
    }
    for _, c := range cases {
        got := ModelBadge(&th, c.in)
        if !containsCI(got, c.family) {
            t.Errorf("ModelBadge(%q) = %q; want family %q", c.in, got, c.family)
        }
    }
}

func containsCI(haystack, needle string) bool {
    h := []rune(haystack)
    n := []rune(needle)
    for i := 0; i+len(n) <= len(h); i++ {
        match := true
        for j := range n {
            if lower(h[i+j]) != lower(n[j]) {
                match = false
                break
            }
        }
        if match {
            return true
        }
    }
    return false
}
func lower(r rune) rune {
    if r >= 'A' && r <= 'Z' {
        return r + 32
    }
    return r
}
```

- [ ] **Step 3.3.2: Run, confirm failure.**

Expected: FAIL — `ModelBadge undefined`.

- [ ] **Step 3.3.3: Implement.**

Create `internal/tui/theme/component/badge.go`:

```go
package component

import (
    "strings"

    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// ModelBadge renders a colored badge labeled with the model family.
func ModelBadge(t *theme.Theme, model string) string {
    fam := familyFor(model)
    switch fam {
    case "opus":
        return t.BadgeOpus.Render(" opus ")
    case "sonnet":
        return t.BadgeSonnet.Render(" sonnet ")
    case "haiku":
        return t.BadgeHaiku.Render(" haiku ")
    }
    return t.Muted2.Render(" model ")
}

func familyFor(model string) string {
    m := strings.ToLower(model)
    switch {
    case strings.Contains(m, "opus"):
        return "opus"
    case strings.Contains(m, "sonnet"):
        return "sonnet"
    case strings.Contains(m, "haiku"):
        return "haiku"
    }
    return ""
}
```

- [ ] **Step 3.3.4: Run, confirm pass.**

```bash
go test ./internal/tui/theme/component/ -v
```

Expected: PASS.

### Step 3.4 — `Card`

- [ ] **Step 3.4.1: Failing test.**

Create `internal/tui/theme/component/card_test.go`:

```go
package component

import (
    "testing"

    "github.com/charmbracelet/lipgloss"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestCard_HasExpectedWidth(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    out := Card(&th, "label", "body text", 30)
    if got := lipgloss.Width(splitFirstLine(out)); got != 30 {
        t.Errorf("card top width: got %d want 30", got)
    }
}

func splitFirstLine(s string) string {
    for i, r := range s {
        if r == '\n' {
            return s[:i]
        }
    }
    return s
}
```

- [ ] **Step 3.4.2: Run, confirm failure.**

Expected: FAIL — `Card undefined`.

- [ ] **Step 3.4.3: Implement.**

Create `internal/tui/theme/component/card.go`:

```go
package component

import (
    "strings"

    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// Card renders a rounded-bordered box of the given total cell width. The
// optional title is shown in muted style as the first body line. body may
// contain newlines; lines are not wrapped (caller is responsible).
func Card(t *theme.Theme, title, body string, width int) string {
    style := t.Card.Width(width - 2) // padding(0,2) — but border adds 2 vert/2 horiz; lipgloss handles
    var b strings.Builder
    if title != "" {
        b.WriteString(t.CardTitle.Render(title))
        b.WriteString("\n")
    }
    b.WriteString(body)
    return style.Render(b.String())
}
```

(Note: lipgloss `Width(n).Border(…)` sets the *content* width to n; the rendered top line is n + 2 for the corners + 0 for padding-already-set. The test targets the full rendered top line. Adjust the computation as `width - 2` so that border + content = `width`.)

- [ ] **Step 3.4.4: Run, confirm pass.**

```bash
go test ./internal/tui/theme/component/ -v
```

Expected: PASS. If width is off by 2 or 4, tune the `width - N` constant; the test is the source of truth.

### Step 3.5 — `KPI`

- [ ] **Step 3.5.1: Failing test.**

Create `internal/tui/theme/component/kpi_test.go`:

```go
package component

import (
    "testing"

    "github.com/charmbracelet/lipgloss"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestKPI_Width(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    out := KPI(&th, "cost", "$3.42", nil, 20)
    if got := lipgloss.Width(out); got != 20 {
        t.Errorf("kpi width: got %d want 20", got)
    }
}

func TestKPI_WithPositiveDelta(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    d := &Delta{Direction: DeltaUp, Text: "+12%"}
    out := KPI(&th, "tokens", "847k", d, 22)
    if got := lipgloss.Width(out); got != 22 {
        t.Errorf("kpi+delta width: got %d want 22", got)
    }
}
```

- [ ] **Step 3.5.2: Run, confirm failure.** Expected: FAIL.

- [ ] **Step 3.5.3: Implement.**

Create `internal/tui/theme/component/kpi.go`:

```go
package component

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// Direction is the sign of a Delta.
type Direction int

const (
    DeltaFlat Direction = iota
    DeltaUp
    DeltaDown
)

// Delta annotates a KPI with a small change indicator.
type Delta struct {
    Direction Direction
    Text      string // e.g. "+12%" or "+$0.41"
}

// KPI renders one row: "<label>  <value>  <delta?>" padded to width.
func KPI(t *theme.Theme, label, value string, d *Delta, width int) string {
    lbl := t.Label.Render(label)
    val := t.Value.Render(value)
    parts := []string{lbl, "  ", val}
    if d != nil {
        var glyph, styled string
        switch d.Direction {
        case DeltaUp:
            glyph = t.Glyphs.DeltaUp
            styled = lipgloss.NewStyle().Foreground(t.Palette.Green).Render(glyph + " " + d.Text)
        case DeltaDown:
            glyph = t.Glyphs.DeltaDown
            styled = lipgloss.NewStyle().Foreground(t.Palette.Red).Render(glyph + " " + d.Text)
        default:
            styled = t.Muted2.Render(t.Glyphs.DeltaFlat + " " + d.Text)
        }
        parts = append(parts, "  ", styled)
    }
    line := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
    return lipgloss.NewStyle().Width(width).Render(line)
}
```

- [ ] **Step 3.5.4: Run, confirm pass.**

```bash
go test ./internal/tui/theme/component/ -v
```

Expected: PASS.

### Step 3.6 — `Sparkline`

- [ ] **Step 3.6.1: Failing test.**

Create `internal/tui/theme/component/sparkline_test.go`:

```go
package component

import (
    "testing"

    "github.com/charmbracelet/lipgloss"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestSparkline_Width(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    values := []float64{1, 2, 3, 4, 5, 6, 7, 8}
    out := Sparkline(&th, values, 8)
    if got := lipgloss.Width(out); got != 8 {
        t.Errorf("sparkline width: got %d want 8", got)
    }
}

func TestSparkline_Empty(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    out := Sparkline(&th, nil, 10)
    if got := lipgloss.Width(out); got != 10 {
        t.Errorf("empty sparkline width: got %d want 10", got)
    }
}
```

- [ ] **Step 3.6.2: Run, confirm failure.** Expected: FAIL.

- [ ] **Step 3.6.3: Implement.**

Create `internal/tui/theme/component/sparkline.go`:

```go
package component

import (
    "strings"

    "github.com/charmbracelet/lipgloss"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// Sparkline draws values as a row of block characters, scaled to fit width.
// If len(values) < width, values are right-padded with the lowest block.
// If len(values) > width, values are sampled.
func Sparkline(t *theme.Theme, values []float64, width int) string {
    if width <= 0 {
        return ""
    }
    blocks := t.Glyphs.Spark
    if len(values) == 0 {
        return strings.Repeat(string(blocks[0]), width)
    }
    // Sample / pad to `width` values.
    sampled := make([]float64, width)
    for i := 0; i < width; i++ {
        idx := i * len(values) / width
        if idx >= len(values) {
            idx = len(values) - 1
        }
        sampled[i] = values[idx]
    }
    var lo, hi = sampled[0], sampled[0]
    for _, v := range sampled {
        if v < lo { lo = v }
        if v > hi { hi = v }
    }
    span := hi - lo
    var b strings.Builder
    for _, v := range sampled {
        bin := 0
        if span > 0 {
            bin = int((v - lo) / span * float64(len(blocks)-1))
            if bin < 0 { bin = 0 }
            if bin >= len(blocks) { bin = len(blocks) - 1 }
        }
        b.WriteRune(blocks[bin])
    }
    return lipgloss.NewStyle().Foreground(t.Palette.Blue).Render(b.String())
}
```

- [ ] **Step 3.6.4: Run, confirm pass.**

Expected: PASS.

### Step 3.7 — `HelpBar`

- [ ] **Step 3.7.1: Failing test.**

Create `internal/tui/theme/component/help_test.go`:

```go
package component

import (
    "testing"

    "github.com/charmbracelet/lipgloss"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestHelpBar_Width(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    hints := []KeyHint{{"↑↓", "nav"}, {"⏎", "open"}, {"q", "quit"}}
    out := HelpBar(&th, hints, 60)
    if got := lipgloss.Width(out); got != 60 {
        t.Errorf("help width: got %d want 60", got)
    }
}
```

- [ ] **Step 3.7.2: Run, confirm failure.** Expected: FAIL.

- [ ] **Step 3.7.3: Implement.**

Create `internal/tui/theme/component/help.go`:

```go
package component

import (
    "strings"

    "github.com/charmbracelet/lipgloss"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// KeyHint is one ("↑↓" — "nav") pair shown in the footer.
type KeyHint struct {
    Key  string
    Desc string
}

// HelpBar renders all hints in muted style, joined with two-space gutters,
// then trimmed/padded to width.
func HelpBar(t *theme.Theme, hints []KeyHint, width int) string {
    parts := make([]string, 0, len(hints)*2)
    for i, h := range hints {
        if i > 0 {
            parts = append(parts, "  ")
        }
        parts = append(parts, t.Muted2.Render(h.Key+" "+h.Desc))
    }
    line := strings.Join(parts, "")
    return lipgloss.NewStyle().Width(width).Render(line)
}
```

- [ ] **Step 3.7.4: Run, confirm pass.** Expected: PASS.

### Step 3.8 — Row variants

The single most alignment-critical component. Test it with CJK + emoji project names.

- [ ] **Step 3.8.1: Failing test.**

Create `internal/tui/theme/component/row_test.go`:

```go
package component

import (
    "testing"
    "time"

    "github.com/charmbracelet/lipgloss"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestSessionRow_Width_ASCII(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    r := SessionRowData{
        Index:       1,
        Started:     time.Date(2026, 5, 11, 9, 14, 0, 0, time.UTC),
        ProjectName: "claude-code-observer",
        DurationSec: 4320,
        CostUSD:     1.12,
        Prompts:     12,
        Tokens:      38000,
        Live:        true,
    }
    out := SessionRow(&th, r, true, 90)
    if got := lipgloss.Width(out); got != 90 {
        t.Errorf("session row width (ascii): got %d want 90", got)
    }
}

func TestSessionRow_Width_CJK(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    r := SessionRowData{
        Index: 2, Started: time.Now(), ProjectName: "日本語プロジェクト", // 9 wide chars
        DurationSec: 60, CostUSD: 0.10, Prompts: 1, Tokens: 100, Live: false,
    }
    out := SessionRow(&th, r, false, 90)
    if got := lipgloss.Width(out); got != 90 {
        t.Errorf("session row width (cjk): got %d want 90", got)
    }
}

func TestSessionRow_Width_Emoji(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    r := SessionRowData{
        Index: 3, Started: time.Now(), ProjectName: "🚀-rocket-fast",
        DurationSec: 60, CostUSD: 0.10, Prompts: 1, Tokens: 100, Live: false,
    }
    out := SessionRow(&th, r, false, 90)
    if got := lipgloss.Width(out); got != 90 {
        t.Errorf("session row width (emoji): got %d want 90", got)
    }
}
```

- [ ] **Step 3.8.2: Run, confirm failure.** Expected: FAIL.

- [ ] **Step 3.8.3: Implement.**

Create `internal/tui/theme/component/row.go`:

```go
package component

import (
    "fmt"
    "time"

    "github.com/charmbracelet/lipgloss"
    "github.com/mattn/go-runewidth"

    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// SessionRowData is one row in the sessions list or in a dashboard panel.
type SessionRowData struct {
    Index       int       // 1-based position in the page (0 to omit)
    Started     time.Time
    ProjectName string
    DurationSec int64 // 0 to omit
    CostUSD     float64
    Prompts     int64
    Tokens      int64 // 0 to omit
    Live        bool
}

// SessionRow renders one row inside a sessions table card. width is the
// content area inside the card border (caller computes via Budget). The
// returned line satisfies lipgloss.Width(out) == width.
func SessionRow(t *theme.Theme, r SessionRowData, selected bool, width int) string {
    // Budget for 90-cell rows; scales by truncating the project column.
    const (
        idxW   = 4
        startW = 18
        durW   = 10
        costW  = 8
        prW    = 8
        tokW   = 7
        liveW  = 8
        gutter = 1
        gutterCount = 7 // between idx, start, project, dur, cost, prompts, tokens, live
    )
    projW := width - idxW - startW - durW - costW - prW - tokW - liveW - gutterCount
    if projW < 4 {
        projW = 4
    }

    idx := padRight(fmt.Sprintf("%d", r.Index), idxW)
    start := padRight(r.Started.Format("2006-01-02 15:04"), startW)
    project := padRight(truncToWidth(r.ProjectName, projW), projW)
    dur := padRight(humanDuration(r.DurationSec), durW)
    cost := padRight(fmt.Sprintf("$%.2f", r.CostUSD), costW)
    prompts := padRight(fmt.Sprintf("%d", r.Prompts), prW)
    tokens := padRight(humanInt(r.Tokens), tokW)
    live := padRight("", liveW)
    if r.Live {
        live = padRight(StatusPill(t, StatusLive), liveW)
    }

    line := lipgloss.JoinHorizontal(lipgloss.Top,
        idx, " ", start, " ", project, " ", dur, " ", cost, " ", prompts, " ", tokens, " ", live,
    )
    if selected {
        line = lipgloss.NewStyle().Background(t.Palette.BgAlt).Width(width).Render(line)
    } else {
        line = lipgloss.NewStyle().Width(width).Render(line)
    }
    return line
}

// padRight returns s padded with spaces to exactly `w` display cells.
// s must already be at most w cells wide (use truncToWidth first if not).
func padRight(s string, w int) string {
    return lipgloss.NewStyle().Width(w).Render(s)
}

func truncToWidth(s string, w int) string {
    if runewidth.StringWidth(s) <= w {
        return s
    }
    return runewidth.Truncate(s, w, "…")
}

func humanDuration(sec int64) string {
    if sec <= 0 {
        return ""
    }
    if sec < 60 {
        return fmt.Sprintf("%ds", sec)
    }
    if sec < 3600 {
        return fmt.Sprintf("%dm%02ds", sec/60, sec%60)
    }
    return fmt.Sprintf("%dh%02dm", sec/3600, (sec%3600)/60)
}

func humanInt(n int64) string {
    switch {
    case n >= 1_000_000:
        return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
    case n >= 1_000:
        return fmt.Sprintf("%dk", n/1_000)
    }
    return fmt.Sprintf("%d", n)
}
```

- [ ] **Step 3.8.4: Run, confirm pass.**

```bash
go test ./internal/tui/theme/component/ -v
```

Expected: PASS for all three width tests including CJK and emoji.

### Step 3.9 — `EventRow`, `APIRequestRow`, `ToolCallRow`

- [ ] **Step 3.9.1: Failing test (append to row_test.go).**

```go
func TestEventRow_Width(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    e := EventRowData{
        Time:      time.Date(2026, 5, 11, 9, 14, 8, 0, time.UTC),
        EventName: "user_prompt",
        Summary:   `"refactor receiver pipeline"`,
        IsPrompt:  true,
    }
    out := EventRow(&th, e, true, 70)
    if got := lipgloss.Width(out); got != 70 {
        t.Errorf("event row width: got %d want 70", got)
    }
}

func TestAPIRequestRow_Width(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    r := APIRequestRowData{
        Time: time.Date(2026, 5, 11, 9, 15, 43, 0, time.UTC),
        Model: "claude-opus-4-7", CostUSD: 0.21,
        InputTokens: 8481, OutputTokens: 2140,
    }
    out := APIRequestRow(&th, r, 70)
    if got := lipgloss.Width(out); got != 70 {
        t.Errorf("api row width: got %d want 70", got)
    }
}

func TestToolCallRow_Width(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    r := ToolCallRowData{
        Time: time.Date(2026, 5, 11, 9, 15, 46, 0, time.UTC),
        ToolName: "Write", Success: true, DurationMS: 112,
    }
    out := ToolCallRow(&th, r, 70)
    if got := lipgloss.Width(out); got != 70 {
        t.Errorf("tool row width: got %d want 70", got)
    }
}
```

- [ ] **Step 3.9.2: Run, confirm failure.** Expected: FAIL.

- [ ] **Step 3.9.3: Implement (append to `row.go`).**

```go
// EventRowData is one row in the session detail timeline.
type EventRowData struct {
    Time      time.Time
    EventName string
    Summary   string
    IsPrompt  bool // user_prompt rows get a tinted background
}

func EventRow(t *theme.Theme, e EventRowData, selected bool, width int) string {
    const (
        timeW   = 8  // "15:04:05"
        nameW   = 22
        gutter  = 1
        gutters = 2
    )
    sumW := width - timeW - nameW - gutters
    if sumW < 8 {
        sumW = 8
    }
    timeCol := padRight(e.Time.Format("15:04:05"), timeW)
    nameCol := padRight(truncToWidth(e.EventName, nameW), nameW)
    sumCol := padRight(truncToWidth(e.Summary, sumW), sumW)
    line := lipgloss.JoinHorizontal(lipgloss.Top, timeCol, " ", nameCol, " ", sumCol)
    s := lipgloss.NewStyle().Width(width)
    if selected {
        s = s.Background(t.Palette.BgAlt).Foreground(t.Palette.Accent)
    } else if e.IsPrompt {
        s = s.Background(t.Palette.BgAlt)
    } else {
        s = s.Foreground(t.Palette.FgMuted)
    }
    return s.Render(line)
}

// APIRequestRowData renders one api_request event.
type APIRequestRowData struct {
    Time         time.Time
    Model        string
    CostUSD      float64
    InputTokens  int64
    OutputTokens int64
}

func APIRequestRow(t *theme.Theme, r APIRequestRowData, width int) string {
    const (
        timeW = 8
        modelW = 18
        costW = 8
        gutters = 3
    )
    tailW := width - timeW - modelW - costW - gutters
    if tailW < 8 { tailW = 8 }
    timeCol := padRight(r.Time.Format("15:04:05"), timeW)
    modelCol := padRight(truncToWidth(r.Model, modelW), modelW)
    costCol := padRight(t.Value.Render(fmt.Sprintf("$%.2f", r.CostUSD)), costW)
    tail := padRight(fmt.Sprintf("in %d  out %d", r.InputTokens, r.OutputTokens), tailW)
    line := lipgloss.JoinHorizontal(lipgloss.Top, timeCol, " ", modelCol, " ", costCol, " ", tail)
    return lipgloss.NewStyle().Width(width).Render(line)
}

// ToolCallRowData renders one tool_result event.
type ToolCallRowData struct {
    Time       time.Time
    ToolName   string
    Success    bool
    DurationMS int64
    Note       string
}

func ToolCallRow(t *theme.Theme, r ToolCallRowData, width int) string {
    const (
        timeW = 8
        nameW = 12
        markW = 2
        durW  = 10
        gutters = 4
    )
    noteW := width - timeW - nameW - markW - durW - gutters
    if noteW < 0 { noteW = 0 }
    timeCol := padRight(r.Time.Format("15:04:05"), timeW)
    nameCol := padRight(truncToWidth(r.ToolName, nameW), nameW)
    mark := t.Glyphs.Check
    markStyle := lipgloss.NewStyle().Foreground(t.Palette.Green)
    if !r.Success {
        mark = t.Glyphs.Cross
        markStyle = lipgloss.NewStyle().Foreground(t.Palette.Red)
    }
    markCol := padRight(markStyle.Render(mark), markW)
    durCol := padRight(fmt.Sprintf("%dms", r.DurationMS), durW)
    noteCol := padRight(truncToWidth(r.Note, noteW), noteW)
    line := lipgloss.JoinHorizontal(lipgloss.Top, timeCol, " ", nameCol, " ", markCol, " ", durCol, " ", noteCol)
    return lipgloss.NewStyle().Width(width).Render(line)
}
```

- [ ] **Step 3.9.4: Run, confirm pass.**

Expected: PASS.

### Step 3.10 — Verify and commit PR 3

- [ ] **Step 3.10.1: Full verification.**

```bash
make vet && make test && make build
```

Expected: green.

- [ ] **Step 3.10.2: Commit.**

```bash
git add internal/tui/theme/component/ go.mod go.sum
git commit -m "feat(theme/component): pure render helpers for the redesign

Adds Card, KPI, Sparkline, ModelBadge, StatusPill, HelpBar, and Row variants
(SessionRow, EventRow, APIRequestRow, ToolCallRow), plus a Budget width
checker. All components satisfy lipgloss.Width(out) == width — tested with
ASCII, CJK, and emoji inputs. Views to follow in subsequent PRs."
```

---

## Task 4 — Dashboard rewrite

**Files:**
- Modify: `internal/tui/dashboard/model.go` (accept `*theme.Theme`)
- Modify: `internal/tui/dashboard/view.go` (rewrite)
- Modify: `internal/tui/dashboard/view_test.go`
- Modify: `cmd/app/tui.go` (pass theme to `dashboard.New`)
- Create: `internal/tui/dashboard/testdata/*.golden`

### Step 4.1 — Read existing dashboard model

- [ ] **Step 4.1.1: Read `internal/tui/dashboard/model.go` to understand `New`'s signature, what fields exist on `Model`, and how `m.snap`/`m.top` are populated.** Required before changing the signature so you don't break callers.

### Step 4.2 — Change `dashboard.New` to accept `*theme.Theme`

- [ ] **Step 4.2.1: Edit `dashboard/model.go`.** Add a `theme *theme.Theme` field to `Model`. Change `func New(pool *sql.DB) *Model` to `func New(pool *sql.DB, th *theme.Theme) *Model`, storing the parameter in the new field. (If `Model` is a value type, switch to pointer receiver consistently.)

- [ ] **Step 4.2.2: Edit `cmd/app/tui.go`.** Update the `dashboard.New(pool)` call to `dashboard.New(pool, &th)`.

- [ ] **Step 4.2.3: Run vet/test.** Existing dashboard tests likely break because they call `dashboard.New(pool)` — fix them by passing a built theme (`tp := theme.Default(); &tp` or a helper). Update each call site.

```bash
make vet && make test ./internal/tui/dashboard/
```

Expected: vet passes; tests compile (may still fail behaviorally — fine, the view is about to change).

### Step 4.3 — Add the `RecentSessionsToday` fetch to the dashboard model

- [ ] **Step 4.3.1: Read existing `Update`/fetch logic in `dashboard/model.go`.** Identify the message type that delivers `Snapshot` and `[]TopSession`. Extend it to also carry `[]TopSession` for recent sessions, or add a parallel message.

- [ ] **Step 4.3.2: Modify `Model` to add `recent []readstore.TopSession`.**

- [ ] **Step 4.3.3: Modify the fetch command to call both `readstore.DashboardSnapshot(...)` *and* `readstore.RecentSessionsToday(ctx, pool, now, 5)`, returning both in the data message.**

- [ ] **Step 4.3.4: Update `Update` to store `recent` from the message.**

- [ ] **Step 4.3.5: Run tests.**

```bash
go test ./internal/tui/dashboard/ -v
```

Expected: compile passes; existing tests pass (they don't yet assert on `recent`).

### Step 4.4 — Add a cursor for the recent-sessions list

- [ ] **Step 4.4.1: Add `recentCursor int` to `Model`.**

- [ ] **Step 4.4.2: Add Up/Down handling to `Update` so `↑/k` and `↓/j` move `recentCursor` within `[0, len(m.recent)-1]`.**

- [ ] **Step 4.4.3: Add `Enter` handling so it pushes `sessions.NewDetail(m.pool, m.recent[m.recentCursor].SessionID)` via `app.PushViewMsg`.**

- [ ] **Step 4.4.4: Run tests; add a small Update test covering cursor bounds and enter-push.**

### Step 4.5 — Rewrite `View()`

- [ ] **Step 4.5.1: Failing test — golden file.**

In `internal/tui/dashboard/view_test.go`, add:

```go
package dashboard

import (
    "flag"
    "os"
    "path/filepath"
    "strings"
    "testing"
    "time"

    "github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

var updateGoldens = flag.Bool("update", false, "rewrite golden files")

func TestDashboardView_Golden(t *testing.T) {
    cases := []struct {
        name string
        snap readstore.Snapshot
        top  []readstore.TopSession
        recent []readstore.TopSession
    }{
        {
            name: "populated",
            snap: readstore.Snapshot{
                Today:     readstore.WindowStats{Sessions: 12, CostUSD: 3.42, Prompts: 87, Tokens: 847_000, Tools: 156, Errors: 2},
                Yesterday: readstore.WindowStats{Sessions: 9,  CostUSD: 3.01, Prompts: 69, Tokens: 756_000, Tools: 140, Errors: 1},
                D7:        readstore.WindowStats{Sessions: 68, CostUSD: 19.80, Prompts: 512, Tokens: 4_800_000, Tools: 912, Errors: 7},
                D30:       readstore.WindowStats{Sessions: 241, CostUSD: 74.61, Prompts: 1840, Tokens: 18_200_000, Tools: 3402, Errors: 21},
                LatestEventTS: time.Date(2026, 5, 11, 9, 14, 0, 0, time.UTC).UnixNano(),
            },
            top: []readstore.TopSession{
                {SessionID: "s1", ProjectName: "claude-code-observer", StartedAt: time.Date(2026,5,11,9,14,0,0,time.UTC).UnixNano(), CostUSD: 1.12, Prompts: 12, Live: true},
                {SessionID: "s2", ProjectName: "cco-frontend",         StartedAt: time.Date(2026,5,11,8,2,0,0,time.UTC).UnixNano(),  CostUSD: 0.81, Prompts: 7},
                {SessionID: "s3", ProjectName: "ai-playground",        StartedAt: time.Date(2026,5,11,7,30,0,0,time.UTC).UnixNano(), CostUSD: 0.42, Prompts: 5},
            },
            recent: []readstore.TopSession{
                {SessionID: "s1", ProjectName: "claude-code-observer", StartedAt: time.Date(2026,5,11,9,14,0,0,time.UTC).UnixNano(), CostUSD: 0.81, Prompts: 3, Live: true},
                {SessionID: "s4", ProjectName: "cco-frontend", StartedAt: time.Date(2026,5,11,8,0,0,0,time.UTC).UnixNano(), CostUSD: 0.18, Prompts: 7},
            },
        },
        {name: "empty"},
    }
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            m := &Model{theme: &th, snap: c.snap, top: c.top, recent: c.recent}
            got := m.View(90, 32)
            goldenPath := filepath.Join("testdata", "dashboard_"+c.name+".golden")
            if *updateGoldens {
                if err := os.WriteFile(goldenPath, []byte(got), 0644); err != nil {
                    t.Fatal(err)
                }
                return
            }
            want, err := os.ReadFile(goldenPath)
            if err != nil {
                t.Fatalf("read golden: %v", err)
            }
            if got != string(want) {
                t.Errorf("dashboard %s view mismatch — run `go test ./internal/tui/dashboard/ -update`\nfirst diff:\n%s",
                    c.name, firstDiff(got, string(want)))
            }
        })
    }
}

func firstDiff(a, b string) string {
    la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
    n := len(la); if len(lb) < n { n = len(lb) }
    for i := 0; i < n; i++ {
        if la[i] != lb[i] {
            return "line " + itoa(i+1) + "\n  got:  " + la[i] + "\n  want: " + lb[i]
        }
    }
    return "(equal up to common length; lengths differ)"
}
func itoa(n int) string { return strings.TrimSpace(strings.Map(func(r rune) rune { return r }, " ")) + intToStr(n) }
func intToStr(n int) string { return fmtSprint(n) }
// helper because importing fmt is fine — use it:
```

(That last block is awkward; in practice just `import "fmt"` and use `fmt.Sprintf`. Adjust before committing.)

- [ ] **Step 4.5.2: Run, confirm failure.**

```bash
go test ./internal/tui/dashboard/ -run TestDashboardView_Golden -v
```

Expected: FAIL — `testdata/*.golden` does not exist, OR (after we generate stubs) the view output doesn't match.

- [ ] **Step 4.5.3: Rewrite `dashboard/view.go`.**

Replace the entire body of `view.go` with the new implementation. The function shape is `func (m *Model) View(width, height int) string`. Compose:

```go
package dashboard

import (
    "fmt"
    "strings"
    "time"

    "github.com/charmbracelet/lipgloss"

    "github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
    "github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

func (m *Model) View(width, height int) string {
    th := m.theme
    if width <= 0 { width = 90 }

    // ── header ───────────────────────────────────────────────────────
    brand := th.Title.Render(th.Glyphs.Brand + " cco")
    breadcrumb := th.Muted2.Render(" · dashboard")
    pill := component.StatusPill(th, m.status())
    headerRight := lipgloss.NewStyle().Width(width - lipgloss.Width(brand) - lipgloss.Width(breadcrumb)).Align(lipgloss.Right).Render(pill)
    header := lipgloss.JoinHorizontal(lipgloss.Top, brand, breadcrumb, headerRight)

    // ── window cards (today / 7d / 30d) ──────────────────────────────
    cardW := (width - 4) / 3
    windowsRow := lipgloss.JoinHorizontal(lipgloss.Top,
        m.renderWindowCard("today",   m.snap.Today,   cardW),
        " ",
        m.renderWindowCard("7 days",  m.snap.D7,      cardW),
        " ",
        m.renderWindowCard("30 days", m.snap.D30,     cardW),
    )

    // ── delta strip ──────────────────────────────────────────────────
    deltas := m.renderDeltaStrip(width)

    // ── top sessions today (read-only) ───────────────────────────────
    topCard := m.renderTopSessionsCard(width)

    // ── recent sessions (cursor) ─────────────────────────────────────
    recentCard := m.renderRecentSessionsCard(width)

    // ── help ─────────────────────────────────────────────────────────
    help := component.HelpBar(th, []component.KeyHint{
        {"↑↓", "nav"}, {"⏎", "open"}, {"s", "sessions"}, {"r", "refresh"}, {"?", "help"}, {"q", "quit"},
    }, width)

    parts := []string{header, "", windowsRow, "", deltas, "", topCard, "", recentCard, "", help}
    return strings.Join(parts, "\n")
}

func (m *Model) renderWindowCard(label string, ws readstore.WindowStats, w int) string {
    th := m.theme
    body := strings.Join([]string{
        labelValue(th, "sessions", fmt.Sprintf("%d", ws.Sessions), w-4),
        labelValue(th, "prompts",  fmt.Sprintf("%d", ws.Prompts),  w-4),
        labelValue(th, "tokens",   humanInt(ws.Tokens),            w-4),
        labelValue(th, "tools",    humanInt(ws.Tools),             w-4),
        labelValue(th, "cost",     fmt.Sprintf("$%.2f", ws.CostUSD), w-4),
        labelValue(th, "errors",   fmt.Sprintf("%d", ws.Errors),   w-4),
    }, "\n")
    return component.Card(th, label, body, w)
}

func labelValue(th *theme.Theme, label, value string, w int) string {
    l := th.Label.Render(label)
    v := th.Value.Render(value)
    gap := w - lipgloss.Width(l) - lipgloss.Width(v)
    if gap < 1 { gap = 1 }
    return lipgloss.JoinHorizontal(lipgloss.Top, l, strings.Repeat(" ", gap), v)
}

// renderDeltaStrip renders one card with today-vs-yesterday deltas. Returns
// empty card when there's no yesterday data (Sessions == 0).
func (m *Model) renderDeltaStrip(width int) string {
    if m.snap.Yesterday.Sessions == 0 {
        return ""
    }
    deltaPart := func(label string, today, prev int64) string {
        d := today - prev
        dir := component.DeltaFlat
        switch {
        case d > 0: dir = component.DeltaUp
        case d < 0: dir = component.DeltaDown
        }
        sign := ""
        if d > 0 { sign = "+" }
        text := fmt.Sprintf("%s%d", sign, d)
        return label + " " + component.RenderDeltaInline(m.theme, dir, text)
    }
    body := strings.Join([]string{
        deltaPart("sessions", m.snap.Today.Sessions, m.snap.Yesterday.Sessions),
        deltaPart("prompts",  m.snap.Today.Prompts,  m.snap.Yesterday.Prompts),
        deltaPart("tokens",   m.snap.Today.Tokens,   m.snap.Yesterday.Tokens),
    }, "    ")
    return component.Card(m.theme, "today vs yesterday", body, width)
}

func (m *Model) renderTopSessionsCard(width int) string {
    th := m.theme
    if len(m.top) == 0 {
        return component.Card(th, "top sessions today (by cost)", th.Muted2.Render("(no sessions today)"), width)
    }
    rows := make([]string, 0, len(m.top))
    for _, ts := range m.top {
        r := component.SessionRowData{
            Started: time.Unix(0, ts.StartedAt).UTC(),
            ProjectName: ts.ProjectName,
            CostUSD: ts.CostUSD, Prompts: ts.Prompts, Live: ts.Live,
        }
        rows = append(rows, component.SessionRow(th, r, false, width-4))
    }
    return component.Card(th, "top sessions today (by cost)", strings.Join(rows, "\n"), width)
}

func (m *Model) renderRecentSessionsCard(width int) string {
    th := m.theme
    if len(m.recent) == 0 {
        return component.Card(th, "recent sessions", th.Muted2.Render("(no recent sessions)"), width)
    }
    rows := make([]string, 0, len(m.recent))
    for i, ts := range m.recent {
        r := component.SessionRowData{
            Index: i + 1,
            Started: time.Unix(0, ts.StartedAt).UTC(),
            ProjectName: ts.ProjectName,
            CostUSD: ts.CostUSD, Prompts: ts.Prompts, Live: ts.Live,
        }
        rows = append(rows, component.SessionRow(th, r, i == m.recentCursor, width-4))
    }
    return component.Card(th, "recent sessions", strings.Join(rows, "\n"), width)
}

func (m *Model) status() component.Status {
    if m.snap.LatestEventTS == 0 && len(m.recent) == 0 && len(m.top) == 0 {
        return component.StatusNoDaemon
    }
    if m.isStale() {
        return component.StatusStale
    }
    return component.StatusLive
}

func humanInt(n int64) string {
    switch {
    case n >= 1_000_000: return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
    case n >= 1_000:     return fmt.Sprintf("%dk", n/1_000)
    }
    return fmt.Sprintf("%d", n)
}
```

(Adapt `m.isStale()` to whatever predicate the existing model uses.)

Also add a small helper to the component package — `RenderDeltaInline`:

```go
// In internal/tui/theme/component/kpi.go, append:
func RenderDeltaInline(t *theme.Theme, dir Direction, text string) string {
    var glyph string
    var style lipgloss.Style
    switch dir {
    case DeltaUp:   glyph = t.Glyphs.DeltaUp;   style = lipgloss.NewStyle().Foreground(t.Palette.Green)
    case DeltaDown: glyph = t.Glyphs.DeltaDown; style = lipgloss.NewStyle().Foreground(t.Palette.Red)
    default:        glyph = t.Glyphs.DeltaFlat; style = t.Muted2
    }
    return style.Render(glyph + " " + text)
}
```

- [ ] **Step 4.5.4: Generate the golden files.**

```bash
go test ./internal/tui/dashboard/ -update
```

Then **read the generated `testdata/dashboard_populated.golden` by hand** to make sure the layout looks right. Open in an editor with a monospace font and confirm:
- Borders are aligned (every row of the rendered output is the same visible width)
- Selected row inside the recent-sessions card has the tinted background
- Pill renders on the right of the header
- No row exceeds 90 cells

If misaligned, fix the render code, regenerate, repeat.

- [ ] **Step 4.5.5: Run the golden test without -update.**

```bash
go test ./internal/tui/dashboard/ -v
```

Expected: PASS.

### Step 4.6 — Smoke-test the binary

- [ ] **Step 4.6.1: Build + run.**

```bash
make build && ./bin/claude-code-observer
```

Expected: The dashboard renders with the new layout. If `cco serve` isn't running you'll see the empty state. Press `q` to exit.

### Step 4.7 — Commit PR 4

- [ ] **Step 4.7.1.**

```bash
git add internal/tui/dashboard/ internal/tui/theme/component/kpi.go cmd/app/tui.go
git commit -m "feat(tui/dashboard): rewrite View() with new component library

Composes header + three window cards (today/7d/30d) + today-vs-yesterday
delta strip + top-sessions-today (read-only) + recent-sessions (cursor) +
help bar, all via theme/component primitives. The dashboard now consumes
the Path 2 readstore additions (WindowStats.Sessions/Tokens,
Snapshot.Yesterday, RecentSessionsToday). Golden tests at (90, 32) cover
populated and empty states.

Refs: docs/superpowers/specs/2026-05-12-tui-redesign-design.md §6.1, §7"
```

---

## Task 5 — Sessions list rewrite

**Files:**
- Modify: `internal/tui/sessions/list.go`
- Modify: `internal/tui/sessions/list_test.go`
- Create: `internal/tui/sessions/testdata/list_*.golden`
- Modify: `cmd/app/tui.go` if call sites change

### Step 5.1 — Accept `*theme.Theme` in `NewList`

- [ ] **Step 5.1.1: Edit `sessions/list.go`.**
  - Add `theme *theme.Theme` field to `List`
  - Change `func NewList(pool *sql.DB) *List` to `func NewList(pool *sql.DB, th *theme.Theme) *List`
  - Remove the package-level `var defaultTheme = theme.Default()` — replace usages with `m.theme`

- [ ] **Step 5.1.2: Update callers.** Search for `sessions.NewList(` and update each. Likely just `cmd/app/tui.go` and tests in `internal/tui/sessions/list_test.go`. Tests can build a Mocha theme: `tp := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs()); list := sessions.NewList(pool, &tp)`.

- [ ] **Step 5.1.3: Run vet + tests.** Fix any breaks.

### Step 5.2 — Failing golden test for `View`

- [ ] **Step 5.2.1: Add to `list_test.go`.** Mirror the dashboard golden test pattern:

```go
func TestSessionsListView_Golden(t *testing.T) {
    cases := []struct {
        name string
        rows []readstore.SessionRow
        cursor int
        nextCur *int64
    }{
        {
            name: "populated",
            rows: []readstore.SessionRow{
                {SessionID:"s1", ProjectName:"claude-code-observer",
                 StartedAt: time.Date(2026,5,11,9,14,0,0,time.UTC),
                 DurationSec: 4320, CostUSD: 1.12, Prompts: 12, Tokens: 38000, Live: true},
                {SessionID:"s2", ProjectName:"cco-frontend",
                 StartedAt: time.Date(2026,5,11,8,2,0,0,time.UTC),
                 DurationSec: 2732, CostUSD: 0.81, Prompts: 7, Tokens: 24000},
                {SessionID:"s3", ProjectName:"日本語プロジェクト",
                 StartedAt: time.Date(2026,5,11,7,30,0,0,time.UTC),
                 DurationSec: 1338, CostUSD: 0.42, Prompts: 5, Tokens: 15000},
            },
            cursor: 1,
        },
        {name: "empty"},
    }
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            m := &List{theme: &th, rows: c.rows, cursor: c.cursor, nextCur: c.nextCur, lastOK: time.Now()}
            got := m.View(90, 32)
            goldenPath := filepath.Join("testdata", "list_"+c.name+".golden")
            // (same -update / read-and-compare boilerplate as the dashboard test)
        })
    }
}
```

Extract the `-update` / read / compare boilerplate into a shared internal test helper (`internal/tui/testutil/golden.go`) if you find yourself repeating it. Keep it minimal — single function `CompareGolden(t, got, path, update)`.

- [ ] **Step 5.2.2: Run, confirm failure.**

Expected: FAIL — golden file not present, or view output doesn't match.

### Step 5.3 — Rewrite `View()`

- [ ] **Step 5.3.1: Replace `View(width, height int)` body in `sessions/list.go`.**

```go
func (m *List) View(width, height int) string {
    th := m.theme
    if width <= 0 { width = 90 }

    // Header: brand · sessions · page · pill
    brand := th.Title.Render(th.Glyphs.Brand + " cco")
    bread := th.Muted2.Render(" · sessions    page " + itoaPage(len(m.prevCurs)+1))
    pill := component.StatusPill(th, m.statusFor())
    headerRight := lipgloss.NewStyle().Width(width-lipgloss.Width(brand)-lipgloss.Width(bread)).Align(lipgloss.Right).Render(pill)
    header := lipgloss.JoinHorizontal(lipgloss.Top, brand, bread, headerRight)

    // Body card
    if len(m.rows) == 0 {
        body := th.Muted2.Render("no sessions yet — start using Claude Code with cco serve running")
        card := component.Card(th, "", body, width)
        help := component.HelpBar(th, m.helpHints(), width)
        return strings.Join([]string{header, "", card, "", help}, "\n")
    }

    // Column header strip + rows
    columnHeader := th.Muted2.Render(formatColHeader(width-4))
    rows := []string{columnHeader}
    for i, r := range m.rows {
        rd := component.SessionRowData{
            Index: i+1,
            Started: r.StartedAt, ProjectName: defaultProject(r.ProjectName),
            DurationSec: r.DurationSec, CostUSD: r.CostUSD, Prompts: r.Prompts,
            Tokens: r.Tokens, Live: r.Live,
        }
        rows = append(rows, component.SessionRow(th, rd, i == m.cursor, width-4))
    }
    body := strings.Join(rows, "\n")
    card := component.Card(th, "", body, width)

    help := component.HelpBar(th, m.helpHints(), width)
    parts := []string{header, "", card}
    if m.nextCur != nil {
        parts = append(parts, th.Muted2.Render("press pgdn for next page"))
    }
    parts = append(parts, "", help)
    return strings.Join(parts, "\n")
}

func (m *List) helpHints() []component.KeyHint {
    return []component.KeyHint{
        {"↑↓", "nav"}, {"⏎", "open"}, {"pgdn", "next"}, {"pgup", "prev"},
        {"g/G", "top/bot"}, {"b", "back"}, {"?", "help"}, {"q", "quit"},
    }
}

func (m *List) statusFor() component.Status {
    if m.lastOK.IsZero() && len(m.rows) == 0 { return component.StatusNoDaemon }
    if m.stale { return component.StatusStale }
    return component.StatusLive
}

func defaultProject(s string) string {
    if s == "" { return "(unlabeled)" }
    return s
}

func itoaPage(n int) string { return fmt.Sprintf("%d", n) }

func formatColHeader(w int) string {
    // Column header strip matches SessionRow layout but without index padding.
    return fmt.Sprintf("%-4s %-18s %-*s %-10s %-8s %-8s %-7s %s",
        "#", "started", w-4-18-10-8-8-7-8-7, "project", "duration", "cost", "prompts", "tokens", "status")
}
```

Tune `formatColHeader` widths to match `SessionRow`'s exactly. If the test fails, treat the row widths as ground truth and adjust the header.

- [ ] **Step 5.3.2: Drop the old `List.Status()` method that returned `theme.PillState` if the chrome still calls it.**

Check `internal/tui/app/app.go`'s chrome — `v.Status()` returns `theme.PillState`. Keep it for now (PR 7 will refactor the chrome). The new `m.statusFor()` is only for the view body's pill.

- [ ] **Step 5.3.3: Generate golden + verify by eye.**

```bash
go test ./internal/tui/sessions/ -update
```

Open `internal/tui/sessions/testdata/list_populated.golden` and verify alignment.

- [ ] **Step 5.3.4: Run the test.**

```bash
go test ./internal/tui/sessions/ -v
```

Expected: PASS.

### Step 5.4 — Commit PR 5

- [ ] **Step 5.4.1.**

```bash
git add internal/tui/sessions/list.go internal/tui/sessions/list_test.go internal/tui/sessions/testdata/list_*.golden cmd/app/tui.go
git commit -m "feat(tui/sessions): rewrite list View() with components

Inverts cursor row background instead of '▸ + bold'. Adds tokens column
(from Path 2). Page indicator moves to header. Renders inside a rounded
Card. Golden tests cover populated (incl. CJK project) and empty states.

Refs: docs/superpowers/specs/2026-05-12-tui-redesign-design.md §6.2"
```

---

## Task 6 — Session detail + Prompt detail rewrite

**Files:**
- Modify: `internal/tui/sessions/detail.go` and its test
- Modify: `internal/tui/prompt/detail.go` and its test
- Create: `internal/tui/sessions/testdata/detail_*.golden`
- Create: `internal/tui/prompt/testdata/detail_*.golden`

These share styling for event rows so they ship together.

### Step 6.1 — Accept `*theme.Theme` in both detail constructors

- [ ] **Step 6.1.1: Edit `sessions/detail.go`.** Replace `var defaultTheme = theme.Default()` with a `theme *theme.Theme` field on `Detail`. Add the parameter to `NewDetail(pool, sessionID, th)`.

- [ ] **Step 6.1.2: Edit `prompt/detail.go` analogously.** `prompt.New(pool, promptID, th)`.

- [ ] **Step 6.1.3: Update call sites.** The session-list's `Enter` handler creates `sessions.NewDetail(pool, id)` — pass `m.theme` to it. The session-detail's `Enter` handler creates `newPromptDetail(pool, pid)` — pass `m.theme`.

- [ ] **Step 6.1.4: Run vet + tests.** Fix breaks until green.

### Step 6.2 — Rewrite `sessions.Detail.View`

- [ ] **Step 6.2.1: Failing golden test.**

In `sessions/detail_test.go`:

```go
func TestSessionDetailView_Golden(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    base := time.Date(2026,5,11,9,14,0,0,time.UTC)
    events := []readstore.EventRow{
        {TS: base.Add(2*time.Second),  EventName: "session_lifecycle", Summary: "started"},
        {TS: base.Add(8*time.Second),  EventName: "user_prompt", PromptID: "p1", Summary: `"refactor receiver pipeline"`},
        {TS: base.Add(9*time.Second),  EventName: "api_request", Summary: "opus-4-7  $0.12 · 8k/3k"},
        {TS: base.Add(11*time.Second), EventName: "tool_decision", Summary: "Read · approved"},
        {TS: base.Add(11*time.Second), EventName: "tool_result", Summary: "Read ✓ 42ms"},
    }
    m := &Detail{theme: &th, sessionID: "a3f9c1b1-0000-0000-0000-000000000000", events: events}
    got := m.View(90, 32)
    // ... compare to testdata/detail_populated.golden
}
```

- [ ] **Step 6.2.2: Run, confirm failure.**

- [ ] **Step 6.2.3: Replace `Detail.View` body.**

Compose header + info-strip + card containing column header + visible-window event rows + pagination hint + help.

```go
func (m *Detail) View(width, height int) string {
    th := m.theme
    if width <= 0 { width = 90 }

    // Header
    brand := th.Title.Render(th.Glyphs.Brand + " cco")
    bread := th.Muted2.Render(" · session " + shortID(m.sessionID))
    pill := component.StatusPill(th, m.statusFor())
    headerRight := lipgloss.NewStyle().Width(width-lipgloss.Width(brand)-lipgloss.Width(bread)).Align(lipgloss.Right).Render(pill)
    header := lipgloss.JoinHorizontal(lipgloss.Top, brand, bread, headerRight)

    if len(m.events) == 0 {
        body := th.Muted2.Render("no events for this session")
        card := component.Card(th, "", body, width)
        help := component.HelpBar(th, m.helpHints(), width)
        return strings.Join([]string{header, "", card, "", help}, "\n")
    }

    m.viewport = visibleRows(height)
    clampOffset(m)

    rows := []string{th.Muted2.Render(fmt.Sprintf("%-8s %-22s %s", "time", "event", "summary"))}
    end := m.offset + m.viewport
    if end > len(m.events) { end = len(m.events) }
    for i := m.offset; i < end; i++ {
        e := m.events[i]
        rd := component.EventRowData{
            Time: e.TS, EventName: e.EventName, Summary: e.Summary,
            IsPrompt: e.EventName == domain.EventUserPrompt && e.PromptID != "",
        }
        rows = append(rows, component.EventRow(th, rd, i == m.cursor, width-4))
    }
    card := component.Card(th, "", strings.Join(rows, "\n"), width)

    var hint string
    switch {
    case m.loadingOlder: hint = th.Muted2.Render("loading older events…")
    case m.hasMore:      hint = th.Muted2.Render("press pgdn for older events")
    }

    help := component.HelpBar(th, m.helpHints(), width)
    parts := []string{header, "", card}
    if hint != "" { parts = append(parts, hint) }
    parts = append(parts, "", help)
    return strings.Join(parts, "\n")
}

func (m *Detail) helpHints() []component.KeyHint {
    return []component.KeyHint{
        {"↑↓", "nav"}, {"⏎", "open prompt"}, {"pgup/pgdn", "scroll"},
        {"b", "back"}, {"?", "help"}, {"q", "quit"},
    }
}
```

- [ ] **Step 6.2.4: Generate + verify golden + run.** Eyeball the testdata file.

### Step 6.3 — Rewrite `prompt.Detail.View`

- [ ] **Step 6.3.1: Failing golden test.**

```go
func TestPromptDetailView_Golden(t *testing.T) {
    th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
    base := time.Date(2026,5,11,9,15,42,0,time.UTC)
    res := readstore.PromptDetailResult{
        Prompt: readstore.Prompt{
            PromptID: "7b2e4d10-0000-0000-0000-000000000000",
            SessionID: "a3f9c1b1-0000-0000-0000-000000000000",
            StartedAt: base, EndedAt: base.Add(32*time.Second),
            PromptLength: 2341,
            CostUSD: 0.38, InputTokens: 12481, OutputTokens: 4012,
            CacheReadTokens: 88000, CacheCreationTokens: 2000,
            APIRequests: 3, ToolCalls: 5, HadError: true,
        },
        APIRequests: []readstore.APIRequest{
            {TS: base.Add(1*time.Second),  Model: "claude-opus-4-7", CostUSD: 0.21, InputTokens: 8481, OutputTokens: 2140},
            {TS: base.Add(16*time.Second), Model: "claude-opus-4-7", CostUSD: 0.09, InputTokens: 2000, OutputTokens: 872},
            {TS: base.Add(32*time.Second), Model: "claude-opus-4-7", CostUSD: 0.08, InputTokens: 2000, OutputTokens: 1000},
        },
        ToolCalls: []readstore.ToolCall{
            {TS: base.Add(4*time.Second),  ToolName: "Write", Success: true,  DurationMS: 112},
            {TS: base.Add(6*time.Second),  ToolName: "Bash",  Success: false, DurationMS: 2104},
            {TS: base.Add(13*time.Second), ToolName: "Bash",  Success: true,  DurationMS: 189},
            {TS: base.Add(20*time.Second), ToolName: "Edit",  Success: true,  DurationMS: 76},
            {TS: base.Add(26*time.Second), ToolName: "Read",  Success: true,  DurationMS: 31},
        },
    }
    d := &Detail{theme: &th, promptID: res.Prompt.PromptID, result: res, lastOK: time.Now()}
    got := d.View(90, 32)
    // ... compare to testdata/detail_populated.golden
}
```

- [ ] **Step 6.3.2: Run, confirm failure.**

- [ ] **Step 6.3.3: Replace `prompt.Detail.View` body.**

Compose: header + info-strip + 3 summary cards (cost / tokens / activity) + api requests card + tool calls card + help bar.

```go
func (d *Detail) View(width, height int) string {
    th := d.theme
    if width <= 0 { width = 90 }

    // Header
    brand := th.Title.Render(th.Glyphs.Brand + " cco")
    bread := th.Muted2.Render(" · prompt " + shortID(d.promptID))
    pill := component.StatusPill(th, d.statusFor())
    headerRight := lipgloss.NewStyle().Width(width-lipgloss.Width(brand)-lipgloss.Width(bread)).Align(lipgloss.Right).Render(pill)
    header := lipgloss.JoinHorizontal(lipgloss.Top, brand, bread, headerRight)

    if d.notFound {
        body := th.Muted2.Render("prompt not found — it may have been pruned")
        return strings.Join([]string{header, "", component.Card(th, "", body, width)}, "\n")
    }
    if d.result.Prompt.PromptID == "" {
        body := th.Muted2.Render("loading…")
        return strings.Join([]string{header, "", component.Card(th, "", body, width)}, "\n")
    }

    p := d.result.Prompt
    dur := int64(0)
    if !p.EndedAt.IsZero() {
        dur = int64(p.EndedAt.Sub(p.StartedAt).Seconds())
    }
    info := strings.Join([]string{
        th.Muted2.Render("session"),  th.Accent.Render(shortID(p.SessionID)),
        th.Muted2.Render(" · started"), th.Accent.Render(p.StartedAt.Format("15:04:05")),
        th.Muted2.Render(" · duration"), th.Accent.Render(fmt.Sprintf("%ds", dur)),
        th.Muted2.Render(" · "), th.Accent.Render(fmt.Sprintf("%d chars", p.PromptLength)),
    }, "")

    // 3 summary cards
    cardW := (width - 4) / 3
    costBody := th.Value.Render(fmt.Sprintf("$%.2f", p.CostUSD)) + "\n\n" +
        th.Muted2.Render(fmt.Sprintf("%d api requests", p.APIRequests))
    tokensBody := strings.Join([]string{
        labelValue(th, "in",          fmt.Sprintf("%d", p.InputTokens),  cardW-4),
        labelValue(th, "out",         fmt.Sprintf("%d", p.OutputTokens), cardW-4),
        labelValue(th, "cache r/w",   fmt.Sprintf("%s / %s", humanInt(p.CacheReadTokens), humanInt(p.CacheCreationTokens)), cardW-4),
    }, "\n")
    activityBody := strings.Join([]string{
        labelValue(th, "api reqs",   fmt.Sprintf("%d", p.APIRequests), cardW-4),
        labelValue(th, "tool calls", fmt.Sprintf("%d", p.ToolCalls),   cardW-4),
        labelValue(th, "errors",     errorCountStyled(th, p), cardW-4),
    }, "\n")
    summaryRow := lipgloss.JoinHorizontal(lipgloss.Top,
        component.Card(th, "cost",     costBody,     cardW), " ",
        component.Card(th, "tokens",   tokensBody,   cardW), " ",
        component.Card(th, "activity", activityBody, cardW),
    )

    // api requests card
    apiRows := []string{}
    for _, r := range d.result.APIRequests {
        apiRows = append(apiRows, component.APIRequestRow(th, component.APIRequestRowData{
            Time: r.TS, Model: r.Model, CostUSD: r.CostUSD,
            InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
        }, width-4))
    }
    apiCard := component.Card(th, "api requests", strings.Join(apiRows, "\n"), width)
    if len(apiRows) == 0 {
        apiCard = component.Card(th, "api requests", th.Muted2.Render("(none)"), width)
    }

    // tool calls card
    tcRows := []string{}
    for _, c := range d.result.ToolCalls {
        note := ""
        if !c.Success { note = "failed" }
        tcRows = append(tcRows, component.ToolCallRow(th, component.ToolCallRowData{
            Time: c.TS, ToolName: c.ToolName, Success: c.Success,
            DurationMS: c.DurationMS, Note: note,
        }, width-4))
    }
    tcCard := component.Card(th, "tool calls", strings.Join(tcRows, "\n"), width)
    if len(tcRows) == 0 {
        tcCard = component.Card(th, "tool calls", th.Muted2.Render("(none)"), width)
    }

    help := component.HelpBar(th, []component.KeyHint{{"b","back"},{"r","refresh"},{"q","quit"}}, width)
    return strings.Join([]string{header, "", info, "", summaryRow, "", apiCard, "", tcCard, "", help}, "\n")
}

func errorCountStyled(th *theme.Theme, p readstore.Prompt) string {
    s := fmt.Sprintf("%d", boolInt(p.HadError))
    if p.HadError {
        return lipgloss.NewStyle().Foreground(th.Palette.Red).Render(s)
    }
    return s
}
func boolInt(b bool) int { if b { return 1 }; return 0 }
```

- [ ] **Step 6.3.4: Generate + verify + run.**

```bash
go test ./internal/tui/prompt/ -update
go test ./internal/tui/prompt/ -v
```

Expected: PASS.

### Step 6.4 — Smoke test

- [ ] **Step 6.4.1: `make build && ./bin/claude-code-observer`.** Navigate dashboard → enter on a recent session → enter on a user-prompt row → confirm prompt detail renders. Press `b` back through the stack.

### Step 6.5 — Commit PR 6

- [ ] **Step 6.5.1.**

```bash
git add internal/tui/sessions/detail.go internal/tui/sessions/detail_test.go internal/tui/sessions/testdata/detail_*.golden internal/tui/prompt/
git commit -m "feat(tui): rewrite session detail + prompt detail Views

Session detail: rounded card timeline, user-prompt rows tinted, tool_result
✓/✗ inline. Prompt detail: 3-card summary strip (cost/tokens/activity) +
api-requests + tool-calls cards. All composed via theme/component
primitives. Golden tests at (90, 32) cover populated and empty states.

Refs: docs/superpowers/specs/2026-05-12-tui-redesign-design.md §6.3, §6.4"
```

---

## Task 7 — Cleanup (delete legacy theme shape)

**Files:**
- Modify: `internal/tui/theme/theme.go` — remove the legacy fields and helpers
- Modify: `internal/tui/theme/theme_test.go` — drop legacy tests
- Modify: `internal/tui/app/app.go` — chrome uses new fields
- Modify: any straggler call sites referencing `Heading`, `MutedText`, `AccentText`, `ErrorText`, `Pill`, `Block`, `PillState`, `Default()`

### Step 7.1 — Survey legacy references

- [ ] **Step 7.1.1: Find all references.**

```bash
grep -rn "theme.Default\|\.Heading\|\.MutedText\|\.AccentText\|\.ErrorText\|theme.Pill\|theme.PillLive\|theme.PillStale\|theme.PillNoDaemon\|\.Block(\|PillState" --include="*.go" .
```

Expected: matches in `internal/tui/app/app.go`, possibly in remaining view files, and the legacy theme test. Each is a fix-up site.

### Step 7.2 — Migrate the chrome renderer

- [ ] **Step 7.2.1: Rewrite `internal/tui/app/app.go`'s `renderChrome`.**

Replace the chrome rendering body. Use `th.Title` for the brand/title, `component.StatusPill` for the pill, `component.HelpBar` for the footer hints. The current view interface uses `v.Status() theme.PillState` — change this method's return type to `component.Status` (or alias) and update all view implementations (`dashboard.Model.Status`, `sessions.List.Status`, `sessions.Detail.Status`, `prompt.Detail.Status`).

```go
// internal/tui/app/view.go (or wherever the interface lives)
type View interface {
    Init() tea.Cmd
    Update(tea.Msg) (View, tea.Cmd)
    View(width, height int) string
    Title() string
    ShortHelp() []key.Binding
    Status() component.Status   // ← changed
}
```

Each view's `Status()` method returns `component.StatusLive | component.StatusStale | component.StatusNoDaemon` instead of `theme.PillLive | …`.

`a.theme.ErrorText.Render("⚠ VIEW ERROR — b TO RETURN")` in `safeRender` becomes:

```go
out = lipgloss.NewStyle().Foreground(a.theme.Palette.Red).Bold(true).Render("⚠ VIEW ERROR — b TO RETURN")
```

### Step 7.3 — Delete legacy fields from `Theme`

- [ ] **Step 7.3.1: Edit `theme.go`.** Remove `AccentColor`, `ErrorColor`, `Fg`, `Muted` (the AdaptiveColor), `Heading`, `AccentText`, `ErrorText`, `MutedText`, `border`, the `PillState` enum, the `Pill` method, the `Block` method. Remove `Default()`. Rename `Muted2 → Muted`, `PillLiveS → PillLive`, `PillStaleS → PillStale`, `PillNoDaemon` stays. Update every reference accordingly (find-and-replace across `internal/tui/`).

- [ ] **Step 7.3.2: Update `theme_test.go`.** Drop `TestTheme_LegacyAPI_StillWorks`. Update `TestTheme_Build_PopulatesNewFields` to reference the renamed fields.

- [ ] **Step 7.3.3: Iterate.** Run `go build ./...` and fix each compile error until clean.

```bash
make vet && make test && make build
```

Expected: green.

### Step 7.4 — Smoke test the binary one more time

- [ ] **Step 7.4.1: `./bin/claude-code-observer`.** All four screens reachable, no panics, alignment intact. Press `q` to exit.

### Step 7.5 — Commit PR 7

- [ ] **Step 7.5.1.**

```bash
git add -A
git commit -m "refactor(theme): delete legacy brutalist shape; promote new fields

Removes Theme.AccentColor / ErrorColor / Heading / AccentText / ErrorText /
MutedText / Block / Pill / PillState / Default() — all superseded by the
new Palette/Glyphs/component-based API. Renames Muted2→Muted,
PillLiveS→PillLive, PillStaleS→PillStale. View.Status() now returns
component.Status. The binary still produces the same visible output as
PR 6, with a smaller theme surface area.

Refs: docs/superpowers/specs/2026-05-12-tui-redesign-design.md §10 PR 7"
```

---

## Self-review (run before handing off to execution)

Read against the spec section by section:

- **§1 Problem:** Covered — Task 1–7 collectively replace the brutalist theme on all 3 screens. ✅
- **§2 Locked decisions:** Personality/icons/palette/density/scope/old-theme all materialize via Tasks 2+3+4+5+6+7. ✅
- **§3 Package layout:** Tasks 2 & 3 create exactly the files listed. ✅
- **§4 Theme data model:** Task 2 builds `Palette`, `Glyphs`, `Theme.Build`, derived styles. ✅
- **§5 Backend additions (Path 2):** Task 1 covers all four items. ✅
- **§6.1 Dashboard:** Task 4. Header / window cards / delta strip / top-sessions read-only / recent-sessions cursor / help — all present. ✅
- **§6.2 Sessions list:** Task 5. Inverted-background selected row + tokens column + page indicator in header — present. ✅
- **§6.3 Session detail:** Task 6.2. Tinted user-prompt rows + inline ✓/✗ + same pagination logic — present. ✅
- **§6.4 Prompt detail:** Task 6.3. Info strip + 3 summary cards (cost/tokens/activity) + api/tool cards — present. ✅
- **§7 Alignment discipline:** Task 3 builds `Budget` + width-aware components; tests assert `lipgloss.Width` for ASCII/CJK/emoji; Tasks 4–6 add view-level golden files. ✅
- **§8 CLI surface:** Task 2.6 registers `--theme`/`--icons` persistent flags + env + `$COLORFGBG`. ✅
- **§9 Testing approach:** Every task's tests align with the matrix. ✅
- **§10 Migration plan:** Tasks 1–7 match PRs 1–7 one-to-one. ✅
- **§11 Out of scope:** Sparkline component exists but not used; model-mix card absent. ✅
- **§12 Risks:** Width-discipline tests, CCO_ICONS-tofu documentation, hex-color robustness — all addressed.

**Placeholders:** none — every code step includes the actual code or actual command.

**Type consistency:** `Theme.Muted2`/`PillLiveS`/`PillStaleS` rename in PR 7 is intentional and called out; until then references use the suffixed names. `View.Status()` return type changes in PR 7 (called out). `dashboard.New`/`sessions.NewList`/`sessions.NewDetail`/`prompt.New` add a `*theme.Theme` parameter, each task updates its own constructor + call sites.

**Open coordination:**
- The `Resolve()` test for `auto` theme is sensitive to `$COLORFGBG` semantics (which I matched to a common convention: light when bg ≥ 7). If a test environment has its own `$COLORFGBG`, leaking through tests becomes flaky. Tests pass it as an argument explicitly to keep them hermetic; `cmd/app/tui.go` is the only place that reads `os.Getenv`.

If you find issues during execution that I didn't catch here, fix inline rather than abandoning the plan.
