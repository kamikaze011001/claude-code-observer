# Subagent Waterfall View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a full-screen TUI page, reachable with `w` from Prompt Detail, that renders every `api_request`/`api_error` under a prompt as a horizontal timeline banded into `main`/`subagent`/`auxiliary` lanes.

**Architecture:** A new `internal/tui/waterfall/` package with an `app.View` implementation. Pure layout math (lane bucketing, offset/scale, greedy row packing) lives in `layout.go` and is unit-tested in isolation; rendering lives in `view.go` and is golden-tested. A new `readstore.PromptWaterfall` query reads the already-persisted `events.attrs` JSON — no schema or eventparser changes.

**Tech Stack:** Go 1.25, Bubble Tea / lipgloss, SQLite (`database/sql`), existing `internal/tui/theme` + `theme/component` packages.

**Spec:** `docs/superpowers/specs/2026-05-14-subagent-waterfall-view-design.md`

---

## File Structure

| Path | Responsibility | Action |
|------|----------------|--------|
| `internal/tui/readstore/queries.go` | Add `WaterfallRequest` type + `PromptWaterfall` query | Modify |
| `internal/tui/readstore/queries_test.go` | Test `PromptWaterfall` against a temp SQLite | Modify |
| `internal/tui/waterfall/doc.go` | Package doc comment | Create |
| `internal/tui/waterfall/layout.go` | Pure layout math: lane bucketing, bar building, packing, scaling | Create |
| `internal/tui/waterfall/layout_test.go` | Table-driven tests for `layout.go` | Create |
| `internal/tui/waterfall/model.go` | `Model` struct, `Init`/`Update`, fetch command | Create |
| `internal/tui/waterfall/model_test.go` | Unit tests for `Model` state transitions | Create |
| `internal/tui/waterfall/view.go` | `View()` rendering + `Title`/`ShortHelp`/`Status` | Create |
| `internal/tui/waterfall/view_test.go` | Golden-file tests for `View()` | Create |
| `internal/tui/waterfall/testdata/*.golden` | Golden fixtures | Create (via update flag) |
| `internal/tui/prompt/detail.go` | Wire `w` key → push waterfall view | Modify |
| `internal/tui/prompt/detail_test.go` | Test the `w` key emits a `PushViewMsg` | Modify |
| `docs/CLAUDE-CODE-OTEL.md` | Correct `query_source` description in §8.2 / §8.3 | Modify |

---

## Task 1: `readstore.PromptWaterfall` query

**Files:**
- Modify: `internal/tui/readstore/queries.go`
- Test: `internal/tui/readstore/queries_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/readstore/queries_test.go`:

```go
func TestPromptWaterfall_ParsesAndOrders(t *testing.T) {
	home := t.TempDir()
	repo, err := repository.Open(home)
	if err != nil {
		t.Fatalf("repository.Open: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	insertEvent := func(tsNano int64, name, attrs string) {
		_, err := repo.DB().ExecContext(context.Background(),
			`INSERT INTO events (ts, session_id, prompt_id, event_name, attrs)
			 VALUES (?, ?, ?, ?, ?)`,
			tsNano, "sess-1", "prompt-1", name, attrs)
		if err != nil {
			t.Fatalf("insert event: %v", err)
		}
	}

	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	// Out-of-order inserts; query must return ts-ascending.
	insertEvent(base.Add(5*time.Second).UnixNano(), "api_request",
		`{"query_source":"subagent","duration_ms":2000,"model":"claude-sonnet-4-6","cost_usd":0.04,"input_tokens":1200,"output_tokens":800}`)
	insertEvent(base.Add(1*time.Second).UnixNano(), "api_request",
		`{"query_source":"repl_main_thread","duration_ms":900,"model":"claude-opus-4-7","cost_usd":0.21,"input_tokens":8000,"output_tokens":2000}`)
	insertEvent(base.Add(7*time.Second).UnixNano(), "api_error",
		`{"query_source":"subagent","duration_ms":1500,"model":"claude-sonnet-4-6","error":"overloaded"}`)
	// Unrelated event must be ignored.
	insertEvent(base.Add(2*time.Second).UnixNano(), "tool_result",
		`{"tool_name":"Bash","duration_ms":100}`)

	pool, err := readstore.OpenRO(filepath.Join(home, "db.sqlite"))
	if err != nil {
		t.Fatalf("OpenRO: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	got, err := readstore.PromptWaterfall(context.Background(), pool, "prompt-1")
	if err != nil {
		t.Fatalf("PromptWaterfall: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 rows, got %d", len(got))
	}
	if got[0].QuerySource != "repl_main_thread" {
		t.Fatalf("row 0 not ts-ascending: %+v", got[0])
	}
	if got[0].Model != "claude-opus-4-7" || got[0].DurationMS != 900 || got[0].CostUSD != 0.21 {
		t.Fatalf("row 0 fields wrong: %+v", got[0])
	}
	if got[0].InputTokens != 8000 || got[0].OutputTokens != 2000 {
		t.Fatalf("row 0 tokens wrong: %+v", got[0])
	}
	if got[1].QuerySource != "subagent" || got[1].IsError {
		t.Fatalf("row 1 wrong: %+v", got[1])
	}
	if !got[2].IsError || got[2].QuerySource != "subagent" {
		t.Fatalf("row 2 should be the api_error: %+v", got[2])
	}
}

func TestPromptWaterfall_EmptyWhenNoRequests(t *testing.T) {
	home := t.TempDir()
	repo, err := repository.Open(home)
	if err != nil {
		t.Fatalf("repository.Open: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	pool, err := readstore.OpenRO(filepath.Join(home, "db.sqlite"))
	if err != nil {
		t.Fatalf("OpenRO: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	got, err := readstore.PromptWaterfall(context.Background(), pool, "missing")
	if err != nil {
		t.Fatalf("PromptWaterfall: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty slice, got %d rows", len(got))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/readstore/ -run TestPromptWaterfall -v`
Expected: FAIL — `undefined: readstore.PromptWaterfall` (compile error).

