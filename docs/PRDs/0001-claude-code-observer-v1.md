# PRD-0001: claude-code-observer v1

- **Status:** ready-for-agent
- **Date:** 2026-05-10
- **Author:** sonanh
- **Related:** [docs/CLAUDE-CODE-OTEL.md](../CLAUDE-CODE-OTEL.md), [docs/CONTEXT.md](../CONTEXT.md), [docs/ARCHITECTURE.md](../ARCHITECTURE.md), [docs/DATA-MODELS.md](../DATA-MODELS.md), [ADR-001](../decisions/ADR-001-thin-otlp-receiver.md), [ADR-002](../decisions/ADR-002-events-table-with-json-attrs.md), [ADR-003](../decisions/ADR-003-logs-as-primary-signal.md)

## Problem Statement

A solo developer using Claude Code daily across several projects has no visibility into where their API spend goes. Anthropic's billing dashboard is a single aggregate number per day; nothing tells the developer *which session*, *which prompt*, *which model*, or *which tool-heavy stretch* drove the cost. Past Sessions are forgotten — there's no way to look back at "how much did the auth refactor I did Tuesday actually cost, and how many subagents did Claude spawn?"

Claude Code already exposes this data via OpenTelemetry (it can be enabled with a few env vars), but consuming it requires standing up a full OTel Collector + storage backend + dashboarding stack. That's overkill for a personal local-only use case and the off-the-shelf dashboards (Grafana, etc.) don't model the prompt-as-a-unit-of-work concept that the developer actually wants to drill into.

## Solution

A single Go binary, `claude-code-observer`, with two subcommands:

- `serve` — a long-lived daemon that accepts Claude Code's OTLP/gRPC export on `127.0.0.1:4317`, parses every Event into typed records, and persists them to a local SQLite database alongside maintained Session and Prompt rollups.
- (no args) — opens a Bubble Tea TUI that reads the SQLite (read-only) and lets the user navigate Dashboard → Sessions list → Session detail → Prompt detail. Drill-down covers cost, token usage by type and model, tool calls, tool denials, Subagent Requests, API errors, and (when the user has enabled `OTEL_LOG_TOOL_DETAILS`) the actual Bash commands and subagent types.

Setup is one paste of an `env` block into a project's `.claude/settings.json`. The binary requires no remote services, no auth, no internet.

## User Stories

