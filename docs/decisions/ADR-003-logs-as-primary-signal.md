# ADR-003: OTel Logs is the primary signal; Metrics is supplementary

- **Status:** accepted
- **Date:** 2026-05-10
- **Deciders:** sonanh

## Context

Claude Code's documentation talks about "OpenTelemetry metrics" and the natural assumption — including the project's original framing — is that metrics is where the per-session, per-prompt detail lives. It is not.

The OTel **Metrics** signal carries only aggregated counters: total tokens by `(token_type, model)`, total cost in USD, total commits, total tool decisions. Useful for a daily total, but the data is flushed every **60 seconds** and carries no `prompt.id` or `tool_use_id`. There is no per-prompt drill-down possible from metrics alone.

The OTel **Logs** signal carries one record per event: `claude_code.api_request` (with cost, tokens, model, `query_source`, `prompt.id`), `claude_code.tool_result` (with `tool_use_id`, `tool_name`, sizes), `claude_code.tool_decision` (with `decision`, `decision_source`), `claude_code.user_prompt`, `claude_code.api_error`. Flushed every **5 seconds**. Joined by `prompt.id`.

This is widely misunderstood — see [issue #15417](https://github.com/anthropics/claude-code/issues/15417) — and a future contributor working from the metric names alone would build the wrong thing.

## Decision

Treat the OTel **Logs** signal as the source of truth for everything the UI displays. The Receiver subscribes to both signals, but:

- The `events` table is populated **only from log records**.
- Metric datapoints are persisted to a separate, much smaller `metric_snapshots` table for sanity-checking aggregates against our log-derived rollups.
- All UI queries read from `events` / `sessions` / `prompts`. None read from `metric_snapshots`.

Internally we say **Event**, never "metric", to avoid the confusion. The `claude_code.token.usage` metric is not what populates token columns in the Dashboard — `claude_code.api_request` events are.

## Alternatives Considered

### Option A: Metrics as primary (rejected)

- **Pros:** Simpler — metric counters arrive pre-aggregated.
- **Cons:** No per-prompt detail. No tool drill-down. Defeats the project's purpose. Also: 60 s flush means the live dashboard is up to a minute stale.

### Option B: Logs only, ignore metrics entirely (considered)

- **Pros:** Less code. Single signal to reason about.
- **Cons:** Lose a free sanity check. If our rollup logic ever drifts from reality, the `claude_code.cost.usage` metric is an independent witness. Cheap to keep.

### Option C: Logs primary, metrics supplementary (chosen)

- **Pros:** Right data shape for the use case. Independent witness for aggregates.
- **Cons:** Two ingest paths to maintain (small).

## Consequences

### Positive
- A new contributor reading `internal/receiver/logs.go` immediately sees that this is where the interesting data lives.
- Sanity check available: if `metric_snapshots.cost_usd` diverges materially from `SUM(events.cost_usd)`, something is wrong.

### Negative
- Two ingest handlers and two storage tables for a project that "just" wants a CLI dashboard.
- Naming hazard: when someone says "the metric for tokens", we have to clarify which source they mean.

### Risks
- If Anthropic ever moves per-prompt detail to traces or to a different signal, this ADR is the place to revise the assumption.

## References

- [docs/CLAUDE-CODE-OTEL.md](../CLAUDE-CODE-OTEL.md) §1, §7, §8
- [GitHub issue #15417: OTEL telemetry exports events via logs protocol, not metrics](https://github.com/anthropics/claude-code/issues/15417)
