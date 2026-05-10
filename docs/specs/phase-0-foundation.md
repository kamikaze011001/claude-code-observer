# Phase 0 — Foundation (Design Spec)

> Date: 2026-05-10
> Source roadmap: [../ROADMAP.md](../ROADMAP.md)
> Source PRD: [../PRDs/0001-claude-code-observer-v1.md](../PRDs/0001-claude-code-observer-v1.md)

Phase 0 stands up the empty-but-runnable binary and the persistent SQLite store with full schema. Two milestones: **M0.1 scaffolding** and **M0.2 SQLite + migrations**.

## Stack decisions (locked this phase)

| Decision | Choice |
|---|---|
| Module path | `github.com/kamikaze011001/claude-code-observer` |
| Go version | 1.23 (per CLAUDE.md) |
| CLI library | `github.com/spf13/cobra` |
| SQLite driver | `modernc.org/sqlite` (pure-Go, no CGO) |
| Migrations | Handwritten loop over embedded `migrations/*.sql` (no `golang-migrate`) |
| Logging | stdlib `log/slog` with JSON handler to stderr |
| Data dir | `~/.claude-code-observer/` (override via `CCO_HOME`) |
| Config loading | Env vars only in v1 |
| Binary name | `claude-code-observer` (built from `cmd/app`) |

These are the only new decisions Phase 0 introduces beyond what the PRD and ADRs already pinned.

## M0.1 — Repo scaffolding & subcommand skeleton

### Deliverables

1. `go.mod` at module path above, Go 1.23
2. `cmd/app/main.go` — cobra root, four subcommands stubbed
3. Empty packages with one type each so the project compiles end-to-end:
   - `internal/domain/` — `Event`, `Session`, `Prompt`, `ToolResult`, `MetricSnapshot` structs (fields per `docs/DATA-MODELS.md`)
   - `internal/receiver/`, `internal/service/`, `internal/repository/`, `internal/eventparser/`, `internal/rollup/`, `internal/retention/`, `internal/tui/` — placeholder file with package declaration only
4. `Makefile` (or `Taskfile`) with the targets from CLAUDE.md (`vet`, `test`, `build`, `lint`)

### File layout after M0.1

```
claude-code-observer/
├── cmd/app/
│   ├── main.go                  # cobra root
│   ├── serve.go                 # claude-code-observer serve  — stub: blocks on signal
│   ├── tui.go                   # claude-code-observer        — stub: prints "TUI not yet wired"
│   ├── init.go                  # claude-code-observer init   — stub
│   └── rebuild.go               # claude-code-observer rebuild-rollups — stub
├── internal/
│   ├── domain/types.go
│   ├── receiver/doc.go
│   ├── service/doc.go
│   ├── repository/doc.go
│   ├── eventparser/doc.go
│   ├── rollup/doc.go
│   ├── retention/doc.go
│   └── tui/doc.go
├── go.mod
├── go.sum
└── Makefile
```

### Cobra command shape

```
claude-code-observer [global flags] <subcommand> [subflags]

Global flags:
  --home string    Data dir override (default: $CCO_HOME or ~/.claude-code-observer)
  --log-level      debug|info|warn|error (default: info)

Subcommands:
  (no subcommand)     Open the TUI
  serve               Run the OTLP receiver daemon
  init                Write/update .claude/settings.json in $PWD
  rebuild-rollups     Recompute sessions/prompts from events
  version             Print version + commit SHA
```

`version` is added because cobra users expect it and it costs ~5 LOC. Not in roadmap but trivial.

### Domain types (M0.1)

Defined in `internal/domain/types.go`. No methods yet — Phase 1 adds them. Fields exactly match `docs/DATA-MODELS.md`:

```go
type Event struct {
    ID         int64
    TS         int64           // unix nanoseconds
    SessionID  string
    PromptID   string          // empty for session-level events
    EventName  string
    Attrs      map[string]any  // serialized to JSON column
}

type Session struct {
    SessionID            string
    ProjectName          string
    ProjectCWD           string
    StartedAt            int64
    LastSeenAt           int64
    EndedAt              *int64
    AppVersion           string
    OSType               string
    UserID               string
    CostUSD              float64
    InputTokens          int64
    OutputTokens         int64
    CacheReadTokens      int64
    CacheCreationTokens  int64
    APIRequests          int64
    APIErrors            int64
    SubagentRequests     int64
    AuxiliaryRequests    int64
    ToolCalls            int64
    ToolDenied           int64
    Prompts              int64
}

type Prompt struct {
    PromptID            string
    SessionID           string
    StartedAt           int64
    EndedAt             *int64
    PromptLength        int64
    CommandName         string
    CommandSource       string
    CostUSD             float64
    InputTokens         int64
    OutputTokens        int64
    CacheReadTokens     int64
    CacheCreationTokens int64
    APIRequests         int64
    SubagentRequests    int64
    ToolCalls           int64
    HadError            bool
}

type MetricSnapshot struct {
    ID         int64
    TS         int64
    SessionID  string
    MetricName string
    Value      float64
    Attrs      map[string]any
}
```

