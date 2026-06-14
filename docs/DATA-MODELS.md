# Data Models — claude-code-observer

> Last verified: 2026-05-10
> Schema source: `internal/repository/migrations/*.sql`

## Entity Relationship Overview

```
[Project]* 1──N [Session] 1──N [Prompt] 1──N [Event]
                    │
                    └─────── 1──N [MetricSnapshot]

* Project is a virtual entity — not a row, but a session.project_name value.
```

## Core Tables

### `events` (raw event log — source of truth)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| id | INTEGER | ✅ | Primary key, autoincrement |
| ts | INTEGER | ✅ | Unix nanoseconds (OTel `time_unix_nano`) |
| session_id | TEXT | ✅ | From resource or record attribute |
| prompt_id | TEXT | ❌ | Null on session-level events |
| event_name | TEXT | ✅ | e.g. `claude_code.api_request` |
| attrs | TEXT | ✅ | JSON object: all log-record attributes flattened, plus selected resource attributes (`project.name`, `app.version`) |

**Indexes:**
- `(session_id, ts)` — Session detail timeline, idle-timeout sweep
- `(prompt_id)` — Prompt detail
- `(event_name, ts)` — debugging, future analytics

**Retention:** Pruner deletes rows where `ts < now - 30 days` once per day. Configurable via `CCO_RETENTION_DAYS`.

### `sessions` (rollup)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| session_id | TEXT | ✅ | Primary key |
| project_name | TEXT | ❌ | From `OTEL_RESOURCE_ATTRIBUTES`; null = "(unlabeled)" |
| project_cwd | TEXT | ❌ | Optional, from same source |
| started_at | INTEGER | ✅ | Min `ts` of any event with this `session_id` |
| last_seen_at | INTEGER | ✅ | Max `ts`, updated on every insert |
| ended_at | INTEGER | ❌ | Set by idle-timeout sweep (30 min default) |
| app_version | TEXT | ❌ | Resource attr |
| os_type | TEXT | ❌ | Resource attr |
| user_id | TEXT | ❌ | Resource attr |
| cost_usd | REAL | ✅ | Sum of `cost_usd` across `api_request` events |
| input_tokens | INTEGER | ✅ | Sum |
| output_tokens | INTEGER | ✅ | Sum |
| cache_read_tokens | INTEGER | ✅ | Sum |
| cache_creation_tokens | INTEGER | ✅ | Sum |
| api_requests | INTEGER | ✅ | Count of `api_request` events |
| api_errors | INTEGER | ✅ | Count of `api_error` events |
| subagent_requests | INTEGER | ✅ | Count where `query_source = 'subagent'` |
| auxiliary_requests | INTEGER | ✅ | Count where `query_source = 'auxiliary'` |
| tool_calls | INTEGER | ✅ | Count of `tool_result` events |
| tool_denied | INTEGER | ✅ | Count of `tool_decision` events with `decision = 'reject'` |
| prompts | INTEGER | ✅ | Count of distinct `prompt_id` |

**Indexes:** `(started_at DESC)`, `(project_name, started_at DESC)`, `(last_seen_at DESC)`. The session list and dashboard recent-sessions panel order by `last_seen_at DESC` (most recently active first).

### `prompts` (rollup)

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| prompt_id | TEXT | ✅ | Primary key |
| session_id | TEXT | ✅ | FK to `sessions.session_id` |
| started_at | INTEGER | ✅ | `ts` of `user_prompt` event (or first event with this `prompt_id`) |
| ended_at | INTEGER | ❌ | Max `ts` of any event with this `prompt_id` (best-effort) |
| prompt_length | INTEGER | ❌ | From `user_prompt.prompt_length` |
| command_name | TEXT | ❌ | Slash command if any |
| command_source | TEXT | ❌ | `builtin` / `custom` / `mcp` |
| cost_usd | REAL | ✅ | Sum across api_request events for this prompt |
| input_tokens | INTEGER | ✅ | Sum |
| output_tokens | INTEGER | ✅ | Sum |
| cache_read_tokens | INTEGER | ✅ | Sum |
| cache_creation_tokens | INTEGER | ✅ | Sum |
| api_requests | INTEGER | ✅ | |
| subagent_requests | INTEGER | ✅ | |
| tool_calls | INTEGER | ✅ | |
| had_error | INTEGER | ✅ | 1 if any `api_error` event seen, 0 otherwise |

**Indexes:** `(session_id, started_at)`.

### `metric_snapshots` (sanity-check store, see [ADR-003](decisions/ADR-003-logs-as-primary-signal.md))

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| id | INTEGER | ✅ | Primary key |
| ts | INTEGER | ✅ | OTel datapoint time |
| session_id | TEXT | ❌ | Null if cardinality control stripped it |
| metric_name | TEXT | ✅ | e.g. `claude_code.cost.usage` |
| value | REAL | ✅ | Counter cumulative value |
| attrs | TEXT | ✅ | JSON: model, token_type, language, etc. |

**Retention:** Pruned with the same 30-day window as `events`.

### `schema_version` (migrations)

| Field | Type | Notes |
|-------|------|-------|
| version | INTEGER | Latest applied migration |
| applied_at | INTEGER | Unix seconds |

## Key Constraints

- **A Session is created on first sight.** No explicit `session_start` event needed; the first event with a new `session_id` upserts a row.
- **A Prompt belongs to exactly one Session.** Enforced at write time: rejecting prompt events with a `prompt_id` whose `session_id` doesn't match the session row's `session_id`. (Should never happen in practice.)
- **`ended_at` on `sessions` is best-effort.** Set by the idle-timeout sweep; may be null for in-progress sessions.
- **Rollup totals are derived.** If they ever drift from `events`, the truth is `events`. `claude-code-observer rebuild-rollups` regenerates rollup tables by re-scanning events.
- **`events.attrs` is opaque to migrations.** New attribute names appear inside the JSON without schema changes.

## Migration Strategy

- All migrations live in `internal/repository/migrations/NNNN_name.sql`. Numbered sequentially.
- Applied at daemon startup inside a single transaction.
- Schema additions only — no destructive changes to `events`. Rollup tables can be dropped and rebuilt from `events` if necessary.
- Backwards compatibility: a v1.x daemon must read a v1.0 SQLite file without issue (additive migrations only).

## Example queries

```sql
-- Today's cost
SELECT SUM(cost_usd) FROM sessions
WHERE started_at >= unixepoch('now', 'start of day') * 1000000000;

-- Prompt detail timeline (uses JSON1)
SELECT ts, event_name,
       json_extract(attrs, '$.tool_name')   AS tool_name,
       json_extract(attrs, '$.duration_ms') AS duration_ms,
       json_extract(attrs, '$.cost_usd')    AS cost_usd
FROM events
WHERE prompt_id = ?
ORDER BY ts;

-- Subagent count for a session
SELECT COUNT(*) FROM events
WHERE session_id = ?
  AND event_name = 'claude_code.api_request'
  AND json_extract(attrs, '$.query_source') = 'subagent';

-- Top 10 most expensive prompts in last 7 days
SELECT prompt_id, session_id, cost_usd
FROM prompts
WHERE started_at >= unixepoch('now', '-7 days') * 1000000000
ORDER BY cost_usd DESC LIMIT 10;
```