1. As a developer, I want to enable telemetry once per project and have all subsequent Claude Code Sessions in that project recorded automatically, so that I never forget to "start tracking" before a coding session.
2. As a developer, I want a single binary I can install with `go install` (or download a release), so that I don't have to run a Docker stack to observe a single CLI tool.
3. As a developer, I want the daemon to run automatically at login (via launchd on macOS, systemd on Linux), so that observation is passive — I don't have to remember to start anything.
4. As a developer, I want to open `claude-code-observer` and see today's total cost and the list of recent Sessions on the landing screen, so that I can answer "how much have I spent today" in under a second.
5. As a developer, I want to see today's total cost broken down by model, so that I can tell whether I'm over-relying on the most expensive model.
6. As a developer, I want to see today's total token usage broken into Input / Output / Cache Read / Cache Creation, so that I can understand where my context budget is going.
7. As a developer, I want to filter the Sessions list by Project, so that I can see how much one specific repo is costing me.
8. As a developer, I want to sort the Sessions list by cost, by start time, or by Prompt count, so that I can find Sessions matching different shapes of question.
9. As a developer, I want to drill from a Session in the list into a Session detail view, so that I can see what happened in that specific Claude run.
10. As a developer, I want a Session detail view that shows total cost, total tokens, breakdown by model, count of API Requests, count of Subagent Requests, count of Tool Calls, count of denied Tool Decisions, and count of API Errors, so that I have a complete picture of one Session at a glance.
11. As a developer, I want a Session detail view that lists every Prompt in chronological order with per-Prompt cost, duration, and tool count, so that I can spot which Prompt drove the cost.
12. As a developer, I want to drill from a Prompt in a Session into a Prompt detail view, so that I can see exactly what Claude did within a single user turn.
13. As a developer, I want the Prompt detail view to render a chronological timeline of API Requests, Tool Decisions, and Tool Results with relative timestamps, so that I can see the shape of a Prompt's execution.
14. As a developer with `OTEL_LOG_TOOL_DETAILS=1` set, I want the Prompt detail view to show the actual Bash command for each Tool Result, so that I can recall what Claude actually did.
15. As a developer with `OTEL_LOG_TOOL_DETAILS=1` set, I want the Prompt detail view to label Subagent Requests with their `subagent_type` (Explore, Plan, etc.), so that I can tell which kind of subagent was dispatched.
16. As a developer, I want Tool Decisions to show whether the decision was `accept` or `reject` and whether it came from config / user / hook, so that I can audit my own auto-approve patterns.
17. As a developer, I want a label on each Session showing the Project it ran in, so that I can identify Sessions without remembering their UUIDs.
18. As a developer, I want a setup snippet I can paste into `.claude/settings.json` to enable telemetry, including a placeholder for `project.name`, so that onboarding a new repo takes 30 seconds.
19. As a developer, I want the daemon to bind only to `127.0.0.1`, so that no other machine on my network can speak to it.
20. As a developer, I want the daemon to fail-fast and log clearly if `:4317` is already taken, so that I can diagnose port conflicts immediately.
21. As a developer, I want raw events older than 30 days to be pruned automatically, so that the SQLite file doesn't grow unbounded.
22. As a developer, I want Session and Prompt rollups to be retained forever, so that I can see long-term cost trends across months.
23. As a developer, I want the daemon and the TUI to coexist without database lock errors, so that I can browse history while a Claude Code Session is actively writing.
24. As a developer, I want the TUI to refresh automatically while a Session is in progress, so that I can watch costs accumulate live.
25. As a developer, I want a manual refresh shortcut in the TUI, so that I can force an update without waiting for the auto-poll.
26. As a developer, I want familiar keyboard navigation (j/k, ↑/↓, enter, esc, q), so that the TUI feels at home for terminal users.
27. As a developer, I want the TUI to gracefully handle Sessions that haven't ended (no `ended_at`), so that in-progress Sessions show as "active" rather than appearing broken.
28. As a developer, I want a Session that's been idle for 30 minutes to be marked ended automatically, so that "active" Sessions reflect reality even though Claude Code doesn't emit a reliable session-end event.
29. As a developer, I want the daemon to ingest both the OTel Logs signal (primary) and the OTel Metrics signal (sanity check), so that I have an independent witness for aggregate cost numbers.
30. As a developer, I want the daemon to keep running and log a warning if Anthropic adds a new event attribute, so that future Claude Code releases don't crash my tracker.
31. As a developer, I want the daemon to keep running and log a warning if it receives an unknown event name, so that one weird record doesn't take down ingestion.
32. As a developer, I want a `claude-code-observer rebuild-rollups` subcommand, so that if I ever spot a discrepancy between the rollups and the raw events I can rebuild without losing data.
33. As a developer, I want a `claude-code-observer init` subcommand that writes a `.claude/settings.json` snippet with `project.name` set to the current directory's basename, so that I don't have to copy-paste manually.
34. As a developer, I want the SQLite path and retention duration to be configurable via env vars, so that I can override defaults without recompiling.
35. As a developer, I want the daemon to back up Claude Code's Sessions over crashes — i.e. an event in the middle of a Session being written successfully even if a later event fails — so that I never lose half a Session's data.
36. As a developer, I want the binary to be small (under ~10MB), so that distribution and updates feel lightweight.
37. As a developer, I want the daemon to start in well under 1 second, so that launchd-managed restarts don't notice it.
38. As a developer, I want all data stored locally and never sent anywhere, so that my prompt content (when prompt-logging is enabled) never leaves my machine.
39. As a developer, I want to opt in to `OTEL_LOG_TOOL_DETAILS` separately from `OTEL_LOG_USER_PROMPTS`, so that I can see tool detail without persisting prompt text.
40. As a developer, I want the TUI to render reasonably on terminals as narrow as 80 columns, so that I can use it inside a tmux split.
41. As a developer, I want a help screen / footer in the TUI showing keyboard shortcuts, so that I don't have to memorize them.