- [ ] **Step 3: Write the implementation**

Append to `internal/tui/readstore/queries.go`:

```go
// WaterfallRequest is one api_request or api_error event under a prompt,
// carrying the fields the waterfall view needs. TS is the event timestamp
// (fired at stream-end); the request start is TS - DurationMS.
type WaterfallRequest struct {
	TS           time.Time
	DurationMS   int64
	QuerySource  string // raw, free-form (e.g. "repl_main_thread", "compact", a subagent name)
	Model        string
	CostUSD      float64
	InputTokens  int64
	OutputTokens int64
	IsError      bool // true when the row came from an api_error event
}

// PromptWaterfall returns the api_request and api_error events for a prompt,
// ordered ascending by ts. Returns an empty slice (not an error) when the
// prompt has no such events.
func PromptWaterfall(ctx context.Context, db *sql.DB, promptID string) ([]WaterfallRequest, error) {
	rows, err := db.QueryContext(ctx, `
SELECT ts, event_name, attrs
FROM events
WHERE prompt_id = ? AND event_name IN ('api_request','api_error')
ORDER BY ts`, promptID)
	if err != nil {
		return nil, fmt.Errorf("prompt waterfall: %w", err)
	}
	defer rows.Close()

	var out []WaterfallRequest
	for rows.Next() {
		var (
			ts        int64
			eventName string
			attrs     []byte
		)
		if err := rows.Scan(&ts, &eventName, &attrs); err != nil {
			return nil, fmt.Errorf("prompt waterfall scan: %w", err)
		}
		var a map[string]any
		_ = json.Unmarshal(attrs, &a)
		r := WaterfallRequest{
			TS:      time.Unix(0, ts).Local(),
			IsError: eventName == domain.EventAPIError,
		}
		r.QuerySource, _ = a["query_source"].(string)
		r.Model, _ = a["model"].(string)
		if v, ok := a["duration_ms"].(float64); ok {
			r.DurationMS = int64(v)
		}
		if v, ok := a["cost_usd"].(float64); ok {
			r.CostUSD = v
		}
		if v, ok := a["input_tokens"].(float64); ok {
			r.InputTokens = int64(v)
		}
		if v, ok := a["output_tokens"].(float64); ok {
			r.OutputTokens = int64(v)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("prompt waterfall iter: %w", err)
	}
	return out, nil
}
```

Confirm `domain.EventAPIError` exists; if the constant is named differently, check `internal/domain/wire.go` and use the actual name for the `api_error` event.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/readstore/ -run TestPromptWaterfall -v`
Expected: PASS (both tests).

- [ ] **Step 5: Run full verification**

Run: `make vet && go test ./internal/tui/readstore/`
Expected: clean vet, all readstore tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/readstore/queries.go internal/tui/readstore/queries_test.go
git commit -m "feat(readstore): add PromptWaterfall query for api_request/api_error events

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: `query_source` lane bucketing

**Files:**
- Create: `internal/tui/waterfall/doc.go`
- Create: `internal/tui/waterfall/layout.go`
- Test: `internal/tui/waterfall/layout_test.go`

- [ ] **Step 1: Create the package doc file**

Create `internal/tui/waterfall/doc.go`:

```go
// Package waterfall renders a per-prompt timeline of api_request and
// api_error events, banded by query_source into main/subagent/auxiliary
// lanes. See docs/superpowers/specs/2026-05-14-subagent-waterfall-view-design.md.
package waterfall
```

- [ ] **Step 2: Write the failing test**

Create `internal/tui/waterfall/layout_test.go`:

```go
package waterfall

import "testing"

func TestBucketLane(t *testing.T) {
	t.Parallel()
	cases := []struct {
		querySource string
		want        LaneKind
	}{
		{"main", LaneMain},
		{"repl_main_thread", LaneMain},
		{"", LaneMain},
		{"auxiliary", LaneAuxiliary},
		{"compact", LaneAuxiliary},
		{"subagent", LaneSubagent},
		{"general-purpose", LaneSubagent},
		{"Explore", LaneSubagent},
	}
	for _, c := range cases {
		if got := bucketLane(c.querySource); got != c.want {
			t.Errorf("bucketLane(%q) = %v, want %v", c.querySource, got, c.want)
		}
	}
}

