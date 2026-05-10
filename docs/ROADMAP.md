# Roadmap — claude-code-observer v1

> Last updated: 2026-05-10
> Source PRD: [PRDs/0001-claude-code-observer-v1.md](PRDs/0001-claude-code-observer-v1.md)

Five phases, 13 milestones. Each milestone has a **demo** (manual end-to-end check that proves the capability works against a real or realistic input) and a **test gate** (named test suite + coverage threshold on the relevant package).

A milestone is "done" only when **both** the demo passes and the test gate is green.

The shipping point is the end of **Phase 3**. Phase 4 is install ergonomics — nice-to-have, ship-without-it-if-needed.

## Phase overview

```
Phase 0  Foundation             M0.1 → M0.2                 scaffolding, schema
Phase 1  Ingest path            M1.1 → M1.2 → M1.3          OTLP → events table
Phase 2  Rollups + retention    M2.1 → M2.2 → M2.3          sessions/prompts, pruner
Phase 3  TUI                    M3.1 → M3.2 → M3.3          Dashboard → Prompt detail
Phase 4  Install ergonomics     M4.1 → M4.2                 init wizard, launchd/systemd
```

Phases are strictly sequential. Within a phase, milestones are ordered by dependency; trying to parallelize them generally won't pay off.

## Cross-phase quality gates

Every milestone, before it counts as passed:

- `go vet ./...` clean
- `golangci-lint run` clean
- `go test ./...` green
- `go build -o bin/cco ./cmd/app` succeeds
- No new `TODO` / `FIXME` comments without a tracked follow-up
- New code paths have tests added in the same change

These are non-negotiable; they are not repeated in each milestone below.

---

## Phase 0 — Foundation

**Goal:** Empty-but-runnable binary, persistent SQLite with the full schema migrated in.

### M0.1 — Repo scaffolding & subcommand skeleton

Set up `cmd/app/` entry point with the four subcommands stubbed (`serve`, default→TUI, `init`, `rebuild-rollups`), plus the package layout from `docs/ARCHITECTURE.md` (`internal/{receiver,service,repository,domain,tui,eventparser,rollup,retention}`).

**Demo (passes when):**
- `go build -o bin/cco ./cmd/app` produces a binary
- `./bin/cco --help` lists `serve`, `init`, `rebuild-rollups`, and "(no args) → TUI"
- `./bin/cco serve` starts and exits cleanly on Ctrl-C (no DB or receiver yet — just a goroutine that blocks on signal)

**Test gate:**
- `go test ./...` green (placeholder tests in each package are fine)
- All packages compile with `internal/domain` types defined: `Event`, `Session`, `Prompt`, `ToolResult`, `MetricSnapshot`

### M0.2 — SQLite repository + migrations

Implement `internal/repository/` with the schema from `docs/DATA-MODELS.md`. Migrations live in `internal/repository/migrations/0001_initial.sql` (events, sessions, prompts, metric_snapshots, schema_version + all indexes). WAL mode enabled. Single write connection (daemon), separate read pool (TUI will use later).

**Demo (passes when):**
- `./bin/cco serve` creates `~/.claude-code-observer/db.sqlite`
- `sqlite3 ~/.claude-code-observer/db.sqlite ".schema"` shows all 5 tables with correct columns and indexes
- `sqlite3 ... "SELECT version FROM schema_version"` returns `1`
- Stop and restart `cco serve` — no errors, schema_version still 1 (idempotent)

**Test gate:**
- Integration tests against a temp SQLite file:
  - migration applies on empty DB
  - migration is idempotent on re-apply
  - all expected indexes exist (`PRAGMA index_list`)
  - WAL mode is active (`PRAGMA journal_mode`)
- Coverage ≥ 80% on `internal/repository/`

---

## Phase 1 — Ingest path

**Goal:** Real Claude Code can send OTLP to `:4317` and rows appear in `events`. No rollups yet.

### M1.1 — gRPC OTLP receiver wiring

