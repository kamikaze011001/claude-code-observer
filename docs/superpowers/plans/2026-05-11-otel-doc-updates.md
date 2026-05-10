# OTel Doc Updates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Update live design docs to reflect the corrected OTel wire format (`reject`/`accept`, not `deny`/`allow`) and the new typed-constants layer introduced by the pipeline-realignment work.

**Architecture:** Mechanical literal swaps in five files plus one additive paragraph in `docs/ARCHITECTURE.md`. No code changes, no tests, single commit.

**Tech Stack:** Markdown only.

---

## File Structure

**Modify:**
- `docs/DATA-MODELS.md` — fix `decision = 'deny'` → `decision = 'reject'`.
- `docs/CONTEXT.md` — fix `\`deny\`` → `\`reject\`` in domain-expert quote.
- `docs/ROADMAP.md` — fix `decision = 'deny'` → `decision = 'reject'`.
- `docs/PRDs/0001-claude-code-observer-v1.md` — two edits: `allow`/`deny` → `accept`/`reject`; `tool_decision deny` → `tool_decision reject`.
- `docs/ARCHITECTURE.md` — add a one-bullet wire-format note in "Key Boundaries".

---

## Task 1: Fix DATA-MODELS.md decision literal

**Files:**
- Modify: `docs/DATA-MODELS.md:59`

- [ ] **Step 1: Replace the line**

Find this exact line (line 59):

```
| tool_denied | INTEGER | ✅ | Count of `tool_decision` events with `decision = 'deny'` |
```

Replace with:

```
| tool_denied | INTEGER | ✅ | Count of `tool_decision` events with `decision = 'reject'` |
```

- [ ] **Step 2: Verify no other `decision = 'deny'` references remain in the file**

Run: `grep -n "decision = 'deny'\|decision = 'allow'" docs/DATA-MODELS.md`
Expected: no output.

---

## Task 2: Fix CONTEXT.md domain-expert quote

**Files:**
- Modify: `docs/CONTEXT.md:81`

- [ ] **Step 1: Replace the line**

Find this exact line (line 81):

```
> **Domain expert:** "No. A Tool Decision can be `deny`, in which case there is no Tool Result. They are correlated by `tool_use_id` only when both fire."
```

Replace with:

```
> **Domain expert:** "No. A Tool Decision can be `reject`, in which case there is no Tool Result. They are correlated by `tool_use_id` only when both fire."
```

- [ ] **Step 2: Verify**

Run: `grep -n "Tool Decision can be" docs/CONTEXT.md`
Expected: one line, containing `\`reject\``, not `\`deny\``.

---

## Task 3: Fix ROADMAP.md rollup-engine note

**Files:**
- Modify: `docs/ROADMAP.md:155`

- [ ] **Step 1: Replace the line**

Find this exact line (line 155):

```
  - `tool_decision` updater: `tool_denied` only when `decision = 'deny'`
```

Replace with:

```
  - `tool_decision` updater: `tool_denied` only when `decision = 'reject'`
```

- [ ] **Step 2: Verify**

Run: `grep -n "decision = 'deny'\|decision = 'allow'" docs/ROADMAP.md`
Expected: no output.

---

## Task 4: Fix PRDs/0001 user story (line 40)

**Files:**
- Modify: `docs/PRDs/0001-claude-code-observer-v1.md:40`

- [ ] **Step 1: Replace the line**

Find this exact line (line 40):

```
16. As a developer, I want Tool Decisions to show whether the decision was `allow` or `deny` and whether it came from config / user / hook, so that I can audit my own auto-approve patterns.
```

Replace with:

```
16. As a developer, I want Tool Decisions to show whether the decision was `accept` or `reject` and whether it came from config / user / hook, so that I can audit my own auto-approve patterns.
```

- [ ] **Step 2: Verify**

Run: `grep -n "\`allow\` or \`deny\`" docs/PRDs/0001-claude-code-observer-v1.md`
Expected: no output.

---

## Task 5: Fix PRDs/0001 test description (line 182)

**Files:**
- Modify: `docs/PRDs/0001-claude-code-observer-v1.md:182`

- [ ] **Step 1: Replace `tool_decision deny` with `tool_decision reject`**

Find this fragment in the test description (line 182):

```
tool_decision deny increments tool_denied
```

Replace with:

```
tool_decision reject increments tool_denied
```

The surrounding sentence stays unchanged.

- [ ] **Step 2: Verify**

Run: `grep -n "tool_decision deny\|tool_decision allow" docs/PRDs/0001-claude-code-observer-v1.md`
Expected: no output.

