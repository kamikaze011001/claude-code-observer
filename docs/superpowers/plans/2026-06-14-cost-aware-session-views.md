# Cost-Aware Session Views Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make cost visible across the TUI — reframe the Session Detail timeline into a turn-grouped view with per-turn cost, and give the sessions list and prompt detail one shared cost-color language.

**Architecture:** Read-side / TUI only. A shared `CostStyle` tiering helper colors any USD amount (green/yellow/red). The Session Detail model changes from a flat `[]EventRow` to an interleaved sequence of collapsible turn headers (from the `prompts` rollup) and ungrouped session-level events (from `events`), with children loaded lazily on expand. No schema changes — everything derives from existing `prompts` and `events` tables.

**Tech Stack:** Go 1.25, Bubble Tea / Lipgloss, SQLite (`database/sql`). Spec: `docs/superpowers/specs/2026-06-14-cost-aware-session-views-design.md`.

---

## Verification (run after every task)

```bash
make vet && make test && make build
```

All three must pass before committing. Renderer tests assert `lipgloss.Width(out) == width`.

---

## Task 1: Shared cost-color helper (`CostStyle`)

**Files:**
- Create: `internal/tui/theme/component/cost.go`
- Test: `internal/tui/theme/component/cost_test.go`

- [ ] **Step 1: Write the failing test**

```go
package component

import (
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestCostTier(t *testing.T) {
	cases := []struct {
		usd  float64
		want CostTier
	}{
		{0, TierCheap},
		{0.0099, TierCheap},
		{0.01, TierNotable},
		{0.05, TierNotable},
		{0.0501, TierHeavy},
		{2.84, TierHeavy},
		{-1, TierCheap}, // guard: negative never panics, treated as cheap
	}
	for _, c := range cases {
		if got := tierOf(c.usd); got != c.want {
			t.Errorf("tierOf(%v)=%v want %v", c.usd, got, c.want)
		}
	}
}

func TestCostText_ColorsByTier(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	// Cheap and heavy must render different ANSI (different foreground).
	cheap := CostText(&th, 0.001)
	heavy := CostText(&th, 0.99)
	if cheap == heavy {
		t.Fatal("cheap and heavy cost text should differ in styling")
	}
	// The numeric portion must be present.
	if want := "$0.00"; !contains(cheap, want) {
		t.Errorf("cheap=%q missing %q", cheap, want)
	}
	if want := "$0.99"; !contains(heavy, want) {
		t.Errorf("heavy=%q missing %q", heavy, want)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/theme/component/ -run TestCost -v`
Expected: FAIL — `undefined: tierOf`, `CostTier`, `CostText`.

- [ ] **Step 3: Write minimal implementation**

```go
package component

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

// CostTier buckets a USD amount into a visual severity tier. Thresholds are
// absolute (not relative to a session) so users build intuition for real cents.
type CostTier int

const (
	TierCheap   CostTier = iota // < $0.01
	TierNotable                 // $0.01 – $0.05
	TierHeavy                   // > $0.05
)

// Tunable thresholds for the cost-color scale.
const (
	cheapMax   = 0.01 // strictly below => cheap
	notableMax = 0.05 // at or below => notable; above => heavy
)

func tierOf(usd float64) CostTier {
	switch {
	case usd < cheapMax:
		return TierCheap
	case usd <= notableMax:
		return TierNotable
	default:
		return TierHeavy
	}
}

// CostColor returns the palette color for a USD amount's tier.
func CostColor(t *theme.Theme, usd float64) lipgloss.Color {
	switch tierOf(usd) {
	case TierHeavy:
		return t.Palette.Red
	case TierNotable:
		return t.Palette.Yellow
	default:
		return t.Palette.Green
	}
}

// CostText renders a USD amount as "$0.00" foreground-colored by its tier.
func CostText(t *theme.Theme, usd float64) string {
	return lipgloss.NewStyle().Foreground(CostColor(t, usd)).Render(fmt.Sprintf("$%.2f", usd))
}

// CostText4 is CostText with 4 decimal places, for sub-cent per-call amounts.
func CostText4(t *theme.Theme, usd float64) string {
	return lipgloss.NewStyle().Foreground(CostColor(t, usd)).Render(fmt.Sprintf("$%.4f", usd))
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/theme/component/ -run TestCost -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/theme/component/cost.go internal/tui/theme/component/cost_test.go
git commit -m "feat(tui): add shared cost-color tiering helper

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 2: Sessions list — tier-colored cost + proportional spend bar

**Files:**
- Modify: `internal/tui/theme/component/row.go` (`SessionRowData`, `SessionRow`)
- Modify: `internal/tui/sessions/list.go` (`View` page-max calc, `formatColHeader`)
- Test: `internal/tui/theme/component/row_test.go`

- [ ] **Step 1: Write the failing test** (append to `row_test.go`)

```go
func TestSessionRow_WidthWithBar(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := SessionRowData{
		Index: 1, Started: time.Date(2026, 6, 14, 15, 4, 0, 0, time.UTC),
		ProjectName: "claude-code-observer", DurationSec: 662,
		CostUSD: 2.84, MaxCostUSD: 2.84, Prompts: 14, Tokens: 1_200_000, Live: false,
	}
	out := SessionRow(&th, r, false, 100)
	if got := lipgloss.Width(out); got != 100 {
		t.Errorf("session row width with bar: got %d want 100", got)
	}
}

