# Decision Records

This folder tracks important decisions made during the project's lifetime.

## Decision Types

| Type | When to use | Template |
|------|-------------|----------|
| **ADR** (Architecture Decision Record) | Architecture decisions affecting the entire system | [_TEMPLATES/ADR-TEMPLATE.md](_TEMPLATES/ADR-TEMPLATE.md) |
| **Y-Statement** (Agent Decision Record) | Lightweight decisions made during coding | Append to [DECISION-LOG.md](DECISION-LOG.md) |

## When to Create an ADR

Answer YES to ≥ 2 of these questions:
- Does it affect code across many modules?
- Is it hard to reverse (> 1 day to revert)?
- Does it change an external interface (API, DB schema)?
- Do other team members need to know to code correctly?
- Does it change an important dependency?

## How to Create an ADR

1. Copy `_TEMPLATES/ADR-TEMPLATE.md` → `ADR-NNN-short-title.md`
2. Fill in ALL sections — context and alternatives are the most important
3. Status: `proposed` → get team review → `accepted`
4. Add entry to the index below

## Decision Index

| ID | Title | Status | Date |
|----|-------|--------|------|
| [ADR-001](ADR-001-thin-otlp-receiver.md) | Implement OTLP/gRPC receiver directly, do not use otelcol-contrib libs | accepted | 2026-05-10 |
| [ADR-002](ADR-002-events-table-with-json-attrs.md) | Store events in one table with JSON attributes; maintain rollups | accepted | 2026-05-10 |
| [ADR-003](ADR-003-logs-as-primary-signal.md) | OTel Logs is the primary signal; Metrics is supplementary | accepted | 2026-05-10 |
