# Phase 0 Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the empty-but-runnable `claude-code-observer` Go binary with cobra subcommands, a persistent SQLite store created via embedded migrations, and the full v1 schema migrated in.

**Architecture:** Single Go binary (`cmd/app`) using cobra for subcommand dispatch. Storage lives in `~/.claude-code-observer/db.sqlite` (override via `CCO_HOME`) accessed through `internal/repository`, which embeds SQL migration files via `//go:embed` and applies them at `Open` time inside per-migration transactions. SQLite runs in WAL mode with foreign keys enabled. All other `internal/*` packages are stubbed for later phases.

**Tech Stack:** Go 1.23, `github.com/spf13/cobra`, `modernc.org/sqlite` (pure-Go driver, no CGO), stdlib `database/sql`, stdlib `log/slog`, stdlib `embed` + `io/fs`. Tests use `testing/fstest.MapFS` for migration injection and `t.TempDir()` for repository integration.

**Source documents:**
- Spec: `docs/specs/phase-0-foundation.md`
- Roadmap: `docs/ROADMAP.md`
- Data model: `docs/DATA-MODELS.md`
- PRD: `docs/PRDs/0001-claude-code-observer-v1.md`

---

## File Structure

After Phase 0, the repo looks like:

```
claude-code-observer/
├── go.mod
├── go.sum
├── Makefile
├── .gitignore
├── cmd/app/
│   ├── main.go               # cobra root command, global flags, Execute()
│   ├── serve.go              # `serve` subcommand: opens repo, blocks on signal
│   ├── tui.go                # default (no subcommand) → TUI stub
│   ├── init.go               # `init` subcommand stub
│   ├── rebuild.go            # `rebuild-rollups` subcommand stub
│   ├── version.go            # `version` subcommand
│   └── main_test.go          # cobra wiring smoke tests
├── internal/
│   ├── domain/
│   │   └── types.go          # Event, Session, Prompt, MetricSnapshot
│   ├── receiver/doc.go       # package placeholder
│   ├── service/doc.go        # package placeholder
│   ├── eventparser/doc.go    # package placeholder
│   ├── rollup/doc.go         # package placeholder
│   ├── retention/doc.go      # package placeholder
│   ├── tui/doc.go            # package placeholder
│   └── repository/
│       ├── repository.go     # Open, Close, schema-version helpers
│       ├── repository_test.go
│       ├── migrate.go        # migration runner, embed FS
│       ├── migrate_test.go
│       └── migrations/
│           └── 0001_initial.sql  # full v1 schema
└── docs/...                  # (already exists)
```

**Boundaries:**
- `cmd/app/` only orchestrates — no business logic.
- `internal/repository/` is the only package that imports `database/sql` or the SQLite driver.
- `internal/domain/` is dependency-free (no imports from other internal packages).
- The migration runner takes an `fs.FS` so tests can swap in `fstest.MapFS` without touching the embedded files.

---

## Task 1: Initialize Go module, Makefile, .gitignore

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.gitignore`

- [ ] **Step 1.1: Initialize Go module**

Run from repo root:

```bash
cd /Users/sonanh/Documents/AIBLES/claude-code-observer
go mod init github.com/kamikaze011001/claude-code-observer
```

Expected: creates `go.mod` containing:

```
module github.com/kamikaze011001/claude-code-observer

go 1.23
```

- [ ] **Step 1.2: Add Makefile**

Create `Makefile`:

```makefile
.PHONY: build test vet lint clean run

BIN_DIR := bin
BIN := $(BIN_DIR)/claude-code-observer
PKG := ./...
LDFLAGS := -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev) \
           -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/app

test:
	go test $(PKG)

test-cover:
	go test -cover $(PKG)

vet:
	go vet $(PKG)

lint:
	golangci-lint run

run: build
	$(BIN)

clean:
	rm -rf $(BIN_DIR)
```

- [ ] **Step 1.3: Add .gitignore**

Create `.gitignore`:

```
# Binaries
bin/
*.exe

# Go
*.test
*.out
coverage.txt

# Local data
*.sqlite
*.sqlite-shm
*.sqlite-wal

# Editor
.vscode/
.idea/
.DS_Store
```

- [ ] **Step 1.4: Verify module builds (with no source files)**

Run:

```bash
go vet ./...
```

Expected: no output, exit 0 (no Go files yet — vet has nothing to do but must not error).

- [ ] **Step 1.5: Commit**

```bash
git init   # only if not already a git repo; safe to skip if it errors
git add go.mod Makefile .gitignore
git commit -m "chore: initialize Go module, Makefile, gitignore"
```

If `git init` was needed, also run `git add CLAUDE.md README.md docs/ .claude/ .github/ .agents/ skills-lock.json CLAUDE.local.md.example` and amend the commit so existing project files are tracked. Use `git status` to confirm nothing important is left untracked.

---

## Task 2: Domain types

**Files:**
- Create: `internal/domain/types.go`

These are plain data structs. No methods, no behavior, so no unit test — tests would just re-state the field list. They will be exercised indirectly by repository and parser tests in later phases.

- [ ] **Step 2.1: Create `internal/domain/types.go`**

```go
// Package domain defines the core data types shared across receiver,
// service, repository, and TUI layers.
package domain