Implement `internal/receiver/` with `LogsServiceServer` and `MetricsServiceServer` (per [ADR-001](decisions/ADR-001-thin-otlp-receiver.md)). Bind `127.0.0.1:4317`. Both services accept requests and return success, but for this milestone they only **log** the request shape — no DB writes yet.

**Demo (passes when):**
- `./bin/cco serve` listens on `:4317`
- `grpcurl -plaintext localhost:4317 list` lists `opentelemetry.proto.collector.logs.v1.LogsService` and `…metrics.v1.MetricsService`
- A hand-crafted `ExportLogsServiceRequest` (via `grpcurl` or a tiny Go client) returns success and is logged

**Test gate:**
- Unit tests against the gRPC server using `bufconn` (in-memory transport):
  - empty request → success
  - well-formed request → success, request count incremented
  - malformed request → no panic, returns gRPC error
- Coverage ≥ 70% on `internal/receiver/`

### M1.2 — Event parser (deep module)

Implement `internal/eventparser/` — pure function from OTel `LogRecord` → `domain.Event`. Handles all 5 documented Claude Code events (`user_prompt`, `api_request`, `api_error`, `tool_decision`, `tool_result`) plus the 3 community-observed ones from `docs/CLAUDE-CODE-OTEL.md`. Selects which resource attributes to flatten into `event.attrs`. Drops events missing `session.id` with a warn.

