# Phase 1 — Ingest Path Design

> Date: 2026-05-10
> Roadmap: [docs/ROADMAP.md](../../ROADMAP.md) — Phase 1 (M1.1, M1.2, M1.3)
> ADR: [docs/decisions/ADR-001-thin-otlp-receiver.md](../../decisions/ADR-001-thin-otlp-receiver.md)
> Status: Approved — ready for implementation plan

## Goal

Real Claude Code can send OTLP gRPC to `127.0.0.1:4317` and rows appear in `events` and `metric_snapshots`. No rollups in this phase — that is Phase 2.

## Non-goals

- Rollup tables (`sessions`, `prompts`) — Phase 2.
- Retention pruning — Phase 2.
- TUI — Phase 3.
- HTTP/OTLP-HTTP receiver — gRPC only per ADR-001.

## Module boundaries

```
gRPC server (internal/receiver)        thin transport, no domain knowledge
   │  hands off ExportLogsServiceRequest / ExportMetricsServiceRequest
   ▼
internal/service.Service               orchestration, transaction boundary
   │  per record: eventparser.Parse → domain.Event (or ErrDrop sentinel)
   │  collects survivors, writes all in ONE sqlite tx
   │  on tx error → returns error → gRPC Unavailable
   ▼
internal/repository                    bulk insert into events / metric_snapshots
```

Dependencies:

- `internal/receiver` defines `LogIngester` and `MetricIngester` interfaces (consumer-side, per CLAUDE.md). Service implements them.
- `internal/service` depends on `eventparser` and `repository`.
- `internal/eventparser` is pure: input is OTel proto types, output is `domain.Event`. No I/O, no DB, no gRPC concerns.

## Failure semantics

**One transaction per OTLP request, fail loud.**

- All N parsed records in a request commit in a single SQLite tx.
- Records that fail to parse with `ErrDrop` (e.g., missing `session.id`) are logged at WARN and skipped — they do **not** fail the batch.
- Any DB error during the tx → return `codes.Unavailable` so the client retries the whole batch.
- Rationale: simplest atomic story; OTLP exporters are designed to retry on Unavailable; partial-state never observable to the rest of the system.

## Metric scope

Persist **every** datapoint Claude Code emits into `metric_snapshots`, with `metric_name` as a column. No whitelist.

- Future-proof against new Anthropic metrics with no code change.
- Retention pruner (Phase 2) controls disk volume.
- Phase 2 rollup engine consumes only the named metrics it cares about; storage is a superset.

## eventparser (deep module)

Single exported entry point:

```go
func Parse(rec *logspb.LogRecord, resource *resourcepb.Resource) (domain.Event, error)
```

- Returns `ErrDrop` sentinel when `session.id` is absent on the record (Service warns + skips).
- Internal dispatch table keyed on `event.name` body attribute. Known names get typed extraction; unknown names are stored verbatim with all attrs preserved.
- Resource attributes `project.name` and `app.version` are flattened into `event.attrs` so consumers don't need to join.
- Covers the 5 documented event names plus the 3 community-observed names listed in `docs/CLAUDE-CODE-OTEL.md`.

## Service ingest path

Two methods:

```go
func (s *Service) IngestLogs(ctx context.Context, req *logspb.ExportLogsServiceRequest) error
func (s *Service) IngestMetrics(ctx context.Context, req *metricspb.ExportMetricsServiceRequest) error
```

Both follow the same pattern:

1. Walk `ResourceLogs`/`ResourceMetrics` → `ScopeLogs`/`ScopeMetrics` → records/datapoints.
2. Parse each item; collect survivors into a slice; log+skip drops.
3. Open one write transaction on the daemon's single write connection.
4. Bulk insert into the relevant table.
5. Commit; on any error, rollback and return.

Single-write-connection serialization is already guaranteed by the M0.2 repository.

## Receiver

