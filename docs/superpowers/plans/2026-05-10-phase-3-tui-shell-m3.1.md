# Phase 3 — TUI shell + M3.1 Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the Bubble Tea TUI shell (page-stack navigation, read-only sqlite pool, neo-brutalist theme, 1 s ticker) and the M3.1 Dashboard view that shows today / 7-day / 30-day rollups + top-3 sessions today, polling every second.

**Architecture:** Root `App` model (`tea.Model`) holds a stack of `View` interface implementations and forwards messages. A separate `readstore` package opens the same SQLite file with `mode=ro` + `query_only=1` and exposes a single `DashboardSnapshot(ctx)` query. A `theme` package owns all lipgloss styles (Border, Heading, Block, Pill, AccentText). The Dashboard view is a leaf `View` impl that fires a query on tick, dedupes in-flight fetches, and renders via theme primitives.

**Tech Stack:** Go 1.25, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `github.com/charmbracelet/bubbles/key`, `modernc.org/sqlite`.

**Source spec:** `docs/superpowers/specs/2026-05-10-phase-3-tui-shell-m3.1-design.md`

---

## File structure

**Created:**

```
internal/tui/
├── app/
│   ├── doc.go
│   ├── view.go        # View interface
│   ├── messages.go    # TickMsg, ErrMsg, PushViewMsg, PopViewMsg
│   ├── keys.go        # global key bindings
│   ├── app.go         # type App (tea.Model)
│   └── app_test.go
├── theme/
│   ├── doc.go
│   ├── theme.go       # palette + 5 primitives
│   └── theme_test.go
├── readstore/
│   ├── doc.go
│   ├── pool.go        # OpenRO
│   ├── pool_test.go
│   ├── queries.go     # DashboardSnapshot, TopSession
│   └── queries_test.go
└── dashboard/
    ├── doc.go
    ├── model.go       # implements app.View
    ├── model_test.go
    ├── view.go        # render
    ├── view_test.go   # golden tests
    └── testdata/
        ├── happy.golden
        ├── empty.golden
        ├── stale.golden
        └── narrow.golden
docs/MANUAL-VERIFICATION.md   # new file with M3.1 section
```

**Modified:**

- `go.mod` / `go.sum` — add bubbletea, lipgloss, bubbles
- `cmd/app/tui.go` — replace stub with real wiring

---

## Task 1: Add Bubble Tea dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add modules**

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
go mod tidy
```

- [ ] **Step 2: Verify the build still passes**

Run: `go build -o bin/cco ./cmd/app`
Expected: build succeeds, binary produced.

Run: `go vet ./...`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add bubbletea, lipgloss, bubbles for TUI"
```

---

## Task 2: Theme package — palette & primitives

**Files:**
- Create: `internal/tui/theme/doc.go`
- Create: `internal/tui/theme/theme.go`
- Create: `internal/tui/theme/theme_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/theme/theme_test.go`:

```go
package theme

import (
	"strings"
	"testing"
)

func TestDefault_HasAccentColor(t *testing.T) {
	th := Default()
	got := th.AccentText.Render("$4.21")
	if got == "$4.21" {
		t.Fatalf("AccentText should add ANSI styling, got plain %q", got)
	}
	if !strings.Contains(got, "$4.21") {
		t.Fatalf("AccentText should contain the original text, got %q", got)
	}
}

func TestDefault_BlockHasBorder(t *testing.T) {
	th := Default()
	got := th.Block(20).Render("hello")
	// Thick border characters appear in the rendered output.
	if !strings.ContainsAny(got, "┏┓┗┛━┃") {
		t.Fatalf("Block should render with thick border, got %q", got)
	}
}

func TestDefault_PillStates(t *testing.T) {
	th := Default()
	cases := []PillState{PillLive, PillStale, PillNoDaemon}
	for _, s := range cases {
		got := th.Pill(s)
		if got == "" {
			t.Fatalf("Pill(%v) should not be empty", s)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/theme/...`
Expected: FAIL with "undefined: Default" (or similar).

- [ ] **Step 3: Write the package doc**

Create `internal/tui/theme/doc.go`:

```go
// Package theme is the single source of lipgloss styles for the TUI.
// View packages render only via the primitives defined here; no inline
// lipgloss.NewStyle() calls live outside this package.
package theme
```

- [ ] **Step 4: Implement the theme**

Create `internal/tui/theme/theme.go`:

```go
package theme

import "github.com/charmbracelet/lipgloss"

// PillState identifies which footer pill to render.
type PillState int

const (
	PillLive PillState = iota
	PillStale
	PillNoDaemon
)

// Theme is the neo-brutalist style set. Single accent color, thick borders,
// ALL CAPS labels. See docs/superpowers/specs/2026-05-10-phase-3-tui-shell-m3.1-design.md §4.
type Theme struct {
	AccentColor lipgloss.Color
	ErrorColor  lipgloss.Color
	Fg          lipgloss.AdaptiveColor
	Muted       lipgloss.AdaptiveColor

	Heading    lipgloss.Style
	AccentText lipgloss.Style
	ErrorText  lipgloss.Style
	MutedText  lipgloss.Style

	border lipgloss.Border
}

// Default returns the locked v1 theme: hot yellow accent, semantic red,
// adaptive black/white foreground.
func Default() Theme {
	accent := lipgloss.Color("#FFD400")
	errCol := lipgloss.Color("#FF3B30")
	fg := lipgloss.AdaptiveColor{Light: "#000000", Dark: "#FFFFFF"}
	muted := lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"}

	return Theme{
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
}

// Block returns a thick-bordered style sized to minWidth (cells). Used for
// KPI tiles and section frames.
func (t Theme) Block(minWidth int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(t.border).
		BorderForeground(t.Fg).
		Padding(1, 2).
		Width(minWidth)
}

// Pill renders the footer state pill. Returns a styled, fully formed string
// (caller embeds it as-is, no further styling).
func (t Theme) Pill(s PillState) string {
	switch s {
	case PillLive:
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#000000")).
			Background(t.AccentColor).
			Padding(0, 1).
			Render("● LIVE")
	case PillStale:
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(t.Muted).
			Padding(0, 1).
			Render("STALE")
	case PillNoDaemon:
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(t.ErrorColor).
			Padding(0, 1).
			Render("⚠ NO DAEMON")
	}
	return ""
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/tui/theme/...`
Expected: PASS.