## Implementation Decisions

### Binary structure

A single Go binary with subcommand dispatch. Subcommands:

- `serve` — runs daemon (Receiver + Repository + retention sweeper). Foreground process; long-running. Designed to be supervised by launchd / systemd.
- (no args) — opens TUI. Read-only against the same SQLite file.
- `init` — writes a `.claude/settings.json` template into the current working directory's `.claude/` (creates the dir if missing). Sets `project.name` from `basename $PWD`. Refuses to overwrite an existing `settings.json` without `--force`.
- `rebuild-rollups` — drops `sessions` and `prompts` and rebuilds them by re-scanning `events`. Runs against a stopped daemon (acquires the write lock).

### Module decomposition

Three deep modules (testable in isolation, simple interface) plus thin orchestration around them:

- **`internal/service/eventparser`** — pure function from OTLP `(Resource, ScopeLogs, LogRecord)` to a typed `domain.Event`. All knowledge of OTel attribute names lives here. Returns a typed error on missing required fields (`session.id`).
- **`internal/service/rollup`** — pure function from `(Event, prior Session row, prior Prompt row)` to `(next Session row, next Prompt row)`. Encodes which event kinds contribute to which rollup columns. No DB access.
- **`internal/service/retention`** — given a `Clock` interface and the repository, prunes events older than the retention window and marks idle Sessions as ended. Cron-style: runs once per minute for idle sweep, once per day for prune.

Shallow surrounding layers:

- **`internal/domain`** — value types only.
- **`internal/repository`** — SQLite access; one write connection (held by daemon), pooled read connections (used by TUI). Transactions wrap event-insert + rollup-upsert atomically.
- **`internal/receiver`** — gRPC server implementing `LogsServiceServer.Export` and `MetricsServiceServer.Export`. Decodes OTLP protobuf, calls `eventparser`, hands typed Events to a Service that calls `rollup` then writes via `repository`.
- **`internal/tui`** — Bubble Tea models, one per screen. Polls the read-only repository.
- **`cmd/app`** — flag parsing, subcommand dispatch.

### Receiver

Direct gRPC implementation per [ADR-001](../decisions/ADR-001-thin-otlp-receiver.md). Dependencies: `go.opentelemetry.io/proto/otlp` + `google.golang.org/grpc` only — not `otelcol-contrib`. gRPC `:4317` only in v1; HTTP/protobuf is a future addition. Bind to `127.0.0.1` strictly.

### Storage shape

Per [ADR-002](../decisions/ADR-002-events-table-with-json-attrs.md):

- `events(id, ts, session_id, prompt_id, event_name, attrs JSON)` — append-only source of truth.
- `sessions` — typed rollup, indexed on `(started_at DESC)` and `(project_name, started_at DESC)`.
- `prompts` — typed rollup, indexed on `(session_id, started_at)`.
- `metric_snapshots` — independent Metrics signal, used as a sanity check only ([ADR-003](../decisions/ADR-003-logs-as-primary-signal.md)).
- `schema_version` — migration tracking.

SQLite WAL mode. Default DB path: `~/.claude-code-observer/db.sqlite`. Configurable via `CCO_DB_PATH`. Retention configurable via `CCO_RETENTION_DAYS` (default 30). Idle-session timeout configurable via `CCO_SESSION_IDLE_MIN` (default 30 min).

Full table definitions in [docs/DATA-MODELS.md](../DATA-MODELS.md).

### Event ingestion contract

The Receiver's `Export` handler is **synchronous** with respect to SQLite write completion. We do not buffer in-memory beyond what gRPC itself provides. Rationale: provides natural backpressure to Claude Code's exporter, and any in-process buffer that survives crash without fsync is a lie about durability. The SQLite write batches at the SDK layer (5-second log flush interval is already a batch) so per-batch fsync is fine.

If parsing fails (e.g. missing `session.id`), the Service drops the record, increments an internal `events_dropped_total` counter, logs a warning, and **acks success to the caller**. We never NACK — Claude Code retrying a malformed record won't fix anything.

