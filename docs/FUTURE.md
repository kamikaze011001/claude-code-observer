# Future Features

> Captured during design grilling on 2026-05-10. None of these are committed — this is a parking lot of ideas worth revisiting once v1 ships and we have real usage data.

Items are roughly ordered by "useful + cheap" first.

## Near-term wins (small effort, high signal)

### Live tail view (TUI screen)
A new TUI page that streams events as they arrive — like `tail -f` for Claude Code. Useful while Claude is mid-task: watch the tool-call cadence, spot stalls, see which subagent is running. Same data we already store, just rendered differently.

### Setup wizard — `claude-code-observer init`
A subcommand that auto-writes `.claude/settings.json` for the current directory: enables telemetry, sets endpoint, sets `project.name=$(basename $PWD)`. Removes the only manual step in onboarding.

### Sample launchd plist (macOS) and systemd unit (Linux)
Ship templates so `serve` auto-starts at login. The daemon runs unattended; we should make that the default install path.

### Daily cost notification
When the daemon detects that `today's cost > $threshold`, fire a macOS notification (`osascript`) or a beep. Threshold configurable via env or a config file. Useful for keeping personal API spend in check.

### Top-N expensive prompts view
Already a one-liner SQL — surface it as its own TUI page. "Show me the 10 prompts I should have written more carefully this week."

### Per-project filter on Dashboard
Press `p` on Dashboard to cycle through projects. Useful when you have several active projects and want to know which one is burning the most.

### Copy-to-clipboard shortcuts in TUI
On Prompt detail, `c` copies the Bash command under cursor. On a tool_result with `tool_input` (file path), `c` copies the path. Cheap, friendly.

## Mid-term (moderate effort)

### Subagent waterfall view
On Prompt detail, render subagent API requests as a horizontal flame graph: parent prompt's main request, then nested subagent requests with their durations. Shows where time is actually spent within a complex prompt. Needs careful event-ordering logic; we have all the timestamps already.

### Session annotations / tags
Let the user attach a label to a session ("auth refactor", "investigating prod incident"). Stored in a `session_tags` table. Surfaces in Sessions list. Encourages going back and learning from past work.

### Anomaly detection
"This prompt cost 8× the median for this project." A simple rolling z-score per project, computed at rollup time. Surface a warning indicator in Sessions list.

### Prompt-text full-text search
When `OTEL_LOG_USER_PROMPTS=1`, build a SQLite FTS5 index over the prompt text. Lets the user find "that prompt where I asked about X". Off by default; opt-in.

### Cost-trend charts in Dashboard
ASCII sparklines (lipgloss compatible) for daily cost over the last 30 days. Same data, just visualized.

### Effort/model distribution
A pie or bar chart showing how often each `model` and `effort` value was used. Lets you see whether you're over-relying on Opus when Sonnet would do.

### Session diff stats
Per session, show net lines added/removed across all `tool_result` events for `Edit` and `Write`. Already partially exposed via the `claude_code.lines_of_code` metric, but joining it to a session needs cardinality-control flags off.

### Export a session as Markdown
`claude-code-observer export <session-id> > session.md` — a human-readable rendering: prompts, tool calls, costs. Useful for retros and incident postmortems.

### Import / replay from a JSON dump
Inverse of export. Useful for sharing an interesting session with someone else (sanitized) or for testing our own rollup logic against a frozen sample.

## Long-term / speculative

### Trace support (when traces leave beta)
`CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1` is in beta now. Once stable, add a Traces receiver and use spans to render true latency timelines (currently we only have event timestamps, not span hierarchies).

### Multi-machine sync
Sync the SQLite via a chosen backend — S3, Tailscale-shared volume, git-tracked file with a small custom merge driver. Lets the user see laptop + desktop usage in one TUI. Significant scope creep; only worth it if we hit the use case.

### Web UI alongside TUI
Reuse the data layer behind a chi HTTP server (chi is already in `go.mod`). Same SQLite, browser frontend, real-time SSE. Pairs with multi-machine sync.

### `/cost` slash command for Claude Code itself
A small Claude Code skill that shells out to `claude-code-observer today --json` and formats the result inline. So you can ask Claude Code "how much have I spent today" and get an answer.

### Budget guardrails
Configurable hard caps: "no Opus prompts over $X each" or "stop using fast mode after $Y/day". Probably implemented as a Claude Code hook that calls `claude-code-observer budget-check` in PreToolUse. Out of scope of the observer itself but a natural integration point.

### Comparison view (two sessions side-by-side)
Pick two sessions of similar work; render per-prompt cost/duration in a diff layout. Useful for "did this prompt-engineering change actually save tokens?"

### Privacy redaction filters
When `OTEL_LOG_USER_PROMPTS=1` is on, run prompt text through a redactor (regex-based: emails, API keys, file paths) before persisting. Trades fidelity for safety.

### MCP server latency derivation
We can't measure MCP server latency directly (Claude Code doesn't instrument it), but we can derive lower bounds from `tool_result.duration` proxied via consecutive event timestamps. Worth it only if MCP perf becomes a question.

### Plan/skill correlation
Cross-reference sessions with a project's `.claude/skills/` and `.claude/plans/` to label "this session ran /loop" or "this session had a Plan" and aggregate cost per skill. Needs hook integration to be reliable.

### Compaction visibility
The undocumented `claude_code.compact` event — if/when documented — gives us tokens-before/tokens-after-compaction. Render as a "context recovered" stat per prompt.

### Anonymous opt-in usage telemetry (about us, not them)
If we ever publish this tool, an opt-in ping to learn what versions are running. **Only if we publish.** For personal use this is unnecessary.

## Won't-do (explicitly out of scope)

These have come up but are deliberately rejected:

- **Multi-tenant / SaaS hosting.** This is a local-only tool. Multi-tenant changes the auth, schema, and operational story dramatically — different product.
- **Direct integration with Claude Code's source.** We consume the public OTel surface; we do not patch or fork Claude Code.
- **Real-time alerting integrations (PagerDuty, Slack).** This is a personal observability tool, not an oncall system.
- **Anthropic-side data** — we cannot see anything that doesn't come over OTLP. No API key inspection, no Anthropic Console scraping.
