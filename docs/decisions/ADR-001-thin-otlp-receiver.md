# ADR-001: Implement OTLP/gRPC receiver directly, do not use otelcol-contrib libs

- **Status:** accepted
- **Date:** 2026-05-10
- **Deciders:** sonanh

## Context

Claude Code emits telemetry over OTLP. We need to receive it. The obvious path is to import `go.opentelemetry.io/collector/receiver/otlpreceiver` from the upstream OpenTelemetry Collector and wire it into a pipeline. That brings 100+ transitive deps and produces a 30 MB+ binary.

We are local-only, single-user, and only need two methods: `LogsService.Export` and `MetricsService.Export`. We do not need batching, retry queues, fan-out to multiple exporters, dynamic config reload, or any of the other features the collector framework exists to provide.

## Decision

Implement the OTLP/gRPC receiver directly. Depend only on:

- `go.opentelemetry.io/proto/otlp` — generated protobuf types
- `google.golang.org/grpc` — gRPC server

Implement `colmetricspb.MetricsServiceServer` and `collogspb.LogsServiceServer` ourselves. Each `Export` handler iterates `ResourceLogs` → `ScopeLogs` → `LogRecords`, flattens attributes, and hands the result to the Service layer for persistence.

Listen on `:4317` (gRPC) only. HTTP/protobuf on `:4318` is not supported in v1.

## Alternatives Considered

### Option A: otelcol-contrib receiver libs (rejected)

- **Pros:** Free batching, retry, multi-protocol (gRPC + HTTP), well-tested.
- **Cons:** 100+ transitive deps. 30 MB+ binary. We use ~1% of the surface area. Upgrades drag in unrelated breakage. Local-only daemon doesn't benefit from collector features.

### Option B: Hand-rolled HTTP/protobuf only (rejected)

- **Pros:** Tiniest deps — just `proto.Unmarshal` and chi. ~3 MB binary.
- **Cons:** Most Claude Code monitoring docs and copy-paste configs in the wild use `OTEL_EXPORTER_OTLP_PROTOCOL=grpc`. Requiring users to set a non-default protocol is friction. Loses drop-in compatibility with existing setup snippets.

### Option C: Direct gRPC, our chosen path

- **Pros:** Drop-in compatible with the most common Claude Code OTel setup. ~5 MB binary. Full control over decode path. ~50 lines per service.
- **Cons:** No HTTP fallback in v1. We hand-write the receive loop (but it's small).

## Consequences

### Positive
- Tiny binary (~5 MB), fast cold start, fast `go build`.
- We see exactly what arrives on the wire — no opaque collector pipeline.
- Easier to reason about backpressure: a synchronous SQLite write inside `Export` naturally provides flow control (Claude Code's exporter retries on slow ack).

### Negative
- We will reimplement small slices of behavior the collector provides for free (e.g. graceful drain on shutdown, attribute deduplication if it ever matters).
- Adding HTTP/protobuf later means writing a second receiver. Not hard, but extra work.

### Risks
- If Anthropic changes the OTel SDK to use a non-standard exporter or to compress payloads in a non-standard way, we'll have to handle it ourselves rather than getting it for free from the collector libs.

## References

- [docs/CLAUDE-CODE-OTEL.md](../CLAUDE-CODE-OTEL.md) §11.6 — collector config that we are *replacing*
- [go.opentelemetry.io/proto/otlp](https://pkg.go.dev/go.opentelemetry.io/proto/otlp)
