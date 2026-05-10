# claude-code-observer

> **claude-code-observer** — Local telemetry dashboard for Claude Code. See every prompt, tool call, and dollar — in your terminal, never the cloud.

[![Go Report Card](https://goreportcard.com/badge/github.com/kamikaze011001/claude-code-observer)](https://goreportcard.com/report/github.com/kamikaze011001/claude-code-observer)
![Go](https://img.shields.io/badge/Go-1.25-blue)
![License](https://img.shields.io/badge/License-MIT-green)

```
┌─ claude-code-observer ────────────────────────────────────────────────┐
│ Today    $4.27   142 prompts    1,284 tool calls    3 errors          │
├───────────────────────────────────────────────────────────────────────┤
│ Sessions                                                              │
│   ▸ 14:02  refactor auth module          $1.84   38 prompts           │
│   ▸ 11:47  fix flaky integration test    $0.92   21 prompts           │
│   ▸ 09:15  draft phase-4 plan            $1.51   83 prompts           │
├───────────────────────────────────────────────────────────────────────┤
│ Top tools    Read 412   Edit 287   Bash 198   Grep 134   Write 92     │
└───────────────────────────────────────────────────────────────────────┘
  ↵ open  b back  q quit
```

> _An animated demo GIF will replace this placeholder once recorded with [vhs](https://github.com/charmbracelet/vhs). See [Task 11](docs/superpowers/plans/2026-05-10-readme-enhancement.md) in the README plan._

## Why

- Claude Code emits OTLP telemetry but has no built-in dashboard — costs and tool usage stay invisible until your bill arrives.
- One Go binary: ingests OTLP/gRPC on `localhost:4317`, writes SQLite, renders a TUI. No cloud, no account, no daemon you didn't install.
- Per-project setup is one command (`cco init`) — drops seven OTel env vars into `.claude/settings.json` and probes the daemon.

## Features

- Real-time ingestion of Claude Code OTLP logs and metrics over gRPC.
- Local SQLite store — no network egress, full data ownership.
- Bubble Tea TUI — session list, cost breakdown, tool-call detail, error log.
- Per-project tagging via `OTEL_RESOURCE_ATTRIBUTES`'s `project.name`.
- Single static binary — `go build` and you're done.
- Unattended daemon — launchd plist + systemd user unit shipped.
- Idempotent project setup — `cco init` is safe to re-run.

## Install

**Stack:** Go 1.25 · gRPC · SQLite (modernc) · Bubble Tea TUI · cobra

### 1. Build

```bash
git clone https://github.com/kamikaze011001/claude-code-observer.git
cd claude-code-observer
mkdir -p ~/.claude-code-observer/bin ~/.claude-code-observer/logs
go build -o ~/.claude-code-observer/bin/cco ./cmd/app
```

Add `~/.claude-code-observer/bin` to your `PATH` to run `cco` from anywhere.

### 2. Install the daemon

<details open>
<summary><strong>macOS (launchd)</strong></summary>

```bash
sed "s|__HOME__|$HOME|g" scripts/com.claude-code-observer.plist \
  > ~/Library/LaunchAgents/com.claude-code-observer.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.claude-code-observer.plist
launchctl kickstart gui/$(id -u)/com.claude-code-observer
```

</details>

<details>
<summary><strong>Linux (systemd user unit)</strong></summary>

```bash
mkdir -p ~/.config/systemd/user
cp scripts/claude-code-observer.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now claude-code-observer
```

</details>

The daemon listens on `127.0.0.1:4317` and writes logs to `~/.claude-code-observer/logs/cco.log`.

### 3. Configure a project

```bash
cd path/to/your/project
cco init
```

`cco init` writes seven OTel env vars under `env` in `.claude/settings.json` and probes the daemon. Existing keys (`model`, `theme`, `hooks`, etc.) are preserved.

Run any `claude` command in the project — events flow within ~20 seconds — then open the dashboard:

```bash
cco
```

## Usage

| Command | Purpose |
|---|---|
| `cco` | Open the TUI dashboard (default) |
| `cco init` | Wire current project's `.claude/settings.json` to the local daemon |
| `cco serve` | Run the OTLP ingest daemon in the foreground (launchd/systemd run this) |
| `cco rebuild` | Rebuild aggregates from raw events |
| `cco version` | Print version and commit |

All commands accept `--home <dir>` (overrides `$CCO_HOME`, default `~/.claude-code-observer`) and `--log-level debug|info|warn|error`.

## Configuration

`cco init` writes these seven keys into `.claude/settings.json` under `env`:

| Key | Value |
|---|---|
| `CLAUDE_CODE_ENABLE_TELEMETRY` | `1` |
| `OTEL_METRICS_EXPORTER` | `otlp` |
| `OTEL_LOGS_EXPORTER` | `otlp` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4317` |
| `OTEL_RESOURCE_ATTRIBUTES` | merges `project.name=<basename($PWD)>` |
| `OTEL_METRIC_EXPORT_INTERVAL` | `20000` |

Run `cco init --print` to preview the rendered file without writing. Run `cco init --force` to overwrite conflicting keys without prompting.

See [docs/CLAUDE-CODE-OTEL.md](docs/CLAUDE-CODE-OTEL.md) for what Claude Code emits, and the [Phase 4 install ergonomics spec](docs/superpowers/specs/2026-05-10-phase-4-install-ergonomics-design.md) for the rationale of each key.

## Architecture

Three components: a gRPC OTLP receiver on `127.0.0.1:4317` (`cmd/app/serve.go`), a SQLite store under `~/.claude-code-observer/cco.sqlite`, and a Bubble Tea TUI. The receiver writes raw events; an aggregation pass (`cco rebuild` or the live aggregator) produces session, cost, and tool-call rollups consumed by the TUI.

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — layer boundaries
- [docs/DATA-MODELS.md](docs/DATA-MODELS.md) — schema
- [docs/decisions/](docs/decisions/) — ADRs
- [docs/ROADMAP.md](docs/ROADMAP.md) — milestone tracker

## Troubleshooting

- **macOS daemon logs:** `tail -f ~/.claude-code-observer/logs/cco.log` or `log show --predicate 'subsystem == "com.claude-code-observer"' --last 10m`
- **Linux daemon logs:** `journalctl --user -u claude-code-observer -f` (or the same `cco.log` file)
- **`cco init` says daemon not reachable:** the launchd/systemd unit didn't start. Check service status with `launchctl print gui/$(id -u)/com.claude-code-observer` or `systemctl --user status claude-code-observer`.
- **Log rotation:** v1 does not rotate `cco.log`. On Linux, drop a `logrotate` config; on macOS, truncate manually.

## Stopping / Uninstall

**macOS:**

```bash
launchctl bootout gui/$(id -u)/com.claude-code-observer
rm ~/Library/LaunchAgents/com.claude-code-observer.plist
```

**Linux:**

```bash
systemctl --user disable --now claude-code-observer
rm ~/.config/systemd/user/claude-code-observer.service
```

Data lives in `~/.claude-code-observer/`; remove the directory to wipe state.

## Contributing

- Current milestone: [docs/ROADMAP.md](docs/ROADMAP.md)
- Architecture decisions: [docs/decisions/](docs/decisions/)
- Code conventions: [CLAUDE.md](CLAUDE.md)

Run `go vet ./... && go test ./... && go build -o bin/cco ./cmd/app` before opening a PR.

## License

MIT.
