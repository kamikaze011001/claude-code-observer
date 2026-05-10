# Claude Code OpenTelemetry (OTel) Observability — Complete Reference

> **Purpose:** Engineering reference for building a Go OTLP receiver that ingests Claude Code
> metrics and log-events and renders a CLI UI. Sourced from official Anthropic documentation
> and confirmed community implementations as of May 2026.
>
> **Status note:** The telemetry system is actively developed. Traces support is in beta
> (`CLAUDE_CODE_ENHANCED_TELEMETRY_BETA`). Field names and export intervals are subject to
> change; always cross-reference against `code.claude.com/docs/en/monitoring-usage`.

---

## Table of Contents

1. [Signal Architecture Overview](#1-signal-architecture-overview)
2. [Enabling Telemetry — Environment Variables](#2-enabling-telemetry--environment-variables)
3. [Cardinality Control Variables](#3-cardinality-control-variables)
4. [Privacy / Content Logging Variables](#4-privacy--content-logging-variables)
5. [Dynamic Header Refresh](#5-dynamic-header-refresh)
6. [settings.json Configuration](#6-settingsjson-configuration)
7. [Metric Catalogue](#7-metric-catalogue)
8. [Log-Event Catalogue](#8-log-event-catalogue)
9. [Standard Resource Attributes](#9-standard-resource-attributes)
10. [Common Datapoint Attributes (on every signal)](#10-common-datapoint-attributes-on-every-signal)
11. [Sample Minimal Configs](#11-sample-minimal-configs)
12. [Caveats, Known Issues, and Limitations](#12-caveats-known-issues-and-limitations)
13. [Sources](#13-sources)

---

## 1. Signal Architecture Overview

Claude Code exports **three independent OTel signals**. Each has its own enable switch and
exporter configuration, so you can turn on only the ones you need.

| Signal | OTel protocol path | Default export interval | Notes |
|--------|--------------------|------------------------|-------|
| **Metrics** | OTLP Metrics | 60 000 ms (60 s) | Counters/gauges — "how much" totals |
| **Logs / Events** | OTLP Logs | 5 000 ms (5 s) | Structured log records; the primary signal for per-interaction detail |
| **Traces** | OTLP Traces | N/A (push on completion) | **Beta** — requires `CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1` |

> **Critical implementation note:** Most of the interesting per-request data (token counts,
> costs, tool details) is exported as **log records** via the OTLP Logs protocol, not as
> traditional metrics. Your receiver must consume the Logs signal to capture granular
> per-session data. Metrics carry aggregated totals. See GitHub issue #15417.

---

## 2. Enabling Telemetry — Environment Variables

### 2.1 Master switch

| Variable | Required value | Description |
|----------|----------------|-------------|
| `CLAUDE_CODE_ENABLE_TELEMETRY` | `1` | Must be set to `1` to enable any OTel export. No other variable has effect without this. |

### 2.2 Exporter selection (one per signal)

| Variable | Supported values | Description |
|----------|-----------------|-------------|
| `OTEL_METRICS_EXPORTER` | `otlp` | Enables metric export. Set to `otlp` to use OTLP. |
| `OTEL_LOGS_EXPORTER` | `otlp` | Enables log/event export. Set to `otlp` to use OTLP. |
| `OTEL_TRACES_EXPORTER` | `otlp` | Enables trace export. **Requires beta flag** (see §2.5). |

Setting a variable to anything other than `otlp` or leaving it unset disables that signal.
`none` explicitly disables. `console` prints to stdout (useful for local debugging).

### 2.3 OTLP endpoint and protocol

These apply to all three signals unless overridden per-signal (§2.4).

| Variable | Example value | Description |
|----------|---------------|-------------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4317` | Base endpoint for all signals. For gRPC omit the path; for HTTP the SDK appends `/v1/metrics`, `/v1/logs`, `/v1/traces`. |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | Transport protocol. One of `grpc`, `http/protobuf`, `http/json`. Default: `http/protobuf`. |
| `OTEL_EXPORTER_OTLP_HEADERS` | `Authorization=Bearer token` | Static comma-separated `key=value` auth headers sent with every request. For gRPC only static headers are used. Dynamic header refresh requires a helper script (§5). |
| `OTEL_EXPORTER_OTLP_CERTIFICATE` | `/path/to/ca.pem` | CA certificate for TLS verification. |
| `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` | `/path/to/client.pem` | Client certificate for mTLS. **gRPC only.** |
| `OTEL_EXPORTER_OTLP_CLIENT_KEY` | `/path/to/client.key` | Private key for mTLS. **gRPC only.** |
| `CLAUDE_CODE_CLIENT_CERT` | `/path/to/client.pem` | Client certificate for mTLS. **HTTP (`http/protobuf`, `http/json`) only.** |
| `CLAUDE_CODE_CLIENT_KEY` | `/path/to/client.key` | Private key for mTLS. **HTTP only.** |
| `CLAUDE_CODE_CLIENT_KEY_PASSPHRASE` | `<passphrase>` | Optional passphrase for `CLAUDE_CODE_CLIENT_KEY` if the key is encrypted. **HTTP only.** |

> **mTLS protocol split (important):** The generic `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` /
> `OTEL_EXPORTER_OTLP_CLIENT_KEY` pair only takes effect for the gRPC exporter. For the
> HTTP exporters (`http/protobuf`, `http/json`) you must use the Claude Code-specific
> `CLAUDE_CODE_CLIENT_CERT` / `CLAUDE_CODE_CLIENT_KEY` pair instead.

### 2.4 Per-signal endpoint overrides

Standard OTel per-signal overrides are honoured. Use these when sending metrics and logs to
different backends.

| Variable | Description |
|----------|-------------|
| `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | Endpoint override for metrics only |
| `OTEL_EXPORTER_OTLP_METRICS_PROTOCOL` | Protocol override for metrics only |
| `OTEL_EXPORTER_OTLP_METRICS_HEADERS` | Headers override for metrics only |
| `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` | Endpoint override for logs/events only |
| `OTEL_EXPORTER_OTLP_LOGS_PROTOCOL` | Protocol override for logs/events only |
| `OTEL_EXPORTER_OTLP_LOGS_HEADERS` | Headers override for logs/events only |
| `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | Endpoint override for traces only (beta) |
| `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` | Protocol override for traces only (beta) |
| `OTEL_EXPORTER_OTLP_TRACES_HEADERS` | Headers override for traces only (beta) |

> **Dynamic headers note:** Dynamic headers (from a helper script — see §5) apply **only** to
> `http/protobuf` and `http/json` protocols. The gRPC exporter uses only the static
> `OTEL_EXPORTER_OTLP_HEADERS` value.

### 2.5 Export interval / batch settings

| Variable | Default | Unit | Description |
|----------|---------|------|-------------|
| `OTEL_METRIC_EXPORT_INTERVAL` | `60000` | ms | How often the metrics SDK pushes accumulated metric data. Reduce to `10000` for debugging. |
| `OTEL_LOGS_EXPORT_INTERVAL` | `5000` | ms | How often the log batch processor flushes. Already aggressive; reduce for debugging only. |
| `OTEL_TRACES_EXPORT_INTERVAL` | `5000` | ms | Span batch export interval (beta traces only). |
| `OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE` | `delta` | — | Metrics temporality preference. Set to `cumulative` if your backend (e.g. Prometheus) expects cumulative temporality. |

No documented per-signal batch size or queue depth variables; these use OTel SDK defaults.

### 2.6 Beta traces flag

| Variable | Required value | Description |
|----------|----------------|-------------|
| `CLAUDE_CODE_ENHANCED_TELEMETRY_BETA` | `1` | Must be set alongside `CLAUDE_CODE_ENABLE_TELEMETRY=1` to activate trace span export. Traces reuse the common OTLP endpoint, protocol, headers, and mTLS configuration. |

### 2.7 Custom resource attributes

| Variable | Example | Description |
|----------|---------|-------------|
| `OTEL_RESOURCE_ATTRIBUTES` | `enduser.id=jdoe@example.com,deployment.environment=prod` | Appends arbitrary key=value pairs to the OTel Resource attached to all signals. Useful when Claude Code runs without a Claude account (e.g., direct API key, Bedrock, Vertex) and `user.email` / `organization.id` would otherwise be absent. |
| `OTEL_SERVICE_NAME` | `claude-code` | Overrides the `service.name` resource attribute. Rarely needed — Claude Code sets this to `claude-code` by default. |

---

## 3. Cardinality Control Variables

These boolean variables control which attributes are attached to **metric datapoints**.
Excluding high-cardinality attributes (session IDs, version strings) reduces the number of
unique time-series created in your metrics backend.

| Variable | Default | Description |
|----------|---------|-------------|
| `OTEL_METRICS_INCLUDE_SESSION_ID` | `true` | Include `session.id` as a metric attribute. Set to `false` to collapse all sessions into a single series per user. |
| `OTEL_METRICS_INCLUDE_VERSION` | `false` | Include `app.version` (Claude Code version string) as a metric attribute. **Off by default** — set to `true` to enable. |
| `OTEL_METRICS_INCLUDE_ACCOUNT_UUID` | `true` | Include `user.account_uuid` and `user.account_id` as metric attributes. |

> **Note:** These affect only metric datapoint attributes. Log record attributes are not
> controlled by these variables — log records always carry the full attribute set.

---

## 4. Privacy / Content Logging Variables

By default Claude Code does **not** log prompt text, tool arguments, or tool output content.
Only metadata (sizes, counts, identifiers) is included.

| Variable | Default | What it enables when set to `1` |
|----------|---------|----------------------------------|
| `OTEL_LOG_USER_PROMPTS` | `0` (off) | Include the full text of user prompts in `user_prompt` log records. Off by default to protect sensitive developer input. |
| `OTEL_LOG_TOOL_DETAILS` | `0` (off) | Include `tool_parameters` in `tool_result` records: Bash commands (`bash_command`, `full_command`), MCP server and tool names, skill names, file paths, URLs, search patterns. |
| `OTEL_LOG_TOOL_CONTENT` | `0` (off) | Include tool output content in trace spans and log records. Potentially very large. |
| `OTEL_LOG_RAW_API_BODIES` | `0` (off) | Emit the full Anthropic Messages API request and response JSON bodies as `api_request_body` / `api_response_body` log events. Highest verbosity — payloads can be very large and contain prompt content. |

**What is always logged (cannot be suppressed without disabling the signal):**

- `prompt_length` (character count of the prompt) — metadata only, no text
- `tool_input_size_bytes` and `tool_result_size_bytes` — sizes only
- All identifiers: `session.id`, `prompt.id`, `user.id`, `organization.id`

---

## 5. Dynamic Header Refresh

For OTLP HTTP (`http/protobuf` or `http/json`) you can provide a helper script that generates
fresh authentication headers on a schedule. This is the recommended approach for backends
that use short-lived tokens (AWS SigV4, OAuth bearer, etc.).

**Configuration variable:**

| Variable | Default | Description |
|----------|---------|-------------|
| `CLAUDE_CODE_OTEL_HEADERS_HELPER` | unset | Path to an executable script that prints a JSON object of headers to stdout, e.g. `{"Authorization": "Bearer <token>"}`. |
| `CLAUDE_CODE_OTEL_HEADERS_HELPER_DEBOUNCE_MS` | `1740000` (29 min) | How often the helper script is re-executed to refresh headers. |

**Example helper script (`/usr/local/bin/otel-auth-headers.sh`):**

```bash
#!/bin/bash
TOKEN=$(curl -s http://169.254.169.254/latest/meta-data/iam/security-credentials/my-role | jq -r .Token)
echo "{\"Authorization\": \"Bearer ${TOKEN}\"}"
```

> The gRPC exporter ignores the helper script entirely; set `OTEL_EXPORTER_OTLP_HEADERS`
> statically for gRPC targets.

---

## 6. settings.json Configuration

The `env` key in `~/.claude/settings.json` (or managed settings at
`/Library/Application Support/ClaudeCode/managed-settings.json` on macOS) accepts
OTel environment variables as key-value pairs. Variables defined in the managed settings
file have **high precedence and cannot be overridden by users**.

### 6.1 Minimal gRPC config (localhost:4317)

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317"
  }
}
```

### 6.2 Minimal HTTP/Protobuf config (localhost:4318)

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318"
  }
}
```

### 6.3 Full enterprise config with cardinality control and traces (beta)

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "CLAUDE_CODE_ENHANCED_TELEMETRY_BETA": "1",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_TRACES_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://collector.internal:4317",
    "OTEL_EXPORTER_OTLP_HEADERS": "Authorization=Bearer <static-token>",
    "OTEL_METRIC_EXPORT_INTERVAL": "60000",
    "OTEL_LOGS_EXPORT_INTERVAL": "5000",
    "OTEL_METRICS_INCLUDE_SESSION_ID": "true",
    "OTEL_METRICS_INCLUDE_VERSION": "false",
    "OTEL_METRICS_INCLUDE_ACCOUNT_UUID": "true"
  }
}
```

### 6.4 Equivalent shell environment variables

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317

# Debugging: faster flush
export OTEL_METRIC_EXPORT_INTERVAL=10000
export OTEL_LOGS_EXPORT_INTERVAL=2000
```

---

## 7. Metric Catalogue

Metrics are **aggregated counters or gauges** exported on the metric export interval (default
60 s). They answer "how much in total" questions. For per-request detail, use log events (§8).

All metrics carry the [common datapoint attributes](#10-common-datapoint-attributes-on-every-signal)
plus any metric-specific attributes listed in the "Additional Attributes" column.

### 7.1 Metric table

| Metric name | OTel type | Unit | Description | Additional attributes |
|-------------|-----------|------|-------------|----------------------|
| `claude_code.session.count` | Counter | `{sessions}` | Incremented by 1 at the start of each new Claude Code session. | _(none beyond common)_ |
| `claude_code.lines_of_code.count` | Counter | `{lines}` | Incremented when Claude Code adds or removes lines of code in a file. Reports net change per write operation. | `type` (`"added"` \| `"removed"`), `language` (e.g. `TypeScript`, `Python`, `Go`, `Markdown`; `unknown` for unrecognised extensions) |
| `claude_code.pull_request.count` | Counter | `{pull_requests}` | Incremented when Claude Code creates a pull request or merge request via a shell command or MCP tool. | _(none beyond common)_ |
| `claude_code.commit.count` | Counter | `{commits}` | Incremented when Claude Code creates a git commit via a shell command or MCP tool. | _(none beyond common)_ |
| `claude_code.cost.usage` | Counter | `USD` | Cumulative API cost in US dollars. Incremented after each API request by that request's cost. | `model` (e.g. `claude-opus-4-5`, `claude-sonnet-4-5`) |
| `claude_code.token.usage` | Counter | `{tokens}` | Token consumption broken down by type and model. A separate datapoint series is emitted for each `(type, model)` combination. | `type` (`"input"` \| `"output"` \| `"cacheRead"` \| `"cacheCreation"` — note camelCase), `model` |
| `claude_code.active_time.total` | Counter | `s` (seconds) | Cumulative time spent actively using Claude Code in the session, excluding idle time. Incremented during user interactions and during CLI processing (tool execution, AI response generation). | _(none beyond common)_ |
| `claude_code.code_edit_tool.decision` | Counter | `{decisions}` | Incremented when the user accepts or rejects an `Edit`, `Write`, or `NotebookEdit` tool invocation. **Scoped to code-edit tools only — not a generic tool permission counter.** | `tool_name` (`"Edit"` \| `"Write"` \| `"NotebookEdit"`), `decision` (`"accept"` \| `"reject"`) |

> **Token type definitions** (values of the `type` attribute on `claude_code.token.usage`):
> - `input` — Tokens in the prompt that are not served from cache (i.e., tokens after the
>   last cache breakpoint).
> - `output` — Tokens generated by the model in the response.
> - `cacheRead` — Tokens retrieved from the prompt cache for this request. **Note camelCase.**
> - `cacheCreation` — Tokens written to the prompt cache when creating a new cache entry. **Note camelCase.**

---

## 8. Log-Event Catalogue

> **Wire format note:** Claude Code emits these events with **bare** names on `LogRecord.event_name` — e.g. `user_prompt`, not `claude_code.user_prompt`. The `claude_code.` namespace is reserved for metric names (§7). Receivers should match on bare names; reference implementations may also strip a leading `claude_code.` defensively to survive a future re-prefix.

Claude Code exports events as **OTel log records** via the OTLP Logs protocol. The
`event.name` attribute (or the log record body/name field depending on the collector)
identifies the event type.

All events carry a `prompt.id` attribute — a UUID that correlates every event triggered by
a single user prompt turn. Use this to join all API calls, tool executions, and tool
decisions that belong to one prompt.

All events also carry the [common datapoint attributes](#10-common-datapoint-attributes-on-every-signal).

### 8.1 `user_prompt`

Fired when a user submits a prompt (presses Enter in the REPL, or triggers a slash command).

| Attribute | Type | Description |
|-----------|------|-------------|
| `prompt.id` | string (UUID) | Unique identifier for this prompt turn. All downstream events share this value. |
| `prompt_length` | int | Character count of the prompt text. Always present. |
| `prompt_text` | string | Full prompt text. **Only present when `OTEL_LOG_USER_PROMPTS=1`.** |
| `command_name` | string | Name of the slash command invoked, if any. Built-in and bundled command names are emitted as-is; custom, plugin, and MCP command names are collapsed to `custom` or `mcp` unless `OTEL_LOG_TOOL_DETAILS=1`. |
| `command_source` | string | Origin of the command: `builtin`, `custom`, or `mcp`. |
| `event.sequence` | int | Monotonically increasing counter within the session, for ordering events. |

### 8.2 `api_request`

Fired after each successful call to the Claude API (after streaming completes).

| Attribute | Type | Description |
|-----------|------|-------------|
| `prompt.id` | string (UUID) | Links to the parent user prompt. |
| `model` | string | Model ID used for this request (e.g. `claude-opus-4-5-20251101`). |
| `input_tokens` | int | Prompt tokens not served from cache. |
| `output_tokens` | int | Response tokens generated by the model. |
| `cache_read_tokens` | int | Tokens served from the prompt cache. |
| `cache_creation_tokens` | int | Tokens written to the prompt cache. |
| `cost_usd` | float | Cost of this single API request in USD. |
| `duration_ms` | int | Elapsed time from request start to stream-end in milliseconds. |
| `request_id` | string | Anthropic API request ID from the response `request-id` header. Present only when the API returns one. |
| `query_source` | string | Subsystem that issued the request: `main` (primary REPL loop), `subagent` (spawned subagent), or `auxiliary` (background tasks such as compaction). |
| `speed` | string | `fast` when the request used fast mode (extended thinking shortcut); absent otherwise. |
| `effort` | string | Effort level applied when the model supports it: `low`, `medium`, `high`, `xhigh`, or `max`. Absent when the model does not support effort levels. |

### 8.3 `api_error`

Fired when an API request fails after all internal retries are exhausted. Intermediate retry
attempts are **not** logged as separate events.

| Attribute | Type | Description |
|-----------|------|-------------|
| `prompt.id` | string (UUID) | Links to the parent user prompt. |
| `model` | string | Model that was being called. |
| `error` | string | Human-readable error description. |
| `status_code` | int | HTTP status code of the final failed response (e.g. `429`, `500`, `529`). Absent for non-HTTP errors such as connection failures. |
| `duration_ms` | int | Total time spent across all retry attempts before giving up. |
| `attempt` | int | Total number of attempts made (including the initial attempt and all retries). |
| `request_id` | string | Anthropic API request ID, if the server returned one before failing. |
| `query_source` | string | Same as `api_request`: `main`, `subagent`, or `auxiliary`. |
| `effort` | string | Effort level, if applicable. |

### 8.4 `tool_result`

Fired when a tool invocation completes (both successful and failed executions).

| Attribute | Type | Description |
|-----------|------|-------------|
| `prompt.id` | string (UUID) | Links to the parent user prompt. |
| `tool_use_id` | string | Unique identifier for this tool invocation. Matches the `tool_use_id` passed to PreToolUse/PostToolUse hooks. |
| `tool_name` | string | Name of the tool (e.g. `Bash`, `Read`, `Write`, `Edit`, `mcp__server__tool`). |
| `success` | bool | `true` if the tool executed without error, `false` otherwise. |
| `duration_ms` | int | Wall-clock duration of the tool invocation in milliseconds. |
| `error` | string | Human-readable error message when `success=false`. Absent on success. |
| `error_type` | string | Error category when `success=false` (e.g. `timeout`, `permission_denied`). Absent on success. |
| `decision_type` | string | Mirrors the `decision` value from the matching `tool_decision` event (`accept`/`reject`). |
| `decision_source` | string | Mirrors the `source` value from the matching `tool_decision` event (`config`, `hook`, `user_permanent`, `user_temporary`, `user_abort`, `user_reject`). |
| `tool_input_size_bytes` | int | Size in bytes of the tool's input arguments. |
| `tool_result_size_bytes` | int | Size in bytes of the tool's output/result. |
| `mcp_server_scope` | string | For MCP tools: the scope of the MCP server (`local`, `project`, `user`). |
| `tool_parameters` | string (JSON) | **Only present when `OTEL_LOG_TOOL_DETAILS=1`.** For Bash: includes `bash_command`, `full_command`, `timeout`, `description`, `dangerouslyDisableSandbox`, `git_commit_id` (the commit SHA when a git commit command succeeds). For MCP tools: includes `mcp_server_name`, `mcp_tool_name`. For skills: includes skill name. |
| `tool_input` | string | **Only present when `OTEL_LOG_TOOL_DETAILS=1`.** File paths, URLs, search patterns, and other arguments. |

### 8.5 `tool_decision`

Fired each time Claude Code evaluates whether a tool invocation is permitted.

| Attribute | Type | Description |
|-----------|------|-------------|
| `prompt.id` | string (UUID) | Links to the parent user prompt. |
| `tool_use_id` | string | Matches the `tool_use_id` of the corresponding `tool_result` event. |
| `tool_name` | string | Name of the tool for which the decision was made. |
| `decision` | string | `accept` or `reject`. |
| `source` | string | Where the decision originated. One of: `config` (automatic, from project settings / allow rules / enterprise policy / flags — no user prompt shown), `hook` (a PreToolUse or PermissionRequest hook returned the decision), `user_permanent` (user chose "Yes, and don't ask again" — written to settings), `user_temporary` (user approved for this session only), `user_abort` (user pressed Escape or Ctrl-C), `user_reject` (user explicitly denied). |

### 8.6 `compaction`

Fired when context compaction runs.

| Attribute | Type | Description |
|-----------|------|-------------|
| `trigger` | string | What triggered the compaction (e.g. automatic threshold, explicit `/compact` command). |
| `success` | bool | Whether compaction completed successfully. |
| `duration_ms` | int | Time spent compacting. |
| `pre_tokens` | int | Approximate token count before compaction. |
| `post_tokens` | int | Approximate token count after compaction. |

### 8.7 Other officially-documented events

The following events are also defined in the official source. Each has its own dedicated
attribute schema — consult `code.claude.com/docs/en/monitoring-usage` for full details.
A receiver should at minimum recognise the `event.name` so unknown events can be logged
or dispatched generically.

| Event name | When fired |
|------------|-----------|
| `permission_mode_changed` | User changes permission mode (e.g. `default` → `acceptEdits` → `plan` → `bypassPermissions`). |
| `auth` | Authentication state change (login, logout, refresh). |
| `mcp_server_connection` | MCP server connect / disconnect / error. |
| `internal_error` | Internal Claude Code error (not an API error). |
| `plugin_installed` | A plugin is installed. |
| `skill_activated` | A skill is activated/invoked. |
| `at_mention` | An `@`-mention reference resolved (e.g. `@file`, `@folder`). |
| `api_retries_exhausted` | All retries for an API request were used (the subsequent failure produces `api_error`). |
| `hook_execution_start` | Hook script invocation begins. |
| `hook_execution_complete` | Hook script invocation finishes. |
| `api_request_body` | **Only when `OTEL_LOG_RAW_API_BODIES=1`.** Full Anthropic Messages API request JSON. |
| `api_response_body` | **Only when `OTEL_LOG_RAW_API_BODIES=1`.** Full Anthropic Messages API response JSON. |

### 8.8 Community-observed events (not in official docs)

The following event names have been observed in community implementations but are **not**
listed in the official monitoring-usage reference. Treat them as best-effort — names and
schemas may not be stable.

| Event name | Notes |
|------------|------|
| `session_start` / `session_end` | Not in official docs as of this writing; session lifecycle is conveyed via the `claude_code.session.count` metric. |
| `subagent_dispatch` | Not in official docs; subagent activity is inferred from `query_source` on `api_request`. |

> **Not natively exposed:** Per-tool latency histograms, time-to-first-token, subagent
> counts as a dedicated metric, per-file diff sizes, MCP server latency, context window
> utilization, and intra-request streaming checkpoints.

---

## 9. Standard Resource Attributes

The OTel **Resource** describes the entity (the Claude Code process) that produced the
telemetry. Resource attributes are attached to every metric, log, and trace from a given
process.

| Attribute | Value | Notes |
|-----------|-------|-------|
| `service.name` | `claude-code` | Always set. Override via `OTEL_SERVICE_NAME`. |
| `service.version` | e.g. `2.x.y` | The Claude Code CLI version string. |
| `app.version` | Same as `service.version` | Redundant; present for compatibility. |
| `host.arch` | `amd64`, `arm64`, etc. | Detected from the runtime environment. |
| `os.type` | `darwin`, `linux`, `windows` | Detected automatically. |
| `os.version` | e.g. `14.5` | Detected automatically. |
| `process.pid` | integer | OS process ID. |
| `user.id` | string (hashed/opaque) | Installation-scoped stable identifier. Always present. |
| `user.email` | string | **Present only when signed in via Claude.ai OAuth.** Absent for direct API key, Bedrock, Vertex, or Foundry. |
| `user.account_uuid` | UUID | **Present only when signed in via Claude.ai OAuth.** |
| `user.account_id` | string | Tagged-format account ID matching the Anthropic admin APIs (e.g. `user_01BWBeN28...`). Present when authenticated. |
| `organization.id` | UUID | **Present only when authenticated against a Team/Enterprise account.** |
| `session.id` | UUID | Per-invocation session identifier. Changes each time `claude` is launched. |
| `terminal.type` | string | Terminal type Claude Code is running under (e.g. `iTerm.app`, `vscode`, `cursor`, `tmux`). Detected automatically. |

### 9.1 Attribute presence matrix

| Auth mode | `user.id` | `user.email` | `user.account_uuid` | `organization.id` |
|-----------|-----------|-------------|--------------------|--------------------|
| Claude.ai OAuth (personal) | yes | yes | yes | no |
| Claude.ai OAuth (Team/Enterprise) | yes | yes | yes | yes |
| Direct API key (`ANTHROPIC_API_KEY`) | yes | no | no | no |
| Amazon Bedrock | yes | no | no | no |
| Google Vertex AI | yes | no | no | no |
| Microsoft Azure Foundry | yes | no | no | no |

---

## 10. Common Datapoint Attributes (on every signal)

| Attribute | Type | Description |
|-----------|------|-------------|
| `user.id` | string | Installation-scoped user identifier. Always present. |
| `user.email` | string | OAuth-only. |
| `organization.id` | UUID | Team/Enterprise OAuth-only. |
| `session.id` | UUID | Current session identifier. |
| `prompt.id` | UUID | **Log events only.** Identifies the user prompt turn. |
| `event.sequence` | int | **Log events only.** Monotonic counter within the session. |
| `app.version` | string | Claude Code CLI version. |

---

## 11. Sample Minimal Configs

### 11.1 gRPC to localhost:4317 — shell

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
```

### 11.2 gRPC to localhost:4317 — settings.json

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "grpc",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4317"
  }
}
```

### 11.3 HTTP/Protobuf to localhost:4318 — shell

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
# SDK will POST to http://localhost:4318/v1/metrics and http://localhost:4318/v1/logs
```

### 11.4 HTTP/Protobuf to localhost:4318 — settings.json

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_TELEMETRY": "1",
    "OTEL_METRICS_EXPORTER": "otlp",
    "OTEL_LOGS_EXPORTER": "otlp",
    "OTEL_EXPORTER_OTLP_PROTOCOL": "http/protobuf",
    "OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318"
  }
}
```

### 11.5 Separate endpoints for metrics and logs

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_METRICS_EXPORTER=otlp
export OTEL_LOGS_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_METRICS_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_LOGS_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_LOGS_ENDPOINT=http://siem.internal:4318
```

---

## 12. Caveats, Known Issues, and Limitations

### 12.1 What is in beta / may change

- **Traces are beta.** Span structure and attribute names may change without notice.
- **Event attribute names** are evolving: `effort`, `command_name`, `command_source` were
  added in recent releases.
- New metrics are added in minor releases — monitor the changelog.

### 12.2 Known bugs (as of May 2026)

- **OTLP exporter packages not bundled in 2.1.113** (issue #50567) — `OTEL_METRICS_EXPORTER=otlp`
  silently no-ops. Upgrade past this version.
- **OTel broken between 2.64–2.67** (issue #13803) — upgrade past 2.67.
- **`organization.id` mismatch** (issue #4339) — may not match `oauthAccount.organizationUuid`
  in `~/.claude.json`.
- **Windows enterprise managed accounts** (issue #46204) — third-party OTel may not
  initialize.
- **`OTEL_*` vars not propagated to subprocesses** — Bash tool, hooks, MCP servers, and
  language servers spawned by Claude Code do not inherit OTel configuration.

### 12.3 What is not exposed

| Capability | Status |
|-----------|--------|
| Per-tool latency histograms | Not instrumented |
| Time-to-first-token / inter-token latency | Not instrumented |
| Subagent count as a distinct metric | Inferred from `query_source=subagent` on `api_request` |
| Per-file diff sizes | `lines_of_code` carries `language` only; filenames in `tool_parameters` (with `OTEL_LOG_TOOL_DETAILS=1`) |
| Streaming checkpoints | Not instrumented |
| MCP server latency | Not instrumented |
| Context window utilization | Inferred from `cache_creation_tokens` |
| Number of retries per request | Only final `attempt` count on `api_error` |

### 12.4 OTel vars don't affect Anthropic-side telemetry

`CLAUDE_CODE_ENABLE_TELEMETRY=1` controls only the third-party OTLP export. It does not
affect privacy-controlled telemetry to Anthropic's backends.

### 12.5 Metric latency vs log latency

Metrics flush every 60 s by default → up to 60 s stale. Logs flush every 5 s → near-real-
time. **For a live CLI dashboard, consume the Logs signal** (`api_request`) rather than
metrics.

### 12.6 No native Prometheus endpoint

Claude Code does not expose a `/metrics` HTTP endpoint. Bridge via OTel Collector or
direct OTLP receiver.

---

## 13. Sources

Primary (official):

- https://docs.claude.com/en/docs/claude-code/monitoring-usage
- https://code.claude.com/docs/en/monitoring-usage
- https://code.claude.com/docs/en/agent-sdk/observability
- https://support.claude.com/en/articles/14477985-monitor-claude-cowork-activity-with-opentelemetry
- https://support.claude.com/en/articles/14447276-configure-a-custom-opentelemetry-collector-for-office-agents
- https://github.com/anthropics/claude-code-monitoring-guide

Community references (used to fill gaps):

- https://signoz.io/blog/claude-code-monitoring-with-opentelemetry/
- https://signoz.io/docs/claude-code-monitoring/
- https://quesma.com/blog/track-claude-code-usage-and-limits-with-grafana-cloud/
- https://www.elastic.co/security-labs/claude-code-cowork-monitoring-otel-elastic/
- https://ma.rtin.so/posts/monitoring-claude-code-with-datadog/
- https://bindplane.com/blog/claude-code-opentelemetry-per-session-cost-and-token-tracking
- https://github.com/ColeMurray/claude-code-otel
- https://axiom.co/docs/guides/opentelemetry-claude-code

GitHub issues referenced:

- https://github.com/anthropics/claude-code/issues/15417
- https://github.com/anthropics/claude-code/issues/50567
- https://github.com/anthropics/claude-code/issues/13803
- https://github.com/anthropics/claude-code/issues/4339
- https://github.com/anthropics/claude-code/issues/46204
- https://github.com/anthropics/claude-code/issues/9584