`ToolResult` is intentionally not a separate domain type — it's just an `Event` with `EventName == "claude_code.tool_result"`. The PRD listed it as a domain type; spec-self-review caught it as redundant.

### M0.1 demo

- `make build` produces `bin/claude-code-observer`
- `./bin/claude-code-observer --help` lists the five subcommands above with descriptions
- `./bin/claude-code-observer serve` starts, logs `"daemon started"` (no actual listener), exits cleanly on SIGINT (Ctrl-C)
- `./bin/claude-code-observer version` prints version + commit SHA
- `make test` green; `make vet` clean

### M0.1 test gate

- One smoke test per subcommand verifying the command parses its flags and the handler is invoked
- Cobra command tree assertion: `rootCmd.Commands()` contains `serve`, `init`, `rebuild-rollups`, `version`
- Coverage % not enforced this milestone (placeholder code) but tests must exist for `cmd/app/`

## M0.2 — SQLite repository + migrations

### Deliverables

1. `internal/repository/repository.go` — `Repository` struct holding the DB pool; constructor `Open(home string) (*Repository, error)`
2. `internal/repository/migrations/0001_initial.sql` — schema from `docs/DATA-MODELS.md`
3. `internal/repository/migrate.go` — embedded migrations via `//go:embed migrations/*.sql`, applied at `Open` time inside one transaction per migration
4. WAL mode enabled on connect; `_pragma=foreign_keys(1)` on the connection string
5. `cmd/app/serve.go` updated: at startup, `repository.Open(home)` is called and logs the resolved schema_version

### Connection model

- One `*sql.DB` per process — Go's `database/sql` pool handles concurrency
- Daemon writes via this pool; TUI (Phase 3) opens a separate read-only `*sql.DB` (`?_pragma=query_only(1)`)
- WAL mode via `?_journal_mode=WAL` on first open; verified by `PRAGMA journal_mode` returning `wal`

### Migration loop

Pseudocode:

```
files := embed.ReadDir("migrations") sorted by filename
for each file (NNNN_name.sql):
    n := parse leading number
    if n <= currentVersion: skip
    BEGIN; exec contents; INSERT INTO schema_version; COMMIT
```

Each migration is its own transaction. Failing migration N leaves schema_version at N-1; restart re-attempts N.

### M0.2 demo

- `./bin/claude-code-observer serve` creates `~/.claude-code-observer/db.sqlite`
- `sqlite3 ~/.claude-code-observer/db.sqlite ".schema"` shows: `events`, `sessions`, `prompts`, `metric_snapshots`, `schema_version`
- All indexes from `docs/DATA-MODELS.md` exist (`PRAGMA index_list('events')` etc.)
- `sqlite3 ... "SELECT version FROM schema_version"` returns `1`
- `sqlite3 ... "PRAGMA journal_mode"` returns `wal`
- Stop daemon, restart — schema_version still 1, no errors logged
- `CCO_HOME=/tmp/cco-test ./bin/claude-code-observer serve` creates the DB at the override path

### M0.2 test gate

Integration tests in `internal/repository/repository_test.go` against a temp directory:

- **Migration applies on empty DB** — schema_version inserted, all tables present
- **Migration is idempotent** — re-opening doesn't re-apply or error
- **All indexes exist** — assert via `PRAGMA index_list` for each table
- **WAL mode active** — `PRAGMA journal_mode == "wal"`
- **Foreign keys enabled** — `PRAGMA foreign_keys == 1`
- **Bad migration recovery** — inject a malformed migration, assert schema_version unchanged and error returned
- **Coverage ≥ 80% on `internal/repository/`**

## What Phase 0 deliberately does NOT include

- No event ingestion — receiver is a stub. Phase 1 wires the gRPC service.
- No reads beyond `schema_version` — repository CRUD comes with Phase 1/2.
- No TUI logic — Phase 3.
- No `init` wizard logic — Phase 4. The cobra subcommand is just a stub printing "not implemented".
- No retention logic — Phase 2.

## Dependencies introduced this phase

```
github.com/spf13/cobra       v1.x   (CLI)
modernc.org/sqlite           v1.x   (SQLite driver, pure Go)
```

That is the entire third-party dep list at the end of Phase 0. Phase 1 will add the OTLP protos and gRPC.

## Risks and mitigations

| Risk | Mitigation |
|---|---|
| `modernc.org/sqlite` perf surprise | Benchmark on M0.2 with 10k synthetic events; confirm <5ms median insert. If it fails, swap to `mattn/go-sqlite3` (interface is identical) |
| `embed` directive path issues | Use `//go:embed all:migrations` and test via `migrations.ReadDir` immediately in `init` |
| Cobra flag ordering quirks | Add a smoke test verifying `claude-code-observer --home /tmp serve` and `claude-code-observer serve --home /tmp` both work |

## Definition of Done — Phase 0

Both milestones (M0.1 + M0.2) must satisfy their demo and test gate per [ROADMAP.md](../ROADMAP.md). When that's true, Phase 0 is done and we can begin Phase 1 (Ingest path).
