# Decision Log — Lightweight Decisions (Y-Statements)

> Append decisions here using the Y-Statement format.
> For major decisions, create a full ADR instead (see [README.md](README.md)).
> For module-scoped design decisions, use [DDR-TEMPLATE.md](_TEMPLATES/DDR-TEMPLATE.md).

## Format

```
### YYYY-MM-DD — [Short title]

**Agent:** [claude-sonnet-4 / claude-opus-4 / human]
**Scope:** [file or module affected]

In the context of [situation],
facing [concern],
I decided [decision]
to achieve [goal],
accepting [tradeoff].
```

## Log

<!-- Append new decisions above this line -->

### YYYY-MM-DD — [Example: Use date-fns instead of dayjs]

**Agent:** [claude-sonnet-4 / human]
**Scope:** [src/lib/utils/date.ts]

In the context of [formatting dates for the order history page],
facing [need for locale support + relative time],
I decided [to use date-fns],
to achieve [smaller bundle via tree-shaking],
accepting [more verbose API than dayjs chain syntax].