func TestLaneKindString(t *testing.T) {
	t.Parallel()
	cases := map[LaneKind]string{
		LaneMain:      "main",
		LaneSubagent:  "subagent",
		LaneAuxiliary: "auxiliary",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/tui/waterfall/ -run TestBucketLane -v`
Expected: FAIL — `undefined: LaneKind` / `undefined: bucketLane`.

- [ ] **Step 4: Write the implementation**

Create `internal/tui/waterfall/layout.go`:

```go
package waterfall

// LaneKind is one of the three fixed waterfall lanes.
type LaneKind int

const (
	LaneMain LaneKind = iota
	LaneSubagent
	LaneAuxiliary
)

func (l LaneKind) String() string {
	switch l {
	case LaneSubagent:
		return "subagent"
	case LaneAuxiliary:
		return "auxiliary"
	default:
		return "main"
	}
}

// bucketLane maps a free-form query_source string (as seen on log events)
// to one of the three fixed lanes. Empty / unknown values fall through to
// the subagent lane, except the explicit main/auxiliary aliases.
func bucketLane(querySource string) LaneKind {
	switch querySource {
	case "", "main", "repl_main_thread":
		return LaneMain
	case "auxiliary", "compact":
		return LaneAuxiliary
	default:
		return LaneSubagent
	}
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/tui/waterfall/ -run 'TestBucketLane|TestLaneKindString' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/waterfall/doc.go internal/tui/waterfall/layout.go internal/tui/waterfall/layout_test.go
git commit -m "feat(waterfall): add query_source lane bucketing

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 3: Bar building (offsets + total span)

**Files:**
- Modify: `internal/tui/waterfall/layout.go`
- Test: `internal/tui/waterfall/layout_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/waterfall/layout_test.go`:

```go
import (
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)
```

(Merge this with the existing import block — keep a single `import (...)`.)

```go
func TestBuildBars_OffsetsAndSpan(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	// req A: starts at base+0ms, runs 1000ms  (TS = base+1000ms)
	// req B: starts at base+500ms, runs 2000ms (TS = base+2500ms)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(1000 * time.Millisecond), DurationMS: 1000, QuerySource: "main"},
		{TS: base.Add(2500 * time.Millisecond), DurationMS: 2000, QuerySource: "subagent"},
	}
	bars, totalSpanMS := buildBars(reqs)
	if len(bars) != 2 {
		t.Fatalf("want 2 bars, got %d", len(bars))
	}
	if bars[0].OffsetMS != 0 {
		t.Errorf("bar 0 offset = %d, want 0", bars[0].OffsetMS)
	}
	if bars[1].OffsetMS != 500 {
		t.Errorf("bar 1 offset = %d, want 500", bars[1].OffsetMS)
	}
	if bars[0].Lane != LaneMain || bars[1].Lane != LaneSubagent {
		t.Errorf("lanes wrong: %v %v", bars[0].Lane, bars[1].Lane)
	}
	// span = latest end (base+2500ms) - earliest start (base+0ms) = 2500ms
	if totalSpanMS != 2500 {
		t.Errorf("totalSpanMS = %d, want 2500", totalSpanMS)
	}
}

func TestBuildBars_Empty(t *testing.T) {
	t.Parallel()
	bars, span := buildBars(nil)
	if len(bars) != 0 || span != 0 {
		t.Fatalf("want empty, got %d bars span %d", len(bars), span)
	}
}

func TestBuildBars_ZeroDurationClamped(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	reqs := []readstore.WaterfallRequest{
		{TS: base, DurationMS: 0, QuerySource: "main"},
	}
	bars, span := buildBars(reqs)
	if len(bars) != 1 || bars[0].OffsetMS != 0 {
		t.Fatalf("unexpected bars: %+v", bars)
	}
	if span != 0 {
		t.Errorf("span = %d, want 0", span)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/waterfall/ -run TestBuildBars -v`
Expected: FAIL — `undefined: buildBars` / `undefined: Bar`.

- [ ] **Step 3: Write the implementation**

Append to `internal/tui/waterfall/layout.go`:

```go
import (
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)
```

(Add this import block at the top of `layout.go`, just under the `package waterfall` line.)

```go
// Bar is a single request positioned on the timeline.
type Bar struct {
	Req      readstore.WaterfallRequest
	Lane     LaneKind
	OffsetMS int64 // start offset from the earliest bar start
}

// startOf returns the request start time: TS is stream-end, so start = TS - duration.
func startOf(r readstore.WaterfallRequest) time.Time {
	return r.TS.Add(-time.Duration(r.DurationMS) * time.Millisecond)
}

// buildBars converts raw requests (assumed ts-ascending) into Bars with a
// computed lane and a start offset relative to the earliest start. It also
// returns the total timeline span in milliseconds (latest end - earliest start).
func buildBars(reqs []readstore.WaterfallRequest) (bars []Bar, totalSpanMS int64) {
	if len(reqs) == 0 {
		return nil, 0
	}
	earliest := startOf(reqs[0])
	for _, r := range reqs[1:] {
		if s := startOf(r); s.Before(earliest) {
			earliest = s
		}
	}
	var latestEnd time.Time
	for _, r := range reqs {
		offset := startOf(r).Sub(earliest).Milliseconds()
		bars = append(bars, Bar{
			Req:      r,
			Lane:     bucketLane(r.QuerySource),
			OffsetMS: offset,
		})
		if r.TS.After(latestEnd) {
			latestEnd = r.TS
		}
	}
	totalSpanMS = latestEnd.Sub(earliest).Milliseconds()
	if totalSpanMS < 0 {
		totalSpanMS = 0
	}
	return bars, totalSpanMS
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/waterfall/ -run TestBuildBars -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/waterfall/layout.go internal/tui/waterfall/layout_test.go
git commit -m "feat(waterfall): build bars with relative offsets and total span

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 4: Greedy lane packing + bar scaling

**Files:**
- Modify: `internal/tui/waterfall/layout.go`
- Test: `internal/tui/waterfall/layout_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/waterfall/layout_test.go`:

```go
func barAt(offset, dur int64) Bar {
	return Bar{OffsetMS: offset, Req: readstore.WaterfallRequest{DurationMS: dur}}
}

func TestPackLane(t *testing.T) {
	t.Parallel()
	t.Run("no overlap stays one row", func(t *testing.T) {
		rows := packLane([]Bar{barAt(0, 100), barAt(100, 100), barAt(200, 100)})
		if len(rows) != 1 || len(rows[0]) != 3 {
			t.Fatalf("want 1 row of 3, got %d rows", len(rows))
		}
	})
	t.Run("full overlap splits into rows", func(t *testing.T) {
		rows := packLane([]Bar{barAt(0, 1000), barAt(0, 1000), barAt(0, 1000)})
		if len(rows) != 3 {
			t.Fatalf("want 3 rows, got %d", len(rows))
		}
	})
	t.Run("partial overlap packs greedily", func(t *testing.T) {
		// A:[0,500) B:[200,700) C:[600,900)
		// A and C don't overlap -> same row; B -> second row.
		rows := packLane([]Bar{barAt(0, 500), barAt(200, 500), barAt(600, 300)})
		if len(rows) != 2 {
			t.Fatalf("want 2 rows, got %d", len(rows))
		}
		if len(rows[0]) != 2 {
			t.Fatalf("row 0 should hold A and C, got %d bars", len(rows[0]))
		}
	})
	t.Run("empty input", func(t *testing.T) {
		if rows := packLane(nil); rows != nil {
			t.Fatalf("want nil, got %v", rows)
		}
	})
}

func TestScaleBar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                                       string
		offsetMS, durationMS, totalSpanMS          int64
		contentWidth                               int
		wantStart, wantWidth                       int
	}{
		{"half offset full width", 5000, 5000, 10000, 100, 50, 50},
		{"min width clamp", 0, 1, 10000, 100, 0, 1},
		{"zero span single col", 0, 0, 0, 100, 0, 1},
		{"end of timeline", 9000, 1000, 10000, 100, 90, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotStart, gotWidth := scaleBar(c.offsetMS, c.durationMS, c.totalSpanMS, c.contentWidth)
			if gotStart != c.wantStart || gotWidth != c.wantWidth {
				t.Errorf("scaleBar(%d,%d,%d,%d) = (%d,%d), want (%d,%d)",
					c.offsetMS, c.durationMS, c.totalSpanMS, c.contentWidth,
					gotStart, gotWidth, c.wantStart, c.wantWidth)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/waterfall/ -run 'TestPackLane|TestScaleBar' -v`
Expected: FAIL — `undefined: packLane` / `undefined: scaleBar`.

- [ ] **Step 3: Write the implementation**

Append to `internal/tui/waterfall/layout.go`:

```go
// endMS returns the bar's end offset (start offset + duration).
func (b Bar) endMS() int64 { return b.OffsetMS + b.Req.DurationMS }

// packLane greedily packs bars into non-overlapping sub-rows. Bars are sorted
// by start offset; each bar is placed in the first sub-row whose last bar ends
// at or before this bar's start, otherwise a new sub-row is opened.
func packLane(bars []Bar) [][]Bar {
	if len(bars) == 0 {
		return nil
	}
	sorted := make([]Bar, len(bars))
	copy(sorted, bars)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].OffsetMS < sorted[j].OffsetMS
	})

	var rows [][]Bar
	rowEnd := []int64{} // last-bar end offset per row
	for _, b := range sorted {
		placed := false
		for i := range rows {
			if rowEnd[i] <= b.OffsetMS {
				rows[i] = append(rows[i], b)
				rowEnd[i] = b.endMS()
				placed = true
				break
			}
		}
		if !placed {
			rows = append(rows, []Bar{b})
			rowEnd = append(rowEnd, b.endMS())
		}
	}
	return rows
}

// scaleBar maps a bar's millisecond offset/duration onto terminal columns.
// Width is clamped to a minimum of 1 column. When totalSpanMS is 0 the bar
// renders as a single column at the start.
func scaleBar(offsetMS, durationMS, totalSpanMS int64, contentWidth int) (startCol, width int) {
	if totalSpanMS <= 0 || contentWidth <= 0 {
		return 0, 1
	}
	scale := float64(contentWidth) / float64(totalSpanMS)
	startCol = int(float64(offsetMS) * scale)
	width = int(float64(durationMS) * scale)
	if width < 1 {
		width = 1
	}
	if startCol < 0 {
		startCol = 0
	}
	if startCol+width > contentWidth {
		width = contentWidth - startCol
		if width < 1 {
			width = 1
		}
	}
	return startCol, width
}
```

Add `"sort"` to the `import` block at the top of `layout.go` (keep one merged import block: `"sort"`, `"time"`, and the `readstore` import).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/waterfall/ -run 'TestPackLane|TestScaleBar' -v`
Expected: PASS.

- [ ] **Step 5: Run full package verification**

Run: `make vet && go test ./internal/tui/waterfall/`
Expected: clean vet, all `layout_test.go` tests pass.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/waterfall/layout.go internal/tui/waterfall/layout_test.go
git commit -m "feat(waterfall): add greedy lane packing and bar scaling

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 5: Waterfall `Model` — state, Init, Update, fetch

**Files:**
- Create: `internal/tui/waterfall/model.go`
- Test: `internal/tui/waterfall/model_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/waterfall/model_test.go`:

```go
package waterfall

import (
	"errors"
	"testing"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
)

var _ app.View = (*Model)(nil)

func TestModel_Title(t *testing.T) {
	t.Parallel()
	m := New(nil, "abcdef123456", nil).(*Model)
	if m.Title() != "WATERFALL abcdef12…" {
		t.Fatalf("title = %q", m.Title())
	}
}

func TestModel_NotFoundSetsFlag(t *testing.T) {
	t.Parallel()
	m := New(nil, "p", nil).(*Model)
	upd, _ := m.Update(app.ErrMsg{Err: readstore.ErrNotFound})
	if !upd.(*Model).notFound {
		t.Fatal("expected notFound=true on ErrNotFound")
	}
}

func TestModel_GenericErrorSetsStale(t *testing.T) {
	t.Parallel()
	m := New(nil, "p", nil).(*Model)
	upd, _ := m.Update(app.ErrMsg{Err: errors.New("boom")})
	if !upd.(*Model).stale {
		t.Fatal("expected stale=true on generic error")
	}
}

func TestModel_DataPopulatesBars(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	m := New(nil, "p", nil).(*Model)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(time.Second), DurationMS: 1000, QuerySource: "main"},
		{TS: base.Add(3 * time.Second), DurationMS: 2000, QuerySource: "subagent"},
	}
	upd, _ := m.Update(waterfallDataMsg{reqs: reqs, at: time.Now()})
	got := upd.(*Model)
	if len(got.bars) != 2 {
		t.Fatalf("want 2 bars, got %d", len(got.bars))
	}
	if got.inFlight {
		t.Fatal("inFlight should be cleared after data")
	}
}

func TestModel_CursorMovesWithinBounds(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	m := New(nil, "p", nil).(*Model)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(time.Second), DurationMS: 1000, QuerySource: "main"},
		{TS: base.Add(3 * time.Second), DurationMS: 1000, QuerySource: "subagent"},
	}
	upd, _ := m.Update(waterfallDataMsg{reqs: reqs, at: time.Now()})
	m = upd.(*Model)

	// Down moves to 1, second Down clamps at 1.
	upd, _ = m.Update(keyMsg("down"))
	m = upd.(*Model)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.cursor)
	}
	upd, _ = m.Update(keyMsg("down"))
	if upd.(*Model).cursor != 1 {
		t.Fatalf("cursor should clamp at 1")
	}
	// Up moves back to 0, second Up clamps at 0.
	upd, _ = m.Update(keyMsg("up"))
	m = upd.(*Model)
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.cursor)
	}
	upd, _ = m.Update(keyMsg("up"))
	if upd.(*Model).cursor != 0 {
		t.Fatalf("cursor should clamp at 0")
	}
}

func TestModel_TickAfterDataRefetches(t *testing.T) {
	t.Parallel()
	m := New(nil, "p", nil).(*Model)
	m.inFlight = false
	_, cmd := m.Update(app.TickMsg{})
	if cmd == nil {
		t.Fatal("expected fetch cmd from tick")
	}
}
```

Add a `keyMsg` test helper at the bottom of `model_test.go`:

```go
func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}
```

Add the `tea "github.com/charmbracelet/bubbletea"` import to `model_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/waterfall/ -run TestModel -v`
Expected: FAIL — `undefined: Model` / `undefined: New` / `undefined: waterfallDataMsg`.

- [ ] **Step 3: Write the implementation**

Create `internal/tui/waterfall/model.go`:

```go
package waterfall

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

const fetchTimeout = 500 * time.Millisecond

var errNoPool = errors.New("waterfall: no read pool")

// waterfallDataMsg carries a completed PromptWaterfall fetch.
type waterfallDataMsg struct {
	reqs []readstore.WaterfallRequest
	at   time.Time
}

// Model is the waterfall view: a per-prompt timeline of api_request /
// api_error events banded into main/subagent/auxiliary lanes.
type Model struct {
	pool        *sql.DB
	theme       *theme.Theme
	promptID    string
	reqs        []readstore.WaterfallRequest
	bars        []Bar // ordered by ts (fetch order); cursor indexes this
	totalSpanMS int64
	cursor      int
	notFound    bool
	inFlight    bool
	stale       bool
	lastOK      time.Time
}

// New constructs a waterfall Model bound to a promptID.
func New(pool *sql.DB, promptID string, th *theme.Theme) app.View {
	return &Model{pool: pool, theme: th, promptID: promptID}
}

func (m *Model) th() *theme.Theme {
	if m.theme != nil {
		return m.theme
	}
	t := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	return &t
}

func (m *Model) Init() tea.Cmd {
	m.inFlight = true
	return m.fetchCmd()
}

func (m *Model) Title() string {
	return "WATERFALL " + shortID(m.promptID)
}

func (m *Model) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "select")),
		key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "about")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}

func (m *Model) Status() component.Status {
	if m.notFound || (m.lastOK.IsZero() && len(m.bars) == 0) {
		return component.StatusNoDaemon
	}
	if m.stale {
		return component.StatusStale
	}
	return component.StatusLive
}

func (m *Model) Update(msg tea.Msg) (app.View, tea.Cmd) {
	switch v := msg.(type) {
	case app.TickMsg:
		if m.inFlight {
			return m, nil
		}
		m.inFlight = true
		return m, m.fetchCmd()
	case waterfallDataMsg:
		m.reqs = v.reqs
		m.bars, m.totalSpanMS = buildBars(v.reqs)
		if m.cursor > len(m.bars)-1 {
			m.cursor = max0(len(m.bars) - 1)
		}
		m.notFound = false
		m.stale = false
		m.lastOK = v.at
		m.inFlight = false
		return m, nil
	case app.ErrMsg:
		m.inFlight = false
		if errors.Is(v.Err, readstore.ErrNotFound) {
			m.notFound = true
			return m, nil
		}
		m.stale = true
		return m, nil
	case tea.KeyMsg:
		switch v.String() {
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.bars)-1 {
				m.cursor++
			}
		}
		return m, nil
	}
	return m, nil
}

func (m *Model) fetchCmd() tea.Cmd {
	pool := m.pool
	pid := m.promptID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), fetchTimeout)
		defer cancel()
		if pool == nil {
			return app.ErrMsg{Err: errNoPool}
		}
		reqs, err := readstore.PromptWaterfall(ctx, pool, pid)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return waterfallDataMsg{reqs: reqs, at: time.Now()}
	}
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}
```

Note: `PromptWaterfall` never returns `ErrNotFound` today (an unknown prompt yields an empty slice). The `notFound` branch is kept for symmetry with `prompt.Detail` and in case the query later distinguishes a missing prompt; the empty-slice case is handled by `View()` in Task 6.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/waterfall/ -run TestModel -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/waterfall/model.go internal/tui/waterfall/model_test.go
git commit -m "feat(waterfall): add Model with Init/Update/fetch and cursor

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 6: Waterfall `View()` rendering + golden tests

**Files:**
- Create: `internal/tui/waterfall/view.go`
- Test: `internal/tui/waterfall/view_test.go`
- Create: `internal/tui/waterfall/testdata/*.golden` (generated via update flag)

- [ ] **Step 1: Write the failing test**

Create `internal/tui/waterfall/view_test.go`:

```go
package waterfall

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func TestMain(m *testing.M) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

var updateWaterfall = flag.Bool("update-waterfall", false, "update waterfall goldens")

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	got = stripANSI(got)
	if *updateWaterfall {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != string(want) {
		t.Fatalf("golden %s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

func newModelWith(reqs []readstore.WaterfallRequest) *Model {
	th := theme.Build(theme.MochaPalette(), theme.UnicodeGlyphs())
	m := New(nil, "7b2e4d10-0000-0000-0000-000000000000", &th).(*Model)
	m.bars, m.totalSpanMS = buildBars(reqs)
	m.reqs = reqs
	m.lastOK = time.Now()
	return m
}

func TestWaterfallView_Golden_Empty(t *testing.T) {
	m := newModelWith(nil)
	golden(t, "empty", m.View(90, 32))
}

func TestWaterfallView_Golden_MainOnly(t *testing.T) {
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(1 * time.Second), DurationMS: 900, QuerySource: "repl_main_thread", Model: "claude-opus-4-7", CostUSD: 0.21, InputTokens: 8000, OutputTokens: 2000},
		{TS: base.Add(4 * time.Second), DurationMS: 1500, QuerySource: "main", Model: "claude-opus-4-7", CostUSD: 0.18, InputTokens: 6000, OutputTokens: 1800},
	}
	golden(t, "main_only", newModelWith(reqs).View(90, 32))
}

func TestWaterfallView_Golden_AllLanesOverlap(t *testing.T) {
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(1 * time.Second), DurationMS: 1000, QuerySource: "repl_main_thread", Model: "claude-opus-4-7", CostUSD: 0.21, InputTokens: 8000, OutputTokens: 2000},
		{TS: base.Add(5 * time.Second), DurationMS: 3000, QuerySource: "general-purpose", Model: "claude-sonnet-4-6", CostUSD: 0.04, InputTokens: 1200, OutputTokens: 800},
		{TS: base.Add(6 * time.Second), DurationMS: 3500, QuerySource: "Explore", Model: "claude-sonnet-4-6", CostUSD: 0.05, InputTokens: 1400, OutputTokens: 900},
		{TS: base.Add(7 * time.Second), DurationMS: 400, QuerySource: "compact", Model: "claude-haiku-4-5", CostUSD: 0.002, InputTokens: 200, OutputTokens: 50},
		{TS: base.Add(12 * time.Second), DurationMS: 2000, QuerySource: "main", Model: "claude-opus-4-7", CostUSD: 0.19, InputTokens: 7000, OutputTokens: 2100},
	}
	golden(t, "all_lanes_overlap", newModelWith(reqs).View(90, 32))
}

func TestWaterfallView_Golden_Narrow(t *testing.T) {
	base := time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC)
	reqs := []readstore.WaterfallRequest{
		{TS: base.Add(1 * time.Second), DurationMS: 1000, QuerySource: "main", Model: "claude-opus-4-7", CostUSD: 0.21, InputTokens: 8000, OutputTokens: 2000},
		{TS: base.Add(3 * time.Second), DurationMS: 2000, QuerySource: "subagent", Model: "claude-sonnet-4-6", CostUSD: 0.04, InputTokens: 1200, OutputTokens: 800},
	}
	golden(t, "narrow", newModelWith(reqs).View(50, 24))
}

func TestWaterfallView_NotFound(t *testing.T) {
	m := newModelWith(nil)
	m.notFound = true
	golden(t, "not_found", m.View(90, 32))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/waterfall/ -run TestWaterfallView -v`
Expected: FAIL — `m.View undefined` (compile error).

- [ ] **Step 3: Write the implementation**

Create `internal/tui/waterfall/view.go`:

```go
package waterfall

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme/component"
)

const (
	laneLabelWidth = 11 // "auxiliary  " padded
	barGlyph       = "█"
)

// View renders the waterfall body. The shell renders chrome separately, but we
// mirror prompt.Detail and render an inner header + help bar for consistency.
func (m *Model) View(width, height int) string {
	th := m.th()
	if width <= 0 {
		width = 90
	}

	brand := th.Title.Render(th.Glyphs.Brand + " cco")
	bread := th.Muted.Render(" · waterfall " + shortID(m.promptID))
	pill := component.StatusPill(th, m.Status())
	headerRight := lipgloss.NewStyle().
		Width(width - lipgloss.Width(brand) - lipgloss.Width(bread)).
		Align(lipgloss.Right).Render(pill)
	header := lipgloss.JoinHorizontal(lipgloss.Top, brand, bread, headerRight)

	help := component.HelpBar(th, []component.KeyHint{
		{Key: "↑↓", Desc: "select"},
		{Key: "b", Desc: "back"},
		{Key: "r", Desc: "refresh"},
		{Key: "?", Desc: "about"},
		{Key: "q", Desc: "quit"},
	}, width)

	if m.notFound {
		body := th.Muted.Render("prompt not found — it may have been pruned")
		return strings.Join([]string{header, "", component.Card(th, "", body, width), "", help}, "\n")
	}
	if len(m.bars) == 0 {
		body := th.Muted.Render("no api requests for this prompt")
		return strings.Join([]string{header, "", component.Card(th, "", body, width), "", help}, "\n")
	}

	contentWidth := width - laneLabelWidth
	if contentWidth < 10 {
		contentWidth = 10
	}

	axis := m.renderAxis(th, contentWidth)
	lanes := []string{
		m.renderLane(th, LaneMain, contentWidth),
		m.renderLane(th, LaneSubagent, contentWidth),
		m.renderLane(th, LaneAuxiliary, contentWidth),
	}
	detail := m.renderDetail(th, width)

	parts := []string{header, "", axis, ""}
	parts = append(parts, lanes...)
	parts = append(parts, "", detail, "", help)
	return strings.Join(parts, "\n")
}

// renderAxis draws the relative time-axis header: "0ms ...... <span>ms".
func (m *Model) renderAxis(th *theme.Theme, contentWidth int) string {
	left := "0ms"
	right := fmt.Sprintf("%dms", m.totalSpanMS)
	gap := contentWidth - len(left) - len(right)
	if gap < 1 {
		gap = 1
	}
	line := left + strings.Repeat("─", gap) + right
	return strings.Repeat(" ", laneLabelWidth) + th.Muted.Render(line)
}

// renderLane renders one lane: a padded label plus one or more packed sub-rows
// of bars. An empty lane renders a single "(none)" row.
func (m *Model) renderLane(th *theme.Theme, lane LaneKind, contentWidth int) string {
	var laneBars []Bar
	for _, b := range m.bars {
		if b.Lane == lane {
			laneBars = append(laneBars, b)
		}
	}
	label := padRight(lane.String(), laneLabelWidth)

	if len(laneBars) == 0 {
		return th.Label.Render(label) + th.Muted.Render("(none)")
	}

	rows := packLane(laneBars)
	var out []string
	for i, row := range rows {
		prefix := strings.Repeat(" ", laneLabelWidth)
		if i == 0 {
			prefix = th.Label.Render(label)
		}
		out = append(out, prefix+m.renderBarRow(th, row, contentWidth))
	}
	return strings.Join(out, "\n")
}

// renderBarRow paints one sub-row of non-overlapping bars onto a rune buffer.
// The bar under the cursor is rendered with the accent style; error bars use red.
func (m *Model) renderBarRow(th *theme.Theme, row []Bar, contentWidth int) string {
	cells := make([]string, contentWidth)
	for i := range cells {
		cells[i] = " "
	}
	for _, b := range row {
		startCol, w := scaleBar(b.OffsetMS, b.Req.DurationMS, m.totalSpanMS, contentWidth)
		style := th.Muted
		if b.Req.IsError {
			style = lipgloss.NewStyle().Foreground(th.Palette.Red)
		} else if m.isSelected(b) {
			style = th.Accent
		}
		for c := startCol; c < startCol+w && c < contentWidth; c++ {
			cells[c] = style.Render(barGlyph)
		}
	}
	return strings.Join(cells, "")
}

// renderDetail renders the "selected" panel for the bar under the cursor.
func (m *Model) renderDetail(th *theme.Theme, width int) string {
	if m.cursor < 0 || m.cursor >= len(m.bars) {
		return component.Card(th, "selected", th.Muted.Render("(no selection)"), width)
	}
	b := m.bars[m.cursor]
	r := b.Req
	qs := r.QuerySource
	if qs == "" {
		qs = "(unset)"
	}
	status := "ok"
	if r.IsError {
		status = "error"
	}
	lines := []string{
		labelValue(th, "model", orDash(r.Model), width-6),
		labelValue(th, "query_source", qs, width-6),
		labelValue(th, "duration", fmt.Sprintf("%d ms", r.DurationMS), width-6),
		labelValue(th, "cost", fmt.Sprintf("$%.4f", r.CostUSD), width-6),
		labelValue(th, "tokens", fmt.Sprintf("in %d / out %d", r.InputTokens, r.OutputTokens), width-6),
		labelValue(th, "started", startOf(r).Format("15:04:05")+" · "+status, width-6),
	}
	return component.Card(th, "selected", strings.Join(lines, "\n"), width)
}

func (m *Model) isSelected(b Bar) bool {
	if m.cursor < 0 || m.cursor >= len(m.bars) {
		return false
	}
	sel := m.bars[m.cursor]
	return sel.OffsetMS == b.OffsetMS &&
		sel.Lane == b.Lane &&
		sel.Req.TS.Equal(b.Req.TS) &&
		sel.Req.DurationMS == b.Req.DurationMS
}

// labelValue renders a "label   value" line padded to width.
func labelValue(th *theme.Theme, label, value string, width int) string {
	lbl := th.Label.Render(label)
	gap := width - lipgloss.Width(lbl) - lipgloss.Width(value)
	if gap < 1 {
		gap = 1
	}
	return lbl + strings.Repeat(" ", gap) + value
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
```

Before running, confirm these referenced symbols exist (they are used identically in `internal/tui/prompt/detail.go`): `th.Title`, `th.Muted`, `th.Accent`, `th.Label`, `th.Glyphs.Brand`, `th.Palette.Red`, `component.StatusPill`, `component.Card`, `component.HelpBar`, `component.KeyHint`. If any name differs, match the name used in `prompt/detail.go`.

- [ ] **Step 4: Generate the golden files**

Run: `go test ./internal/tui/waterfall/ -run TestWaterfallView -update-waterfall`
Expected: PASS (goldens written to `internal/tui/waterfall/testdata/`).

- [ ] **Step 5: Inspect the goldens**

Run: `cat internal/tui/waterfall/testdata/all_lanes_overlap.golden`
Expected: a header line, an axis line, three lanes (`main`, `subagent` with two packed sub-rows, `auxiliary`), a `selected` card, and a help bar. Visually confirm the subagent lane shows two sub-rows (the `general-purpose` and `Explore` bars overlap in time). If the layout looks wrong, fix `view.go`/`layout.go` and regenerate.

- [ ] **Step 6: Run test to verify it passes (without update flag)**

Run: `go test ./internal/tui/waterfall/ -run TestWaterfallView -v`
Expected: PASS — goldens match.

- [ ] **Step 7: Run full package verification**

Run: `make vet && go test ./internal/tui/waterfall/`
Expected: clean vet, all waterfall tests pass.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/waterfall/view.go internal/tui/waterfall/view_test.go internal/tui/waterfall/testdata/
git commit -m "feat(waterfall): render lanes, bars, axis and detail panel

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 7: Wire `w` key in Prompt Detail to push the waterfall view

**Files:**
- Modify: `internal/tui/prompt/detail.go`
- Test: `internal/tui/prompt/detail_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/prompt/detail_test.go`:

```go
func TestDetail_WKeyPushesWaterfall(t *testing.T) {
	t.Parallel()
	d := New(nil, "abcdef123456", nil).(*Detail)
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if cmd == nil {
		t.Fatal("expected a command from 'w' key")
	}
	msg := cmd()
	pv, ok := msg.(app.PushViewMsg)
	if !ok {
		t.Fatalf("expected app.PushViewMsg, got %T", msg)
	}
	if _, ok := pv.V.(app.View); !ok {
		t.Fatal("pushed value is not an app.View")
	}
}
```

Add `tea "github.com/charmbracelet/bubbletea"` to the `detail_test.go` import block.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/prompt/ -run TestDetail_WKeyPushesWaterfall -v`
Expected: FAIL — `cmd == nil` (the `w` key is not handled yet).

- [ ] **Step 3: Write the implementation**

In `internal/tui/prompt/detail.go`:

a) Add the import for the waterfall package. To avoid an import cycle (waterfall imports `app`, `prompt` imports `app` — no cycle with `waterfall` itself, this is safe), add to the import block:

```go
	"github.com/kamikaze011001/claude-code-observer/internal/tui/waterfall"
```

b) Add a package-level indirection variable near the bottom of the file (mirrors `sessions/detail.go`'s `newPromptDetail = prompt.New` pattern, and keeps the test seam):

```go
var newWaterfall = waterfall.New
```

c) In `Update`, add a `tea.KeyMsg` case. The current `Update` has no `tea.KeyMsg` case, so add one to the `switch v := msg.(type)`:

```go
	case tea.KeyMsg:
		if v.String() == "w" {
			pool := d.pool
			pid := d.promptID
			th := d.theme
			return d, func() tea.Msg {
				return app.PushViewMsg{V: newWaterfall(pool, pid, th)}
			}
		}
		return d, nil
```

d) Add `w` to `ShortHelp()`:

```go
func (d *Detail) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "waterfall")),
		key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "back")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "about")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}
