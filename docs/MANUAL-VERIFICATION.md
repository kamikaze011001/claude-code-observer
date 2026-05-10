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
