# claude-code-observer

Local observability for Claude Code via OTLP. A single Go binary ingests OTLP/gRPC telemetry into a SQLite store and renders it in a TUI — costs, prompts, tool calls, errors — all on `localhost`, no cloud.

**Stack:** Go 1.25 · gRPC · SQLite (modernc) · Bubble Tea TUI

## Install

Five steps from clone to dashboard.

### 1. Build

```bash
git clone https://github.com/kamikaze011001/claude-code-observer.git
cd claude-code-observer
mkdir -p ~/.claude-code-observer/bin ~/.claude-code-observer/logs
go build -o ~/.claude-code-observer/bin/cco ./cmd/app
```

Add `~/.claude-code-observer/bin` to your `PATH` if you want `cco` invocable from anywhere.

### 2. Install the service

**macOS (launchd):**

```bash
sed "s|__HOME__|$HOME|g" scripts/com.claude-code-observer.plist \
  > ~/Library/LaunchAgents/com.claude-code-observer.plist
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.claude-code-observer.plist
launchctl kickstart gui/$(id -u)/com.claude-code-observer
```

**Linux (systemd user unit):**

```bash
mkdir -p ~/.config/systemd/user
cp scripts/claude-code-observer.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now claude-code-observer
```

Verify: `cco` (no args, opens TUI) — the dashboard should load without errors. The daemon listens on `127.0.0.1:4317` and writes logs to `~/.claude-code-observer/logs/cco.log`.

### 3. Configure a project

In any project where you use Claude Code:

```bash
cd path/to/your/project
cco init
```

This writes seven OTel env vars under `env` in `.claude/settings.json` and probes the daemon. Existing keys (your `model`, `theme`, `hooks`, etc.) are preserved.

### 4. Use Claude Code

Run any `claude` command in the configured project. Each prompt, API call, tool invocation, and error flows into SQLite within ~20 seconds.

### 5. Open the dashboard

```bash
cco
```

You should see today's cost, prompt count, and the most expensive sessions. Drill in with `Enter`, back out with `b`.

## Troubleshooting

- **macOS daemon logs:** `tail -f ~/.claude-code-observer/logs/cco.log` or `log show --predicate 'subsystem == "com.claude-code-observer"' --last 10m`
- **Linux daemon logs:** `journalctl --user -u claude-code-observer -f` (or the same `cco.log` file)
- **`cco init` says daemon not reachable:** the launchd/systemd unit didn't start. Check the service status (`launchctl print gui/$(id -u)/com.claude-code-observer` or `systemctl --user status claude-code-observer`).
- **Log rotation:** v1 does not rotate `cco.log`. On Linux, drop a `logrotate` config; on macOS, truncate manually.

## Stopping / Uninstall

**macOS:** `launchctl bootout gui/$(id -u)/com.claude-code-observer && rm ~/Library/LaunchAgents/com.claude-code-observer.plist`

**Linux:** `systemctl --user disable --now claude-code-observer && rm ~/.config/systemd/user/claude-code-observer.service`

Data lives in `~/.claude-code-observer/`; remove the directory to wipe state.

## Architecture & Decisions

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — layer boundaries
- [docs/DATA-MODELS.md](docs/DATA-MODELS.md) — schema
- [docs/decisions/](docs/decisions/) — ADRs
- [docs/CLAUDE-CODE-OTEL.md](docs/CLAUDE-CODE-OTEL.md) — what Claude Code emits
- [docs/ROADMAP.md](docs/ROADMAP.md) — milestone tracker

## License

MIT.