### Project labelling

Sessions are labeled via the `project.name` resource attribute, set per-repo in `.claude/settings.json` via `OTEL_RESOURCE_ATTRIBUTES`. Sessions without `project.name` appear as `(unlabeled)` in the UI. Optional companion attribute `project.cwd` for the absolute path. The `init` subcommand seeds these.

### Process model

Two subcommands; users supervise via launchd/systemd. The repo will ship sample plist + unit files under `dist/`. The daemon owns the SQLite write lock for its lifetime; the TUI uses `?mode=ro&_journal_mode=WAL` and never writes.

### TUI views (v1)

- **Dashboard** — Today + 7-day totals (cost, sessions, tokens). By-model breakdown for today. Recent Sessions list (last 10).
- **Sessions list** — Sortable by start time / cost / prompt count / duration. Filterable by Project. Press `enter` to drill in.
- **Session detail** — Header (Project, started, duration, totals). Per-Prompt list. Press `enter` on a Prompt to drill in.
- **Prompt detail** — Header (started, duration, length, cost, token breakdown). Chronological event timeline with relative timestamps.

Polling: 1-second interval against the read-only DB. Re-renders only the active view's queries.

Keyboard: `↑/↓`/`j/k` nav, `enter` drill, `esc` back, `q` quit, `r` force-refresh, `?` help. Per-view shortcuts (`p` to filter by Project on Sessions list, `s` to cycle sort) shown in the footer.

### Setup snippet (the one users paste)

```jsonc
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317",
    "OTEL_LOG_TOOL_DETAILS": "1",
    "OTEL_RESOURCE_ATTRIBUTES": "project.name=my-app"
  }
}
```

`OTEL_LOG_USER_PROMPTS` is intentionally absent — it's an opt-in users add manually if they want prompt-text capture.

### Concurrency model

- Daemon: one goroutine for gRPC `Logs.Export`, one for gRPC `Metrics.Export`, one cron loop for retention/idle sweep. All writes funnel through a single `Repository.Write*` method that holds the SQLite write connection (no concurrent writers).
- TUI: one goroutine for Bubble Tea, one for the polling loop, both reading via a pooled read-only sqlite connection. No locks needed; SQLite WAL handles reader/writer concurrency.

### Failure-mode contracts

| Failure | Behavior |
|---|---|
| Daemon not running when Claude Code emits | OTel SDK retries with backoff per its config; eventually drops. We do not buffer client-side. |
| Disk full / SQLite write error | gRPC NACK back to Claude Code → SDK retry. If sustained, Claude Code drops events after its retry budget. |
| Malformed event missing `session.id` | Drop, increment counter, ack success. |
| Unknown event name | Persist to `events` verbatim; rollup updater is a no-op. |
| Future Anthropic event attribute | Persisted in `attrs` JSON; surfaced in TUI without code changes. |

## Testing Decisions

### What "good test" means here

Tests assert observable behavior at module boundaries. Inputs and outputs are typed values; nobody pokes at internal state. A test name reads as a fact about the module: "an api_request event with `query_source=subagent` increments `sessions.subagent_requests` by 1". Implementation-detail tests ("verifies that `parseAttrs` was called with X") are out of scope.

Given the project is greenfield, prior art is the Go standard library convention: `_test.go` files alongside source, table-driven tests, `testdata/` for fixtures. CLAUDE.md already calls out "Table-driven tests for all pure functions" — we follow it.

### Modules covered in v1

- **`internal/service/eventparser`** — table-driven. Fixtures under `internal/service/eventparser/testdata/` containing OTLP `LogRecord` JSON for every event type (`user_prompt`, `api_request`, `api_error`, `tool_result`, `tool_decision`, plus an unknown-event-name case and a missing-`session.id` case). For each, assert the parsed `domain.Event` matches expectations. Pure function — no I/O, no test infrastructure.