---

## Task 6: Add wire-format note to ARCHITECTURE.md

**Files:**
- Modify: `docs/ARCHITECTURE.md` — append a bullet to the "Key Boundaries" list (currently lines 55–61).

- [ ] **Step 1: Insert the new bullet**

Find this exact block in `docs/ARCHITECTURE.md` (around lines 55–61):

```
## Key Boundaries

- **TUI is read-only.** Never opens a write connection. Polls the read-only DB every 1 s.
- **Receiver never touches SQLite directly** — always via the Service.
- **The `events` table is append-only** at runtime. Pruner is the only deleter and runs in a single goroutine inside the daemon.
- **Rollup updates happen in the same SQLite transaction as the event insert.** No async backfill at runtime.
- **Logs is the primary signal** — see [ADR-003](decisions/ADR-003-logs-as-primary-signal.md). Metrics is persisted only for sanity-checking aggregates.
```

Replace with:

```
## Key Boundaries

- **TUI is read-only.** Never opens a write connection. Polls the read-only DB every 1 s.
- **Receiver never touches SQLite directly** — always via the Service.
- **The `events` table is append-only** at runtime. Pruner is the only deleter and runs in a single goroutine inside the daemon.
- **Rollup updates happen in the same SQLite transaction as the event insert.** No async backfill at runtime.
- **Logs is the primary signal** — see [ADR-003](decisions/ADR-003-logs-as-primary-signal.md). Metrics is persisted only for sanity-checking aggregates.
- **Wire-format constants live in `internal/domain/wire.go`.** Event names (`claude_code.*`) and metric names are declared there once. The rollup registry indexes its updater map by these constants, and `TestApply_AllDomainEventsHaveAHandler` asserts every entry in `domain.AllEventNames` has a handler — real or no-op. Unknown event names emit a `rollup: no handler for event` debug log so future Claude Code releases surface immediately. Source of truth for what Claude Code emits: [docs/CLAUDE-CODE-OTEL.md](CLAUDE-CODE-OTEL.md).
```

- [ ] **Step 2: Verify**

Run: `grep -n "wire.go" docs/ARCHITECTURE.md`
Expected: one match in the "Key Boundaries" section.

---

## Task 7: Whole-corpus sweep and commit

- [ ] **Step 1: Run the cross-doc grep sweep**

Run:

```bash
grep -rnE "decision = 'deny'|decision = 'allow'|tool_decision deny|tool_decision allow|\`allow\` or \`deny\`|\`deny\` or \`allow\`" docs/ --include='*.md' | grep -v '^docs/superpowers/' | grep -v '^docs/CLAUDE-CODE-OTEL.md'
```

Expected: no output. (Hits inside `docs/superpowers/` are historical and excluded; `CLAUDE-CODE-OTEL.md` legitimately documents the spec values and is also excluded.)

- [ ] **Step 2: Confirm no other doc edits are pending**

Run: `git status -s -- docs/`
Expected: 5 modified files (`docs/ARCHITECTURE.md`, `docs/CONTEXT.md`, `docs/DATA-MODELS.md`, `docs/PRDs/0001-claude-code-observer-v1.md`, `docs/ROADMAP.md`).

If `docs/CLAUDE-CODE-OTEL.md` shows as modified from earlier work, that's fine — leave it staged or unstaged as it already was; this plan does not touch it.

- [ ] **Step 3: Commit only the five files this plan modified**

```bash
git add docs/ARCHITECTURE.md docs/CONTEXT.md docs/DATA-MODELS.md docs/PRDs/0001-claude-code-observer-v1.md docs/ROADMAP.md
git commit -m "docs: align live docs with OTel wire-format realignment"
```

- [ ] **Step 4: Verify the commit**

Run: `git show --stat HEAD`
Expected: 5 files changed, ~6 deletions, ~6 insertions, plus the new architecture bullet.

---

## Self-Review

**Spec coverage:** Audit findings 1–6 in the spec map directly to Tasks 1–6. Spec's verification grep maps to Task 7 Step 1.

**Placeholders:** None — every step shows the actual before/after text.

**Type consistency:** N/A (markdown only).

**Scope check:** All edits are in live docs (`docs/*.md`, `docs/PRDs/`, `docs/decisions/` if relevant — none in this plan). Historical superpowers docs are excluded by Task 7's grep filter.

**Note on line numbers:** Each task quotes the exact line content as well, so the engineer can locate it even if other unrelated edits shift line numbers between now and execution.
