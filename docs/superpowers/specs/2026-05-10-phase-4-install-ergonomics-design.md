# Phase 4 — Install Ergonomics Design

> Date: 2026-05-10
> Roadmap: [docs/ROADMAP.md](../../ROADMAP.md) — Phase 4 (M4.1, M4.2)
> Status: Approved — ready for implementation plan

## Goal

Lower friction for first-time setup and unattended operation:

1. `cco init` configures a project to export OTLP to `localhost:4317` in one command.
2. `scripts/com.claude-code-observer.plist` and `scripts/claude-code-observer.service` let the daemon run unattended on macOS / Linux.
3. README walks a new user from `git clone` to "open `cco` and see your prompts" in under 5 minutes.

## Non-goals

- Global / user-level install (`~/.claude/settings.json`). Project-only in v1; deferred to FUTURE.md.
- Homebrew / apt packages. v1 ships source-build only.
- Auto-start of `cco serve` from `cco init`. Lifecycle belongs to launchd/systemd, not init.
- Health-check / `cco doctor` subcommand. Deferred.
- Log rotation. Single file in v1; documented as a known limitation in README.

## M4.1 — `cco init` setup wizard

### Module boundaries

```
internal/init                          pure file-mutation logic
   │  inputs:   pwd, existing settings.json (or absent), flags
   │  outputs:  rendered settings.json, conflict report, probe result
   ▼
cmd/app                                wires init module to flags + stdio + clock
```

`internal/init` has no dependencies on `receiver`, `service`, or `repository`. It has one boundary to the network (the `:4317` daemon probe) which is injected as an interface so tests can stub it.

### Owned key set

`cco init` writes (and prompts on conflict for) exactly these seven keys under the JSON `env` object:

| Key | Value |
|---|---|
| `CLAUDE_CODE_ENABLE_TELEMETRY` | `"1"` |
| `OTEL_METRICS_EXPORTER` | `"otlp"` |
| `OTEL_LOGS_EXPORTER` | `"otlp"` |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `"grpc"` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `"http://localhost:4317"` |
| `OTEL_RESOURCE_ATTRIBUTES` | merged sub-key `project.name=<basename($PWD)>` |
| `OTEL_METRIC_EXPORT_INTERVAL` | `"20000"` |

All other `env` keys and all non-`env` top-level keys (e.g. `model`, `theme`, `hooks`) are passed through untouched.

### Conflict resolution

For each owned key:

- **Absent** → write our value silently.
- **Present, equal** → no-op silently.
- **Present, different** → in interactive mode, print a one-line diff and prompt `[y/N]`. With `--force`, overwrite without prompt. With `--print`, render to stdout and skip writing entirely.

`OTEL_RESOURCE_ATTRIBUTES` is special-cased:

- Parse the existing string as a comma-separated `k=v` list.
- Set/replace only the `project.name` sub-entry. Preserve all other entries (e.g. `enduser.id`, `deployment.environment`).
- Re-serialize in stable order: `project.name` first, then the original entries in their original order.
- Conflict prompt fires only if `project.name` already exists with a *different* value.

### File creation

- `$PWD/.claude/` is created with mode `0755` if absent.
- `$PWD/.claude/settings.json` is created if absent.
- Existing files: parsed as JSON, top-level key order preserved, only `env` mutated. Output is 2-space-indented with a trailing newline.
- Re-running `cco init` on a project where every owned key is already present is a no-op (no diff, no file mtime change beyond what JSON re-serialization may produce — see open question below).

### Daemon probe

After the file is written (or after `--print` renders), `cco init` opens a gRPC dial to `localhost:4317` with a 500ms timeout and prints one of:

- `✓ daemon reachable at localhost:4317`
- `✗ daemon not reachable at localhost:4317`
  followed by a hint: `→ start it with: cco serve` (and a one-liner for the launchd/systemd path documented in M4.2).

The probe is a successful `grpc.Dial` followed by an `ExportLogs` call with an empty request. Failure does **not** make `cco init` exit non-zero — the config write succeeded; daemon state is informational.

### Flags

- `--force`: skip all conflict prompts; owned keys overwrite existing values.
- `--print`: render the merged `settings.json` to stdout, do not write or probe.

### CLI output (interactive, happy path)

```
✓ wrote .claude/settings.json (7 keys)
✓ project.name = claude-code-observer
✓ daemon reachable at localhost:4317
```

### CLI output (daemon down)

```
✓ wrote .claude/settings.json (7 keys)
✓ project.name = claude-code-observer
✗ daemon not reachable at localhost:4317
  → start it with: cco serve
    or install as a service: see README §Install
```

### Test plan

Table-driven tests against `internal/init`:

- fresh dir (no `.claude/`) → dir created, file created, all 7 keys present
- `.claude/` exists, `settings.json` absent → file created
- `settings.json` exists, no `env` block → `env` added; other top-level keys untouched
- `settings.json` exists, `env` partial → missing owned keys added; existing non-owned `env` keys untouched
- `settings.json` exists, owned key conflicts → prompts in interactive mode; `--force` overwrites
- `OTEL_RESOURCE_ATTRIBUTES` merge: existing `enduser.id=foo` preserved, `project.name` added/updated
- `--print` renders to stdout, no FS write, no probe
- daemon probe: reachable, unreachable (with stub server in test)
- idempotent: running twice on a configured dir is a no-op

Coverage target: ≥ 90% on `internal/init/` (matches roadmap).

## M4.2 — Service files + README install section

### Files shipped

`scripts/com.claude-code-observer.plist` (macOS launchd) and `scripts/claude-code-observer.service` (Linux systemd user service).

Both invoke `$HOME/.claude-code-observer/bin/cco serve` (canonical install path documented in README), with logs written to `$HOME/.claude-code-observer/logs/cco.log`.

### launchd plist (macOS)

Key knobs:

- `Label`: `com.claude-code-observer`
- `ProgramArguments`: `[<install-prefix>/bin/cco, serve]`
- `RunAtLoad`: `true` — start on login.
- `KeepAlive`: `{ SuccessfulExit = false }` — restart on crash, stay stopped on clean exit.
- `StandardOutPath` / `StandardErrorPath`: `<HOME>/.claude-code-observer/logs/cco.log`
- `WorkingDirectory`: `<HOME>/.claude-code-observer`

User installs with `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.claude-code-observer.plist` (per roadmap demo).

### systemd unit (Linux user service)

```
[Unit]
Description=Claude Code Observer daemon
After=network.target

[Service]
ExecStart=%h/.claude-code-observer/bin/cco serve
Restart=on-failure
RestartSec=5
StandardOutput=append:%h/.claude-code-observer/logs/cco.log
StandardError=append:%h/.claude-code-observer/logs/cco.log
WorkingDirectory=%h/.claude-code-observer

[Install]
WantedBy=default.target
```

User installs with `systemctl --user enable --now claude-code-observer` (per roadmap demo). `WantedBy=default.target` means it starts at user-session start once enabled.

### Log file

- Single path: `$HOME/.claude-code-observer/logs/cco.log`.
- Both service files redirect stdout + stderr there.
- The daemon does not rotate logs in v1. README has a one-line note suggesting `logrotate` (Linux) or manual truncation; rotation is in FUTURE.md.

### README install section structure

Walks a new user end-to-end:

1. **Build** — `git clone … && cd claude-code-observer && go build -o ~/.claude-code-observer/bin/cco ./cmd/app`
2. **Install service** — macOS: copy plist + `launchctl bootstrap`. Linux: copy unit + `systemctl --user enable --now`.
3. **Configure project** — `cd path/to/your/project && cco init`
4. **Use Claude Code** — run any `claude` invocation; events flow.
5. **Open dashboard** — `cco`.

Each step ≤ 3 commands. Total wall-clock target: under 5 minutes on a clean machine, validated by the M4.2 demo against a fresh user account.

### Test plan (M4.2)

- `shellcheck` clean on any install scripts (none planned beyond the unit/plist files themselves; if a helper script lands, lint it).
- Manual install verification recorded in `docs/MANUAL-VERIFICATION.md`:
  - macOS: install plist, run Claude Code, see dashboard update; logout/login, daemon still running.
  - Linux: install unit, run Claude Code, see dashboard update; restart user session, daemon still running.
- README dry-run: a fresh user account follows the install section without help and reaches the dashboard.

## Failure semantics

- `cco init` errors only when it cannot write the file (FS permission, disk full, malformed existing JSON). Daemon-down is **not** an init error.
- Service files: a missing binary or unwritable log path makes the service fail-fast on launch — both launchd and systemd will surface this in their respective logs (`log show --predicate 'subsystem == "com.claude-code-observer"'` / `journalctl --user -u claude-code-observer`). README points to these commands in a Troubleshooting subsection.

## Open questions

- **JSON serialization stability**: Go's `encoding/json` does not guarantee key order matches input. To meet the "re-running cco init is a no-op" criterion, we either (a) use a JSON library that preserves order, (b) write a small ordered-map type, or (c) document that key order may shuffle on first run and stabilize after. **Decision deferred to implementation plan**; recommend (b) — small targeted struct since we only need to preserve top-level + `env` order.
- **Install prefix**: README assumes `$HOME/.claude-code-observer/bin/cco`. If users prefer `/usr/local/bin/cco`, the plist/unit need a templating step. v1 hardcodes `$HOME/.claude-code-observer/bin/cco`; document the override path users can edit.

## What "Phase 4 done" looks like

- `cco init` works on a fresh project in one command, writes 7 keys, probes the daemon, prints status.
- `scripts/com.claude-code-observer.plist` + `scripts/claude-code-observer.service` install cleanly on macOS / Linux respectively, restart on crash, start at login.
- README install section gets a new user from clone to dashboard in <5 minutes.
- Coverage ≥ 90% on `internal/init/`. Service files lint clean.