func TestSessionRow_ZeroMaxCostNoPanic(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := SessionRowData{Index: 1, Started: time.Now(), ProjectName: "x", CostUSD: 0, MaxCostUSD: 0}
	out := SessionRow(&th, r, false, 100) // must not divide by zero
	if got := lipgloss.Width(out); got != 100 {
		t.Errorf("zero-max width: got %d want 100", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/theme/component/ -run TestSessionRow -v`
Expected: FAIL — `unknown field MaxCostUSD`.

- [ ] **Step 3: Add `MaxCostUSD` to `SessionRowData` and the bar to `SessionRow`**

In `row.go`, add the field to the struct:

```go
type SessionRowData struct {
	Index       int
	Started     time.Time
	ProjectName string
	DurationSec int64
	CostUSD     float64
	MaxCostUSD  float64 // largest cost on the page; scales the spend bar (0 => empty bar)
	Prompts     int64
	Tokens      int64
	Live        bool
}
```

Replace the body of `SessionRow` with the version below. It adds an `barW=10` spend
column between cost and prompts, tier-colors the cost via `CostColor`, and keeps the
width invariant.

```go
func SessionRow(t *theme.Theme, r SessionRowData, selected bool, width int) string {
	const (
		idxW        = 4
		startW      = 18
		durW        = 10
		costW       = 8
		barW        = 10
		prW         = 8
		tokW        = 7
		liveW       = 8
		gutterCount = 8 // one more gutter than before (bar column added)
	)
	projW := width - idxW - startW - durW - costW - barW - prW - tokW - liveW - gutterCount
	if projW < 4 {
		projW = 4
	}

	idx := padRight(fmt.Sprintf("%d", r.Index), idxW)
	start := padRight(r.Started.Format("2006-01-02 15:04"), startW)
	project := padRight(truncToWidth(r.ProjectName, projW), projW)
	dur := padRight(humanDuration(r.DurationSec), durW)
	costStyled := lipgloss.NewStyle().Foreground(CostColor(t, r.CostUSD)).Render(fmt.Sprintf("$%.2f", r.CostUSD))
	cost := padRight(costStyled, costW)
	bar := padRight(costBar(t, r.CostUSD, r.MaxCostUSD, barW), barW)
	prompts := padRight(fmt.Sprintf("%d", r.Prompts), prW)
	tokens := padRight(HumanInt(r.Tokens), tokW)
	live := padRight("", liveW)
	if r.Live {
		live = padRight(StatusPill(t, StatusLive), liveW)
	}

	line := lipgloss.JoinHorizontal(lipgloss.Top,
		idx, " ", start, " ", project, " ", dur, " ", cost, " ", bar, " ", prompts, " ", tokens, " ", live,
	)
	if selected {
		line = lipgloss.NewStyle().Background(t.Palette.BgAlt).Width(width).Render(line)
	} else {
		line = lipgloss.NewStyle().Width(width).Render(line)
	}
	return line
}

// costBar renders a proportional spend bar `w` cells wide: filled cells are
// tier-colored, the remainder is muted track. max<=0 => empty track.
func costBar(t *theme.Theme, cost, max float64, w int) string {
	if w <= 0 {
		return ""
	}
	filled := 0
	if max > 0 {
		filled = int(cost / max * float64(w))
		if filled > w {
			filled = w
		}
		if filled < 0 {
			filled = 0
		}
	}
	full := lipgloss.NewStyle().Foreground(CostColor(t, cost)).Render(strings.Repeat("█", filled))
	track := lipgloss.NewStyle().Foreground(t.Palette.BgAlt).Render(strings.Repeat("░", w-filled))
	return full + track
}
```

Add `"strings"` to the `row.go` import block.

- [ ] **Step 4: Update `formatColHeader` and page-max calc in `list.go`**

In `list.go`, update the column-width constants and header string in `formatColHeader`
to include the `spend` column (match the renderer exactly):

```go
func formatColHeader(w int) string {
	const (
		idxW        = 4
		startW      = 18
		durW        = 10
		costW       = 8
		barW        = 10
		prW         = 8
		tokW        = 7
		liveW       = 8
		gutterCount = 8
	)
	projW := w - idxW - startW - durW - costW - barW - prW - tokW - liveW - gutterCount
	if projW < 4 {
		projW = 4
	}
	return fmt.Sprintf("%-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s %-*s",
		idxW, "#",
		startW, "started",
		projW, "project",
		durW, "duration",
		costW, "cost",
		barW, "spend",
		prW, "prompts",
		tokW, "tokens",
		liveW, "status",
	)
}
```

In `list.go` `View`, before the row loop, compute the page max cost and pass it in:

```go
		var maxCost float64
		for _, r := range m.rows {
			if r.CostUSD > maxCost {
				maxCost = r.CostUSD
			}
		}
		columnHeader := th.Muted.Render(formatColHeader(inner))
		rows := []string{columnHeader}
		for i, r := range m.rows {
			rd := component.SessionRowData{
				Index:       i + 1,
				Started:     r.StartedAt,
				ProjectName: defaultProject(r.ProjectName),
				DurationSec: r.DurationSec,
				CostUSD:     r.CostUSD,
				MaxCostUSD:  maxCost,
				Prompts:     r.Prompts,
				Tokens:      r.Tokens,
				Live:        r.Live,
			}
			rows = append(rows, component.SessionRow(th, rd, i == m.cursor, inner))
		}
```

- [ ] **Step 5: Run tests + vet/build**

Run: `go test ./internal/tui/... -run TestSessionRow -v && make vet && make build`
Expected: PASS / clean / compiles.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/theme/component/row.go internal/tui/theme/component/row_test.go internal/tui/sessions/list.go
git commit -m "feat(tui): tier-colored cost + spend bar in sessions list

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 3: Prompt detail — tier-colored cost, cumulative column, turn total

**Files:**
- Modify: `internal/tui/theme/component/row.go` (`APIRequestRowData`, `APIRequestRow`)
- Modify: `internal/tui/prompt/detail.go` (build rows with cumulative; card header total)
- Test: `internal/tui/theme/component/row_test.go`

- [ ] **Step 1: Write the failing test** (append to `row_test.go`)

```go
func TestAPIRequestRow_WidthWithCumulative(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := APIRequestRowData{
		Time: time.Date(2026, 6, 14, 15, 4, 9, 0, time.UTC),
		Model: "claude-opus-4-8", CostUSD: 0.031, CumulativeUSD: 0.035,
		InputTokens: 4800, OutputTokens: 910,
	}
	out := APIRequestRow(&th, r, 80)
	if got := lipgloss.Width(out); got != 80 {
		t.Errorf("api row width with cumulative: got %d want 80", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/theme/component/ -run TestAPIRequestRow -v`
Expected: FAIL — `unknown field CumulativeUSD`.

- [ ] **Step 3: Add `CumulativeUSD` and rewrite `APIRequestRow`**

```go
type APIRequestRowData struct {
	Time          time.Time
	Model         string
	CostUSD       float64
	CumulativeUSD float64
	InputTokens   int64
	OutputTokens  int64
}

func APIRequestRow(t *theme.Theme, r APIRequestRowData, width int) string {
	const (
		timeW   = 8
		modelW  = 18
		costW   = 8
		cumW    = 10
		gutters = 4 // four single-space gutters
	)
	tailW := width - timeW - modelW - costW - cumW - gutters
	if tailW < 8 {
		tailW = 8
	}
	timeCol := padRight(r.Time.Format("15:04:05"), timeW)
	modelCol := padRight(truncToWidth(r.Model, modelW), modelW)
	costStyled := lipgloss.NewStyle().Foreground(CostColor(t, r.CostUSD)).Render(fmt.Sprintf("$%.4f", r.CostUSD))
	costCol := padRight(costStyled, costW)
	cumCol := padRight(t.Muted.Render(fmt.Sprintf("Σ $%.4f", r.CumulativeUSD)), cumW)
	tail := padRight(fmt.Sprintf("in %d  out %d", r.InputTokens, r.OutputTokens), tailW)
	line := lipgloss.JoinHorizontal(lipgloss.Top, timeCol, " ", modelCol, " ", costCol, " ", cumCol, " ", tail)
	return lipgloss.NewStyle().Width(width).Render(line)
}
```

- [ ] **Step 4: Wire cumulative + total in `prompt/detail.go`**

Replace the `api requests card` block (the `apiRows` loop) with a running-total version,
and add the turn total to the card title:

```go
	// api requests card — running cumulative + tier-colored cost
	apiRows := []string{}
	var cum float64
	for _, r := range d.result.APIRequests {
		cum += r.CostUSD
		apiRows = append(apiRows, component.APIRequestRow(th, component.APIRequestRowData{
			Time: r.TS, Model: r.Model, CostUSD: r.CostUSD, CumulativeUSD: cum,
			InputTokens: r.InputTokens, OutputTokens: r.OutputTokens,
		}, width-4))
	}
	apiTitle := fmt.Sprintf("api requests · total %s", component.CostText(th, p.CostUSD))
	apiCard := component.Card(th, apiTitle, strings.Join(apiRows, "\n"), width)
	if len(apiRows) == 0 {
		apiCard = component.Card(th, apiTitle, th.Muted.Render("(none)"), width)
	}
```

(`p` is already in scope as `d.result.Prompt`. `component.CostText` comes from Task 1.)

- [ ] **Step 5: Run tests + vet/build**

Run: `go test ./internal/tui/... -run 'TestAPIRequestRow|TestPrompt' -v && make vet && make build`
Expected: PASS / clean / compiles. (If a prompt-detail golden test exists, regenerate it
with its `-update*` flag and eyeball the diff.)

- [ ] **Step 6: Commit**

```bash
git add internal/tui/theme/component/row.go internal/tui/theme/component/row_test.go internal/tui/prompt/detail.go
git commit -m "feat(tui): tier-colored cost + cumulative + turn total in prompt detail

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 4: Read-store — `SessionTurns` (interleaved turns + session events)

**Files:**
- Modify: `internal/tui/readstore/queries.go` (new types + `SessionTurns`)
- Test: `internal/tui/readstore/queries_test.go`

**Design:** one ordered stream via `UNION ALL` over `prompts` (turns) and the
session-level `events` (prompt_id NULL/''), ordered `ts DESC`, single keyset cursor
(`ts`), single `LIMIT`. Column lists are aligned with NULLs.

- [ ] **Step 1: Write the failing test** (append to `queries_test.go`, mirroring the
existing in-memory SQLite fixture pattern used by other tests in this file)

```go
func TestSessionTurns_InterleavesAndPaginates(t *testing.T) {
	db := newTestDB(t) // existing helper in queries_test.go that creates schema
	// Two prompts + one session-level event (mcp connection) for session s1.
	mustExec(t, db, `INSERT INTO prompts(prompt_id,session_id,started_at,ended_at,prompt_length,command_name,cost_usd,input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,api_requests,subagent_requests,tool_calls,had_error)
		VALUES ('p1','s1',1000,2000,412,'refactor',0.036,6000,1338,0,0,3,0,2,0)`)
	mustExec(t, db, `INSERT INTO prompts(prompt_id,session_id,started_at,ended_at,prompt_length,command_name,cost_usd,input_tokens,output_tokens,cache_read_tokens,cache_creation_tokens,api_requests,subagent_requests,tool_calls,had_error)
		VALUES ('p2','s1',3000,4000,220,'explain',0.002,300,88,0,0,1,0,0,0)`)
	mustExec(t, db, `INSERT INTO events(ts,session_id,prompt_id,event_name,attrs)
		VALUES (2500,'s1','','mcp_server_connection','{"server_name":"x","state":"connected"}')`)

	items, more, err := SessionTurns(context.Background(), db, "s1", nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	if more {
		t.Errorf("hasMore=true want false")
	}
	// Newest-first by ts: p2(3000) > event(2500) > p1(1000)
	if len(items) != 3 {
		t.Fatalf("len=%d want 3", len(items))
	}
	if items[0].Kind != ItemTurn || items[0].Turn.PromptID != "p2" {
		t.Errorf("item0=%+v want turn p2", items[0])
	}
	if items[1].Kind != ItemEvent || items[1].Event.EventName != "mcp_server_connection" {
		t.Errorf("item1=%+v want mcp event", items[1])
	}
	if items[2].Kind != ItemTurn || items[2].Turn.PromptID != "p1" {
		t.Errorf("item2=%+v want turn p1", items[2])
	}
	if items[2].Turn.CostUSD != 0.036 {
		t.Errorf("p1 cost=%v want 0.036", items[2].Turn.CostUSD)
	}
}
```

If `newTestDB`/`mustExec` helpers are named differently in `queries_test.go`, use the
existing equivalents — do not introduce new helpers.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/readstore/ -run TestSessionTurns -v`
Expected: FAIL — `undefined: SessionTurns`, `ItemTurn`.

- [ ] **Step 3: Add types + query to `queries.go`**

```go
// TurnHeader is a prompt rollup row rendered as a collapsible turn header.
type TurnHeader struct {
	PromptID     string
	StartedAt    time.Time
	EndedAt      time.Time // zero when the turn is still open
	DurationSec  int64
	CommandName  string
	PromptLength int64
	CostUSD      float64
	APIRequests  int64
	ToolCalls    int64
}

// SessionItemKind discriminates the SessionItem union.
type SessionItemKind int

const (
	ItemTurn SessionItemKind = iota
	ItemEvent
)

// SessionItem is one entry in the turn-grouped timeline: either a turn header
// (Kind==ItemTurn, Turn valid) or an ungrouped session-level event
// (Kind==ItemEvent, Event valid).
type SessionItem struct {
	Kind  SessionItemKind
	Turn  TurnHeader
	Event EventRow
	TS    time.Time
}

// SessionTurns returns the turn-grouped timeline for a session, newest-first:
// prompt rollups (turns) interleaved with session-level events (events whose
// prompt_id is NULL or ''). beforeTS is the keyset cursor (ts) — nil for the
// first page. hasMore is true when len(returned) == limit.
func SessionTurns(ctx context.Context, db *sql.DB, sessionID string, beforeTS *int64, limit int) ([]SessionItem, bool, error) {
	if limit <= 0 {
		limit = 50
	}
	const q = `
SELECT kind, ts, prompt_id, started_at, ended_at, prompt_length, command_name,
       cost_usd, api_requests, tool_calls, event_name, attrs
FROM (
  SELECT 'turn' AS kind, started_at AS ts, prompt_id,
         started_at, ended_at, COALESCE(prompt_length,0) AS prompt_length,
         COALESCE(command_name,'') AS command_name, cost_usd,
         api_requests, tool_calls,
         NULL AS event_name, NULL AS attrs
  FROM prompts WHERE session_id = ?
  UNION ALL
  SELECT 'event' AS kind, ts, NULL AS prompt_id,
         NULL AS started_at, NULL AS ended_at, 0 AS prompt_length,
         '' AS command_name, 0 AS cost_usd,
         0 AS api_requests, 0 AS tool_calls,
         event_name, attrs
  FROM events WHERE session_id = ? AND (prompt_id IS NULL OR prompt_id = '')
)
WHERE (? IS NULL OR ts < ?)
ORDER BY ts DESC
LIMIT ?`
	var cur sql.NullInt64
	if beforeTS != nil {
		cur = sql.NullInt64{Int64: *beforeTS, Valid: true}
	}
	rows, err := db.QueryContext(ctx, q, sessionID, sessionID, cur, cur, limit)
	if err != nil {
		return nil, false, fmt.Errorf("session turns: %w", err)
	}
	defer rows.Close()

	out := make([]SessionItem, 0, limit)
	for rows.Next() {
		var (
			kind        string
			ts          int64
			promptID    sql.NullString
			startedAt   sql.NullInt64
			endedAt     sql.NullInt64
			promptLen   int64
			commandName string
			costUSD     float64
			apiReqs     int64
			toolCalls   int64
			eventName   sql.NullString
			attrs       []byte
		)
		if err := rows.Scan(&kind, &ts, &promptID, &startedAt, &endedAt, &promptLen,
			&commandName, &costUSD, &apiReqs, &toolCalls, &eventName, &attrs); err != nil {
			return nil, false, fmt.Errorf("session turns scan: %w", err)
		}
		item := SessionItem{TS: time.Unix(0, ts).Local()}
		if kind == "turn" {
			item.Kind = ItemTurn
			th := TurnHeader{
				PromptID: promptID.String, PromptLength: promptLen,
				CommandName: commandName, CostUSD: costUSD,
				APIRequests: apiReqs, ToolCalls: toolCalls,
			}
			if startedAt.Valid {
				th.StartedAt = time.Unix(0, startedAt.Int64).Local()
			}
			if endedAt.Valid {
				th.EndedAt = time.Unix(0, endedAt.Int64).Local()
				th.DurationSec = (endedAt.Int64 - startedAt.Int64) / int64(time.Second)
			}
			item.Turn = th
		} else {
			item.Kind = ItemEvent
			item.Event = EventRow{
				TS: item.TS, EventName: eventName.String,
				Summary: summarize(eventName.String, attrs),
			}
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("session turns iter: %w", err)
	}
	return out, len(out) == limit, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/readstore/ -run TestSessionTurns -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/readstore/queries.go internal/tui/readstore/queries_test.go
git commit -m "feat(readstore): SessionTurns interleaves prompt rollups + session events

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 5: Read-store — `SessionTurnChildren` (lazy per-turn child rows)

**Files:**
- Modify: `internal/tui/readstore/queries.go`
- Test: `internal/tui/readstore/queries_test.go`

- [ ] **Step 1: Write the failing test** (append to `queries_test.go`)

```go
func TestSessionTurnChildren_OrdersAndTypes(t *testing.T) {
	db := newTestDB(t)
	mustExec(t, db, `INSERT INTO events(ts,session_id,prompt_id,event_name,attrs)
		VALUES (1100,'s1','p1','api_request','{"model":"claude-opus-4-8","cost_usd":0.004,"input_tokens":1200,"output_tokens":340}')`)
	mustExec(t, db, `INSERT INTO events(ts,session_id,prompt_id,event_name,attrs)
		VALUES (1200,'s1','p1','tool_result','{"tool_name":"Read","duration_ms":"38","success":"true"}')`)
	children, err := SessionTurnChildren(context.Background(), db, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 {
		t.Fatalf("len=%d want 2", len(children))
	}
	if children[0].Kind != "api" || children[0].CostUSD != 0.004 {
		t.Errorf("child0=%+v want api 0.004", children[0])
	}
	if children[1].Kind != "tool" || children[1].ToolName != "Read" || !children[1].Success {
		t.Errorf("child1=%+v want tool Read ok", children[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/readstore/ -run TestSessionTurnChildren -v`
Expected: FAIL — `undefined: SessionTurnChildren`.

- [ ] **Step 3: Add type + query**

```go
// TurnChild is one api_request or tool_result event under a turn, normalized
// for inline rendering. Kind is "api" or "tool".
type TurnChild struct {
	TS           time.Time
	Kind         string
	Model        string
	CostUSD      float64
	InputTokens  int64
	OutputTokens int64
	ToolName     string
	Success      bool
	DurationMS   int64
}

// SessionTurnChildren returns the api_request and tool_result events for a
// prompt, ordered ascending by ts. Empty slice (not error) when none.
func SessionTurnChildren(ctx context.Context, db *sql.DB, promptID string) ([]TurnChild, error) {
	rows, err := db.QueryContext(ctx, `
SELECT ts, event_name, attrs
FROM events
WHERE prompt_id = ? AND event_name IN ('api_request','tool_result')
ORDER BY ts`, promptID)
	if err != nil {
		return nil, fmt.Errorf("turn children: %w", err)
	}
	defer rows.Close()

	out := make([]TurnChild, 0)
	for rows.Next() {
		var (
			ts        int64
			eventName string
			attrs     []byte
		)
		if err := rows.Scan(&ts, &eventName, &attrs); err != nil {
			return nil, fmt.Errorf("turn children scan: %w", err)
		}
		var a map[string]any
		_ = json.Unmarshal(attrs, &a)
		c := TurnChild{TS: time.Unix(0, ts).Local()}
		switch eventName {
		case domain.EventAPIRequest:
			c.Kind = "api"
			c.Model, _ = a["model"].(string)
			if v, ok := a["cost_usd"].(float64); ok {
				c.CostUSD = v
			}
			if v, ok := a["input_tokens"].(float64); ok {
				c.InputTokens = int64(v)
			}
			if v, ok := a["output_tokens"].(float64); ok {
				c.OutputTokens = int64(v)
			}
		case domain.EventToolResult:
			c.Kind = "tool"
			c.ToolName, _ = a["tool_name"].(string)
			if v, ok := attrInt(a, "duration_ms"); ok {
				c.DurationMS = v
			}
			if v, ok := attrBool(a, "success"); ok {
				c.Success = v
			}
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("turn children iter: %w", err)
	}
	return out, nil
}
```

(`attrInt`/`attrBool` live in `readstore/summarize.go`; `domain` is already imported in `queries.go`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/readstore/ -run TestSessionTurnChildren -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/readstore/queries.go internal/tui/readstore/queries_test.go
git commit -m "feat(readstore): SessionTurnChildren for lazy per-turn child rows

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 6: Component renderers — `TurnHeaderRow` + `TurnChildRow`

**Files:**
- Modify: `internal/tui/theme/component/row.go`
- Test: `internal/tui/theme/component/row_test.go`

- [ ] **Step 1: Write the failing test** (append to `row_test.go`)

```go
func TestTurnHeaderRow_Width(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	r := TurnHeaderRowData{
		Time: time.Date(2026, 6, 14, 15, 4, 1, 0, time.UTC),
		Label: "/refactor", PromptLength: 412, DurationSec: 11,
		Calls: 3, CostUSD: 0.036, Expanded: true,
	}
	for _, w := range []int{60, 90, 120} {
		out := TurnHeaderRow(&th, r, true, w)
		if got := lipgloss.Width(out); got != w {
			t.Errorf("turn header width: got %d want %d", got, w)
		}
	}
}

func TestTurnChildRow_Width(t *testing.T) {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	api := TurnChildRowData{Kind: "api", Model: "claude-opus-4-8", CostUSD: 0.031, InputTokens: 4800, OutputTokens: 910, Last: false}
	tool := TurnChildRowData{Kind: "tool", ToolName: "Read", Success: true, DurationMS: 38, Last: true}
	for _, rd := range []TurnChildRowData{api, tool} {
		out := TurnChildRow(&th, rd, 90)
		if got := lipgloss.Width(out); got != 90 {
			t.Errorf("turn child width (%s): got %d want 90", rd.Kind, got)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/theme/component/ -run 'TestTurn' -v`
Expected: FAIL — `undefined: TurnHeaderRowData`.

- [ ] **Step 3: Implement the renderers in `row.go`**

```go
// TurnHeaderRowData is one collapsible turn header in the session timeline.
type TurnHeaderRowData struct {
	Time         time.Time
	Label        string // command name (e.g. "/refactor") or "prompt"
	PromptLength int64
	DurationSec  int64
	Calls        int64
	CostUSD      float64
	Expanded     bool
}

func TurnHeaderRow(t *theme.Theme, r TurnHeaderRowData, selected bool, width int) string {
	const (
		glyphW = 2 // "▾ " / "▸ "
		timeW  = 8
		costW  = 8
		gutter = 3
	)
	glyph := "▸"
	if r.Expanded {
		glyph = "▾"
	}
	glyphCol := padRight(glyph, glyphW)
	timeCol := padRight(r.Time.Format("15:04:05"), timeW)
	// meta: "412ch · 11s · 3 calls"
	meta := fmt.Sprintf("%dch", r.PromptLength)
	if d := humanDuration(r.DurationSec); d != "" {
		meta += " · " + d
	}
	meta += fmt.Sprintf(" · %d calls", r.Calls)
	labelW := width - glyphW - timeW - costW - gutter
	if labelW < 4 {
		labelW = 4
	}
	label := truncToWidth(r.Label+"  "+meta, labelW)
	labelCol := padRight(label, labelW)
	costStyled := lipgloss.NewStyle().Foreground(CostColor(t, r.CostUSD)).Render(fmt.Sprintf("$%.3f", r.CostUSD))
	costCol := padRight(costStyled, costW)
	line := lipgloss.JoinHorizontal(lipgloss.Top, glyphCol, " ", timeCol, " ", labelCol, " ", costCol)
	s := lipgloss.NewStyle().Width(width)
	if selected {
		s = s.Background(t.Palette.BgAlt).Foreground(t.Palette.Accent)
	} else {
		s = s.Background(t.Palette.BgAlt)
	}
	return s.Render(line)
}

// TurnChildRowData is one api/tool row nested under an expanded turn.
type TurnChildRowData struct {
	Kind         string // "api" | "tool"
	Model        string
	CostUSD      float64
	InputTokens  int64
	OutputTokens int64
	ToolName     string
	Success      bool
	DurationMS   int64
	Last         bool // draws ╰ connector instead of ├
}

func TurnChildRow(t *theme.Theme, r TurnChildRowData, width int) string {
	const (
		connW = 4 // "   ├" / "   ╰"
		costW = 8
		gutter = 2
	)
	conn := "   ├"
	if r.Last {
		conn = "   ╰"
	}
	connCol := lipgloss.NewStyle().Foreground(t.Palette.FgMuted).Render(conn)
	bodyW := width - connW - costW - gutter
	if bodyW < 4 {
		bodyW = 4
	}
	var body, costCol string
	if r.Kind == "api" {
		tag := lipgloss.NewStyle().Foreground(t.Palette.Green).Render("api ")
		txt := fmt.Sprintf("%s   in %s · out %s", truncToWidth(r.Model, 18),
			HumanInt(r.InputTokens), HumanInt(r.OutputTokens))
		body = padRight(tag+truncToWidth(txt, bodyW-4), bodyW)
		costStyled := lipgloss.NewStyle().Foreground(CostColor(t, r.CostUSD)).Render(fmt.Sprintf("$%.4f", r.CostUSD))
		costCol = padRight(costStyled, costW)
	} else {
		mark := t.Glyphs.Check
		markStyle := lipgloss.NewStyle().Foreground(t.Palette.Green)
		if !r.Success {
			mark = t.Glyphs.Cross
			markStyle = lipgloss.NewStyle().Foreground(t.Palette.Red)
		}
		tag := lipgloss.NewStyle().Foreground(t.Palette.FgMuted).Render("tool ")
		txt := fmt.Sprintf("%s %s %dms", truncToWidth(r.ToolName, 16), markStyle.Render(mark), r.DurationMS)
		body = padRight(tag+txt, bodyW)
		costCol = padRight(lipgloss.NewStyle().Foreground(t.Palette.FgMuted).Render("—"), costW)
	}
	line := lipgloss.JoinHorizontal(lipgloss.Top, connCol, " ", body, " ", costCol)
	return lipgloss.NewStyle().Width(width).Render(line)
}
```

> Note on width math: `truncToWidth` accounts for display cells, but styled `mark`
> glyphs add zero display width beyond the rune. If any width assertion fails by a
> cell, adjust the literal `-4` / inner truncations until `lipgloss.Width(out) == width`
> holds at all three test widths — the test is the source of truth.

- [ ] **Step 4: Run tests + vet/build**

Run: `go test ./internal/tui/theme/component/ -run 'TestTurn' -v && make vet && make build`
Expected: PASS / clean / compiles.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/theme/component/row.go internal/tui/theme/component/row_test.go
git commit -m "feat(tui): TurnHeaderRow + TurnChildRow renderers

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 7: Detail model — turn-grouped data load + render

**Files:**
- Modify: `internal/tui/sessions/detail.go`
- Test: `internal/tui/sessions/detail_test.go`

This task replaces the flat `[]EventRow` model with a turn-grouped item model. Expand/collapse
and navigation behavior come in Task 8; this task lands the data structures, the fetch, the
default-latest-expanded rule, and rendering of headers/ungrouped-events (children render only
when expanded, populated in Task 8).

- [ ] **Step 1: Write the failing test** (replace `TestDetail_KeyJK` and add a load test;
keep `TestDetail_Title`/`TestDetail_StatusPill` as-is)

```go
func TestDetail_DefaultLatestExpanded(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.applyItems([]readstore.SessionItem{
		{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p2"}, TS: time.Unix(0, 3000)},
		{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p1"}, TS: time.Unix(0, 1000)},
	}, false)
	if !m.items[0].expanded {
		t.Error("latest turn (items[0]) should be expanded by default")
	}
	if m.items[1].expanded {
		t.Error("older turn should be collapsed by default")
	}
}

func TestDetail_KeyJK(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.applyItems([]readstore.SessionItem{
		{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p1"}, TS: time.Unix(0, 2000)},
		{Kind: readstore.ItemEvent, Event: readstore.EventRow{EventName: "auth"}, TS: time.Unix(0, 1000)},
	}, false)
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if upd.(*Detail).cursor != 1 {
		t.Fatalf("cursor=%d want 1", upd.(*Detail).cursor)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/sessions/ -run TestDetail -v`
Expected: FAIL — `m.applyItems undefined`, `m.items undefined`.

- [ ] **Step 3: Rewrite the model + fetch + render in `detail.go`**

Replace the `Detail` struct's `events` field and related types with the item model. Key
changes (apply throughout the file):

```go
// timelineItem wraps a SessionItem with view state. For turns, children are
// loaded lazily on first expand (Task 8).
type timelineItem struct {
	readstore.SessionItem
	expanded bool
	children []readstore.TurnChild
	loaded   bool
}

type detailDataMsg struct {
	items   []readstore.SessionItem
	hasMore bool
	at      time.Time
}

type detailOlderMsg struct {
	items   []readstore.SessionItem
	hasMore bool
	at      time.Time
}

type Detail struct {
	pool         *sql.DB
	theme        *theme.Theme
	sessionID    string
	items        []timelineItem
	cursor       int
	offset       int
	viewport     int
	hasMore      bool
	loadingOlder bool
	inFlight     bool
	stale        bool
	lastOK       time.Time

	keys listKeys
}

// applyItems replaces the item list (older=false) or appends (older=true),
// expanding the most-recent turn by default on a fresh load.
func (m *Detail) applyItems(items []readstore.SessionItem, older bool) {
	conv := make([]timelineItem, len(items))
	for i, it := range items {
		conv[i] = timelineItem{SessionItem: it}
	}
	if older {
		m.items = append(m.items, conv...)
		return
	}
	// default-latest-expanded: first turn in the (newest-first) list
	for i := range conv {
		if conv[i].Kind == readstore.ItemTurn {
			conv[i].expanded = true
			break
		}
	}
	m.items = conv
}
```

Update `fetchCmd`/`fetchOlderCmd` to call `readstore.SessionTurns` and return
`detailDataMsg{items: ...}` / `detailOlderMsg{items: ...}`. Update the `detailDataMsg`
and `detailOlderMsg` cases in `Update` to call `m.applyItems(v.items, false)` /
`m.applyItems(v.items, true)` (preserve the cursor-restore logic by matching on
`TS`+`Kind` where it previously matched events). The `older` cursor key uses
`m.items[len-1].TS.UnixNano()`.

Replace the `View` row loop to render via the new renderers. For each visible item:

```go
	for i := m.offset; i < end; i++ {
		it := m.items[i]
		selected := i == m.cursor
		switch it.Kind {
		case readstore.ItemTurn:
			th2 := it.Turn
			label := th2.CommandName
			if label == "" {
				label = "prompt"
			} else {
				label = "/" + label
			}
			rows = append(rows, component.TurnHeaderRow(th, component.TurnHeaderRowData{
				Time: th2.StartedAt, Label: label, PromptLength: th2.PromptLength,
				DurationSec: th2.DurationSec, Calls: th2.APIRequests, CostUSD: th2.CostUSD,
				Expanded: it.expanded,
			}, selected, width-6))
			if it.expanded {
				for ci, c := range it.children {
					rows = append(rows, component.TurnChildRow(th, component.TurnChildRowData{
						Kind: c.Kind, Model: c.Model, CostUSD: c.CostUSD,
						InputTokens: c.InputTokens, OutputTokens: c.OutputTokens,
						ToolName: c.ToolName, Success: c.Success, DurationMS: c.DurationMS,
						Last: ci == len(it.children)-1,
					}, width-6))
				}
			}
		case readstore.ItemEvent:
			rows = append(rows, component.EventRow(th, component.EventRowData{
				Time: it.Event.TS, EventName: it.Event.EventName, Summary: it.Event.Summary,
				IsPrompt: false,
			}, selected, width-6))
		}
	}
```

> The visible-window math (`visibleRows`, `clampOffset`, `m.viewport`) operates on
> `len(m.items)` (header rows), not expanded child rows. Children render inside a header's
> slot; keep cursor navigation at the item level. Update the column-header line at the top
> of the card from the old `time/event/summary` to `time  turn / event                       cost`.

Update the empty-state guard from `len(m.events) == 0` to `len(m.items) == 0`, and the
`TickMsg` refresh guard's `len(m.events) > detailPageSize` to `len(m.items) > detailPageSize`.

- [ ] **Step 4: Run tests + vet/build**

Run: `go test ./internal/tui/sessions/ -run TestDetail -v && make vet && make build`
Expected: PASS / clean / compiles. (Regenerate any detail golden via its `-update-detail`
flag and eyeball the diff.)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/sessions/detail.go internal/tui/sessions/detail_test.go
git commit -m "feat(tui): turn-grouped session detail model + rendering

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 8: Detail — expand/collapse, lazy child load, Enter opens prompt detail

**Files:**
- Modify: `internal/tui/sessions/detail.go`
- Test: `internal/tui/sessions/detail_test.go`

- [ ] **Step 1: Write the failing tests** (append to `detail_test.go`)

```go
func TestDetail_SpaceTogglesExpand(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.applyItems([]readstore.SessionItem{
		{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p1"}, TS: time.Unix(0, 1000)},
	}, false)
	m.items[0].expanded = false
	m.items[0].loaded = true // pretend children already loaded; no pool needed
	m.cursor = 0
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if !upd.(*Detail).items[0].expanded {
		t.Fatal("space should expand a collapsed turn")
	}
}

func TestDetail_EnterOnTurnPushesPromptDetail(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.applyItems([]readstore.SessionItem{
		{Kind: readstore.ItemTurn, Turn: readstore.TurnHeader{PromptID: "p1"}, TS: time.Unix(0, 1000)},
	}, false)
	m.cursor = 0
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected push cmd")
	}
	if _, ok := cmd().(app.PushViewMsg); !ok {
		t.Fatal("want PushViewMsg")
	}
}

func TestDetail_EnterOnEventDoesNothing(t *testing.T) {
	t.Parallel()
	m := NewDetail(nil, "s1", nil).(*Detail)
	m.applyItems([]readstore.SessionItem{
		{Kind: readstore.ItemEvent, Event: readstore.EventRow{EventName: "auth"}, TS: time.Unix(0, 1000)},
	}, false)
	m.cursor = 0
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no cmd on session-level event")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/sessions/ -run TestDetail -v`
Expected: FAIL — Space not handled; Enter on turn returns nil.

- [ ] **Step 3: Add key bindings + child-load command**

Add a `Toggle` binding to `listKeys` in `list.go` (`key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "expand"))`).

Add a child-fetch message + command, and handle Space/Enter in `Detail.Update`:

```go
type detailChildrenMsg struct {
	promptID string
	children []readstore.TurnChild
}

func (m *Detail) fetchChildrenCmd(promptID string) tea.Cmd {
	pool := m.pool
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), detailFetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		ch, err := readstore.SessionTurnChildren(ctx, pool, promptID)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return detailChildrenMsg{promptID: promptID, children: ch}
	}
}
```

In `Update`, add the `detailChildrenMsg` case:

```go
	case detailChildrenMsg:
		for i := range m.items {
			if m.items[i].Kind == readstore.ItemTurn && m.items[i].Turn.PromptID == v.promptID {
				m.items[i].children = v.children
				m.items[i].loaded = true
				break
			}
		}
		return m, nil
```

In the `tea.KeyMsg` switch, add Toggle and rework Enter:

```go
		case key.Matches(v, m.keys.Toggle):
			if len(m.items) == 0 || m.items[m.cursor].Kind != readstore.ItemTurn {
				return m, nil
			}
			it := &m.items[m.cursor]
			it.expanded = !it.expanded
			if it.expanded && !it.loaded {
				return m, m.fetchChildrenCmd(it.Turn.PromptID)
			}
			return m, nil
		case key.Matches(v, m.keys.Enter):
			if len(m.items) == 0 || m.items[m.cursor].Kind != readstore.ItemTurn {
				return m, nil
			}
			pid := m.items[m.cursor].Turn.PromptID
			if pid == "" {
				return m, nil
			}
			pool, th := m.pool, m.theme
			return m, func() tea.Msg {
				return app.PushViewMsg{V: newPromptDetail(pool, pid, th)}
			}
```

Update `helpHints` and `ShortHelp` to include `space expand`.

- [ ] **Step 4: Run tests + vet/build**

Run: `go test ./internal/tui/sessions/ -run TestDetail -v && make vet && make test && make build`
Expected: PASS / clean / compiles.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/sessions/detail.go internal/tui/sessions/list.go internal/tui/sessions/detail_test.go
git commit -m "feat(tui): expand/collapse turns with lazy child load; Enter opens prompt detail

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Task 9: Docs + manual verification

**Files:**
- Modify: `docs/CLAUDE-CODE-OTEL.md` only if a new attribute was relied upon (none expected — skip if so).
- Modify: `README.md` if it documents the session-detail view's columns/keys.

- [ ] **Step 1: Manual smoke test**

```bash
make build
./bin/claude-code-observer serve   # in one shell, if not already running
./bin/claude-code-observer          # open TUI; navigate Sessions → a session
```

Verify: sessions list shows colored cost + spend bars; opening a session shows turn
headers with the latest expanded; `space` toggles expand and loads children; `enter`
opens prompt detail; session-level events appear as muted ungrouped rows; prompt detail
shows cumulative + turn total. Confirm at a narrow terminal width that nothing wraps.

- [ ] **Step 2: Update README key/column docs if present**

If `README.md` lists the session-detail keys or sessions-list columns, add `space —
expand/collapse turn` and the `spend` column. (Skip if README doesn't enumerate these.)

- [ ] **Step 3: Final full verification**

Run: `make vet && make test && make build`
Expected: all pass.

- [ ] **Step 4: Commit (if any doc changes)**

```bash
git add README.md
git commit -m "docs: note turn-grouped session detail keys + spend column

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

## Self-Review Notes

- **Spec coverage:** shared cost scale (Task 1) → list (Task 2) → prompt detail (Task 3) →
  SessionTurns interleave + pagination (Task 4) → lazy children (Task 5) → turn renderers
  (Task 6) → grouped model + default-latest-expanded + interleaved events (Task 7) →
  expand/collapse + Enter-opens-detail (Task 8). All spec sections mapped.
- **Type consistency:** `CostColor`/`CostText` (Task 1) used in Tasks 2/3/6;
  `SessionItem`/`ItemTurn`/`TurnHeader` (Task 4) used in Tasks 7/8; `TurnChild` (Task 5)
  used in Tasks 6/8; `TurnHeaderRowData`/`TurnChildRowData` (Task 6) used in Task 7.
- **No schema migration:** all queries read existing `prompts`/`events` tables.
- **Deferred-to-plan decisions now fixed:** expand key = `space`; children load lazily on
  first expand.
