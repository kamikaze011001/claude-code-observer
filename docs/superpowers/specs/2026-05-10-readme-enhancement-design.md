# README Enhancement Design

> Date: 2026-05-10
> Audience: GitHub discovery (drive stars) + new-user onboarding
> Status: Approved — ready for implementation plan

## Goal

Replace the current install-manual README (93 lines, utility-only) with a hybrid discovery + onboarding README modeled on the top-starred Go CLI projects (lazygit, glow, dive, fzf, gum). Lead with value and a visual; keep the proven 5-step install flow but compress and reposition it.

## Why this shape

Research across 8 high-star Go CLI READMEs (cli/cli, lazygit, glow, gum, fzf, cobra, ollama, dive) shows a recurring playbook:

1. Demo GIF or screenshot within the first 10 lines.
2. One-sentence tagline with personality.
3. "Why" / Features section *before* install — convince, then convert.
4. 3–4 meaningful badges, no more.
5. Install section is copy-paste first; OS variations second.

Anti-patterns to avoid: logo-only hero, burying the "why" below a long install block, exhaustive package-manager matrix in v1.

## Section order (canonical)

1. Title + tagline
2. Badge row
3. Hero asset (GIF or ASCII placeholder)
4. Why — 3 bullets
5. Features — 5–7 bullets
6. Install — macOS / Linux tabs, 3 commands each
7. Usage — `cco`, `cco init`, `cco serve`, `cco rebuild`
8. Configuration — the 7 OTel keys `cco init` writes
9. Architecture — one paragraph + link to `docs/ARCHITECTURE.md`
10. Troubleshooting (keep current content)
11. Stopping / Uninstall (keep current content)
12. Contributing — short pointer to ROADMAP, decisions/, CLAUDE.md
13. License

## Tagline

> **claude-code-observer** — Local telemetry dashboard for Claude Code. See every prompt, tool call, and dollar — in your terminal, never the cloud.

## Badges (exactly three)

- Go Report Card — `https://goreportcard.com/badge/github.com/kamikaze011001/claude-code-observer`
- Go version — static badge: `Go-1.25-blue`
- License — static badge: `License-MIT-green`

No CI badge until a public CI workflow exists. No release badge until a tagged release exists. Both can be added later without restructuring.

## Hero asset

**Phase A (this PR):** ASCII-art screenshot of the TUI dashboard inside a fenced code block. Reason: never ship a broken `<img>` reference; ASCII renders everywhere including `cat README.md`.

**Phase B (deferred follow-up):** Record an animated GIF with [charmbracelet/vhs](https://github.com/charmbracelet/vhs):

- Tape file: `scripts/demo.tape`
- Output: `docs/demo.gif` (committed; ≤ 2 MB target)
- Sequence: `cco init` → start daemon → run a `claude` session → open `cco` → drill into a session → quit
- Replace the ASCII placeholder with `![demo](docs/demo.gif)` once recorded

Phase B is **not** a blocker for landing Phase A.

## Why (3 bullets)

- Claude Code emits OTLP telemetry but has no built-in dashboard — costs and tool usage stay invisible until your bill arrives.
- One Go binary: ingests OTLP/gRPC on `localhost:4317`, writes SQLite, renders a TUI. No cloud, no account, no daemon you didn't install.
- Per-project setup is one command (`cco init`) — drops seven OTel env vars into `.claude/settings.json` and probes the daemon.

## Features (5–7 bullets, plain text)

- Real-time ingestion of Claude Code OTLP logs and metrics over gRPC.
- Local SQLite store — no network egress, full data ownership.
- Bubble Tea TUI — session list, cost breakdown, tool-call detail, error log.
- Per-project tagging via `OTEL_RESOURCE_ATTRIBUTES`'s `project.name`.
- Single static binary — `go build` and you're done.
- Unattended daemon — launchd plist + systemd user unit shipped.
- Idempotent project setup — `cco init` is safe to re-run.

## Install (compressed)

Two OS-tab subsections under one `## Install` heading. Each subsection: 3 fenced bash blocks (build / install service / configure project) plus a one-line "verify" hint. Total Install section ≤ 40 lines, down from current ~55.

Content preserved verbatim from current README sections 1–3; only formatting changes.

## Usage

Table format — command → what it does:

| Command | Purpose |
|---|---|
| `cco` | Open the TUI dashboard (default) |
| `cco init` | Wire current project's `.claude/settings.json` to the local daemon |
| `cco serve` | Run the OTLP ingest daemon in the foreground (launchd/systemd run this) |
| `cco rebuild` | Rebuild aggregates from raw events |
| `cco version` | Print version and commit |

## Configuration

List the seven keys `cco init` writes, one line each, with the canonical value. Link to `docs/CLAUDE-CODE-OTEL.md` for what Claude Code emits and to `docs/superpowers/specs/2026-05-10-phase-4-install-ergonomics-design.md` for the rationale of each key.

## Architecture (one paragraph)

Three components: a gRPC receiver on `127.0.0.1:4317` (`cmd/app/serve.go`), a SQLite store under `~/.claude-code-observer/cco.sqlite`, and a Bubble Tea TUI. Receiver writes raw events; an aggregation pass produces session, cost, and tool-call rollups consumed by the TUI. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for layer boundaries and [docs/DATA-MODELS.md](docs/DATA-MODELS.md) for schema.

## Troubleshooting / Uninstall

Keep current content; both sections are already tight. Move under the new section order; no copy edits.

## Contributing

Three lines: link to `docs/ROADMAP.md` for current milestone, `docs/decisions/` for ADRs, `CLAUDE.md` for code conventions. No CONTRIBUTING.md split-out in v1.

## What's removed / changed

- Removed: emoji checkmarks, the standalone "Stack:" line under the title (folded into Why).
- Removed: "Five steps from clone to dashboard" preamble (the structure now speaks for itself).
- Changed: numbered prose steps → command tables / fenced blocks.
- Changed: Install moves from line 7 to roughly line 70 (after Why + Features).

## Length budget

Target: 180–230 lines. Current: 93. New sections add ~150 lines; tightening Install saves ~15. Net: ~225.

## Test plan

- Render in GitHub's Markdown preview — verify badge URLs resolve, tables render, fenced blocks have language hints.
- `markdownlint README.md` — clean (configure `.markdownlint.json` if it doesn't exist).
- Dry-read by a fresh contributor: can they go from "what is this" to "how do I install" in under 60 seconds of reading? (Manual verification, recorded in `docs/MANUAL-VERIFICATION.md` under a new "Phase 4.5 — README" entry.)
- Link checker: every relative link (`docs/...`) resolves to an existing file in the repo.

## Out of scope

- `CONTRIBUTING.md`, `CHANGELOG.md`, `CODE_OF_CONDUCT.md` as separate files — defer until v1.0.
- Auto-generated TOC — not needed under 250 lines.
- Package-manager install (`brew`, `apt`, `go install`) — defer until tagged release + goreleaser pipeline.
- CI / coverage badges — add when public CI exists.
- Logo — none planned; project name in plain text is fine.

## What "done" looks like

- `README.md` follows the section order above.
- ASCII placeholder demo present (Phase A).
- All links resolve. `markdownlint` clean. GitHub preview renders correctly.
- File length 180–230 lines.
- A separate follow-up task captured for the VHS GIF (Phase B) — not blocking this PR.
