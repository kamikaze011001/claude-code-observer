# Demo Tape Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the stale `scripts/demo.tape` with a navigation tour of the redesigned TUI, render `docs/demo.gif`, and embed it in the README.

**Architecture:** A VHS tape file drives the `cco` TUI through dashboard → sessions → detail → prompt → waterfall → about against the maintainer's real `~/.claude-code-observer` store. VHS can't be unit-tested, so verification is render-then-visually-review. The README embed and committed GIF are gated on that human visual review.

**Tech Stack:** VHS (`charmbracelet/vhs`), ffmpeg (already installed at `/opt/homebrew/bin`), the `cco` TUI (bubbletea/lipgloss).

**Spec:** `docs/superpowers/specs/2026-05-30-demo-tape-design.md`

---

## Task 1: Rewrite `scripts/demo.tape`

**Files:**
- Modify (full rewrite): `scripts/demo.tape`

- [ ] **Step 1: Replace the entire file contents**

Overwrite `scripts/demo.tape` with exactly this:

```tape
# VHS tape for claude-code-observer demo GIF
# Record with: vhs scripts/demo.tape
# Output:      docs/demo.gif
#
# DATA SOURCE: this tape records against your REAL ~/.claude-code-observer
# store. `cco` must be installed and on PATH in the recording shell.
#
# BEFORE RECORDING:
#   1. Confirm the store has a representative set of sessions, INCLUDING at
#      least one prompt that spawned subagents (needed for the waterfall).
#   2. SCRUB FOR SENSITIVE DATA — this GIF is public. Check that visible
#      project names, prompt text, and paths are safe to publish.
#   3. Tune the `j` counts marked RECORD-TIME-TUNABLE below so the cursor
#      lands on a session, then a prompt, that has a subagent waterfall.

Output docs/demo.gif

Set FontSize 14
Set Width 1200
Set Height 720
Set Padding 20
Set Theme "Catppuccin Mocha"
Set TypingSpeed 80ms
Set Framerate 24

# Scene 1 — launch into the dashboard
Type "cco"
Enter
Sleep 2.5s

# Scene 2 — sessions list
Type "s"
Sleep 1.5s

# Scene 3 — move the cursor (RECORD-TIME-TUNABLE: adjust count)
Type "j"
Sleep 300ms
Type "j"
Sleep 1s

# Scene 4 — session detail (event timeline)
Enter
Sleep 2.5s

# Scene 5 — move to a user_prompt row (RECORD-TIME-TUNABLE: adjust count)
Type "j"
Sleep 300ms
Type "j"
Sleep 1s

# Scene 6 — prompt detail
Enter
Sleep 2.5s

# Scene 7 — subagent waterfall (the headline feature)
Type "w"
Sleep 3s

# Scene 8 — unwind back to the dashboard
Type "b"
Sleep 400ms
Type "b"
Sleep 400ms
Type "b"
Sleep 400ms
Type "b"
Sleep 400ms

# Scene 9 — about card
Type "?"
Sleep 2s

# Scene 10 — quit
Type "b"
Sleep 400ms
Type "q"
Sleep 500ms
```

- [ ] **Step 2: Sanity-check the file is valid tape syntax**

Run: `head -1 scripts/demo.tape`
Expected: `# VHS tape for claude-code-observer demo GIF`

Run: `grep -c '^Type \|^Enter\|^Sleep\|^Set \|^Output ' scripts/demo.tape`
Expected: a non-zero count (every command line accounted for; no typos in command keywords).

- [ ] **Step 3: Commit the tape**

```bash
git add scripts/demo.tape
git commit -m "docs: rewrite demo tape for redesigned TUI tour"
```

---

## Task 2: Record and visually review the GIF

> This task is **human-in-the-loop**. VHS opens a real `cco` TUI against the real store; an agent cannot judge whether the frames look good or whether the data is safe to publish. Do not auto-approve.

**Files:**
- Create (binary, not yet committed): `docs/demo.gif`

- [ ] **Step 1: Confirm prerequisites**

Run: `which vhs ffmpeg cco`
Expected: all three resolve to a path. If `cco` is missing, build/install it first (`go build -o ~/.claude-code-observer/bin/cco ./cmd/app` and ensure that dir is on PATH).

- [ ] **Step 2: Verify the store has a waterfall session**

Open `cco`, press `s`, drill into a recent session, find a `user_prompt` row, `enter`, then `w`. Confirm a non-empty subagent waterfall renders. Note how many `j` presses it took to reach that session and that prompt — these are the counts to set in Task 1's tape. Press `q` to exit.

- [ ] **Step 3: If the counts differ, update the tape**

If the navigation in Step 2 needed different `j` counts than the tape's defaults (2 and 2), edit `scripts/demo.tape` scenes 3 and 5 to match, then:

```bash
git add scripts/demo.tape
git commit -m "docs: tune demo tape navigation to real store"
```

(Skip this step's commit if no change was needed.)

- [ ] **Step 4: Render**

Run: `vhs scripts/demo.tape`
Expected: completes without error and writes `docs/demo.gif`. Confirm with `ls -lh docs/demo.gif`.

- [ ] **Step 5: Visually review the GIF (STOP — human review)**

Open `docs/demo.gif` and check:
- All ten scenes appear in order; the waterfall (scene 7) is non-empty.
- Text is legible at GIF scale.
- **No sensitive data** — project names, prompt text, paths are safe to publish.
- File size is reasonable (rough sanity ceiling: under ~5 MB for a README GIF; if much larger, consider trimming Sleeps or reducing dimensions).

If anything is wrong, adjust `scripts/demo.tape` and re-render before proceeding. Do not continue to Task 3 until the GIF is approved.

- [ ] **Step 6: Commit the approved GIF**

```bash
git add docs/demo.gif
git commit -m "docs: add recorded demo GIF"
```

---

## Task 3: Embed the GIF in the README

**Files:**
- Modify: `README.md:23`

- [ ] **Step 1: Replace the placeholder line**

The current `README.md:23` is:

```markdown
> _An animated demo GIF will replace this placeholder once recorded with [vhs](https://github.com/charmbracelet/vhs). See [Task 11](docs/superpowers/plans/2026-05-10-readme-enhancement.md) in the README plan._
```

Replace that single line with:

```markdown
![claude-code-observer demo](docs/demo.gif)
```

- [ ] **Step 2: Verify the embed resolves**

Run: `test -f docs/demo.gif && grep -n 'docs/demo.gif' README.md`
Expected: prints the matching README line (and the file exists, so the relative path resolves on GitHub).

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: embed demo GIF in README"
```

---

## Task 4: Open the PR

**Files:** none

- [ ] **Step 1: Push the branch and open a PR against `master`**

```bash
git push -u origin docs/demo-tape
gh pr create --base master --title "docs: demo GIF + tape rewrite" \
  --body "Rewrites scripts/demo.tape for the redesigned TUI (dashboard → sessions → detail → prompt → waterfall → about), records docs/demo.gif against the real store, and embeds it in the README. Spec: docs/superpowers/specs/2026-05-30-demo-tape-design.md"
```

Expected: `gh` prints the PR URL.

---

## Notes for the executor

- **No `make vet`/`make test`/`make build` gate here** — this change touches no Go code. The CLAUDE.md verification trio does not apply; the verification for this plan is the VHS render + human visual review in Task 2.
- **Task 2 is a hard human gate.** An agent executing this plan should pause at Task 2 Step 5 and hand control back for visual approval rather than committing the GIF unreviewed.
- **`Type "j"` vs `Down`:** the TUI binds both `j` and the down-arrow for navigation, so `Type "j"` is intentional and correct. Keep it for readability.
