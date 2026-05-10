# Doc updates for OTel pipeline realignment

**Date:** 2026-05-11
**Status:** Draft → ready for plan
**Driver:** The `2026-05-10-otel-pipeline-realignment` work fixed the receiver/eventparser/rollup/TUI to match the corrected `docs/CLAUDE-CODE-OTEL.md`. Live design docs still reference the old wire-format terminology (`deny`, `allow`) and don't mention the new typed-constants layer (`internal/domain/wire.go`) or the registry-completeness invariant.

## Goal

Make the live, non-historical project docs match the realigned code so a reader of the design corpus does not see contradictions with the source of truth (`docs/CLAUDE-CODE-OTEL.md`).

## Non-goals

- Editing `docs/superpowers/specs/` or `docs/superpowers/plans/`. Those are historical snapshots — they reflect what was true when they were written.
- Adding ADR-004 for the wire-constants decision. Deferred — can be added later if the choice is questioned.
- Documenting per-event rollup semantics for the 13 new event types. They are persisted as raw rows only; documentation will follow real rollup design when it lands.
- Schema changes — none, the schema is unaffected.

## Audit findings

| # | File | Line | Current text | Fix |
|---|------|------|--------------|-----|
| 1 | `docs/DATA-MODELS.md` | 59 | `decision = 'deny'` | `decision = 'reject'` |
| 2 | `docs/CONTEXT.md` | 81 | `Tool Decision can be \`deny\`` | `Tool Decision can be \`reject\`` |
| 3 | `docs/ROADMAP.md` | 155 | `tool_denied only when decision = 'deny'` | `tool_denied only when decision = 'reject'` |
| 4 | `docs/PRDs/0001-claude-code-observer-v1.md` | 40 | `\`allow\` or \`deny\`` | `\`accept\` or \`reject\`` |
| 5 | `docs/PRDs/0001-claude-code-observer-v1.md` | 182 | `tool_decision deny increments tool_denied` | `tool_decision reject increments tool_denied` |
| 6 | `docs/ARCHITECTURE.md` | new paragraph | (silent) | Add a short paragraph naming `internal/domain/wire.go` as SoT for event/metric names and the registry-completeness test |

## Components

### 1. Mechanical literal swaps (items 1–5)

Each is a one-line substitution. Files keep the same shape and tone. No surrounding prose needs to change.

### 2. Architecture note (item 6)

Add a short addition to `docs/ARCHITECTURE.md` explaining the wire-format boundary:

> **Wire-format constants.** Event names (`claude_code.*`) and metric names are declared once in `internal/domain/wire.go`. The rollup registry indexes its updater map by these constants, and `TestApply_AllDomainEventsHaveAHandler` asserts every entry in `domain.AllEventNames` has a handler — real or no-op. New event types declared by Claude Code surface either as a registry-test failure (if added to `AllEventNames`) or as a `rollup: no handler for event` debug log line (if not).

Insert it under the section that currently describes the receiver/eventparser layer boundary. Single paragraph; no diagram changes.

## Architecture

No code, no architectural change. This is documentation hygiene.

## Data flow

Unchanged.

## Error handling

N/A.

## Testing

The plan will not add tests — these are doc edits. Verification is:

1. `git diff` review confirms each substitution is the literal swap from the audit table.
2. Grep sweep after edits: `grep -nE "decision = 'deny'|allow.*deny|deny.*allow" docs/**/*.md` should return zero matches outside `docs/superpowers/` and `docs/CLAUDE-CODE-OTEL.md` (the latter intentionally documents the spec evolution).

## Out of scope

- ADR-004 for wire constants.
- Per-event rollup docs.
- Editing historical specs/plans under `docs/superpowers/`.
- Updating `docs/MANUAL-VERIFICATION.md` (no relevant grep hits).

## Estimated diff

5 files, ~6 lines changed, ~4 lines added. Single commit.