// Event is the in-memory representation of a single OTLP log record after
// parsing. Persisted to the events table; attrs is serialized to JSON.
type Event struct {
	ID        int64
	TS        int64 // unix nanoseconds (OTel time_unix_nano)
	SessionID string
	PromptID  string // empty for session-level events
	EventName string
	Attrs     map[string]any
}

// Session is the rollup row for a single Claude Code invocation.
type Session struct {
	SessionID           string
	ProjectName         string
	ProjectCWD          string
	StartedAt           int64
	LastSeenAt          int64
	EndedAt             *int64
	AppVersion          string
	OSType              string
	UserID              string
	CostUSD             float64
	InputTokens         int64
	OutputTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
	APIRequests         int64
	APIErrors           int64
	SubagentRequests    int64
	AuxiliaryRequests   int64
	ToolCalls           int64
	ToolDenied          int64
	Prompts             int64
}

// Prompt is the rollup row for a single user turn within a session.
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

// MetricSnapshot is a single OTLP metric datapoint persisted for sanity
// checking against the events-derived rollups (see ADR-003).
type MetricSnapshot struct {
	ID         int64
	TS         int64
	SessionID  string
	MetricName string
	Value      float64
	Attrs      map[string]any
}
```

- [ ] **Step 2.2: Verify it compiles**

```bash
go build ./internal/domain/...
```

Expected: exits 0, no output.

- [ ] **Step 2.3: Commit**

```bash
git add internal/domain/types.go
git commit -m "feat(domain): add core types (Event, Session, Prompt, MetricSnapshot)"
```

---

## Task 3: Cobra root command

**Files:**
- Create: `cmd/app/main.go`

- [ ] **Step 3.1: Add cobra dependency**

```bash
go get github.com/spf13/cobra@latest
```

Expected: `go.sum` is created/updated; `go.mod` has `require github.com/spf13/cobra vX.Y.Z`.

- [ ] **Step 3.2: Create `cmd/app/main.go`**

```go
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Set via -ldflags "-X main.version=... -X main.commit=..."
var (
	version = "dev"
	commit  = "none"
)

// Resolved at root PersistentPreRun and read by subcommands.
var (
	homeDir  string
	logLevel string
	logger   *slog.Logger
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "claude-code-observer",
		Short:         "Local observability for Claude Code via OTLP",
		Long:          "claude-code-observer ingests OTLP/gRPC telemetry from Claude Code into a local SQLite store and renders it in a TUI.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := resolveHomeDir(); err != nil {
				return err
			}
			logger = newLogger(logLevel)
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&homeDir, "home", "", "Data directory (default: $CCO_HOME or ~/.claude-code-observer)")
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level: debug|info|warn|error")
	return cmd
}

func resolveHomeDir() error {
	if homeDir != "" {
		return nil
	}
	if env := os.Getenv("CCO_HOME"); env != "" {
		homeDir = env
		return nil
	}
	hd, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	homeDir = filepath.Join(hd, ".claude-code-observer")
	return nil
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}

func main() {
	root := newRootCmd()
	registerSubcommands(root)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// registerSubcommands is implemented across subcommand files.
func registerSubcommands(root *cobra.Command) {
	root.AddCommand(
		newServeCmd(),
		newTUICmd(),
		newInitCmd(),
		newRebuildCmd(),
		newVersionCmd(),
	)
}
```

- [ ] **Step 3.3: Verify it does NOT compile yet**

```bash
go build ./cmd/app/...
```

Expected: FAIL with "undefined: newServeCmd, newTUICmd, newInitCmd, newRebuildCmd, newVersionCmd". Continuing — these are added in Tasks 4–6.

- [ ] **Step 3.4: Do not commit yet**

Wait until Task 6 lands so the build is green.

---

## Task 4: `serve` subcommand stub

**Files:**
- Create: `cmd/app/serve.go`

- [ ] **Step 4.1: Create `cmd/app/serve.go`**

```go
package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the OTLP receiver daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			logger.Info("daemon started",
				"home", homeDir,
				"version", version,
				"commit", commit,
			)
			<-ctx.Done()
			logger.Info("daemon stopped")
			return nil
		},
	}
}
```

- [ ] **Step 4.2: Compile check** (other subcommands still missing — expect errors about them, not about serve)

```bash
go build ./cmd/app/... 2>&1 | grep -v 'newServeCmd' | head
```

Expected: no error mentioning `newServeCmd`. Errors for other subcommands are expected at this point.

---

## Task 5: `tui`, `init`, `rebuild-rollups` stubs

**Files:**
- Create: `cmd/app/tui.go`
- Create: `cmd/app/init.go`
- Create: `cmd/app/rebuild.go`

- [ ] **Step 5.1: Create `cmd/app/tui.go`**

```go
package main

import "github.com/spf13/cobra"