- **`internal/service/rollup`** — table-driven. Each test case is `(prior Session, prior Prompt, incoming Event) → (next Session, next Prompt)`. Covers: first event in a session creates row; api_request sums cost and tokens; subagent api_request increments subagent_requests; tool_decision reject increments tool_denied; api_error increments api_errors; idempotency under same input.

- **`internal/service/retention`** — fake-clock test using a small `Clock` interface (`Now() time.Time`). Insert a fixture set spanning 60 days, advance the fake clock, call `Sweep`, assert that events older than retention are gone and idle sessions have `ended_at` set. Uses an in-memory SQLite via `:memory:` for repo backing.

- **`internal/repository`** — integration tests against a temp SQLite file. Per test: open a fresh DB at a tmp path, run migrations, exercise a write-then-read scenario, verify the visible state. Covers: migrations apply cleanly from empty; `WriteEvent` is atomic (row appears in `events` AND rollup upserts in same transaction); read queries return correct shapes; `rebuild-rollups` produces identical totals.

### Out of v1 test scope

- Receiver gRPC handler (shallow plumbing; tested implicitly by repository + service tests against decoded events).
- TUI rendering (visual; manual smoke-test).
- End-to-end "real Claude Code → daemon → TUI" smoke test (later, once we have a story for capturing OTLP fixtures from a real run).

## Out of Scope

- **Multi-user / multi-tenant.** v1 is local-only, single user. Auth and per-user partitioning are deferred — see [FUTURE.md](../FUTURE.md).
- **HTTP/protobuf OTLP transport (`:4318`).** gRPC `:4317` only in v1.
- **Trace ingestion.** Claude Code's Traces signal is in beta; we don't subscribe in v1.
- **Web UI.** Despite `chi` being mentioned in CLAUDE.md, no HTTP server in v1. Future possibility.
- **Multi-machine sync / shared SQLite.** Out of scope.
- **Anomaly detection, FTS prompt search, cost charts, sparklines, notifications.** All in [FUTURE.md](../FUTURE.md).
- **Subagent-type labelling without `OTEL_LOG_TOOL_DETAILS=1`.** Without this flag, Claude Code does not emit `subagent_type` and we can only count subagent requests, not name them. This is a documented limitation, not a v1 feature.
- **Recomputing cost from token counts.** We always trust the `cost_usd` attribute Anthropic reports. Pricing tables drift; reporting does not.
- **`session_end` event handling.** Claude Code's session-end event is not formally documented; we use idle-timeout heuristic instead.
- **Per-tool latency histograms, time-to-first-token, MCP server latency.** Not exposed by Claude Code's OTel surface.
- **Cross-session correlation of subagent dispatches.** Subagent Requests share `prompt.id` with their parent — no separate child Session ID is emitted; we render them inline under the parent Prompt.

## Further Notes

- **The "metrics" misnomer.** The single most important conceptual point new contributors must internalize: *the per-prompt detail lives in OTel Logs, not OTel Metrics.* [ADR-003](../decisions/ADR-003-logs-as-primary-signal.md) exists specifically to prevent re-discovery of this fact. Internal terminology says "Event" everywhere to avoid the ambiguity.
- **Privacy posture.** Tool-detail logging (`OTEL_LOG_TOOL_DETAILS=1`) is recommended by `init`; prompt-text logging (`OTEL_LOG_USER_PROMPTS=1`) is opt-in only. Even when both are on, data never leaves the machine — no telemetry, no update check, no analytics.
- **Forward compatibility with Anthropic changes.** New event attributes land in `events.attrs` JSON automatically. Adding a new column to a rollup is a tiny migration. Adding a brand-new event name requires extending `eventparser` and `rollup` — both have a single switch on `event_name` to update.
- **Backfill story.** If `rollup` logic ever has a bug that produces wrong totals, `claude-code-observer rebuild-rollups` regenerates rollup tables by re-scanning `events`. Raw events are the source of truth.
- **Verification gates.** Every change must pass `go vet ./...`, `go test ./...`, `go build -o bin/app` per CLAUDE.md.
- **No `chi`, `net/http`, or web stack in v1**, despite CLAUDE.md mentioning chi. The Go module should not import them. (CLAUDE.md should be updated to reflect this once the receiver exists.)
