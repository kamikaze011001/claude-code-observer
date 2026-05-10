# Architecture — claude-code-observer

> Last verified: 2026-05-10

## System Overview

A single Go binary with two subcommands. `serve` runs a long-lived daemon that accepts OTLP/gRPC from Claude Code on `:4317`, parses the log records into typed Events, and persists them to a local SQLite database alongside maintained rollups. The default no-arg invocation opens a Bubble Tea TUI that reads the same SQLite (read-only) and lets the user drill from "today" to a single tool invocation.

Single-user, local-only. No HTTP server, no auth, no remote storage.

## System Diagram

```
                    ┌────────────────────────┐
   Claude Code ───▶ │  Receiver (gRPC :4317) │
   (OTLP/gRPC)      │  internal/receiver     │
                    └────────────┬───────────┘
                                 │ Event{}, MetricSnapshot{}
                                 ▼
                    ┌────────────────────────┐
                    │   Service              │
                    │   internal/service     │
                    │   (parse → rollup)     │
                    └────────────┬───────────┘
                                 │
                                 ▼
                    ┌────────────────────────┐
                    │   Repository (SQLite)  │
                    │   internal/repository  │
                    │   WAL mode             │
                    └────────────┬───────────┘
                                 │
                                 ▼
                       ~/.claude-code-observer/
                              db.sqlite
                                 ▲
                                 │ read-only
                    ┌────────────┴───────────┐
                    │   TUI (Bubble Tea)     │
                    │   internal/tui         │
                    └────────────────────────┘
```

## Layer Responsibilities

| Layer | Location | Responsibilities | Does NOT |
|-------|----------|-----------------|----------|
| Receiver | `internal/receiver/` | Bind gRPC, implement `LogsServiceServer.Export` and `MetricsServiceServer.Export`, decode OTLP protobuf into domain types | Touch SQLite, decide what to keep, run rollup math |
| Service | `internal/service/` | Parse log records into typed `Event` values, dispatch to the correct rollup updater, run periodic retention pruner | Bind ports, render UI |
| Repository | `internal/repository/` | All SQLite access — `events`, `sessions`, `prompts`, `metric_snapshots`. Owns migrations. Single write connection (daemon), pooled read connections (TUI) | Business logic, attribute parsing |
| Domain | `internal/domain/` | `Event`, `Session`, `Prompt`, `ToolResult`, etc. Interfaces consumed by Receiver/Service/TUI | Side effects |
| TUI | `internal/tui/` | Bubble Tea models, lipgloss styling, view rendering, keyboard nav, polling | Write to DB, hold a write connection |
| Entry | `cmd/app/` | `serve`, default (TUI), `rebuild-rollups`, `init` subcommands | Logic — only orchestration |

## Key Boundaries

- **TUI is read-only.** Never opens a write connection. Polls the read-only DB every 1 s.
- **Receiver never touches SQLite directly** — always via the Service.
- **The `events` table is append-only** at runtime. Pruner is the only deleter and runs in a single goroutine inside the daemon.
- **Rollup updates happen in the same SQLite transaction as the event insert.** No async backfill at runtime.
- **Logs is the primary signal** — see [ADR-003](decisions/ADR-003-logs-as-primary-signal.md). Metrics is persisted only for sanity-checking aggregates.

## Data Flow

### A user runs `claude` and types a prompt

```
1. Claude Code reads .claude/settings.json, sees OTLP config, starts SDK exporter.
2. User types prompt → Claude Code emits claude_code.user_prompt log record.
3. Internal API call → claude_code.api_request log record (5 s flush).
4. Tool permission check → claude_code.tool_decision.
5. Tool runs → claude_code.tool_result.
6. All five flow over gRPC :4317 to our Receiver.
7. Receiver decodes ExportLogsServiceRequest → []domain.Event.
8. Service classifies each Event by event.name attribute, validates prompt.id /
   session.id presence.
9. Repository, in a single transaction:
   a. INSERT into events (session_id, prompt_id, event_name, ts, attrs JSON)
   b. UPSERT sessions row (extending last_seen, accumulating cost/tokens)
   c. UPSERT prompts row (creating on first sight, accumulating)
   d. COMMIT.
10. TUI's poll loop sees new max(events.id), invalidates its cached views,
    re-renders.
```

### Session "end" detection

Claude Code does not emit a reliable `session_end` event. The Service runs an idle-timeout sweep every 60 s: any `sessions` row whose `last_seen` is older than 30 minutes gets `ended_at` written. This is a lossy heuristic, accepted in v1.

## Key Technical Decisions

| Decision | Choice | Reason | ADR |
|----------|--------|--------|-----|
| OTLP receiver impl | Direct gRPC + otlp protos | Avoid 100+ collector deps | [ADR-001](decisions/ADR-001-thin-otlp-receiver.md) |
| Storage shape | `events` + JSON attrs + rollup tables | Schema stable across Claude Code releases | [ADR-002](decisions/ADR-002-events-table-with-json-attrs.md) |
| Primary signal | OTel Logs, not Metrics | Per-prompt detail only in logs | [ADR-003](decisions/ADR-003-logs-as-primary-signal.md) |
| Process model | Daemon + TUI as two subcommands | Receiver must outlive TUI | — |
| DB engine | SQLite WAL | Local, single-writer, embedded | — |
| TUI lib | Bubble Tea + lipgloss + bubbles | De-facto Go TUI stack | — |
| Transport | OTLP/gRPC `:4317` only | Drop-in compatible with most Claude Code monitoring docs | [ADR-001](decisions/ADR-001-thin-otlp-receiver.md) |
| Project labelling | `OTEL_RESOURCE_ATTRIBUTES=project.name=...` | Claude Code emits no native cwd/project attribute | — |
| Retention | Raw events 30 d, rollups forever | Bound disk, preserve trends | — |

## External Integrations

None. The Receiver is the only network surface, and it binds to `127.0.0.1:4317` only. No outbound HTTP. No analytics. No update check.

## Performance Considerations

- **Write throughput target: 100 events/sec sustained.** Far above realistic Claude Code emission rate (~10–20 events/sec at peak across multiple parallel sessions). Single SQLite writer in WAL mode handles this easily.
- **TUI poll interval: 1 s.** Re-runs only the queries needed for the visible view. Dashboard rollup queries should return in <5 ms with rollup tables of any realistic size.
- **JSON1 `json_extract` queries** are reserved for drill-down screens (Prompt detail). Hot-path queries (Dashboard, Sessions list) read only typed columns from rollup tables.
- **Indexes:** `events(session_id, ts)`, `events(prompt_id)`, `sessions(started_at DESC)`, `prompts(session_id, ts)`. No FTS in v1 (Future feature — see [FUTURE.md](FUTURE.md)).

## Failure Modes

| Failure | Behavior |
|---------|----------|
| SQLite write blocked / locked | Receiver `Export` blocks → Claude Code retries with backoff → events buffered in Claude Code's exporter queue |
| Daemon not running when Claude Code starts | Claude Code's exporter logs connection failures (visible in `~/.claude/logs/`); events are dropped after the SDK's retry budget. No buffering on our side. |
| Disk full | SQLite write fails → Receiver returns gRPC error → see above |
| Malformed event (missing `session.id`) | Service drops with a warn log, increments an internal `events_dropped_total` counter |
| Unknown event name | Stored verbatim in `events` table; rollup updater is a no-op |