// newTUICmd is invoked when no subcommand is given. We mount it as a
// hidden subcommand and also wire it as the root's RunE in main.go via a
// fall-through. For Phase 0 it prints a stub message.
func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:    "tui",
		Short:  "Open the interactive TUI",
		Hidden: true, // exposed via default invocation in Phase 3
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("tui not yet implemented", "home", homeDir)
			cmd.Println("TUI not yet wired (Phase 3).")
			return nil
		},
	}
}
```

- [ ] **Step 5.2: Create `cmd/app/init.go`**

```go
package main

import "github.com/spf13/cobra"

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Write/update .claude/settings.json in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("init not yet implemented")
			cmd.Println("init not yet wired (Phase 4).")
			return nil
		},
	}
}
```

- [ ] **Step 5.3: Create `cmd/app/rebuild.go`**

```go
package main

import "github.com/spf13/cobra"

func newRebuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild-rollups",
		Short: "Recompute sessions/prompts rollups from the events table",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("rebuild-rollups not yet implemented", "home", homeDir)
			cmd.Println("rebuild-rollups not yet wired (Phase 2).")
			return nil
		},
	}
}
```

---

## Task 6: `version` subcommand

**Files:**
- Create: `cmd/app/version.go`

- [ ] **Step 6.1: Create `cmd/app/version.go`**

```go
package main

import "github.com/spf13/cobra"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and commit SHA",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("claude-code-observer %s (commit %s)\n", version, commit)
		},
	}
}
```

- [ ] **Step 6.2: Build the binary**

```bash
make build
```

Expected: `bin/claude-code-observer` exists, exit 0.

- [ ] **Step 6.3: Manual smoke**

```bash
./bin/claude-code-observer --help
```

Expected: lists subcommands `init`, `rebuild-rollups`, `serve`, `version` (and hidden `tui` is not shown). The "Available Commands" section is non-empty.

```bash
./bin/claude-code-observer version
```

Expected output (exact version/commit may differ):

```
claude-code-observer dev (commit none)
```

- [ ] **Step 6.4: Commit Tasks 3–6**

```bash
git add go.mod go.sum cmd/app/
git commit -m "feat(cmd): add cobra root + serve/init/rebuild/version stubs"
```

---

## Task 7: Cobra wiring smoke tests

**Files:**
- Create: `cmd/app/main_test.go`

- [ ] **Step 7.1: Write the failing tests**

```go
package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommand_HasExpectedSubcommands(t *testing.T) {
	root := newRootCmd()
	registerSubcommands(root)

	want := map[string]bool{
		"serve":           false,
		"tui":             false,
		"init":            false,
		"rebuild-rollups": false,
		"version":         false,
	}
	for _, c := range root.Commands() {
		if _, ok := want[c.Name()]; ok {
			want[c.Name()] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing subcommand: %s", name)
		}
	}
}

func TestVersionCommand_PrintsVersion(t *testing.T) {
	root := newRootCmd()
	registerSubcommands(root)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "claude-code-observer") {
		t.Errorf("output missing binary name: %q", got)
	}
}

