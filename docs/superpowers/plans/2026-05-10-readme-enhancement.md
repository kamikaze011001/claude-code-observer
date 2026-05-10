# README Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current install-manual `README.md` (93 lines) with a hybrid discovery + onboarding README modeled on top-starred Go CLI projects, following the section order and content rules from the spec at `docs/superpowers/specs/2026-05-10-readme-enhancement-design.md`.

**Architecture:** Single-file rewrite of `README.md`. Build the new file section-by-section in order so each commit is a focused, reviewable unit. No code changes; verification is `markdownlint`, GitHub preview render, relative-link resolution, and a line-count check.

**Tech Stack:** Markdown only. Optional: `markdownlint-cli2` (npm) for linting. Optional follow-up: `charmbracelet/vhs` for a demo GIF (separate task, not blocking).

---

## File Structure

- **Modify:** `README.md` — full rewrite, section-by-section across multiple commits.
- **Create:** `.markdownlint.json` — linter config (only if absent).
- **Create:** `scripts/demo.tape` — VHS tape file (Phase B follow-up task).
- **Reference (no change):** `docs/superpowers/specs/2026-05-10-readme-enhancement-design.md` — the spec being implemented.

The new README's section order, mirroring the spec:

1. Title + tagline
2. Badges (3)
3. Hero asset (ASCII placeholder in Phase A)
4. Why (3 bullets)
5. Features (5–7 bullets)
6. Install (macOS / Linux subsections)
7. Usage (command table)
8. Configuration (the 7 OTel keys + links)
9. Architecture (1 paragraph + links)
10. Troubleshooting
11. Stopping / Uninstall
12. Contributing
13. License

---

### Task 1: Reset README to skeleton with title, tagline, badges

**Files:**
- Modify: `README.md` (lines 1–93 → new skeleton)

The existing README will be archived in git history. We start with a complete skeleton and fill sections in subsequent tasks. Each later task replaces a placeholder line with real content.

- [ ] **Step 1: Replace `README.md` with the skeleton**

Overwrite `README.md` with exactly this content:

````markdown
# claude-code-observer

> Local telemetry dashboard for Claude Code. See every prompt, tool call, and dollar — in your terminal, never the cloud.