- `internal/receiver/server.go` exposes `New(ingester LogMetricIngester, addr string) *Server`.
- `Start(ctx) error` binds the gRPC listener and registers `LogsServiceServer` + `MetricsServiceServer`.
- Methods are one-liners that call back into the ingester and translate Go errors into gRPC status codes.
- Bound to `127.0.0.1:4317` only — never exposed beyond loopback.

## Fixture capture workflow

To produce realistic fixtures for `eventparser` tests, ship a small one-shot tool: `cmd/capture-fixtures/`.

- Stands up an OTLP gRPC listener identical to the production one, but writes each incoming `ExportLogsServiceRequest` to a timestamped JSON file under `internal/eventparser/testdata/captured/`.
- Developer runs it once, points a real `claude` session at it (`OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317`), drives ≥1 prompt that triggers tool calls, then Ctrl-C.
- The resulting files are hand-curated into named fixtures (`user_prompt.json`, `api_request.json`, `tool_decision.json`, `tool_result.json`, `api_error.json`, plus the 3 community-observed names) and committed to the repo.
- The tool remains useful long-term: when Anthropic changes the schema, re-capture and update fixtures.

A second tiny tool `cmd/parser-debug/` reads a single fixture file and prints the parsed `domain.Event` for the M1.2 demo.

## Milestone slicing

### M1.1 — gRPC OTLP receiver

- New deps: `google.golang.org/grpc`, `go.opentelemetry.io/proto/otlp`.
- `internal/receiver/` implements `LogsServiceServer` and `MetricsServiceServer` that log the request shape (record count, scope/resource summary) and return success. **No DB writes.**
- `cmd/app serve` starts the receiver alongside the existing repository open.
- Tests use `bufconn`: empty request → success; well-formed request → success + counter incremented; malformed nil → returns gRPC error without panic.
- Coverage gate: ≥70% on `internal/receiver/`.

### M1.2 — eventparser + fixture tooling

- `internal/eventparser/` with `Parse`, `ErrDrop`, dispatch table, resource-attr flattening.
- `cmd/capture-fixtures/` and `cmd/parser-debug/` tools.
- Real fixtures captured from a live Claude session, committed under `internal/eventparser/testdata/`.
- Table-driven tests: one row per documented event (5), one per community event (3), missing `session.id` → ErrDrop, unknown event → verbatim stored, resource attrs flattened.
- Coverage gate: ≥90% on `internal/eventparser/`.

### M1.3 — End-to-end ingest

- Wire `receiver → Service → Repository`. Service implements the two interfaces declared by receiver.
- Repository gains `InsertEvents([]Event)` and `InsertMetricSnapshots([]MetricSnapshot)` methods that operate on a single tx.
- Integration test: in-process receiver on a random port + temp SQLite; send a synthetic `ExportLogsServiceRequest` with `user_prompt` + `api_request` for the same `prompt.id`; assert both rows landed with correct fields and `attrs` JSON content. Same for a metrics request.
- Demo: configure local Claude per `docs/CLAUDE-CODE-OTEL.md`, drive a session, verify rows in `events` and `metric_snapshots`.
- Coverage gate: ≥80% across the integration boundary.

## Cross-cutting

- All code paths that fail must wrap errors with `fmt.Errorf("…: %w", err)` per CLAUDE.md.
- All SQL parameterized — no string concat.
- No `any` without a comment justifying it.
- `go vet`, `golangci-lint`, `go test`, `go build` all green at end of each milestone (per ROADMAP cross-phase gates).

## Open questions resolved during brainstorming

| Question | Decision |
|----------|----------|
| Tx granularity / failure mode | One tx per request, fail loud → gRPC Unavailable |
| Metric storage scope | Store all metrics verbatim |
| Fixture source for parser tests | Capture from live Claude session |
| Service interface location | Defined in `internal/receiver` (consumer side) |
| capture-fixtures placement | Separate `cmd/capture-fixtures/`, sibling to `cmd/app/` |