func TestRoot_HomeFlag_OverridesDefault(t *testing.T) {
	t.Setenv("CCO_HOME", "")
	root := newRootCmd()
	registerSubcommands(root)
	root.SetArgs([]string{"--home", "/tmp/cco-test", "version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if homeDir != "/tmp/cco-test" {
		t.Errorf("homeDir = %q, want /tmp/cco-test", homeDir)
	}
}

func TestRoot_HomeFromEnv(t *testing.T) {
	homeDir = "" // reset package-level
	t.Setenv("CCO_HOME", "/tmp/from-env")
	root := newRootCmd()
	registerSubcommands(root)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if homeDir != "/tmp/from-env" {
		t.Errorf("homeDir = %q, want /tmp/from-env", homeDir)
	}
}
```

- [ ] **Step 7.2: Run the tests**

```bash
go test ./cmd/app/... -v
```

Expected: all four tests PASS. If `TestRoot_HomeFromEnv` flakes due to test order, ensure each test that touches `homeDir` resets it at the top — already handled by `homeDir = ""` and `--home` flag explicitly setting it.

- [ ] **Step 7.3: Verify full build still green**

```bash
go vet ./...
go test ./...
go build -o bin/claude-code-observer ./cmd/app
```

All three exit 0.

- [ ] **Step 7.4: Commit**

```bash
git add cmd/app/main_test.go
git commit -m "test(cmd): cobra wiring smoke tests"
```

---

## Task 8: Stub `internal/*` packages so the tree compiles

**Files:**
- Create: `internal/receiver/doc.go`
- Create: `internal/service/doc.go`
- Create: `internal/eventparser/doc.go`
- Create: `internal/rollup/doc.go`
- Create: `internal/retention/doc.go`
- Create: `internal/tui/doc.go`

Each file is identical except for the package name. This locks the package layout from `docs/ARCHITECTURE.md` so later phases don't have to invent it.

- [ ] **Step 8.1: Create `internal/receiver/doc.go`**

```go
// Package receiver implements the OTLP/gRPC server. Phase 1 wires this up.
package receiver
```

- [ ] **Step 8.2: Create the other five identically**

`internal/service/doc.go`:

```go
// Package service parses domain events and dispatches rollup updates. Phase 1+.
package service
```

`internal/eventparser/doc.go`:

```go
// Package eventparser converts OTLP LogRecord values into domain.Event. Phase 1.
package eventparser
```

`internal/rollup/doc.go`:

```go
// Package rollup maintains the sessions and prompts tables. Phase 2.
package rollup
```

`internal/retention/doc.go`:

```go
// Package retention prunes old events on a configurable schedule. Phase 2.
package retention
```

`internal/tui/doc.go`:

```go
// Package tui implements the Bubble Tea read-only views. Phase 3.
package tui
```

- [ ] **Step 8.3: Verify everything still compiles and tests pass**

```bash
go vet ./...
go test ./...
```

Both exit 0.

- [ ] **Step 8.4: Commit**

```bash
git add internal/receiver internal/service internal/eventparser internal/rollup internal/retention internal/tui
git commit -m "chore: add internal package skeletons"
```

---

**M0.1 demo checkpoint:**

Run all of these and confirm the outcomes:

```bash
make build
./bin/claude-code-observer --help
./bin/claude-code-observer version
./bin/claude-code-observer serve &
SERVE_PID=$!
sleep 1
kill -INT $SERVE_PID
wait $SERVE_PID
```

Expected:
- `--help` lists `init`, `rebuild-rollups`, `serve`, `version`
- `version` prints `claude-code-observer dev (commit none)` (or git-tag-derived values)
- `serve` logs `daemon started` JSON to stderr, then `daemon stopped` after Ctrl-C, exits 0

If any fail, fix before moving to M0.2.

---

## Task 9: Migration runner (TDD with `fstest.MapFS`)

**Files:**
- Create: `internal/repository/migrate.go`
- Create: `internal/repository/migrate_test.go`

The runner takes an `fs.FS` so tests can supply arbitrary migration sets without touching the embedded files.

- [ ] **Step 9.1: Add the SQLite driver dependency**

```bash
go get modernc.org/sqlite@latest
```

Expected: `go.mod` has `require modernc.org/sqlite vX.Y.Z`. Note: this transitively pulls in several `modernc.org/*` libs — that's expected.

- [ ] **Step 9.2: Write the failing test for an empty migration set**

`internal/repository/migrate_test.go`:

```go
package repository

import (
	"database/sql"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)

func openMemory(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRunMigrations_EmptyFS_CreatesSchemaVersionTable(t *testing.T) {
	db := openMemory(t)
	if err := runMigrations(db, fstest.MapFS{}); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}
	var v int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&v)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if v != 0 {
		t.Errorf("version = %d, want 0", v)
	}
}
```

- [ ] **Step 9.3: Run — expect it to fail because `runMigrations` doesn't exist**

```bash
go test ./internal/repository/... -run TestRunMigrations_EmptyFS -v
```

Expected: FAIL — `undefined: runMigrations`.

- [ ] **Step 9.4: Implement minimal `migrate.go`**

`internal/repository/migrate.go`:

```go
package repository

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const createSchemaVersionTable = `
CREATE TABLE IF NOT EXISTS schema_version (
	version    INTEGER PRIMARY KEY,
	applied_at INTEGER NOT NULL
)`

// runMigrations applies any pending NNNN_*.sql files in fsys against db.
// Each migration runs in its own transaction. The schema_version table is
// created on demand and tracks applied migrations.
func runMigrations(db *sql.DB, fsys fs.FS) error {
	if _, err := db.Exec(createSchemaVersionTable); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	current, err := currentSchemaVersion(db)
	if err != nil {
		return err
	}
	files, err := listMigrationFiles(fsys)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.version <= current {
			continue
		}
		if err := applyMigration(db, fsys, f); err != nil {
			return fmt.Errorf("apply %s: %w", f.name, err)
		}
	}
	return nil
}

func currentSchemaVersion(db *sql.DB) (int, error) {
	var v sql.NullInt64
	if err := db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&v); err != nil {
		return 0, fmt.Errorf("query schema_version: %w", err)
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}

type migrationFile struct {
	version int
	name    string // file name within fsys (e.g. "migrations/0001_initial.sql")
}

func listMigrationFiles(fsys fs.FS) ([]migrationFile, error) {
	var out []migrationFile
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}
		base := pathBase(path)
		ver, err := parseLeadingVersion(base)
		if err != nil {
			return fmt.Errorf("migration filename %q: %w", base, err)
		}
		out = append(out, migrationFile{version: ver, name: path})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func pathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func parseLeadingVersion(name string) (int, error) {
	// Expect NNNN_name.sql
	idx := strings.Index(name, "_")
	if idx <= 0 {
		return 0, fmt.Errorf("missing leading number")
	}
	n, err := strconv.Atoi(name[:idx])
	if err != nil {
		return 0, fmt.Errorf("parse leading number: %w", err)
	}
	return n, nil
}

func applyMigration(db *sql.DB, fsys fs.FS, f migrationFile) error {
	body, err := fs.ReadFile(fsys, f.name)
	if err != nil {
		return fmt.Errorf("read %s: %w", f.name, err)
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if _, err := tx.Exec(string(body)); err != nil {
		_ = tx.Rollback()
		return err
	}
	_, err = tx.Exec(
		"INSERT INTO schema_version(version, applied_at) VALUES (?, ?)",
		f.version, time.Now().Unix(),
	)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
```

Note: this file references `migrations/*.sql` which doesn't exist yet. The compile will succeed because `//go:embed` against a glob that matches no files is allowed at compile time only when using `embed.FS` with an explicit pattern — but `embed` actually errors on no-matching-files. To avoid that until Task 10 lands, **the migrations directory must exist with at least one file**. We'll create a placeholder in the next step and replace it in Task 10.

- [ ] **Step 9.5: Create placeholder migrations dir**

```bash
mkdir -p internal/repository/migrations
printf -- '-- placeholder\n' > internal/repository/migrations/.keep.sql
```

Note the filename: it is `.keep.sql` (NOT `.gitkeep`) so that the `//go:embed migrations/*.sql` glob matches it. This will be removed when `0001_initial.sql` is added.

- [ ] **Step 9.6: Run the test — expect PASS**

```bash
go test ./internal/repository/... -run TestRunMigrations_EmptyFS -v
```

Expected: PASS.

But wait — the test uses `fstest.MapFS{}` (empty), not the embedded FS, so the placeholder doesn't matter for this test. It DOES matter for the package to compile against the `//go:embed` directive.

- [ ] **Step 9.7: Add tests for one valid migration, idempotency, malformed SQL**

Append to `migrate_test.go`:

```go
func TestRunMigrations_AppliesValidMigration(t *testing.T) {
	db := openMemory(t)
	fsys := fstest.MapFS{
		"migrations/0001_test.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE foo (id INTEGER PRIMARY KEY)`),
		},
	}
	if err := runMigrations(db, fsys); err != nil {
		t.Fatalf("run: %v", err)
	}
	var v int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&v); err != nil {
		t.Fatalf("schema_version: %v", err)
	}
	if v != 1 {
		t.Errorf("version = %d, want 1", v)
	}
	// foo table exists
	if _, err := db.Exec("INSERT INTO foo(id) VALUES (1)"); err != nil {
		t.Errorf("insert into foo: %v", err)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := openMemory(t)
	fsys := fstest.MapFS{
		"migrations/0001_test.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE foo (id INTEGER PRIMARY KEY)`),
		},
	}
	if err := runMigrations(db, fsys); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := runMigrations(db, fsys); err != nil {
		t.Fatalf("second run: %v", err)
	}
	// CREATE TABLE foo would error on re-apply, so the second pass must
	// have correctly skipped 0001_test.sql.
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("schema_version rows = %d, want 1", n)
	}
}

func TestRunMigrations_MalformedSQL_ReturnsError(t *testing.T) {
	db := openMemory(t)
	fsys := fstest.MapFS{
		"migrations/0001_bad.sql": &fstest.MapFile{
			Data: []byte(`THIS IS NOT VALID SQL;`),
		},
	}
	err := runMigrations(db, fsys)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var v int
	if dbErr := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&v); dbErr != nil {
		t.Fatalf("query: %v", dbErr)
	}
	if v != 0 {
		t.Errorf("version = %d after failed migration, want 0", v)
	}
}

func TestRunMigrations_OrdersByVersion(t *testing.T) {
	db := openMemory(t)
	fsys := fstest.MapFS{
		"migrations/0002_second.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE bar (id INTEGER PRIMARY KEY)`),
		},
		"migrations/0001_first.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE foo (id INTEGER PRIMARY KEY)`),
		},
	}
	if err := runMigrations(db, fsys); err != nil {
		t.Fatalf("run: %v", err)
	}
	var v int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&v); err != nil {
		t.Fatalf("query: %v", err)
	}
	if v != 2 {
		t.Errorf("version = %d, want 2", v)
	}
}
```

- [ ] **Step 9.8: Run all migrate tests**

```bash
go test ./internal/repository/... -v
```

Expected: all five tests PASS.

- [ ] **Step 9.9: Commit**

```bash
git add go.mod go.sum internal/repository/migrate.go internal/repository/migrate_test.go internal/repository/migrations/.keep.sql
git commit -m "feat(repository): migration runner with fs.FS injection"
```

---

## Task 10: Initial schema migration

**Files:**
- Create: `internal/repository/migrations/0001_initial.sql`
- Delete: `internal/repository/migrations/.keep.sql`

- [ ] **Step 10.1: Write the schema migration**

`internal/repository/migrations/0001_initial.sql`:

```sql
-- 0001_initial.sql — full v1 schema for claude-code-observer.
-- See docs/DATA-MODELS.md for column-level documentation.

CREATE TABLE events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          INTEGER NOT NULL,
    session_id  TEXT NOT NULL,
    prompt_id   TEXT,
    event_name  TEXT NOT NULL,
    attrs       TEXT NOT NULL
);
CREATE INDEX idx_events_session_ts ON events(session_id, ts);
CREATE INDEX idx_events_prompt     ON events(prompt_id);
CREATE INDEX idx_events_name_ts    ON events(event_name, ts);

CREATE TABLE sessions (
    session_id            TEXT PRIMARY KEY,
    project_name          TEXT,
    project_cwd           TEXT,
    started_at            INTEGER NOT NULL,
    last_seen_at          INTEGER NOT NULL,
    ended_at              INTEGER,
    app_version           TEXT,
    os_type               TEXT,
    user_id               TEXT,
    cost_usd              REAL    NOT NULL DEFAULT 0,
    input_tokens          INTEGER NOT NULL DEFAULT 0,
    output_tokens         INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens     INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    api_requests          INTEGER NOT NULL DEFAULT 0,
    api_errors            INTEGER NOT NULL DEFAULT 0,
    subagent_requests     INTEGER NOT NULL DEFAULT 0,
    auxiliary_requests    INTEGER NOT NULL DEFAULT 0,
    tool_calls            INTEGER NOT NULL DEFAULT 0,
    tool_denied           INTEGER NOT NULL DEFAULT 0,
    prompts               INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_sessions_started         ON sessions(started_at DESC);
CREATE INDEX idx_sessions_project_started ON sessions(project_name, started_at DESC);

CREATE TABLE prompts (
    prompt_id             TEXT PRIMARY KEY,
    session_id            TEXT NOT NULL,
    started_at            INTEGER NOT NULL,
    ended_at              INTEGER,
    prompt_length         INTEGER,
    command_name          TEXT,
    command_source        TEXT,
    cost_usd              REAL    NOT NULL DEFAULT 0,
    input_tokens          INTEGER NOT NULL DEFAULT 0,
    output_tokens         INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens     INTEGER NOT NULL DEFAULT 0,
    cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
    api_requests          INTEGER NOT NULL DEFAULT 0,
    subagent_requests     INTEGER NOT NULL DEFAULT 0,
    tool_calls            INTEGER NOT NULL DEFAULT 0,
    had_error             INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (session_id) REFERENCES sessions(session_id)
);
CREATE INDEX idx_prompts_session_started ON prompts(session_id, started_at);

CREATE TABLE metric_snapshots (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          INTEGER NOT NULL,
    session_id  TEXT,
    metric_name TEXT NOT NULL,
    value       REAL NOT NULL,
    attrs       TEXT NOT NULL
);
```

- [ ] **Step 10.2: Remove the placeholder**

```bash
rm internal/repository/migrations/.keep.sql
```

- [ ] **Step 10.3: Add a test that the embedded FS applies cleanly**

Append to `migrate_test.go`:

```go
func TestRunMigrations_EmbeddedInitial(t *testing.T) {
	db := openMemory(t)
	sub, err := fs.Sub(migrationsFS, ".")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	if err := runMigrations(db, sub); err != nil {
		t.Fatalf("run: %v", err)
	}
	var v int
	if err := db.QueryRow("SELECT MAX(version) FROM schema_version").Scan(&v); err != nil {
		t.Fatalf("schema_version: %v", err)
	}
	if v != 1 {
		t.Errorf("version = %d, want 1", v)
	}

	for _, table := range []string{"events", "sessions", "prompts", "metric_snapshots"} {
		var n int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&n)
		if err != nil {
			t.Fatalf("check %s: %v", table, err)
		}
		if n != 1 {
			t.Errorf("table %s: count=%d, want 1", table, n)
		}
	}

	wantIndexes := []string{
		"idx_events_session_ts", "idx_events_prompt", "idx_events_name_ts",
		"idx_sessions_started", "idx_sessions_project_started",
		"idx_prompts_session_started",
	}
	for _, ix := range wantIndexes {
		var n int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?`, ix,
		).Scan(&n)
		if err != nil {
			t.Fatalf("check %s: %v", ix, err)
		}
		if n != 1 {
			t.Errorf("index %s: count=%d, want 1", ix, n)
		}
	}
}
```

Add the `"io/fs"` import at the top of `migrate_test.go`:

```go
import (
	"database/sql"
	"io/fs"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)
```

- [ ] **Step 10.4: Run all repository tests**

```bash
go test ./internal/repository/... -v
```

Expected: all six tests PASS.

- [ ] **Step 10.5: Commit**

```bash
git add internal/repository/migrations/
git commit -m "feat(repository): initial v1 schema migration"
```

---

## Task 11: `Repository.Open` wires migrations + PRAGMAs

**Files:**
- Create: `internal/repository/repository.go`
- Create: `internal/repository/repository_test.go`

- [ ] **Step 11.1: Write the failing tests**

`internal/repository/repository_test.go`:

```go
package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_CreatesDatabaseFile(t *testing.T) {
	home := t.TempDir()
	repo, err := Open(home)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer repo.Close()
	want := filepath.Join(home, "db.sqlite")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected %s to exist: %v", want, err)
	}
}

func TestOpen_AppliesMigrations(t *testing.T) {
	repo, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer repo.Close()
	var v int
	err = repo.DB().QueryRow("SELECT MAX(version) FROM schema_version").Scan(&v)
	if err != nil {
		t.Fatalf("schema_version: %v", err)
	}
	if v != 1 {
		t.Errorf("schema_version = %d, want 1", v)
	}
}

func TestOpen_Idempotent(t *testing.T) {
	home := t.TempDir()

	repo1, err := Open(home)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	repo1.Close()

	repo2, err := Open(home)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer repo2.Close()

	var n int
	if err := repo2.DB().QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Errorf("schema_version rows = %d, want 1", n)
	}
}

func TestOpen_WALModeAndForeignKeys(t *testing.T) {
	repo, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer repo.Close()

	var mode string
	if err := repo.DB().QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %s, want wal", mode)
	}

	var fk int
	if err := repo.DB().QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestOpen_CreatesHomeDirIfMissing(t *testing.T) {
	parent := t.TempDir()
	home := filepath.Join(parent, "nested", "home")
	repo, err := Open(home)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer repo.Close()
	if _, err := os.Stat(home); err != nil {
		t.Errorf("home dir not created: %v", err)
	}
}
```

- [ ] **Step 11.2: Run — expect FAIL with `undefined: Open`**

```bash
go test ./internal/repository/... -run TestOpen -v
```

Expected: FAIL.

- [ ] **Step 11.3: Implement `Repository.Open`**

`internal/repository/repository.go`:

```go
// Package repository owns all SQLite access.
package repository

import (
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Repository is a handle to the local SQLite store.
type Repository struct {
	db *sql.DB
}

// Open ensures the home directory and database file exist, opens a pooled
// SQLite connection in WAL mode with foreign keys enabled, and applies any
// pending migrations.
func Open(home string) (*Repository, error) {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil, fmt.Errorf("create home dir: %w", err)
	}

	dbPath := filepath.Join(home, "db.sqlite")
	dsn := buildDSN(dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	migFS, err := fs.Sub(migrationsFS, ".")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("locate migrations: %w", err)
	}
	if err := runMigrations(db, migFS); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Repository{db: db}, nil
}

// Close releases the underlying database connection pool.
func (r *Repository) Close() error {
	if r.db == nil {
		return nil
	}
	return r.db.Close()
}

// DB exposes the underlying *sql.DB. Phase 0 callers and tests use this; later
// phases will add typed methods and stop reaching into the pool directly.
func (r *Repository) DB() *sql.DB { return r.db }

func buildDSN(path string) string {
	q := url.Values{}
	q.Set("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	return fmt.Sprintf("file:%s?%s", path, q.Encode())
}
```

- [ ] **Step 11.4: Run repository tests**

```bash
go test ./internal/repository/... -v
```

Expected: all repository tests PASS.

- [ ] **Step 11.5: Run vet + full test suite**

```bash
go vet ./...
go test ./...
go build -o bin/claude-code-observer ./cmd/app
```

All exit 0.

- [ ] **Step 11.6: Commit**

```bash
git add internal/repository/repository.go internal/repository/repository_test.go
git commit -m "feat(repository): Open with migrations, WAL, foreign keys"
```

---

## Task 12: Wire `Repository.Open` into `serve`

**Files:**
- Modify: `cmd/app/serve.go`

- [ ] **Step 12.1: Update `cmd/app/serve.go` to open the repository**

Replace the file body with:

```go
package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the OTLP receiver daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			repo, err := repository.Open(homeDir)
			if err != nil {
				return fmt.Errorf("open repository: %w", err)
			}
			defer repo.Close()

			version, err := readSchemaVersion(ctx, repo)
			if err != nil {
				return fmt.Errorf("read schema_version: %w", err)
			}

			logger.Info("daemon started",
				"home", homeDir,
				"binary_version", versionString(),
				"schema_version", version,
			)
			<-ctx.Done()
			logger.Info("daemon stopped")
			return nil
		},
	}
}

func readSchemaVersion(ctx context.Context, repo *repository.Repository) (int, error) {
	var v int
	err := repo.DB().QueryRowContext(ctx, "SELECT MAX(version) FROM schema_version").Scan(&v)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func versionString() string {
	return fmt.Sprintf("%s (commit %s)", version, commit)
}
```

- [ ] **Step 12.2: Build and smoke-test**

```bash
make build
rm -rf /tmp/cco-phase0-demo
CCO_HOME=/tmp/cco-phase0-demo ./bin/claude-code-observer serve &
SERVE_PID=$!
sleep 1
kill -INT $SERVE_PID
wait $SERVE_PID
ls /tmp/cco-phase0-demo
```

Expected:
- stderr contains JSON log lines for `daemon started` (with `home`, `schema_version=1`) and `daemon stopped`
- `/tmp/cco-phase0-demo/db.sqlite` exists (and possibly `db.sqlite-shm`, `db.sqlite-wal`)

- [ ] **Step 12.3: Verify schema in the produced DB**

```bash
sqlite3 /tmp/cco-phase0-demo/db.sqlite ".tables"
sqlite3 /tmp/cco-phase0-demo/db.sqlite "SELECT version FROM schema_version"
sqlite3 /tmp/cco-phase0-demo/db.sqlite "PRAGMA journal_mode"
sqlite3 /tmp/cco-phase0-demo/db.sqlite "PRAGMA foreign_keys"
```

Expected:
- `.tables` lists `events`, `metric_snapshots`, `prompts`, `schema_version`, `sessions`
- `SELECT version` returns `1`
- `journal_mode` is `wal`
- `foreign_keys` is `1`

If `sqlite3` is not installed: `brew install sqlite` on macOS, or skip and rely on the test suite.

- [ ] **Step 12.4: Run full verification**

```bash
go vet ./...
go test ./...
go build -o bin/claude-code-observer ./cmd/app
```

All exit 0.

- [ ] **Step 12.5: Commit**

```bash
git add cmd/app/serve.go
git commit -m "feat(cmd): serve opens repository and logs schema_version"
```

---

## Task 13: Phase 0 demo end-to-end

This is the formal acceptance for Phase 0. No code changes — just running and recording the result.

- [ ] **Step 13.1: Clean build**

```bash
make clean
make build
make vet
make test
```

All four exit 0.

- [ ] **Step 13.2: Coverage check on `internal/repository`**

```bash
go test -cover ./internal/repository/...
```

Expected: coverage **≥ 80%** on `internal/repository/`. If below, identify uncovered branches in `migrate.go` or `repository.go` and add a test before considering Phase 0 done.

- [ ] **Step 13.3: Run the spec demo for M0.1**

```bash
./bin/claude-code-observer --help
./bin/claude-code-observer version
```

Confirm:
- Help lists `init`, `rebuild-rollups`, `serve`, `version`
- Version prints non-empty version + commit

- [ ] **Step 13.4: Run the spec demo for M0.2**

```bash
rm -rf /tmp/cco-final-demo
CCO_HOME=/tmp/cco-final-demo ./bin/claude-code-observer serve &
PID=$!
sleep 1
kill -INT $PID
wait $PID

sqlite3 /tmp/cco-final-demo/db.sqlite ".schema events"
sqlite3 /tmp/cco-final-demo/db.sqlite ".schema sessions"
sqlite3 /tmp/cco-final-demo/db.sqlite ".schema prompts"
sqlite3 /tmp/cco-final-demo/db.sqlite ".schema metric_snapshots"
sqlite3 /tmp/cco-final-demo/db.sqlite "SELECT version FROM schema_version"
sqlite3 /tmp/cco-final-demo/db.sqlite "PRAGMA journal_mode"

# Restart and confirm idempotency
CCO_HOME=/tmp/cco-final-demo ./bin/claude-code-observer serve &
PID=$!
sleep 1
kill -INT $PID
wait $PID
sqlite3 /tmp/cco-final-demo/db.sqlite "SELECT COUNT(*) FROM schema_version"
```

Expected:
- All four `.schema` outputs match the columns and indexes from `docs/DATA-MODELS.md`
- `SELECT version` is `1`
- `journal_mode` is `wal`
- `COUNT(*) FROM schema_version` is `1` (no double-application after restart)

- [ ] **Step 13.5: Mark Phase 0 complete**

If every check above passed, Phase 0 is done. Optionally tag the commit:

```bash
git tag -a phase-0-complete -m "Phase 0: foundation + repository" || true
```

---

## Self-Review Notes

Reviewed against the spec:

- **Stack decisions** — all locked stack choices appear in the relevant tasks (cobra in Task 3, modernc.org/sqlite in Task 9, slog in Task 3, embedded migrations in Tasks 9–10). ✅
- **M0.1 deliverables** — go.mod (Task 1), Makefile (Task 1), cobra root + 4 subcommands + version (Tasks 3–6), domain types (Task 2), internal stubs (Task 8). ✅
- **M0.1 test gate** — cobra wiring smoke tests in Task 7. ✅
- **M0.2 deliverables** — Repository.Open (Task 11), 0001_initial.sql (Task 10), embedded migrations (Task 9), WAL + FK (Task 11), serve wires it (Task 12). ✅
- **M0.2 test gate** — migration on empty DB (Task 9), idempotent (Task 9), all indexes (Task 10), WAL + FK (Task 11), bad migration recovery (Task 9). All present. ✅
- **Coverage ≥80% on `internal/repository`** — explicit check in Task 13. ✅
- **Demo scripts** — embedded in Tasks 6, 12, and 13. ✅

No placeholders. No "similar to". All code blocks are complete.

One callout: the spec referenced `ToolResult` as a domain type but flagged it as redundant. The plan honors the redundancy fix — `ToolResult` is not in `domain/types.go`. Tools land in the events table as `event_name="claude_code.tool_result"`.

---

## Execution Handoff

Plan complete and saved to `docs/plans/phase-0-foundation.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