```

e) Add `w` to the in-`View()` help bar (the `component.HelpBar` call near the end of `View`):

```go
	help := component.HelpBar(th, []component.KeyHint{
		{Key: "w", Desc: "waterfall"},
		{Key: "b", Desc: "back"},
		{Key: "r", Desc: "refresh"},
		{Key: "?", Desc: "about"},
		{Key: "q", Desc: "quit"},
	}, width)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/prompt/ -run TestDetail_WKeyPushesWaterfall -v`
Expected: PASS.

- [ ] **Step 5: Regenerate prompt goldens (the help bar changed)**

The `View()` help bar now includes `[w] waterfall`, so the existing prompt golden files need updating.

Run: `go test ./internal/tui/prompt/ -update-prompt`
Then: `git diff internal/tui/prompt/testdata/` — confirm the only change is the added `[w] waterfall` hint in the help bar. If anything else changed, investigate before continuing.

- [ ] **Step 6: Run full verification**

Run: `make vet && go test ./... && make build`
Expected: clean vet, all tests pass across every package, binary builds.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/prompt/detail.go internal/tui/prompt/detail_test.go internal/tui/prompt/testdata/
git commit -m "feat(prompt): bind 'w' to push the subagent waterfall view

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 8: Correct `query_source` description in the OTel doc

**Files:**
- Modify: `docs/CLAUDE-CODE-OTEL.md`

- [ ] **Step 1: Update §8.2 (`api_request`)**

In `docs/CLAUDE-CODE-OTEL.md`, find the `query_source` row in the §8.2 `api_request` attribute table (currently around line 344):

Replace:
```
| `query_source` | string | Subsystem that issued the request: `main` (primary REPL loop), `subagent` (spawned subagent), or `auxiliary` (background tasks such as compaction). |
```
With:
```
| `query_source` | string | Free-form identifier of the subsystem that issued the request. Observed values on log records include `repl_main_thread` (primary REPL loop), `compact` (background compaction), and subagent names (e.g. `general-purpose`, `Explore`). **Note:** the categorical `main` / `subagent` / `auxiliary` form documented elsewhere is the *metrics* cardinality bucketing — log records carry the richer free-form value. Treat this field as a free-form string. |
```

- [ ] **Step 2: Update §8.3 (`api_error`)**

In the §8.3 `api_error` attribute table (currently around line 362):

Replace:
```
| `query_source` | string | Same as `api_request`: `main`, `subagent`, or `auxiliary`. |
```
With:
```
| `query_source` | string | Same as `api_request` — a free-form subsystem identifier (`repl_main_thread`, `compact`, or a subagent name). See §8.2. |
```

- [ ] **Step 3: Update §8.8 cross-reference (optional consistency check)**

In §8.8, the `subagent_dispatch` row already says "subagent activity is inferred from `query_source` on `api_request`." Confirm it is still accurate (it is) — no change needed unless the wording conflicts with the above.

- [ ] **Step 4: Verify the doc renders cleanly**

Run: `grep -n "query_source" docs/CLAUDE-CODE-OTEL.md`
Expected: the §8.2 and §8.3 rows now show the free-form description; no other occurrences changed unexpectedly.

- [ ] **Step 5: Commit**

```bash
git add docs/CLAUDE-CODE-OTEL.md
git commit -m "docs(otel): correct query_source as free-form on log records

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Final verification