[![Go Report Card](https://goreportcard.com/badge/github.com/kamikaze011001/claude-code-observer)](https://goreportcard.com/report/github.com/kamikaze011001/claude-code-observer)
![Go](https://img.shields.io/badge/Go-1.25-blue)
![License](https://img.shields.io/badge/License-MIT-green)

<!-- HERO -->

<!-- WHY -->

<!-- FEATURES -->

<!-- INSTALL -->

<!-- USAGE -->

<!-- CONFIGURATION -->

<!-- ARCHITECTURE -->

<!-- TROUBLESHOOTING -->

<!-- UNINSTALL -->

<!-- CONTRIBUTING -->

<!-- LICENSE -->
````

- [ ] **Step 2: Verify the file renders**

Run: `head -20 README.md`
Expected: title, tagline, three badge lines, then the HTML comment placeholders.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): reset to skeleton with title, tagline, badges"
```

---

### Task 2: Hero — ASCII TUI placeholder

**Files:**
- Modify: `README.md` — replace `<!-- HERO -->`

- [ ] **Step 1: Replace the HERO placeholder with an ASCII screenshot**

Find the line `<!-- HERO -->` in `README.md` and replace it with:

````markdown
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
````

- [ ] **Step 2: Verify**

Run: `grep -n "claude-code-observer ────" README.md`
Expected: one match — the top of the ASCII box.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): add ASCII TUI hero placeholder"
```

---

### Task 3: Why section

**Files:**
- Modify: `README.md` — replace `<!-- WHY -->`

- [ ] **Step 1: Replace the WHY placeholder**

Find `<!-- WHY -->` and replace it with:

```markdown
## Why

- Claude Code emits OTLP telemetry but has no built-in dashboard — costs and tool usage stay invisible until your bill arrives.
- One Go binary: ingests OTLP/gRPC on `localhost:4317`, writes SQLite, renders a TUI. No cloud, no account, no daemon you didn't install.
- Per-project setup is one command (`cco init`) — drops seven OTel env vars into `.claude/settings.json` and probes the daemon.
```

- [ ] **Step 2: Verify**

Run: `grep -c "^## Why$" README.md`
Expected: `1`

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): add Why section"
```

---

### Task 4: Features section

**Files:**
- Modify: `README.md` — replace `<!-- FEATURES -->`

- [ ] **Step 1: Replace the FEATURES placeholder**

Find `<!-- FEATURES -->` and replace it with:

```markdown
## Features

- Real-time ingestion of Claude Code OTLP logs and metrics over gRPC.
- Local SQLite store — no network egress, full data ownership.
- Bubble Tea TUI — session list, cost breakdown, tool-call detail, error log.
- Per-project tagging via `OTEL_RESOURCE_ATTRIBUTES`'s `project.name`.
- Single static binary — `go build` and you're done.
- Unattended daemon — launchd plist + systemd user unit shipped.
- Idempotent project setup — `cco init` is safe to re-run.
```

- [ ] **Step 2: Verify**

Run: `grep -c "^## Features$" README.md`
Expected: `1`

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): add Features section"
```

---

### Task 5: Install section (macOS + Linux subsections)

**Files:**
- Modify: `README.md` — replace `<!-- INSTALL -->`

The current README's 5-step install is preserved verbatim in content but compressed into 3 commands per OS. Verification: a fresh contributor can copy-paste each block top-to-bottom.

- [ ] **Step 1: Replace the INSTALL placeholder**

Find `<!-- INSTALL -->` and replace it with:

````markdown
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
````

- [ ] **Step 2: Verify**

Run: `grep -c "^## Install$" README.md && grep -c "<details" README.md`
Expected: `1` then `2`

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): compress install into OS-tabbed subsections"
```

---

### Task 6: Usage section (command table)

**Files:**
- Modify: `README.md` — replace `<!-- USAGE -->`

- [ ] **Step 1: Replace the USAGE placeholder**

Find `<!-- USAGE -->` and replace it with:

```markdown
## Usage

| Command | Purpose |
|---|---|
| `cco` | Open the TUI dashboard (default) |
| `cco init` | Wire current project's `.claude/settings.json` to the local daemon |
| `cco serve` | Run the OTLP ingest daemon in the foreground (launchd/systemd run this) |
| `cco rebuild` | Rebuild aggregates from raw events |
| `cco version` | Print version and commit |

All commands accept `--home <dir>` (overrides `$CCO_HOME`, default `~/.claude-code-observer`) and `--log-level debug|info|warn|error`.
```

- [ ] **Step 2: Verify the listed commands actually exist**

Run: `grep -E "newServeCmd|newTUICmd|newInitCmd|newRebuildCmd|newVersionCmd" cmd/app/main.go`
Expected: all five constructors present (they are, per `cmd/app/main.go:91-97`).

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): add Usage command table"
```

---

### Task 7: Configuration section (the 7 OTel keys)

**Files:**
- Modify: `README.md` — replace `<!-- CONFIGURATION -->`

- [ ] **Step 1: Replace the CONFIGURATION placeholder**

Find `<!-- CONFIGURATION -->` and replace it with:

```markdown
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
```

- [ ] **Step 2: Verify the linked files exist**

Run: `ls docs/CLAUDE-CODE-OTEL.md docs/superpowers/specs/2026-05-10-phase-4-install-ergonomics-design.md`
Expected: both paths print without error.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): add Configuration section listing the 7 OTel keys"
```

---

### Task 8: Architecture paragraph

**Files:**
- Modify: `README.md` — replace `<!-- ARCHITECTURE -->`

- [ ] **Step 1: Replace the ARCHITECTURE placeholder**

Find `<!-- ARCHITECTURE -->` and replace it with:

```markdown
## Architecture

Three components: a gRPC OTLP receiver on `127.0.0.1:4317` (`cmd/app/serve.go`), a SQLite store under `~/.claude-code-observer/cco.sqlite`, and a Bubble Tea TUI. The receiver writes raw events; an aggregation pass (`cco rebuild` or the live aggregator) produces session, cost, and tool-call rollups consumed by the TUI.

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — layer boundaries
- [docs/DATA-MODELS.md](docs/DATA-MODELS.md) — schema
- [docs/decisions/](docs/decisions/) — ADRs
- [docs/ROADMAP.md](docs/ROADMAP.md) — milestone tracker
```

- [ ] **Step 2: Verify the linked paths exist**

Run: `ls docs/ARCHITECTURE.md docs/DATA-MODELS.md docs/decisions docs/ROADMAP.md`
Expected: all four print without error. If any is missing, remove that bullet rather than fabricating the file.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs(readme): add Architecture paragraph and doc links"
```