- [ ] **Step 6: Verify lint clean**

Run: `go vet ./internal/tui/theme/...`
Expected: no output.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/theme/
git commit -m "feat(tui): add neo-brutalist theme package"
```

---

## Task 3: Readstore — read-only DB pool

**Files:**
- Create: `internal/tui/readstore/doc.go`
- Create: `internal/tui/readstore/pool.go`
- Create: `internal/tui/readstore/pool_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/readstore/pool_test.go`:

```go
package readstore_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

func TestOpenRO_OpensExistingDB(t *testing.T) {
	home := t.TempDir()
	// Initialize the schema via the writer pool first.
	repo, err := repository.Open(home)
	if err != nil {
		t.Fatalf("repository.Open: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("repo.Close: %v", err)
	}

	dbPath := filepath.Join(home, "db.sqlite")
	pool, err := readstore.OpenRO(dbPath)
	if err != nil {
		t.Fatalf("OpenRO: %v", err)
	}
	defer pool.Close()

	// SELECT works.
	var n int
	if err := pool.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sessions").Scan(&n); err != nil {
		t.Fatalf("SELECT: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 sessions, got %d", n)
	}
}

func TestOpenRO_RejectsWrites(t *testing.T) {
	home := t.TempDir()
	repo, _ := repository.Open(home)
	repo.Close()

	pool, err := readstore.OpenRO(filepath.Join(home, "db.sqlite"))
	if err != nil {
		t.Fatalf("OpenRO: %v", err)
	}
	defer pool.Close()

	_, err = pool.ExecContext(context.Background(),
		"INSERT INTO sessions(session_id, started_at, last_seen_at) VALUES('x', 0, 0)")
	if err == nil {
		t.Fatalf("expected write to fail under query_only/mode=ro")
	}
}

