# Demo Tape Enhancement — Design

**Date:** 2026-05-30
**Status:** Approved
**Author:** brainstorming session
**Artifact:** `scripts/demo.tape` (VHS) → `docs/demo.gif`

## Goal

Replace the stale `scripts/demo.tape` with a pure-navigation tour of the
redesigned TUI, recorded against the maintainer's real
`~/.claude-code-observer` store, emitting `docs/demo.gif` to fill the README
placeholder at `README.md:23`.

## Background

The existing tape predates three shipped changes — the TUI redesign (#8, #9)
and the subagent waterfall view (#10). It is wrong in two ways:

1. **Navigation is stale.** It drives the old TUI with bare `Down`/`Enter`.
   The redesigned dashboard uses `s` → sessions list, `j/k` to move, `enter`
   to drill in, and there is now a subagent **waterfall** view the tape never
   reaches.
2. **Data sourcing is fragile.** It runs a live `cco init` + `claude --print
   '…'` to generate one event. That needs working auth and network, costs ~6s
   of waiting, and leaves the dashboard nearly empty.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Data source | **Real store** (`~/.claude-code-observer`) | Authentic, zero new code. Accepts non-determinism + a privacy-scrub step. |
| Tour scope | **Full tour** | Dashboard → sessions → detail → prompt → waterfall → about. Sells the redesign + the new waterfall. |
| Opening | **Jump straight to `cco`** | Cleanest, no side effects on `.claude/settings.json`. Install story stays in README prose. |
| Output format | **GIF** (`docs/demo.gif`) | Auto-plays + loops inline with plain `![](…)` markdown — exactly what the README placeholder expects. |

## Navigation map (verified against source)

- **Dashboard** (root): `s` → sessions list; `enter` on highlighted session →
  session detail; `j/k` move; `?` about; `r` refresh; `q` quit.
- **Sessions list**: `j/k`/arrows move; `enter` → session detail; `u/d` page;
  `g/G` top/bottom; `b` back.
- **Session detail**: `j/k` move cursor over the event timeline; `enter` on a
  `user_prompt` row → prompt detail; `b` back.
- **Prompt detail**: `w` → **waterfall**; `b` back; `?` about.
- **Waterfall**: subagent view; `b` back.
- **About**: reachable via `?` from anywhere; `b` back.

## VHS settings

```
Output docs/demo.gif
Set FontSize 14
Set Width 1200
Set Height 720
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 80ms
Set Framerate 24
```

`Catppuccin Mocha` matches the TUI's default Mocha palette. Height nudged
700 → 720 to give the timeline and waterfall more vertical room.

## Storyboard

| # | Scene | Keys | Sleep | Purpose |
|---|-------|------|-------|---------|
| 1 | Launch | `Type "cco"` `Enter` | 2.5s | Dashboard: totals, sessions, top tools |
| 2 | Sessions list | `s` | 1.5s | Show the list view |
| 3 | Move cursor | `j` `j` | 1s | Demonstrate navigation |
| 4 | Session detail | `Enter` | 2.5s | Event timeline |
| 5 | Find a prompt | `j` ×N* | 1s | Land on a `user_prompt` row |
| 6 | Prompt detail | `Enter` | 2.5s | Single-prompt drill-down |
| 7 | **Waterfall** | `w` | 3s | Headline new feature |
| 8 | Back out | `b` `b` `b` `b` | 0.4s each | Unwind to dashboard |
| 9 | About | `?` | 2s | Version/help card |
| 10 | Quit | `b` `q` | 0.5s | Clean exit |

Total ≈ 25–30s.

## Known fragility (real-data + blind keystrokes)

`*` Scenes 3 and 5 use **blind keystroke counts**. The `j ×N` presses to reach
a subagent-spawning prompt must be tuned at record time against whatever is in
the store. **The store must contain at least one prompt that spawned subagents**
or scene 7 renders an empty waterfall.

The tape will carry a comment block marking these counts as record-time-tunable.

## Pre-record checklist (in the tape header)

1. Confirm the store has a representative set of sessions, including **at least
   one with a subagent waterfall**.
2. **Scrub for sensitive data** — the GIF is public. Check that visible project
   names, prompt text, and paths are safe to publish. Pick a clean moment or
   session if not.
3. Run `vhs scripts/demo.tape`.
4. Tune the `j ×N` counts (scenes 3, 5) if the cursor lands wrong.

## Verification

VHS recordings can't be unit-tested. Verification is:

1. `vhs scripts/demo.tape` renders without error.
2. **Visually review `docs/demo.gif`**: correct scenes, readable text, no
   sensitive data, reasonable file size.
3. Only after visual approval: swap `README.md:23` placeholder for
   `![demo](docs/demo.gif)` and commit the `.gif`.

## Deliverables

1. Rewritten `scripts/demo.tape` (header comment + record-time-tuning notes).
2. `docs/demo.gif` (committed after visual review).
3. `README.md:23` placeholder → `![demo](docs/demo.gif)` (after GIF approved).

## Out of scope

- MP4/WebM outputs (GIF only for v1).
- A deterministic `cco seed --demo` command (considered, rejected in favor of
  real data).
- Automated/CI recording.