This is the first deep module; takes the most-changing surface (Anthropic's event schema) and isolates it behind a stable interface.

**Demo (passes when):**
- A small CLI harness (`go run ./cmd/parser-debug fixtures/user_prompt.json`) reads a JSON-encoded LogRecord and prints the resulting `domain.Event` struct with correct `session_id`, `prompt_id`, `event_name`, and parsed `attrs`
- Fixtures cover all 5 documented event types

**Test gate:**
- Table-driven tests in `internal/eventparser/parser_test.go`:
  - one row per documented event type (5 rows)
  - one row per community-observed event (3 rows)
  - malformed: missing `session.id` → returns drop sentinel
  - malformed: unknown event name → stored verbatim, attrs preserved
  - resource attrs: `project.name` and `app.version` flattened into event attrs
- Coverage ≥ 90% on `internal/eventparser/`

### M1.3 — End-to-end ingest into events table

Wire receiver → service → repository. Each accepted log record is parsed by `eventparser`, classified by Service, and inserted into `events` in a single transaction. `metric_snapshots` is written for incoming metric datapoints.

**Demo (passes when):**
- Configure a local Claude Code with `.claude/settings.json` per `docs/CLAUDE-CODE-OTEL.md`
- Run `claude` in any directory, type a prompt that triggers ≥1 tool call
- `sqlite3 db.sqlite "SELECT COUNT(*) FROM events"` returns > 0
- `sqlite3 db.sqlite "SELECT event_name, COUNT(*) FROM events GROUP BY event_name"` shows at minimum: `claude_code.user_prompt`, `claude_code.api_request`, `claude_code.tool_result`
- `metric_snapshots` has rows for `claude_code.cost.usage` and `claude_code.token.usage`

**Test gate:**
- Integration test that:
  - spins up the receiver in-process on a random port
  - sends a fake `ExportLogsServiceRequest` via gRPC client containing a synthetic `user_prompt` and `api_request` for the same `prompt.id`
  - asserts both rows landed in `events` with correct `session_id`, `prompt_id`, `event_name`, and `attrs` JSON containing the expected fields
  - asserts the same data round-trips through `metric_snapshots` for a synthetic metric request
- Coverage ≥ 80% across the integration boundary

---

## Phase 2 — Rollups + retention

**Goal:** `sessions` and `prompts` tables stay current as events arrive. Old events get pruned. `rebuild-rollups` works.

### M2.1 — Rollup engine (deep module)

Implement `internal/rollup/` — pure functions: given an existing rollup row + a new event, produce the updated rollup row. One updater per event type. Service calls these inside the same SQLite transaction as the event insert. Includes the `rebuild-rollups` command which truncates rollup tables and re-runs all updaters across `events`.

**Demo (passes when):**
- A test harness or a hand-fed sequence of synthetic events produces:
  - `sessions` row with correct `cost_usd`, `input_tokens`, `output_tokens`, `cache_read_tokens`, `cache_creation_tokens`, `api_requests`, `api_errors`, `subagent_requests`, `tool_calls`, `tool_denied`, `prompts`
  - `prompts` row with correct per-prompt totals and `had_error` flag
- `./bin/cco rebuild-rollups` on the M1.3 database produces the same totals as live ingestion did
- The total `SUM(cost_usd)` on `sessions` matches the sum of `cost_usd` JSON-extracted from `events` of name `api_request` (truth check)

**Test gate:**
- Table-driven tests in `internal/rollup/`:
  - `api_request` updater: tokens and cost accumulate; `subagent_requests` counter only ticks when `query_source = 'subagent'`
  - `api_error` updater: `api_errors++`, `had_error = 1` on the prompt
  - `tool_decision` updater: `tool_denied` only when `decision = 'deny'`
  - `tool_result` updater: `tool_calls++`
  - `user_prompt` updater: creates prompt row, sets `prompt_length`, `command_name`, `command_source`
- Coverage ≥ 90% on `internal/rollup/`

### M2.2 — Session idle-timeout sweeper

Implement the periodic goroutine in Service that runs every 60 s, finds `sessions` rows with `ended_at IS NULL AND last_seen_at < now - 30 min`, and sets `ended_at = last_seen_at`. Threshold configurable via `CCO_SESSION_IDLE_MIN`. Uses an injected clock so tests can advance time.

**Demo (passes when):**
- Insert two synthetic sessions: one with `last_seen_at = now`, one with `last_seen_at = now - 31 min`
- Trigger the sweeper manually (test entry point) → only the second has `ended_at` set
- `CCO_SESSION_IDLE_MIN=5 ./bin/cco serve` honors the override (verified by test)

**Test gate:**
- Table-driven tests with a fake clock:
  - default 30-min threshold
  - custom threshold via env
  - in-progress session not closed
  - already-closed session not re-touched
- Coverage ≥ 90% on the sweeper

### M2.3 — Retention pruner

Implement `internal/retention/` — daily goroutine that deletes `events` and `metric_snapshots` rows where `ts < now - 30 days`. Configurable via `CCO_RETENTION_DAYS`. Rollup tables are never touched. Uses the same injected clock.

**Demo (passes when):**
- Insert synthetic events with `ts` 31 days ago and `ts` today
- Trigger pruner → 31-day-old rows gone, today's rows retained
- `sessions` and `prompts` rows are unchanged (rollups are forever)
- `CCO_RETENTION_DAYS=7` actually retains only 7 days

**Test gate:**
- Integration tests on a temp SQLite:
  - default 30-day window
  - custom window via env
  - rollup tables untouched
  - pruner is idempotent (re-running deletes nothing more)
- Coverage ≥ 90% on `internal/retention/`

---

## Phase 3 — TUI

**Goal:** Open `cco`, see today's cost, drill from session → prompt → tool call. This is the user-facing milestone — ship-able point.

### M3.1 — Dashboard view

Bubble Tea model with read-only DB connection (separate from daemon's). Renders today / 7-day / 30-day totals (cost, prompts, tool calls, error count) plus top-3 most expensive sessions today. Polls every 1 s.

**Demo (passes when):**
- Run `cco serve` in one terminal, `cco` in another
- Use Claude Code in a third terminal — Dashboard updates within ~2 s
- Stop using Claude Code — Dashboard remains stable, no flicker
- Numbers match `sqlite3 ... "SELECT SUM(cost_usd) FROM sessions WHERE started_at >= unixepoch('now', 'start of day')*1e9"`

**Test gate:**
- Bubble Tea model unit tests: `Update` returns expected state on tick messages and key events
- Visual verification documented in `docs/MANUAL-VERIFICATION.md` — checklist the developer runs before tagging
- Coverage ≥ 60% on `internal/tui/dashboard/`

### M3.2 — Sessions list + Session detail

A page listing sessions newest-first with project, started_at, duration, cost, prompt count. Enter on a session opens Session Detail: the timeline of events for that session, paged. `q` returns to the list, `b` returns from detail to list.

**Demo (passes when):**
- From Dashboard, key shortcut opens Sessions list
- Arrow keys move selection; Enter drills in
- Session Detail shows the chronological event list with event_name, ts (humanized), and a one-line summary per event (e.g. `tool_result Bash 1.2s`)
- `b` returns to Sessions list with selection preserved

**Test gate:**
- Model unit tests for navigation transitions: list → detail → back
- Coverage ≥ 60% on `internal/tui/sessions/`

### M3.3 — Prompt detail

From Session Detail, Enter on a prompt boundary opens Prompt Detail: cost, token breakdown, list of tool calls with duration and tool name, list of api_requests with model + cost. This is where `json_extract` queries earn their keep.

**Demo (passes when):**
- Drill from Dashboard → Sessions → Session Detail → Prompt Detail
- Cost on Prompt Detail equals `SELECT cost_usd FROM prompts WHERE prompt_id = ?` (sanity check displayed in dev mode)
- Tool calls sorted by ts, durations rendered in ms
- `b` walks back up the chain

**Test gate:**
- Model unit tests for the drill-down chain
- Integration test: synthetic events for a known prompt → query layer returns the exact set of rows the view consumes
- Coverage ≥ 60% on `internal/tui/prompt/`

**End of Phase 3 = ship.** The tool is usable end-to-end.

---

## Phase 4 — Install ergonomics

**Goal:** Lower friction for first-time setup and unattended operation. Skip if you don't care about onboarding others.

### M4.1 — `cco init` setup wizard

`./bin/cco init` writes or updates `.claude/settings.json` in `$PWD` with the OTel env vars and `project.name = $(basename $PWD)`. Idempotent. If existing settings conflict, prompts the user (or `--force` to overwrite).

**Demo (passes when):**
- `cco init` in an empty dir creates `.claude/settings.json` with all required fields
- Re-running `cco init` is a no-op (no diff)
- Running in a dir with a different project.name prompts before overwriting
- `--print` flag prints the rendered settings without writing

**Test gate:**
- Table-driven tests on the init module:
  - fresh dir
  - existing complete settings (no-op)
  - existing partial settings (merge)
  - conflicting settings (prompts; `--force` overrides)
- Coverage ≥ 90% on `internal/init/`

### M4.2 — launchd plist + systemd unit + README install section

Ship `scripts/com.claude-code-observer.plist` (macOS) and `scripts/claude-code-observer.service` (Linux). Document install steps in README. The plist auto-starts on login; the systemd unit runs as user service.

**Demo (passes when):**
- On macOS: `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.claude-code-observer.plist` → daemon running, survives logout/login
- On Linux: `systemctl --user enable --now claude-code-observer` → daemon running, survives session restart
- README install section walks a new user from `git clone` to "open `cco` and see your prompts" in <5 minutes

**Test gate:**
- `shellcheck` clean on any install scripts
- Manual install verification recorded in `docs/MANUAL-VERIFICATION.md`
- A second machine (or a fresh user account) follows the README and reaches the dashboard with no Claude help

---

## What "done with v1" looks like

When all 13 milestones pass:

- Single `cco` binary (~5 MB) installs from `go build`
- `cco init` in any project enables observation
- `cco serve` runs unattended via launchd/systemd
- `cco` opens a TUI showing real-time cost and full session/prompt drill-down
- Raw events retained 30 days, rollups forever
- Test coverage: ≥ 90% on the three deep modules (`eventparser`, `rollup`, `retention`), ≥ 80% on `repository`, ≥ 60% on TUI models
- All 41 PRD user stories satisfied or explicitly deferred to FUTURE.md

After v1, the parking lot in [FUTURE.md](FUTURE.md) drives prioritization.