func TestOpenRO_FileMissing(t *testing.T) {
	pool, err := readstore.OpenRO("/nonexistent/db.sqlite")
	if err != nil {
		// Acceptable: error at open time.
		return
	}
	defer pool.Close()
	// Otherwise the first query should fail.
	var n int
	if err := pool.QueryRowContext(context.Background(),
		"SELECT 1").Scan(&n); err == nil {
		t.Fatalf("expected error querying missing DB")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/readstore/...`
Expected: FAIL with "no Go files" or "undefined: OpenRO".

- [ ] **Step 3: Write the package doc**

Create `internal/tui/readstore/doc.go`:

```go
// Package readstore opens a read-only connection pool to the SQLite file
// owned by the daemon's writer pool. WAL mode keeps reads and writes
// non-blocking. mode=ro and the query_only PRAGMA enforce safety.
package readstore
```

- [ ] **Step 4: Implement OpenRO**

Create `internal/tui/readstore/pool.go`:

```go
package readstore

import (
	"database/sql"
	"fmt"
	"net/url"

	_ "modernc.org/sqlite"
)

// OpenRO opens a read-only pool against the SQLite file at path.
// The pool is small (2 conns); the TUI's worst case is the dashboard
// query plus a drill-down query in flight at once.
//
// Note: open succeeds even when the file is missing — modernc.org/sqlite
// defers the error to the first query. Callers must treat first-query
// errors as "DB not ready" rather than fatal.
func OpenRO(path string) (*sql.DB, error) {
	q := url.Values{}
	q.Set("mode", "ro")
	q.Set("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "busy_timeout(2000)")
	q.Add("_pragma", "query_only(1)")
	dsn := fmt.Sprintf("file:%s?%s", path, q.Encode())

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open ro sqlite: %w", err)
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(0)
	return db, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/tui/readstore/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/readstore/
git commit -m "feat(tui/readstore): add read-only sqlite pool"
```

---

## Task 4: Readstore — DashboardSnapshot query

**Files:**
- Create: `internal/tui/readstore/queries.go`
- Create: `internal/tui/readstore/queries_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/readstore/queries_test.go`:

```go
package readstore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

func TestDashboardSnapshot_AggregatesByWindow(t *testing.T) {
	home := t.TempDir()
	repo, err := repository.Open(home)
	if err != nil {
		t.Fatalf("repository.Open: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	now := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	startOfDay := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	twoDaysAgo := startOfDay.Add(-2 * 24 * time.Hour)
	tenDaysAgo := startOfDay.Add(-10 * 24 * time.Hour)
	fortyDaysAgo := startOfDay.Add(-40 * 24 * time.Hour)

	insertSession := func(id, project string, started time.Time, cost float64, prompts, tools, errors int) {
		_, err := repo.DB().ExecContext(context.Background(),
			`INSERT INTO sessions
			 (session_id, project_name, started_at, last_seen_at,
			  cost_usd, prompts, tool_calls, api_errors)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, project, started.UnixNano(), started.UnixNano(),
			cost, prompts, tools, errors)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	insertSession("today1", "obs", now, 1.50, 5, 20, 1)
	insertSession("today2", "obs", now.Add(time.Hour), 0.80, 3, 12, 0)
	insertSession("d2", "scratch", twoDaysAgo, 2.00, 8, 30, 0)
	insertSession("d10", "obs", tenDaysAgo, 4.00, 10, 40, 2)
	insertSession("d40", "obs", fortyDaysAgo, 99.00, 100, 500, 50) // outside 30d

	pool, err := readstore.OpenRO(filepath.Join(home, "db.sqlite"))
	if err != nil {
		t.Fatalf("OpenRO: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	snap, top, err := readstore.DashboardSnapshot(context.Background(), pool, now)
	if err != nil {
		t.Fatalf("DashboardSnapshot: %v", err)
	}

	// Today: today1 + today2.
	if got, want := snap.Today.CostUSD, 2.30; got != want {
		t.Errorf("today cost: got %.2f want %.2f", got, want)
	}
	if got, want := snap.Today.Prompts, int64(8); got != want {
		t.Errorf("today prompts: got %d want %d", got, want)
	}
	// 7 days: today1 + today2 + d2.
	if got, want := snap.D7.CostUSD, 4.30; got != want {
		t.Errorf("7d cost: got %.2f want %.2f", got, want)
	}
	// 30 days: today1+today2+d2+d10. Excludes d40.
	if got, want := snap.D30.CostUSD, 8.30; got != want {
		t.Errorf("30d cost: got %.2f want %.2f", got, want)
	}
	if got, want := snap.D30.Errors, int64(3); got != want {
		t.Errorf("30d errors: got %d want %d", got, want)
	}

	// Top: today1 (1.50) > today2 (0.80). Two rows total.
	if len(top) != 2 {
		t.Fatalf("top: got %d rows want 2", len(top))
	}
	if top[0].SessionID != "today1" || top[1].SessionID != "today2" {
		t.Errorf("top order wrong: %+v", top)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/readstore/...`
Expected: FAIL with "undefined: DashboardSnapshot".

- [ ] **Step 3: Implement queries**

Create `internal/tui/readstore/queries.go`:

```go
package readstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// WindowStats is the rollup over a single time window.
type WindowStats struct {
	CostUSD  float64
	Prompts  int64
	Tools    int64
	Errors   int64
}

// Snapshot is the dashboard's three-window rollup.
type Snapshot struct {
	Today WindowStats
	D7    WindowStats
	D30   WindowStats
	// LatestEventTS is the maximum events.ts seen, used to detect a stalled
	// daemon. Zero if events table is empty.
	LatestEventTS int64
}

// TopSession is a row in the "top sessions today" panel.
type TopSession struct {
	SessionID    string
	ProjectName  string
	StartedAt    int64
	CostUSD      float64
	Prompts      int64
	Live         bool // ended_at IS NULL
}

// DashboardSnapshot returns the three-window rollup plus the top-3 most
// expensive sessions started today (UTC). now is injected for testability.
func DashboardSnapshot(ctx context.Context, db *sql.DB, now time.Time) (Snapshot, []TopSession, error) {
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	today := startOfDay.UnixNano()
	d7 := startOfDay.Add(-7 * 24 * time.Hour).UnixNano()
	d30 := startOfDay.Add(-30 * 24 * time.Hour).UnixNano()

	const q = `
SELECT
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd     END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts      END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls   END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors   END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd     END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts      END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls   END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors   END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN cost_usd     END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN prompts      END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN tool_calls   END), 0),
  COALESCE(SUM(CASE WHEN started_at >= ? THEN api_errors   END), 0)
FROM sessions
WHERE started_at >= ?`

	var s Snapshot
	err := db.QueryRowContext(ctx, q,
		today, today, today, today,
		d7, d7, d7, d7,
		d30, d30, d30, d30,
		d30,
	).Scan(
		&s.Today.CostUSD, &s.Today.Prompts, &s.Today.Tools, &s.Today.Errors,
		&s.D7.CostUSD, &s.D7.Prompts, &s.D7.Tools, &s.D7.Errors,
		&s.D30.CostUSD, &s.D30.Prompts, &s.D30.Tools, &s.D30.Errors,
	)
	if err != nil {
		return Snapshot{}, nil, fmt.Errorf("snapshot query: %w", err)
	}

	// Latest event timestamp (for stale-daemon detection). Empty events
	// table is fine — we just leave LatestEventTS at zero.
	var ts sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT MAX(ts) FROM events").Scan(&ts); err != nil {
		return Snapshot{}, nil, fmt.Errorf("latest event ts: %w", err)
	}
	if ts.Valid {
		s.LatestEventTS = ts.Int64
	}

	const topQ = `
SELECT session_id, COALESCE(project_name, ''), started_at, cost_usd, prompts, ended_at IS NULL
FROM sessions
WHERE started_at >= ?
ORDER BY cost_usd DESC
LIMIT 3`
	rows, err := db.QueryContext(ctx, topQ, today)
	if err != nil {
		return Snapshot{}, nil, fmt.Errorf("top sessions query: %w", err)
	}
	defer rows.Close()

	var top []TopSession
	for rows.Next() {
		var ts TopSession
		var live int
		if err := rows.Scan(&ts.SessionID, &ts.ProjectName, &ts.StartedAt, &ts.CostUSD, &ts.Prompts, &live); err != nil {
			return Snapshot{}, nil, fmt.Errorf("top session scan: %w", err)
		}
		ts.Live = live == 1
		top = append(top, ts)
	}
	if err := rows.Err(); err != nil {
		return Snapshot{}, nil, fmt.Errorf("top sessions iter: %w", err)
	}
	return s, top, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/readstore/... -run TestDashboardSnapshot -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/readstore/queries.go internal/tui/readstore/queries_test.go
git commit -m "feat(tui/readstore): add DashboardSnapshot rollup query"
```

---

## Task 5: App package — `View` interface and messages

**Files:**
- Create: `internal/tui/app/doc.go`
- Create: `internal/tui/app/view.go`
- Create: `internal/tui/app/messages.go`

- [ ] **Step 1: Write the package doc**

Create `internal/tui/app/doc.go`:

```go
// Package app is the Bubble Tea root model for the cco TUI. It owns the
// page-stack navigation, the 1 s ticker, the global keymap, and the last
// read error. View implementations live in sibling packages and plug in
// via the View interface.
package app
```

- [ ] **Step 2: Implement View interface**

Create `internal/tui/app/view.go`:

```go
package app

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// View is the contract every page in the TUI implements. The shell
// dispatches messages via Update and renders via View.
type View interface {
	// Init runs once when the view is pushed onto the stack. Typically
	// returns a tea.Cmd that fires the view's first data fetch.
	Init() tea.Cmd
	// Update consumes a tea.Msg and returns an updated copy of itself plus
	// any follow-on command. Returning a PushViewMsg drills in; returning
	// a PopViewMsg pops.
	Update(msg tea.Msg) (View, tea.Cmd)
	// View renders the body content (chrome is rendered by the shell).
	// width and height are the inner dimensions available to the view.
	View(width, height int) string
	// Title appears in the top chrome (e.g., "DASHBOARD").
	Title() string
	// ShortHelp lists the keys for the footer strip.
	ShortHelp() []key.Binding
}
```

- [ ] **Step 3: Implement messages**

Create `internal/tui/app/messages.go`:

```go
package app

import "time"

// TickMsg is broadcast every second. The shell forwards it to the top of
// the view stack. View implementations re-emit data fetches in response.
type TickMsg time.Time

// PushViewMsg is returned by a view's Update to drill into a child view.
type PushViewMsg struct{ V View }

// PopViewMsg is returned (or globally generated by b/esc) to pop the
// current view off the stack.
type PopViewMsg struct{}

// ErrMsg is emitted by any view (typically from a fetch goroutine) to tell
// the shell that a read failed. The shell tracks consecutive errors and
// flips the footer pill to STALE.
type ErrMsg struct{ Err error }
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./internal/tui/app/...`
Expected: build succeeds (test for behavior comes with the App model in Task 7).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app/doc.go internal/tui/app/view.go internal/tui/app/messages.go
git commit -m "feat(tui/app): add View interface and message protocol"
```

---

## Task 6: App package — global key bindings

**Files:**
- Create: `internal/tui/app/keys.go`

- [ ] **Step 1: Implement keys**

Create `internal/tui/app/keys.go`:

```go
package app

import "github.com/charmbracelet/bubbles/key"

// GlobalKeys is the set of keys the shell intercepts before forwarding
// any message to the active view.
type GlobalKeys struct {
	Quit    key.Binding
	Back    key.Binding
	Refresh key.Binding
	Help    key.Binding
}

// DefaultKeys returns the locked v1 keymap.
func DefaultKeys() GlobalKeys {
	return GlobalKeys{
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Back:    key.NewBinding(key.WithKeys("b", "esc"), key.WithHelp("b", "back")),
		Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	}
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./internal/tui/app/...`
Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/app/keys.go
git commit -m "feat(tui/app): add global keymap"
```

---

## Task 7: App package — root `App` model with TDD

**Files:**
- Create: `internal/tui/app/app.go`
- Create: `internal/tui/app/app_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/app/app_test.go`:

```go
package app

import (
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// fakeView is a no-op View used for stack assertions.
type fakeView struct {
	title     string
	lastMsg   tea.Msg
	initCalled bool
}

func (v *fakeView) Init() tea.Cmd                 { v.initCalled = true; return nil }
func (v *fakeView) Update(m tea.Msg) (View, tea.Cmd) { v.lastMsg = m; return v, nil }
func (v *fakeView) View(w, h int) string           { return v.title }
func (v *fakeView) Title() string                  { return v.title }
func (v *fakeView) ShortHelp() []key.Binding       { return nil }

func newAppWith(views ...View) *App {
	a := New(theme.Default())
	for _, v := range views {
		a.Push(v)
	}
	return a
}

func TestApp_QuitOnQ(t *testing.T) {
	a := newAppWith(&fakeView{title: "X"})
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("q should return a quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("q cmd should produce tea.QuitMsg, got %T", msg)
	}
}

func TestApp_BackPopsWhenStackDeep(t *testing.T) {
	a := newAppWith(&fakeView{title: "A"}, &fakeView{title: "B"})
	if got := a.StackDepth(); got != 2 {
		t.Fatalf("setup: depth %d", got)
	}
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if got := a.StackDepth(); got != 1 {
		t.Fatalf("after b: depth %d", got)
	}
}

func TestApp_BackNoOpAtRoot(t *testing.T) {
	a := newAppWith(&fakeView{title: "A"})
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if got := a.StackDepth(); got != 1 {
		t.Fatalf("root b should be no-op, depth=%d", got)
	}
}

func TestApp_RefreshForwardsTickToTopView(t *testing.T) {
	v := &fakeView{title: "A"}
	a := newAppWith(v)
	a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if _, ok := v.lastMsg.(TickMsg); !ok {
		t.Fatalf("r should forward TickMsg to top view, got %T", v.lastMsg)
	}
}

func TestApp_PushViewMsgPushesAndInits(t *testing.T) {
	a := newAppWith(&fakeView{title: "A"})
	child := &fakeView{title: "B"}
	a.Update(PushViewMsg{V: child})
	if got := a.StackDepth(); got != 2 {
		t.Fatalf("after push: depth %d", got)
	}
	if !child.initCalled {
		t.Fatalf("Init() should be called on push")
	}
}

func TestApp_ErrMsgIncrementsCounter(t *testing.T) {
	a := newAppWith(&fakeView{title: "A"})
	a.Update(ErrMsg{Err: errors.New("boom")})
	a.Update(ErrMsg{Err: errors.New("boom")})
	if got := a.ConsecutiveErrors(); got != 2 {
		t.Fatalf("consecutive errors: got %d want 2", got)
	}
}

func TestApp_TickForwardedToTop(t *testing.T) {
	v := &fakeView{title: "A"}
	a := newAppWith(v)
	a.Update(TickMsg{})
	if _, ok := v.lastMsg.(TickMsg); !ok {
		t.Fatalf("TickMsg should be forwarded, got %T", v.lastMsg)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/app/... -v`
Expected: FAIL with "undefined: New" (or compilation errors).

- [ ] **Step 3: Implement App**

Create `internal/tui/app/app.go`:

```go
package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// App is the Bubble Tea root model. It owns the view stack, the global
// keymap, the theme, and per-tick error accounting.
type App struct {
	stack       []View
	theme       theme.Theme
	keys        GlobalKeys
	width       int
	height      int
	lastErr     error
	consecErrs  int
}

// New constructs an App with no views. Push at least one before running.
func New(th theme.Theme) *App {
	return &App{theme: th, keys: DefaultKeys()}
}

// Push adds a view to the top of the stack and calls its Init.
// It returns Init's tea.Cmd so the caller can hand it to bubbletea.
func (a *App) Push(v View) tea.Cmd {
	a.stack = append(a.stack, v)
	return v.Init()
}

// StackDepth is exposed for tests.
func (a *App) StackDepth() int { return len(a.stack) }

// ConsecutiveErrors is exposed for tests and the chrome renderer.
func (a *App) ConsecutiveErrors() int { return a.consecErrs }

// Theme is exposed for view packages that need access to the active palette
// (typically through the message round-trip; direct access kept for tests).
func (a *App) Theme() theme.Theme { return a.theme }

// Init returns the initial tea.Cmd: start the 1 s ticker and call Init on
// any pre-pushed view. The shell itself has no first-time fetch.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{tickEvery()}
	for _, v := range a.stack {
		if c := v.Init(); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

// Update routes a tea.Msg. Global keys and shell-level messages are handled
// here; everything else flows to the top view.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		return a, nil
	case tea.KeyMsg:
		switch {
		case keyMatch(m, a.keys.Quit):
			return a, tea.Quit
		case keyMatch(m, a.keys.Back):
			if len(a.stack) > 1 {
				a.stack = a.stack[:len(a.stack)-1]
			}
			return a, nil
		case keyMatch(m, a.keys.Refresh):
			return a.forwardTop(TickMsg(time.Now()))
		}
	case TickMsg:
		// Forward to top view, then schedule next tick.
		model, cmd := a.forwardTop(m)
		return model, tea.Batch(cmd, tickEvery())
	case PushViewMsg:
		initCmd := a.Push(m.V)
		return a, initCmd
	case PopViewMsg:
		if len(a.stack) > 1 {
			a.stack = a.stack[:len(a.stack)-1]
		}
		return a, nil
	case ErrMsg:
		a.lastErr = m.Err
		a.consecErrs++
		return a, nil
	}
	// Anything else: forward to top view.
	return a.forwardTop(msg)
}

// View renders chrome + body.
func (a *App) View() string {
	if len(a.stack) == 0 {
		return ""
	}
	top := a.stack[len(a.stack)-1]
	body := top.View(a.width, a.height)
	return body
}

func (a *App) forwardTop(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(a.stack) == 0 {
		return a, nil
	}
	top := a.stack[len(a.stack)-1]
	updated, cmd := top.Update(msg)
	a.stack[len(a.stack)-1] = updated
	// Reset consec errors on successful (non-Err) forward results.
	// Concrete reset happens when a view emits a non-Err result message;
	// that's a view-side concern, not a shell concern. Keep counter
	// monotonic until next ErrMsg or explicit reset.
	return a, cmd
}

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func keyMatch(m tea.KeyMsg, b interface{ Keys() []string }) bool {
	for _, k := range b.Keys() {
		if m.String() == k {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/app/... -v`
Expected: PASS for all 7 tests.

- [ ] **Step 5: Verify lint/vet clean**

Run: `go vet ./internal/tui/app/...`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app/app.go internal/tui/app/app_test.go
git commit -m "feat(tui/app): add root App model with page-stack navigation"
```

---

## Task 8: Dashboard view — model + Update logic with TDD

**Files:**
- Create: `internal/tui/dashboard/doc.go`
- Create: `internal/tui/dashboard/model.go`
- Create: `internal/tui/dashboard/model_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/dashboard/model_test.go`:

```go
package dashboard

import (
	"errors"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

func TestModel_InitReturnsFetchCmd(t *testing.T) {
	m := New(nil) // nil pool is fine; fetch will fail but Init returns a cmd
	cmd := m.Init()
	if cmd == nil {
		t.Fatalf("Init should return a fetch cmd")
	}
}

func TestModel_TickWhileInFlightSkipsFetch(t *testing.T) {
	m := New(nil)
	m.inFlight = true
	_, cmd := m.Update(app.TickMsg(time.Now()))
	if cmd != nil {
		t.Fatalf("tick during in-flight should not start a new fetch")
	}
}

func TestModel_DataMsgClearsInFlightAndStoresSnapshot(t *testing.T) {
	m := New(nil)
	m.inFlight = true
	snap := readstore.Snapshot{Today: readstore.WindowStats{CostUSD: 1.23}}
	top := []readstore.TopSession{{SessionID: "x"}}
	updated, _ := m.Update(dataMsg{snap: snap, top: top})
	got := updated.(*Model)
	if got.inFlight {
		t.Fatalf("dataMsg should clear inFlight")
	}
	if got.snap.Today.CostUSD != 1.23 {
		t.Fatalf("snapshot not stored: %+v", got.snap)
	}
	if len(got.top) != 1 || got.top[0].SessionID != "x" {
		t.Fatalf("top not stored: %+v", got.top)
	}
}

func TestModel_ErrMsgKeepsLastSnapshotAndSetsStale(t *testing.T) {
	m := New(nil)
	m.snap.Today.CostUSD = 9.99
	m.inFlight = true
	updated, _ := m.Update(app.ErrMsg{Err: errors.New("boom")})
	got := updated.(*Model)
	if got.snap.Today.CostUSD != 9.99 {
		t.Fatalf("ErrMsg should preserve last snapshot")
	}
	if got.inFlight {
		t.Fatalf("ErrMsg should clear inFlight")
	}
	if !got.stale {
		t.Fatalf("ErrMsg should set stale=true")
	}
}

// Ensure Model implements app.View at compile time.
var _ app.View = (*Model)(nil)

// Ensure New returns the right type and Init produces a tea.Cmd that can
// run (not crashed) — the result is a tea.Msg of type ErrMsg here because
// pool is nil, but the cmd itself must be invocable.
func TestModel_InitCmdInvocable(t *testing.T) {
	m := New(nil)
	cmd := m.Init()
	msg := cmd()
	if msg == nil {
		t.Fatalf("init cmd should produce a message even on failure")
	}
	if _, ok := msg.(app.ErrMsg); !ok {
		// dataMsg is also acceptable on a real pool; here we just sanity-check.
		if _, ok := msg.(dataMsg); !ok {
			t.Fatalf("unexpected msg type %T", msg)
		}
	}
}

// Helper: silence unused tea import in case future versions need it.
var _ tea.Cmd = nil
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/dashboard/... -v`
Expected: FAIL with "undefined: New" or "undefined: Model".

- [ ] **Step 3: Write the package doc**

Create `internal/tui/dashboard/doc.go`:

```go
// Package dashboard implements the M3.1 dashboard view: today / 7-day /
// 30-day rollups plus the top-3 most expensive sessions today. It polls
// every TickMsg (1 s) with in-flight de-dup.
package dashboard
```

- [ ] **Step 4: Implement Model**

Create `internal/tui/dashboard/model.go`:

```go
package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

const fetchTimeout = 500 * time.Millisecond

var errNoPool = errors.New("dashboard: no read pool")

// dataMsg is the success result of a dashboard fetch. View-local; never
// crosses the shell.
type dataMsg struct {
	snap readstore.Snapshot
	top  []readstore.TopSession
	at   time.Time
}

// Model is the dashboard's tea model. Implements app.View.
type Model struct {
	pool     *sql.DB
	snap     readstore.Snapshot
	top      []readstore.TopSession
	lastOK   time.Time // last successful fetch wall clock
	inFlight bool
	stale    bool
	now      func() time.Time // injected for tests; defaults to time.Now
}

// New constructs a Model bound to the given read pool. pool may be nil in
// tests; in that case fetches will produce ErrMsg.
func New(pool *sql.DB) *Model {
	return &Model{pool: pool, now: time.Now}
}

// Init satisfies app.View — kicks off the first fetch.
func (m *Model) Init() tea.Cmd {
	m.inFlight = true
	return m.fetchCmd()
}

// Update satisfies app.View.
func (m *Model) Update(msg tea.Msg) (app.View, tea.Cmd) {
	switch v := msg.(type) {
	case app.TickMsg:
		if m.inFlight {
			return m, nil
		}
		m.inFlight = true
		return m, m.fetchCmd()
	case dataMsg:
		m.snap = v.snap
		m.top = v.top
		m.lastOK = v.at
		m.inFlight = false
		m.stale = false
		return m, nil
	case app.ErrMsg:
		m.inFlight = false
		m.stale = true
		return m, nil
	}
	return m, nil
}

// Title satisfies app.View.
func (m *Model) Title() string { return "DASHBOARD" }

// ShortHelp satisfies app.View.
func (m *Model) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sessions")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
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
		snap, top, err := readstore.DashboardSnapshot(ctx, pool, now())
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return dataMsg{snap: snap, top: top, at: now()}
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/tui/dashboard/... -run "TestModel_" -v`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/dashboard/
git commit -m "feat(tui/dashboard): add Model with tick/data/err handling"
```

---

## Task 9: Dashboard view — render with golden tests

**Files:**
- Create: `internal/tui/dashboard/view.go`
- Create: `internal/tui/dashboard/view_test.go`
- Create: `internal/tui/dashboard/testdata/happy.golden`
- Create: `internal/tui/dashboard/testdata/empty.golden`

- [ ] **Step 1: Implement render**

Create `internal/tui/dashboard/view.go`:

```go
package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// View renders the dashboard body.
func (m *Model) View(width, height int) string {
	th := theme.Default()
	if width <= 0 {
		width = 80
	}
	blockW := (width - 4) / 3
	if blockW < 16 {
		blockW = 16
	}

	blocks := []string{
		m.renderBlock(th, blockW, "TODAY", m.snap.Today),
		m.renderBlock(th, blockW, "7 DAYS", m.snap.D7),
		m.renderBlock(th, blockW, "30 DAYS", m.snap.D30),
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, blocks...)

	var banner string
	if m.snap.LatestEventTS == 0 && len(m.top) == 0 {
		banner = th.ErrorText.Render("⚠ NO DATA — IS `cco serve` RUNNING?")
	}

	tableW := blockW*3 + 4
	table := m.renderTopSessions(th, tableW)

	parts := []string{}
	if banner != "" {
		parts = append(parts, banner, "")
	}
	parts = append(parts, row, "", table)
	return strings.Join(parts, "\n")
}

func (m *Model) renderBlock(th theme.Theme, w int, label string, ws readstore.WindowStats) string {
	cost := th.AccentText.Render(fmt.Sprintf("$%.2f", ws.CostUSD))
	prompts := fmt.Sprintf("%d PROMPTS", ws.Prompts)
	tools := fmt.Sprintf("%s TOOLS", humanInt(ws.Tools))

	errLine := th.MutedText.Render(fmt.Sprintf("%d ERRORS", ws.Errors))
	if ws.Errors > 0 {
		errLine = th.ErrorText.Render(fmt.Sprintf("⚠ %d ERRORS", ws.Errors))
	}

	body := strings.Join([]string{
		th.Heading.Render(label),
		cost,
		prompts,
		tools,
		errLine,
	}, "\n")
	return th.Block(w).Render(body)
}

func (m *Model) renderTopSessions(th theme.Theme, w int) string {
	header := th.Heading.Render("TOP SESSIONS TODAY")
	if len(m.top) == 0 {
		return th.Block(w).Render(header + "\n" + th.MutedText.Render("(no sessions today)"))
	}
	rows := []string{header, "#  PROJECT          STARTED  COST    PROMPTS  STATUS"}
	for i, ts := range m.top {
		started := time.Unix(0, ts.StartedAt).Local().Format("15:04")
		project := truncate(ts.ProjectName, 16)
		status := ""
		if ts.Live {
			status = th.AccentText.Render("● LIVE")
		}
		rows = append(rows, fmt.Sprintf("%d  %-16s %-7s %s  %-7d %s",
			i+1, project, started,
			th.AccentText.Render(fmt.Sprintf("$%.2f", ts.CostUSD)),
			ts.Prompts, status))
	}
	return th.Block(w).Render(strings.Join(rows, "\n"))
}

func humanInt(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
```

- [ ] **Step 2: Write golden-file test**

Create `internal/tui/dashboard/view_test.go`:

```go
package dashboard

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

var update = flag.Bool("update", false, "update golden files")

// stripANSI removes ANSI escape codes so goldens stay text-diffable.
var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func clean(s string) string { return ansi.ReplaceAllString(s, "") }

func goldenPath(name string) string {
	return filepath.Join("testdata", name+".golden")
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	got = clean(got)
	path := goldenPath(name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if string(want) != got {
		t.Fatalf("%s mismatch:\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}

func TestView_Empty(t *testing.T) {
	m := New(nil)
	m.now = func() time.Time { return time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC) }
	out := m.View(80, 24)
	assertGolden(t, "empty", out)
}

func TestView_Happy(t *testing.T) {
	m := New(nil)
	m.now = func() time.Time { return time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC) }
	m.snap = readstore.Snapshot{
		Today: readstore.WindowStats{CostUSD: 4.21, Prompts: 37, Tools: 152, Errors: 2},
		D7:    readstore.WindowStats{CostUSD: 28.40, Prompts: 214, Tools: 1100, Errors: 9},
		D30:   readstore.WindowStats{CostUSD: 112.05, Prompts: 892, Tools: 4400, Errors: 41},
		LatestEventTS: time.Date(2026, 5, 10, 11, 59, 0, 0, time.UTC).UnixNano(),
	}
	startedAt := time.Date(2026, 5, 10, 9, 14, 0, 0, time.UTC).UnixNano()
	m.top = []readstore.TopSession{
		{SessionID: "s1", ProjectName: "observer", StartedAt: startedAt, CostUSD: 1.92, Prompts: 14, Live: true},
	}
	out := m.View(80, 24)
	assertGolden(t, "happy", out)
}
```

- [ ] **Step 3: Generate golden files**

Run: `go test ./internal/tui/dashboard/... -run TestView_ -update`
Expected: PASS, two new files at `testdata/empty.golden` and `testdata/happy.golden`.

- [ ] **Step 4: Inspect goldens**

Run: `cat internal/tui/dashboard/testdata/happy.golden`
Expected: a readable, brutalist-styled dashboard with three blocks. Eyeball it. If wrong, fix the renderer in `view.go`, then regenerate with `-update`.

- [ ] **Step 5: Re-run without `-update`**

Run: `go test ./internal/tui/dashboard/...`
Expected: PASS (goldens match).

- [ ] **Step 6: Commit**

```bash
git add internal/tui/dashboard/view.go internal/tui/dashboard/view_test.go internal/tui/dashboard/testdata/
git commit -m "feat(tui/dashboard): add render with golden tests"
```

---

## Task 10: Wire shell into `cco` default subcommand

**Files:**
- Modify: `cmd/app/tui.go`

- [ ] **Step 1: Replace stub with real wiring**

Open `cmd/app/tui.go` and replace its contents with:

```go
package main

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/dashboard"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:    "tui",
		Short:  "Open the interactive TUI",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := filepath.Join(homeDir, "db.sqlite")
			pool, err := readstore.OpenRO(dbPath)
			if err != nil {
				return fmt.Errorf("open read pool: %w", err)
			}
			defer func() { _ = pool.Close() }()

			shell := app.New(theme.Default())
			shell.Push(dashboard.New(pool))

			prog := tea.NewProgram(shell, tea.WithAltScreen(), tea.WithContext(cmd.Context()))
			if _, err := prog.Run(); err != nil {
				return fmt.Errorf("tui: %w", err)
			}
			return nil
		},
	}
}
```

- [ ] **Step 2: Make root cobra command default to TUI**

Open `cmd/app/main.go`. Locate `newRootCmd()` and add a `RunE` that invokes the TUI when no subcommand is given. Replace the `newRootCmd()` body so the returned cmd has:

```go
cmd.RunE = func(cmd *cobra.Command, args []string) error {
    return newTUICmd().RunE(cmd, args)
}
```

(Insert just before `return cmd`.)

- [ ] **Step 3: Verify build**

Run: `go build -o bin/cco ./cmd/app`
Expected: succeeds.

- [ ] **Step 4: Verify help output**

Run: `./bin/cco --help`
Expected: lists `serve`, `init`, `rebuild-rollups`, `version` (and shows that the root command also runs the TUI by default).

- [ ] **Step 5: Verify lint clean across whole repo**

Run: `go vet ./...`
Run: `go test ./...`
Expected: both clean / green.

- [ ] **Step 6: Commit**

```bash
git add cmd/app/tui.go cmd/app/main.go
git commit -m "feat(cmd): wire TUI shell + dashboard as default subcommand (M3.1)"
```

---

## Task 11: Add MANUAL-VERIFICATION.md with M3.1 checklist

**Files:**
- Create: `docs/MANUAL-VERIFICATION.md`

- [ ] **Step 1: Write the checklist**

Create `docs/MANUAL-VERIFICATION.md`:

```markdown
# Manual Verification Checklist

> Run before tagging each milestone. Items here cover what the automated suite cannot.

## Phase 3 — TUI

### M3.1 Dashboard

Prereqs: build the binary (`go build -o bin/cco ./cmd/app`).

- [ ] **Daemon-down empty state.** Delete `~/.claude-code-observer/db.sqlite` (or use a fresh `--home` dir). Run `./bin/cco`. Expect: dashboard renders with zeros and `⚠ NO DATA — IS \`cco serve\` RUNNING?` banner. No crash.
- [ ] **Live updates.** Start `./bin/cco serve` in one terminal. Run `./bin/cco` in another. In a third terminal, run `claude` and issue a prompt that triggers ≥1 tool call. The dashboard's TODAY block should update within ~2 s.
- [ ] **Stable when idle.** Stop using Claude Code. Dashboard remains stable, no flicker.
- [ ] **Numbers match SQL.** With the daemon running and some events in:
  ```bash
  sqlite3 ~/.claude-code-observer/db.sqlite \
    "SELECT printf('%.2f', SUM(cost_usd)) FROM sessions WHERE started_at >= unixepoch('now', 'start of day')*1e9"
  ```
  Compare to the dashboard's TODAY cost. Must match to 2 decimal places.
- [ ] **STALE pill on daemon kill.** Kill `cco serve`. Within ~30 s, dashboard footer pill flips to `STALE`. Restart `cco serve`; pill returns to `● LIVE`.
- [ ] **Quit.** `q` exits cleanly; `Ctrl-C` exits cleanly.
- [ ] **Refresh.** `r` triggers an immediate fetch (verifiable by changing data with `cco serve` running and pressing `r`).
```

- [ ] **Step 2: Commit**

```bash
git add docs/MANUAL-VERIFICATION.md
git commit -m "docs: add MANUAL-VERIFICATION checklist with M3.1 section"
```

---

## Task 12: Chrome rendering + status pill + panic recovery

The shell renders the header bar and footer (with keymap strip and state pill). Views report their state via a new `Status()` method on `View`.

**Files:**
- Modify: `internal/tui/app/view.go` — add `Status() theme.PillState`
- Modify: `internal/tui/app/app.go` — render chrome in `View()`; wrap `forwardTop` in `defer recover()`
- Modify: `internal/tui/dashboard/model.go` — implement `Status()`
- Modify: `internal/tui/dashboard/model_test.go` — assert `Status()` transitions

- [ ] **Step 1: Extend View interface**

Edit `internal/tui/app/view.go`. Add to the interface:

```go
import "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"

type View interface {
    Init() tea.Cmd
    Update(msg tea.Msg) (View, tea.Cmd)
    View(width, height int) string
    Title() string
    ShortHelp() []key.Binding
    // Status reports the current pill state for the footer.
    Status() theme.PillState
}
```

Update the `fakeView` in `app_test.go` to satisfy the new method:

```go
func (v *fakeView) Status() theme.PillState { return theme.PillLive }
```

- [ ] **Step 2: Implement Dashboard Status()**

Append to `internal/tui/dashboard/model.go`:

```go
import "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"

const staleAfter = 30 * time.Second

// Status implements app.View. Returns the pill state the shell should show.
func (m *Model) Status() theme.PillState {
    if m.snap.LatestEventTS == 0 && len(m.top) == 0 && m.lastOK.IsZero() {
        return theme.PillNoDaemon
    }
    if m.stale {
        return theme.PillStale
    }
    if m.snap.LatestEventTS != 0 {
        latest := time.Unix(0, m.snap.LatestEventTS)
        if m.now().Sub(latest) > staleAfter {
            return theme.PillStale
        }
    }
    return theme.PillLive
}
```

(Merge the new `import` into the existing block.)

- [ ] **Step 3: Test the Status transitions**

Append to `internal/tui/dashboard/model_test.go`:

```go
import "github.com/kamikaze011001/claude-code-observer/internal/tui/theme"

func TestModel_StatusNoDaemon(t *testing.T) {
    m := New(nil)
    if got := m.Status(); got != theme.PillNoDaemon {
        t.Fatalf("status: got %v want PillNoDaemon", got)
    }
}

func TestModel_StatusStaleOnError(t *testing.T) {
    m := New(nil)
    m.stale = true
    m.lastOK = time.Now()
    if got := m.Status(); got != theme.PillStale {
        t.Fatalf("status: got %v want PillStale", got)
    }
}

func TestModel_StatusLiveWithRecentEvent(t *testing.T) {
    m := New(nil)
    fakeNow := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
    m.now = func() time.Time { return fakeNow }
    m.lastOK = fakeNow
    m.snap.LatestEventTS = fakeNow.Add(-5 * time.Second).UnixNano()
    if got := m.Status(); got != theme.PillLive {
        t.Fatalf("status: got %v want PillLive", got)
    }
}
```

Run: `go test ./internal/tui/dashboard/... -run TestModel_Status -v`
Expected: PASS.

- [ ] **Step 4: Render chrome in App.View + add panic recovery**

Edit `internal/tui/app/app.go`. Replace the `View()` method and `forwardTop` method with:

```go
func (a *App) View() string {
    if len(a.stack) == 0 {
        return ""
    }
    top := a.stack[len(a.stack)-1]
    inner := a.safeRender(top)
    return a.renderChrome(top, inner)
}

func (a *App) safeRender(v View) (out string) {
    defer func() {
        if r := recover(); r != nil {
            out = a.theme.ErrorText.Render("⚠ VIEW ERROR — b TO RETURN")
        }
    }()
    return v.View(a.width, a.height)
}

func (a *App) renderChrome(v View, body string) string {
    title := a.theme.Heading.Render("CCO  │  " + v.Title())
    pill := a.theme.Pill(v.Status())
    helps := []string{}
    for _, k := range v.ShortHelp() {
        h := k.Help()
        helps = append(helps, "["+h.Key+"] "+h.Desc)
    }
    footer := strings.Join(helps, "  ") + "    " + pill
    return strings.Join([]string{title, body, footer}, "\n")
}
```

Add `"strings"` to the import block.

Wrap `forwardTop` in a recover so a panicking view's `Update` doesn't kill the program:

```go
func (a *App) forwardTop(msg tea.Msg) (tea.Model, tea.Cmd) {
    if len(a.stack) == 0 {
        return a, nil
    }
    defer func() {
        if r := recover(); r != nil {
            a.lastErr = fmt.Errorf("view panic: %v", r)
            a.consecErrs++
        }
    }()
    top := a.stack[len(a.stack)-1]
    updated, cmd := top.Update(msg)
    a.stack[len(a.stack)-1] = updated
    return a, cmd
}
```

Add `"fmt"` to the import block if not already present.

- [ ] **Step 5: Run all TUI tests**

Run: `go test ./internal/tui/...`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/
git commit -m "feat(tui): add chrome rendering, status pill, and panic recovery"
```

---

## Final verification

- [ ] **Step 1: Full lint**

Run: `go vet ./...`
Expected: no output.

Run: `golangci-lint run` (if installed)
Expected: clean.

- [ ] **Step 2: Full test suite**

Run: `go test ./...`
Expected: all green.

Run: `go test -cover ./internal/tui/...`
Expected: dashboard package ≥ 60% (per M3.1 gate); readstore covered through integration tests.

- [ ] **Step 3: Build artifact**

Run: `go build -o bin/cco ./cmd/app`
Expected: ~5–8 MB binary.

- [ ] **Step 4: Run the manual M3.1 checklist**

Walk through `docs/MANUAL-VERIFICATION.md` Phase 3 → M3.1 section. Tick each item.

- [ ] **Step 5: Done.** M3.1 ships.

---

## Notes for executor

- **Module path** is `github.com/kamikaze011001/claude-code-observer`. All internal imports use this prefix.
- **Existing patterns to follow:** `internal/repository/repository.go` shows the modernc-sqlite DSN style with `_pragma`. Mirror it. `internal/scheduler/scheduler.go` shows how `RealClock` and ticker patterns are wired in this codebase — use injected `now` similarly in tests.
- **Bubble Tea version:** v2 is in development. Stick to v1 (`github.com/charmbracelet/bubbletea` latest stable). If `tea.WithContext` doesn't exist on the installed version, drop that option — the cobra context isn't critical here.
- **Goldens:** ANSI is stripped before comparison so goldens diff readably in PRs. To regenerate: `go test ./internal/tui/dashboard/... -update`.
- **Don't expand scope:** M3.2 (Sessions list) and M3.3 (Prompt detail) are explicit follow-up specs. Resist adding "while we're here" navigation to other views.
