# ADR-002: Store events in one table with JSON attributes; maintain rollups

- **Status:** accepted
- **Date:** 2026-05-10
- **Deciders:** sonanh

## Context

Claude Code emits ~6 distinct event types (`user_prompt`, `api_request`, `api_error`, `tool_result`, `tool_decision`, plus undocumented session/compact/subagent events). The attribute set on each event is **actively evolving** — `effort`, `command_name`, and `command_source` were added in recent releases, and more are likely. Some attributes are gated on user-set flags (`OTEL_LOG_TOOL_DETAILS`, `OTEL_LOG_USER_PROMPTS`) so are present only sometimes.

We need to query by `session.id` (Sessions list, Session detail), by `prompt.id` (Prompt detail), and aggregate cost/tokens for the Dashboard.

## Decision

Two-tier storage:

1. **`events` table** — one row per OTel log record, with attributes serialized to a single JSON column. Indexes on `(session_id, ts)` and `prompt_id`. SQLite's JSON1 extension handles attribute extraction in queries.

2. **`sessions` and `prompts` rollup tables** — typed columns for the fields we display frequently (cost, token sums, request counts). Maintained by the Service layer in the same transaction as the event insert. Never queried for raw event detail; only for aggregates.

Schema migrations target only the rollup tables. The `events` table schema is stable: when Anthropic adds a new attribute, it lands in the JSON blob automatically and we surface it in the UI without a migration.

## Alternatives Considered

### Option A: Fully normalized — one typed table per event type (rejected)

- **Pros:** Cleaner queries (no `json_extract`), better SQLite indexes, easier introspection from `sqlite3` CLI.
- **Cons:** Every Anthropic schema change is a migration. We saw three new attributes added in recent Claude Code releases — that's a new migration each time. With six event types, that's six migration code paths to maintain.

### Option B: Raw OTLP protobuf blob + rollups only (rejected)

- **Pros:** Smallest write path. Forensic replay possible.
- **Cons:** Every read goes through the rollups. Want to see a tool's input? Decode protobuf at read time. Adds complexity and slows the drill-down views, which are exactly the views we built this for.

### Option C: events + JSON attrs + rollups, our chosen path

- **Pros:** Schema stability across Claude Code releases. Fast common-case queries (rollups) and full-detail drill-down (events). JSON1 indexes available if a JSON-attribute query becomes hot.
- **Cons:** `json_extract` slower than typed column access — but only for ad-hoc detail queries, never for the hot dashboard paths.

## Consequences

### Positive
- New Anthropic event attributes appear in the TUI for free — no code change, no migration.
- Forensic queries possible: "show me every tool_result with `tool_input_size_bytes > 100000`" is a one-liner.
- Rollups can be rebuilt from `events` if logic changes.

### Negative
- Two write paths per event ingest (event row + rollup update). Both must succeed in the same transaction.
- Rollup logic is duplicated knowledge: it must encode which attributes contribute to which rollup column.

### Risks
- If the rollup logic has a bug, the dashboard shows wrong totals while the underlying truth in `events` is fine. Mitigation: a `claude-code-observer rebuild-rollups` admin command.
- JSON column size: with `OTEL_LOG_USER_PROMPTS=1`, a single event row can hold tens of KB of prompt text. Acceptable for local-only; would not scale to multi-user.

## References

- [docs/DATA-MODELS.md](../DATA-MODELS.md) — full schema
- [SQLite JSON1 documentation](https://www.sqlite.org/json1.html)