---

### Task 9: Troubleshooting + Uninstall (port from current README)

**Files:**
- Modify: `README.md` — replace `<!-- TROUBLESHOOTING -->` and `<!-- UNINSTALL -->`

These two sections are kept verbatim from the original README content; only their placement changes.

- [ ] **Step 1: Replace the TROUBLESHOOTING placeholder**

Find `<!-- TROUBLESHOOTING -->` and replace it with:

```markdown
## Troubleshooting

- **macOS daemon logs:** `tail -f ~/.claude-code-observer/logs/cco.log` or `log show --predicate 'subsystem == "com.claude-code-observer"' --last 10m`
- **Linux daemon logs:** `journalctl --user -u claude-code-observer -f` (or the same `cco.log` file)
- **`cco init` says daemon not reachable:** the launchd/systemd unit didn't start. Check service status with `launchctl print gui/$(id -u)/com.claude-code-observer` or `systemctl --user status claude-code-observer`.
- **Log rotation:** v1 does not rotate `cco.log`. On Linux, drop a `logrotate` config; on macOS, truncate manually.
```

- [ ] **Step 2: Replace the UNINSTALL placeholder**

Find `<!-- UNINSTALL -->` and replace it with:

```markdown
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
```

- [ ] **Step 3: Verify both headings present**

Run: `grep -E "^## (Troubleshooting|Stopping)" README.md`
Expected: two matches.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs(readme): port troubleshooting and uninstall sections"
```

---

### Task 10: Contributing + License

**Files:**
- Modify: `README.md` — replace `<!-- CONTRIBUTING -->` and `<!-- LICENSE -->`

- [ ] **Step 1: Replace the CONTRIBUTING placeholder**

Find `<!-- CONTRIBUTING -->` and replace it with:

```markdown
## Contributing

- Current milestone: [docs/ROADMAP.md](docs/ROADMAP.md)
- Architecture decisions: [docs/decisions/](docs/decisions/)
- Code conventions: [CLAUDE.md](CLAUDE.md)

Run `go vet ./... && go test ./... && go build -o bin/cco ./cmd/app` before opening a PR.
```

- [ ] **Step 2: Replace the LICENSE placeholder**

Find `<!-- LICENSE -->` and replace it with:

```markdown
## License

MIT.
```

- [ ] **Step 3: Verify all placeholders are gone**

Run: `grep -c "<!-- " README.md`
Expected: `0`

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs(readme): add Contributing and License sections"
```

---

### Task 11: Lint, link-check, length budget

**Files:**
- Create (if absent): `.markdownlint.json`
- Modify: `README.md` (only if lint flags issues)

- [ ] **Step 1: Create the lint config if it doesn't exist**

Run: `test -f .markdownlint.json && echo "exists" || echo "missing"`

If `missing`, write this file to `.markdownlint.json`:

```json
{
  "default": true,
  "MD013": false,
  "MD033": { "allowed_elements": ["details", "summary"] },
  "MD041": false
}
```

Rationale: `MD013` (line length) is disabled — Markdown prose can flow naturally. `MD033` allows the `<details>`/`<summary>` tags used in the Install section. `MD041` (first line must be H1) is disabled because GitHub renders fine without it and we may add front-matter later.

- [ ] **Step 2: Run markdownlint**

Run: `npx --yes markdownlint-cli2 README.md`
Expected: exit code `0`, no findings. If findings, fix the smallest reasonable change in `README.md` and re-run until clean.

- [ ] **Step 3: Verify all relative links resolve**

Run this one-liner to extract relative Markdown links and check each path exists:

```bash
grep -oE '\]\([^)]+\)' README.md \
  | sed -E 's/^\]\(([^)]+)\)$/\1/' \
  | grep -vE '^https?://' \
  | grep -vE '^#' \
  | while read -r p; do test -e "${p%%#*}" || echo "MISSING: $p"; done
```

