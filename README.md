# claude-code-observer

> Local telemetry dashboard for Claude Code. See every prompt, tool call, and dollar — in your terminal, never the cloud.

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

<!-- INSTALL -->

<!-- USAGE -->

<!-- CONFIGURATION -->

<!-- ARCHITECTURE -->

<!-- TROUBLESHOOTING -->

<!-- UNINSTALL -->

<!-- CONTRIBUTING -->

<!-- LICENSE -->