- [ ] **Run the full gate**

Run: `make vet && make test && make build`
Expected: `go vet` clean, `go test ./...` green, binary builds.

- [ ] **Manual smoke test**

Run: `make run`, navigate to a Session with a prompt that spawned subagents, open Prompt Detail, press `w`. Confirm: the waterfall page appears, three lanes render, `↑↓` moves the selection and updates the detail panel, `b` returns to Prompt Detail. If there is no real subagent data locally, at minimum confirm a prompt with only `main` requests renders correctly and the `subagent`/`auxiliary` lanes show `(none)`.

- [ ] **Update FUTURE.md**

Remove the "Subagent waterfall view" entry from `docs/FUTURE.md` §Mid-term (it is now shipped). Commit:

```bash
git add docs/FUTURE.md
git commit -m "docs: drop shipped subagent waterfall view from FUTURE.md

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Self-Review Notes

- **Spec coverage:** readstore query (Task 1); lane bucketing (Task 2); offsets/span (Task 3); packing + scaling (Task 4); Model/Init/Update/cursor (Task 5); View with 3 lanes, axis, detail panel, edge cases empty/not-found/narrow (Task 6); `w` entry point from Prompt Detail (Task 7); OTel doc correction (Task 8). Edge cases from the spec table — missing `query_source` → main lane (Task 2 test), zero `duration_ms` → 1-col bar (Tasks 3 & 4 tests), zero span (Task 4 test), narrow width (Task 6 golden), no requests / not found (Task 6 goldens).
- **Out of scope (correctly absent):** per-instance subagent separation, `tool_result` rendering, session-level waterfall, absolute-time axis.
- **Type consistency:** `WaterfallRequest` fields are referenced identically across Tasks 1/3/5/6. `Bar{Req, Lane, OffsetMS}`, `LaneKind` constants, and the `bucketLane`/`buildBars`/`packLane`/`scaleBar`/`startOf` signatures are stable from definition through use. `waterfallDataMsg{reqs, at}` matches between Task 5 definition and Task 5 tests.