Expected: no `MISSING:` lines printed.

- [ ] **Step 4: Verify line-count budget**

Run: `wc -l README.md`
Expected: between `180` and `230` lines (per spec). If outside the band, adjust prose density before committing — but a 10-line overshoot is acceptable; rewriting Why/Features to hit a number is not.

- [ ] **Step 5: Commit**

```bash
git add .markdownlint.json README.md
git commit -m "docs(readme): add markdownlint config and verify links/length"
```

---

### Task 12: VHS demo tape (deferred follow-up)

**Files:**
- Create: `scripts/demo.tape`

This task scaffolds the `vhs` tape file but does **not** record the GIF. Recording requires `vhs` installed locally and an interactive run; capture that as a manual follow-up note in the PR description rather than executing it in this plan.

- [ ] **Step 1: Create `scripts/demo.tape`**

Write this file to `scripts/demo.tape`:

```tape
# VHS tape for claude-code-observer demo GIF
# Record with: vhs scripts/demo.tape
# Output: docs/demo.gif

Output docs/demo.gif

Set FontSize 14
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set Padding 20

Type "cco init"
Enter
Sleep 2s

Type "claude --print 'list three files in this repo'"
Enter
Sleep 6s

Type "cco"
Enter
Sleep 3s

# Drill into top session
Down
Enter
Sleep 3s

# Back out
Type "b"
Sleep 1s

# Quit
Type "q"
Sleep 500ms
```

- [ ] **Step 2: Verify the file is well-formed**

Run: `head -1 scripts/demo.tape`
Expected: `# VHS tape for claude-code-observer demo GIF`

- [ ] **Step 3: Add a follow-up note to the PR description (manual)**

In the PR body, add this line under "Follow-ups":

> Record `docs/demo.gif` with `vhs scripts/demo.tape` and replace the ASCII placeholder in `README.md` § Hero.

This is documentation, not an executable step — no command to run.

- [ ] **Step 4: Commit**

```bash
git add scripts/demo.tape
git commit -m "docs(readme): scaffold VHS tape for follow-up demo GIF"
```

---

## Self-Review

**Spec coverage:**

- Spec § Section order → Tasks 1–10 cover all 13 sections in order ✓
- Spec § Tagline → Task 1 ✓
- Spec § Badges (3 specific URLs) → Task 1 ✓
- Spec § Hero asset Phase A (ASCII placeholder) → Task 2 ✓
- Spec § Hero asset Phase B (VHS GIF deferred) → Task 12 ✓
- Spec § Why (3 bullets) → Task 3 ✓
- Spec § Features (5–7 bullets) → Task 4 (7 bullets) ✓
- Spec § Install (macOS/Linux tabs, ≤40 lines) → Task 5 ✓
- Spec § Usage table → Task 6 ✓
- Spec § Configuration (7 keys, link to OTEL doc + spec) → Task 7 ✓
- Spec § Architecture paragraph + links → Task 8 ✓
- Spec § Troubleshooting / Uninstall (verbatim from current) → Task 9 ✓
- Spec § Contributing (3 links) + License → Task 10 ✓
- Spec § Test plan: markdownlint clean → Task 11 Step 2 ✓
- Spec § Test plan: relative links resolve → Task 11 Step 3 ✓
- Spec § Length budget 180–230 → Task 11 Step 4 ✓
- Spec § Test plan: GitHub preview render → manual; covered implicitly by markdownlint + reviewer eyeball during PR.

**Placeholder scan:** No "TBD" / "implement later" in any task. All code blocks contain final content.

**Type consistency:** N/A (no code types). Section heading names are consistent across the skeleton (Task 1) and the section tasks (Tasks 3–10): `Why`, `Features`, `Install`, `Usage`, `Configuration`, `Architecture`, `Troubleshooting`, `Stopping / Uninstall`, `Contributing`, `License`. The skeleton's HTML comments (`<!-- WHY -->`, etc.) match the placeholders each task replaces.

One minor consistency note: Task 11 Step 3's link checker won't flag broken anchor-only links (e.g., `#install`). The README has no anchor-only links in any task body, so this is moot.
