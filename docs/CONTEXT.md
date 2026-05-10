# claude-code-observer

A local-only Go daemon + TUI that ingests Claude Code's OpenTelemetry export over OTLP/gRPC, persists it to SQLite, and lets the user drill from "today's cost" down to a single tool invocation inside a single prompt.

## Language

**Session**:
A single Claude Code CLI invocation, identified by `session.id` (UUID, regenerated each launch). Bounded by the lifetime of the `claude` process.
_Avoid_: run, instance.

**Prompt**:
A single user-turn within a Session, identified by `prompt.id` (UUID). One Prompt may produce many API Requests and Tool Results.
_Avoid_: turn, message, query.

**API Request**:
One successful call to the Claude API, emitted as a `claude_code.api_request` log event. Carries token counts, cost, model, and a `query_source` of `main`, `subagent`, or `auxiliary`.
_Avoid_: completion, inference, model call.

**API Error**:
A failed API call after all internal retries are exhausted, emitted as `claude_code.api_error`. Distinct from API Request — never both for the same call.

**Tool Decision**:
The permission-check outcome for a tool invocation, emitted as `claude_code.tool_decision`. Independent from the Tool Result that follows. `decision_source` distinguishes config-allow from user-approval.
_Avoid_: permission, allowance, gate.

**Tool Result**:
The completion record of a tool call, emitted as `claude_code.tool_result`. Carries `tool_name`, sizes, and (when `OTEL_LOG_TOOL_DETAILS=1`) the actual command/parameters.
_Avoid_: tool call, tool execution. Use Tool Result when referring to the OTel record specifically.

**Subagent Request**:
An API Request whose `query_source` attribute is `subagent`. Conceptually nested under a parent Prompt but technically a sibling event sharing the same `prompt.id`.
_Avoid_: child agent, nested agent.

**Auxiliary Request**:
An API Request whose `query_source` is `auxiliary` — context compaction, background summarization, etc. Counts toward cost but is not user-initiated.

**Project**:
A logical grouping of Sessions sharing the same `project.name` resource attribute. Set per-repo via `OTEL_RESOURCE_ATTRIBUTES` in `.claude/settings.json`. Optional — Sessions without a Project are grouped as "(unlabeled)".
_Avoid_: repo, workspace, directory. Project is the term we use across the UI.

**Resource**:
The OTel envelope describing the producing process — `service.name`, `service.version`, `host.arch`, `os.type`, `user.id`, `session.id`, `project.name`. Attached to every Event from a given Session.

**Event**:
Any single OTel log record emitted by Claude Code. The atomic unit of storage. Persisted to the `events` table verbatim with attributes serialized as JSON.

**Rollup**:
A materialized aggregation maintained by the Service layer on Event ingest. The `sessions` and `prompts` tables are Rollups — they are derived from Events, never sources of truth.

**Cost**:
USD value reported by Anthropic on the `cost_usd` attribute of each API Request. Always trust the reported value; never recompute from token counts (pricing changes).

**Cache Read / Cache Creation**:
Token counts for prompt-cache hits (`cache_read`) versus tokens written into a new cache entry (`cache_creation`). Distinct from Input tokens (uncached prompt) and Output tokens (model response).

**Receiver**:
The gRPC server (`internal/receiver`) that implements OTLP `LogsService.Export` and `MetricsService.Export`. The only inbound network surface.

**Daemon**:
The long-lived `claude-code-observer serve` process. Owns the Receiver and the SQLite write connection.

**TUI**:
The `claude-code-observer` (no-args) Bubble Tea process. Owns a read-only SQLite connection and renders Dashboard / Sessions / Session detail / Prompt detail views.

## Relationships

- A **Session** contains many **Prompts**.
- A **Prompt** triggers one or more **API Requests** (one if no subagent or compaction; many if either).
- A **Prompt** triggers zero or more **Tool Decisions**, each followed by a **Tool Result** with the same `tool_use_id`.
- A **Subagent Request** is an **API Request** under the same parent **Prompt** as the user-turn that dispatched it (matched via `prompt.id`).
- A **Project** groups many **Sessions**.
- An **Event** belongs to exactly one **Session** and at most one **Prompt** (Session-level events have no `prompt.id`).
- A **Rollup** is computed from many **Events** but never the reverse.

## Example dialogue

> **Dev:** "When the user says 'how much did this session cost', do we sum cost from each Prompt or each API Request?"
> **Domain expert:** "Sum from API Requests — one Prompt produces multiple API Requests when subagents fire or when context is auto-compacted. The Prompt-level cost is itself a sum of its API Requests; the Session-level cost is the sum across all of them."

> **Dev:** "Are Tool Decisions and Tool Results the same thing?"
> **Domain expert:** "No. A Tool Decision can be `reject`, in which case there is no Tool Result. They are correlated by `tool_use_id` only when both fire."

## Flagged ambiguities

- "tool call" was used loosely in early discussions — resolved: use **Tool Result** for the OTel record (post-execution) and **Tool Decision** for the permission check (pre-execution). Never "tool call" in code or UI labels.
- "subagent" sometimes referred to a separate Session — resolved: in the OTel data, a Subagent Request shares the parent Session's `session.id` and the dispatching Prompt's `prompt.id`. There is no child Session.
- "metrics" is used by Claude Code's docs to mean two different things — resolved: the OTel **Metrics** signal carries only aggregate counters; the per-prompt detail the UI cares about lives in the OTel **Logs** signal as Events. We say "Events" everywhere internally to avoid the confusion.
