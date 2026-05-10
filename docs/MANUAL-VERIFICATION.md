# Manual Verification Checklist

> Run before tagging each milestone. Items here cover what the automated suite cannot.

## Phase 3 — TUI

### M3.1 Dashboard

Prereqs: build the binary (`go build -o bin/cco ./cmd/app`).

- [ ] **Daemon-down empty state.** Delete `~/.claude-code-observer/db.sqlite` (or use a fresh `--home` dir). Run `./bin/cco`. Expect: dashboard renders with zeros and `⚠ NO DATA — IS \`cco serve\` RUNNING?` banner. No crash.
- [ ] **Live updates.** Start `./bin/cco serve` in one terminal. Run `./bin/cco` in another. In a third terminal, run `claude` and issue a prompt that triggers ≥1 tool call. The dashboard's TODAY block should update within ~2 s.
- [ ] **Stable when idle.** Stop using Claude Code. Dashboard remains stable, no flicker.
- [ ] **Numbers match SQL.** With the daemon running and some events in:
  ```bash
  sqlite3 ~/.claude-code-observer/db.sqlite \
    "SELECT printf('%.2f', SUM(cost_usd)) FROM sessions WHERE started_at >= unixepoch('now', 'start of day')*1e9"
  ```
  Compare to the dashboard's TODAY cost. Must match to 2 decimal places.
- [ ] **STALE pill on daemon kill.** Kill `cco serve`. Within ~30 s, dashboard footer pill flips to `STALE`. Restart `cco serve`; pill returns to `● LIVE`.
- [ ] **Quit.** `q` exits cleanly; `Ctrl-C` exits cleanly.
- [ ] **Refresh.** `r` triggers an immediate fetch (verifiable by changing data with `cco serve` running and pressing `r`).

### M3.2 Sessions list + Session detail

Prereqs: M3.1 prereqs + at least 3 sessions in the DB.

- [ ] **Open from dashboard.** With `cco serve` running, run `./bin/cco`. Press `s`. Expect: chrome title flips to `CCO  │  SESSIONS`. List shows newest sessions first.
- [ ] **Selection movement.** `j/k` or arrows move the `▶` marker. `g` jumps top, `G` jumps bottom.
- [ ] **Pagination.** With 51+ sessions, press `pgdn`. Expect: page 2 of 50 rows, header reads `page 2`. `pgup` returns to page 1 with cursor at 0.
- [ ] **Drill in.** Press `enter`. Expect: chrome title flips to `CCO  │  SESSION <id>…`. Bold rows are user prompts.
- [ ] **Enter on non-prompt.** Move selection to a `tool_result` row, press `enter`. Expect: nothing happens (no chrome change, no error).
- [ ] **Back preserves selection.** Press `b`. Expect: returns to Sessions list with the same row selected.
- [ ] **Numbers match SQL.**
  ```bash
  sqlite3 ~/.claude-code-observer/db.sqlite \
    "SELECT COUNT(*) FROM sessions ORDER BY started_at DESC LIMIT 50"
  ```
  Compare to first-page row count in the TUI.

### M3.3 Prompt detail

- [ ] **Drill from session detail.** From a Session Detail page, move to a bold (`user_prompt`) row, press `enter`. Expect: chrome title flips to `CCO  │  PROMPT <id>…`. Cost and Tokens panels render side by side.
- [ ] **Cost matches SQL.**
  ```bash
  sqlite3 ~/.claude-code-observer/db.sqlite \
    "SELECT cost_usd FROM prompts WHERE prompt_id = '<id>'"
  ```
  Compare to the cost panel value (4-decimal format).
- [ ] **Tool calls and api requests render.** API REQUESTS section lists model + cost; TOOL CALLS section lists tool name + duration; failed tool calls show `✗`.
- [ ] **Back walks the chain.** `b` returns to Session Detail; another `b` returns to Sessions list; another `b` returns to Dashboard.
- [ ] **Pruned prompt.** Drill into a session whose prompt has since been deleted (or test against a manually-DELETEd row): expect the body to render `prompt not found — it may have been pruned` and no crash.

## Phase 4 — Install Ergonomics

### M4.1: cco init

- [ ] In an empty temp dir, `cco init --force` creates `.claude/settings.json` containing all 7 owned keys.
- [ ] `OTEL_RESOURCE_ATTRIBUTES` value contains `project.name=<dirname>`.
- [ ] Re-running `cco init --force` is a no-op (file mtime updates but content is byte-identical via `diff`).
- [ ] In a dir with `.claude/settings.json` containing `{"model":"opus","env":{"MY_VAR":"42"}}`, `cco init --force` preserves both keys.
- [ ] In a dir where an owned key conflicts, plain `cco init` (no flags) prints the conflict, prompts, and respects y/N.
- [ ] `cco init --print` writes nothing to disk; renders valid JSON to stdout.
- [ ] With daemon stopped: output ends with `✗ daemon not reachable` and the hint lines.
- [ ] With daemon running: output ends with `✓ daemon reachable at 127.0.0.1:4317`.

### M4.2: Service files (macOS)

- [ ] After `launchctl bootstrap`, `launchctl print gui/$(id -u)/com.claude-code-observer` shows the service running.
- [ ] Trigger a Claude Code prompt — TUI dashboard updates within 20–30 s.
- [ ] Logout / login — daemon still running.
- [ ] `kill -9 <pid>` — launchd restarts it within 10 s.
- [ ] `launchctl kill TERM gui/$(id -u)/com.claude-code-observer` followed by `launchctl bootout` — clean shutdown, file removed, no zombie.

### M4.2: Service files (Linux)

- [ ] After `systemctl --user enable --now`, `systemctl --user status claude-code-observer` shows `active (running)`.
- [ ] Trigger a Claude Code prompt — TUI dashboard updates within 20–30 s.
- [ ] User session restart — daemon still running.
- [ ] `kill -9 <pid>` — systemd restarts it within 10 s (RestartSec=5).
- [ ] Disable + remove — clean shutdown, file removed.

### M4.2: README dry-run

- [ ] On a fresh user account (or a clean VM), follow README §Install end-to-end without help. Reach the dashboard with at least one populated session in under 5 minutes.
