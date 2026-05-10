# Phase 1 — Ingest Path Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Real Claude Code can send OTLP/gRPC to `127.0.0.1:4317` and rows land in `events` and `metric_snapshots`.

**Architecture:** Three-layer pipeline. `internal/receiver` is a thin gRPC OTLP server that hands `Export*ServiceRequest` to a consumer-defined ingester interface. `internal/service.Service` parses each record via the pure `internal/eventparser` package, then writes survivors into one SQLite transaction via `internal/repository`. Parse failures (missing `session.id`) warn-and-skip; DB failures return `codes.Unavailable` so the OTLP exporter retries the whole batch.

**Tech Stack:** Go 1.25, `google.golang.org/grpc`, `go.opentelemetry.io/proto/otlp`, `modernc.org/sqlite` (already in go.mod), cobra (already in go.mod), `slog`. Tests use `bufconn` for in-memory gRPC and a temp SQLite file for repository integration.

**Spec:** [docs/superpowers/specs/2026-05-10-phase-1-ingest-design.md](../specs/2026-05-10-phase-1-ingest-design.md)

---

## File Map

**Created:**
- `internal/receiver/server.go` — gRPC server bootstrap (Listen + Start + Stop)
- `internal/receiver/ingester.go` — `LogIngester` and `MetricIngester` consumer interfaces
- `internal/receiver/logs.go` — `LogsServiceServer` impl, delegates to `LogIngester`
- `internal/receiver/metrics.go` — `MetricsServiceServer` impl, delegates to `MetricIngester`
- `internal/receiver/server_test.go` — bufconn tests for both services and Server lifecycle
- `internal/eventparser/parser.go` — `Parse` entry point, `ErrDrop` sentinel, helpers
- `internal/eventparser/dispatch.go` — `event.name` → typed-extraction dispatch
- `internal/eventparser/attrs.go` — KeyValue → `map[string]any` flattening utilities
- `internal/eventparser/parser_test.go` — table-driven tests using fixtures + inline cases
- `internal/eventparser/testdata/fixtures/*.json` — captured real Claude payloads (one LogRecord per file)
- `internal/repository/events.go` — `InsertEvents` and `InsertMetricSnapshots` typed methods
- `internal/repository/events_test.go` — temp-SQLite integration tests for both methods
- `internal/service/service.go` — `Service` struct + `IngestLogs` + `IngestMetrics`
- `internal/service/service_test.go` — unit tests with fake ingester deps; e2e integration test
- `cmd/capture-fixtures/main.go` — one-shot OTLP listener that dumps each request as JSON
- `cmd/parser-debug/main.go` — reads a fixture JSON and prints the parsed `domain.Event`

**Modified:**
- `go.mod` / `go.sum` — add gRPC and OTLP proto deps
- `cmd/app/serve.go` — wire receiver → service → repository
- `internal/receiver/doc.go` — keep, but content gets replaced by `server.go` package comment
- `internal/eventparser/doc.go` — same
- `internal/service/doc.go` — same

---

## Conventions for every task

- Every task ends with a commit. Use [Conventional Commits](https://www.conventionalcommits.org/) prefixes consistent with the existing log: `feat(<pkg>)`, `test(<pkg>)`, `chore(<pkg>)`, `docs(<pkg>)`.
- After each implementation step, run the verification steps from `CLAUDE.md`:
  - `go vet ./...`
  - `go test ./<package>/...`
  - `go build -o bin/cco ./cmd/app`
- Errors are returned wrapped: `fmt.Errorf("context: %w", err)`.
- No `interface{}` / `any` without a `// reason: …` comment.

---

# Milestone M1.1 — gRPC OTLP receiver

Goal at end of M1.1: `cco serve` listens on `127.0.0.1:4317`. Both Logs and Metrics gRPC services accept requests, log a one-line summary, and return success. **No DB writes yet.** All tests use `bufconn`.

## Task 1: Add gRPC and OTLP proto dependencies

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`

- [ ] **Step 1: Add the two dependencies**

Run:

```bash
go get google.golang.org/grpc@latest
go get go.opentelemetry.io/proto/otlp@latest
go mod tidy
```

- [ ] **Step 2: Verify deps**

Run: `go build ./...`
Expected: clean build, no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add grpc and otlp proto deps for Phase 1 receiver"
```

## Task 2: Define receiver ingester interfaces

**Files:**
- Create: `internal/receiver/ingester.go`

The receiver must not import `internal/service` directly. It defines what it needs and lets the caller satisfy it (consumer-side interface, per CLAUDE.md).

- [ ] **Step 1: Write the interfaces**

Create `internal/receiver/ingester.go`:

```go
// Package receiver implements the OTLP/gRPC server. It accepts
// ExportLogsServiceRequest and ExportMetricsServiceRequest payloads from a
// local Claude Code process and hands them off to caller-supplied ingester
// implementations. The receiver is intentionally thin: parsing and persistence
// live in internal/service and internal/repository.
package receiver

import (
	"context"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

// LogIngester accepts a parsed OTLP logs export request. Implementations are
// responsible for parsing, persisting, and any error translation. A non-nil
// error from IngestLogs is mapped to gRPC codes.Unavailable so OTLP clients
// retry the entire batch.
type LogIngester interface {
	IngestLogs(ctx context.Context, req *collogspb.ExportLogsServiceRequest) error
}

// MetricIngester accepts a parsed OTLP metrics export request. Same retry
// semantics as LogIngester.
type MetricIngester interface {
	IngestMetrics(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) error
}
```

- [ ] **Step 2: Delete the old doc.go**

```bash
rm internal/receiver/doc.go
```

- [ ] **Step 3: Verify it compiles**

Run: `go build ./internal/receiver/...`
Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add internal/receiver/
git commit -m "feat(receiver): add LogIngester and MetricIngester interfaces"
```

## Task 3: Implement LogsServiceServer

**Files:**
- Create: `internal/receiver/logs.go`
- Create: `internal/receiver/server_test.go` (initial)

- [ ] **Step 1: Write the failing test**

Create `internal/receiver/server_test.go`:

```go
package receiver

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

const bufSize = 1024 * 1024

type fakeLogIngester struct {
	called int
	err    error
}

func (f *fakeLogIngester) IngestLogs(ctx context.Context, req *collogspb.ExportLogsServiceRequest) error {
	f.called++
	return f.err
}

func newLogsTestServer(t *testing.T, ing LogIngester) collogspb.LogsServiceClient {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	gs := grpc.NewServer()
	collogspb.RegisterLogsServiceServer(gs, NewLogsServer(ing))
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return collogspb.NewLogsServiceClient(conn)
}

func TestLogsServer_Empty(t *testing.T) {
	ing := &fakeLogIngester{}
	cli := newLogsTestServer(t, ing)

	_, err := cli.Export(context.Background(), &collogspb.ExportLogsServiceRequest{})
	if err != nil {
		t.Fatalf("Export error: %v", err)
	}
	if ing.called != 1 {
		t.Fatalf("ingester called %d times, want 1", ing.called)
	}
}

func TestLogsServer_WellFormed(t *testing.T) {
	ing := &fakeLogIngester{}
	cli := newLogsTestServer(t, ing)

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			ScopeLogs: []*logspb.ScopeLogs{{
				LogRecords: []*logspb.LogRecord{{TimeUnixNano: 1}},
			}},
		}},
	}
	if _, err := cli.Export(context.Background(), req); err != nil {
		t.Fatalf("Export error: %v", err)
	}
	if ing.called != 1 {
		t.Fatalf("ingester called %d times, want 1", ing.called)
	}
}
```

- [ ] **Step 2: Run the test (expect compile failure)**

Run: `go test ./internal/receiver/ -run TestLogsServer -v`
Expected: FAIL — `undefined: NewLogsServer`.

- [ ] **Step 3: Implement the server**

Create `internal/receiver/logs.go`:

```go
package receiver

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
)

// LogsServer satisfies collogspb.LogsServiceServer. It counts incoming records
// for observability and delegates the request to the configured LogIngester.
type LogsServer struct {
	collogspb.UnimplementedLogsServiceServer
	ing LogIngester
	log *slog.Logger
}

// NewLogsServer constructs a LogsServer. Pass a nil logger to disable logging.
func NewLogsServer(ing LogIngester) *LogsServer {
	return &LogsServer{ing: ing, log: slog.Default()}
}

// WithLogger returns a copy of s with the supplied logger.
func (s *LogsServer) WithLogger(log *slog.Logger) *LogsServer {
	cp := *s
	cp.log = log
	return &cp
}

// Export implements LogsServiceServer.Export.
func (s *LogsServer) Export(ctx context.Context, req *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	records := countLogRecords(req)
	if s.log != nil {
		s.log.Debug("otlp logs export",
			"resource_logs", len(req.GetResourceLogs()),
			"records", records,
		)
	}
	if err := s.ing.IngestLogs(ctx, req); err != nil {
		if s.log != nil {
			s.log.Warn("ingest logs failed", "err", err, "records", records)
		}
		return nil, status.Errorf(codes.Unavailable, "ingest logs: %v", err)
	}
	return &collogspb.ExportLogsServiceResponse{}, nil
}

func countLogRecords(req *collogspb.ExportLogsServiceRequest) int {
	n := 0
	for _, rl := range req.GetResourceLogs() {
		for _, sl := range rl.GetScopeLogs() {
			n += len(sl.GetLogRecords())
		}
	}
	return n
}
```

- [ ] **Step 4: Run the tests, expect pass**

Run: `go test ./internal/receiver/ -run TestLogsServer -v`
Expected: PASS for both.

- [ ] **Step 5: Add malformed-input test**

Append to `internal/receiver/server_test.go`:

```go
func TestLogsServer_IngesterError(t *testing.T) {
	ing := &fakeLogIngester{err: errBoom}
	cli := newLogsTestServer(t, ing)

	_, err := cli.Export(context.Background(), &collogspb.ExportLogsServiceRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := status.Code(err); got != codes.Unavailable {
		t.Fatalf("status code = %v, want Unavailable", got)
	}
}
```

And add the imports + sentinel at the top of the test file (just after existing imports):

```go
import (
	// ...existing...
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var errBoom = errors.New("boom")
```

Run: `go test ./internal/receiver/ -run TestLogsServer -v`
Expected: all three pass.

- [ ] **Step 6: Commit**

```bash
git add internal/receiver/logs.go internal/receiver/server_test.go
git commit -m "feat(receiver): add LogsServiceServer with ingester delegation"
```

## Task 4: Implement MetricsServiceServer

**Files:**
- Create: `internal/receiver/metrics.go`
- Modify: `internal/receiver/server_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/receiver/server_test.go`:

```go
import (
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

type fakeMetricIngester struct {
	called int
	err    error
}

func (f *fakeMetricIngester) IngestMetrics(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) error {
	f.called++
	return f.err
}

func newMetricsTestServer(t *testing.T, ing MetricIngester) colmetricspb.MetricsServiceClient {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	gs := grpc.NewServer()
	colmetricspb.RegisterMetricsServiceServer(gs, NewMetricsServer(ing))
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return colmetricspb.NewMetricsServiceClient(conn)
}

func TestMetricsServer_WellFormed(t *testing.T) {
	ing := &fakeMetricIngester{}
	cli := newMetricsTestServer(t, ing)

	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{{
			ScopeMetrics: []*metricspb.ScopeMetrics{{
				Metrics: []*metricspb.Metric{{Name: "claude_code.cost.usage"}},
			}},
		}},
	}
	if _, err := cli.Export(context.Background(), req); err != nil {
		t.Fatalf("Export error: %v", err)
	}
	if ing.called != 1 {
		t.Fatalf("ingester called %d times, want 1", ing.called)
	}
}

func TestMetricsServer_IngesterError(t *testing.T) {
	ing := &fakeMetricIngester{err: errBoom}
	cli := newMetricsTestServer(t, ing)

	_, err := cli.Export(context.Background(), &colmetricspb.ExportMetricsServiceRequest{})
	if err == nil {
		t.Fatal("want error")
	}
	if got := status.Code(err); got != codes.Unavailable {
		t.Fatalf("status = %v, want Unavailable", got)
	}
}
```

- [ ] **Step 2: Run, expect compile failure**

Run: `go test ./internal/receiver/ -run TestMetricsServer -v`
Expected: FAIL — `undefined: NewMetricsServer`.

- [ ] **Step 3: Implement the metrics server**

Create `internal/receiver/metrics.go`:

```go
package receiver

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

// MetricsServer satisfies colmetricspb.MetricsServiceServer. Mirrors LogsServer.
type MetricsServer struct {
	colmetricspb.UnimplementedMetricsServiceServer
	ing MetricIngester
	log *slog.Logger
}

// NewMetricsServer constructs a MetricsServer with slog.Default() as the logger.
func NewMetricsServer(ing MetricIngester) *MetricsServer {
	return &MetricsServer{ing: ing, log: slog.Default()}
}

// WithLogger returns a copy of s with the supplied logger.
func (s *MetricsServer) WithLogger(log *slog.Logger) *MetricsServer {
	cp := *s
	cp.log = log
	return &cp
}

// Export implements MetricsServiceServer.Export.
func (s *MetricsServer) Export(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) (*colmetricspb.ExportMetricsServiceResponse, error) {
	count := countMetricDatapoints(req)
	if s.log != nil {
		s.log.Debug("otlp metrics export",
			"resource_metrics", len(req.GetResourceMetrics()),
			"datapoints", count,
		)
	}
	if err := s.ing.IngestMetrics(ctx, req); err != nil {
		if s.log != nil {
			s.log.Warn("ingest metrics failed", "err", err)
		}
		return nil, status.Errorf(codes.Unavailable, "ingest metrics: %v", err)
	}
	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}

func countMetricDatapoints(req *colmetricspb.ExportMetricsServiceRequest) int {
	n := 0
	for _, rm := range req.GetResourceMetrics() {
		for _, sm := range rm.GetScopeMetrics() {
			n += len(sm.GetMetrics())
		}
	}
	return n
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/receiver/ -v`
Expected: PASS — all five tests.

- [ ] **Step 5: Commit**

```bash
git add internal/receiver/metrics.go internal/receiver/server_test.go
git commit -m "feat(receiver): add MetricsServiceServer with ingester delegation"
```

## Task 5: Server bootstrap (Listen + Start + Stop)

**Files:**
- Create: `internal/receiver/server.go`
- Modify: `internal/receiver/server_test.go`

The Server struct ties an `address` and an ingester pair to a real TCP listener.

- [ ] **Step 1: Write the failing test**

Append to `internal/receiver/server_test.go`:

```go
func TestServer_StartAndStop(t *testing.T) {
	srv := NewServer(Config{
		Addr:    "127.0.0.1:0", // random free port
		Logs:    &fakeLogIngester{},
		Metrics: &fakeMetricIngester{},
	})
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := srv.Addr()
	if addr == "" || addr == "127.0.0.1:0" {
		t.Fatalf("Addr() = %q, want resolved port", addr)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve() }()

	srv.Stop()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve returned %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after Stop")
	}
}
```

Add `"time"` to the test file imports.

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/receiver/ -run TestServer_StartAndStop -v`
Expected: FAIL — `undefined: NewServer`.

- [ ] **Step 3: Implement Server**

Create `internal/receiver/server.go`:

```go
package receiver

import (
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

// Config wires a Server to its dependencies.
type Config struct {
	Addr    string         // e.g. "127.0.0.1:4317"
	Logs    LogIngester    // required
	Metrics MetricIngester // required
	Logger  *slog.Logger   // nil → slog.Default()
}

// Server runs the OTLP/gRPC receiver. Lifecycle: Listen → Serve (blocking) → Stop.
// Splitting Listen and Serve lets callers learn the chosen port (handy for tests
// that bind 127.0.0.1:0) before traffic starts.
type Server struct {
	cfg Config
	gs  *grpc.Server
	lis net.Listener
}

// NewServer constructs a Server. Listen must be called before Serve.
func NewServer(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Server{cfg: cfg}
}

// Listen binds the TCP listener. After Listen, Addr returns the resolved address.
func (s *Server) Listen() error {
	lis, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.Addr, err)
	}
	s.lis = lis
	s.gs = grpc.NewServer()
	collogspb.RegisterLogsServiceServer(s.gs, NewLogsServer(s.cfg.Logs).WithLogger(s.cfg.Logger))
	colmetricspb.RegisterMetricsServiceServer(s.gs, NewMetricsServer(s.cfg.Metrics).WithLogger(s.cfg.Logger))
	return nil
}

// Addr returns the resolved listening address. Empty until Listen runs.
func (s *Server) Addr() string {
	if s.lis == nil {
		return ""
	}
	return s.lis.Addr().String()
}

// Serve blocks until Stop is called or the listener errors.
func (s *Server) Serve() error {
	if s.gs == nil || s.lis == nil {
		return fmt.Errorf("server: Listen not called")
	}
	if err := s.gs.Serve(s.lis); err != nil {
		return fmt.Errorf("grpc serve: %w", err)
	}
	return nil
}

// Stop performs a graceful stop of the gRPC server.
func (s *Server) Stop() {
	if s.gs != nil {
		s.gs.GracefulStop()
	}
}
```

- [ ] **Step 4: Run all receiver tests**

Run: `go test ./internal/receiver/ -v`
Expected: all pass.

- [ ] **Step 5: Verify**

Run: `go vet ./...`
Run: `go build -o bin/cco ./cmd/app`

- [ ] **Step 6: Commit**

```bash
git add internal/receiver/server.go internal/receiver/server_test.go
git commit -m "feat(receiver): add Server lifecycle (Listen/Serve/Stop)"
```

## Task 6: Wire receiver into `cco serve` (with stub ingesters)

**Files:**
- Modify: `cmd/app/serve.go`

For M1.1, ingester implementations are inline stubs that just log the count. The real Service comes in M1.3.

- [ ] **Step 1: Edit serve.go**

Replace `cmd/app/serve.go` with:

```go
package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/receiver"
	"github.com/kamikaze011001/claude-code-observer/internal/repository"
)

const defaultListenAddr = "127.0.0.1:4317"

func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
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

			schemaVersion, err := readSchemaVersion(ctx, repo)
			if err != nil {
				return fmt.Errorf("read schema_version: %w", err)
			}

			srv := receiver.NewServer(receiver.Config{
				Addr:    addr,
				Logs:    &logStubIngester{},
				Metrics: &metricStubIngester{},
				Logger:  logger,
			})
			if err := srv.Listen(); err != nil {
				return fmt.Errorf("receiver listen: %w", err)
			}
			logger.Info("daemon started",
				"home", homeDir,
				"binary_version", versionString(),
				"schema_version", schemaVersion,
				"otlp_addr", srv.Addr(),
			)

			errCh := make(chan error, 1)
			go func() { errCh <- srv.Serve() }()

			select {
			case <-ctx.Done():
				srv.Stop()
				<-errCh
			case err := <-errCh:
				if err != nil {
					return err
				}
			}
			logger.Info("daemon stopped")
			return nil
		},
	}
	cmd.Flags().StringVar(&addr, "listen", defaultListenAddr, "OTLP/gRPC listen address")
	return cmd
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

// logStubIngester is the M1.1 placeholder. Replaced by service.Service in M1.3.
type logStubIngester struct{}

func (logStubIngester) IngestLogs(_ context.Context, req *collogspb.ExportLogsServiceRequest) error {
	logger.Info("logs received (stub)", "resource_logs", len(req.GetResourceLogs()))
	return nil
}

type metricStubIngester struct{}

func (metricStubIngester) IngestMetrics(_ context.Context, req *colmetricspb.ExportMetricsServiceRequest) error {
	logger.Info("metrics received (stub)", "resource_metrics", len(req.GetResourceMetrics()))
	return nil
}
```

- [ ] **Step 2: Build**

Run: `go build -o bin/cco ./cmd/app`
Expected: clean build.

- [ ] **Step 3: Manual smoke test**

Run: `./bin/cco serve` in one terminal.
Expected output: `... otlp_addr=127.0.0.1:4317 ... daemon started ...`

In another terminal:

```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest 2>/dev/null
grpcurl -plaintext localhost:4317 list
```

Expected (order may vary):

```
grpc.reflection.v1alpha.ServerReflection
opentelemetry.proto.collector.logs.v1.LogsService
opentelemetry.proto.collector.metrics.v1.MetricsService
```

If you see only the first or get "unimplemented", grpc reflection isn't on yet — that's fine for M1.1; what matters is `cco` is listening. Confirm with: `nc -z 127.0.0.1 4317 && echo open` → `open`.

Stop the daemon with Ctrl-C; expected log: `daemon stopped`.

- [ ] **Step 4: Verify lint + tests**

Run: `go vet ./... && go test ./...`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add cmd/app/serve.go
git commit -m "feat(cmd): wire OTLP receiver into serve with stub ingesters"
```

## Task 7: `cmd/capture-fixtures` tool

This tool reuses `internal/receiver` with a custom ingester that writes each `ExportLogsServiceRequest` to a JSON file. Used by you (the developer) to capture real Claude payloads — those become the testdata for M1.2.

**Files:**
- Create: `cmd/capture-fixtures/main.go`

- [ ] **Step 1: Write the tool**

Create `cmd/capture-fixtures/main.go`:

```go
// capture-fixtures runs an OTLP/gRPC server and writes every received
// ExportLogsServiceRequest (and ExportMetricsServiceRequest) to a timestamped
// JSON file under the configured output directory. Used to gather real
// fixtures for internal/eventparser tests.
//
// Usage:
//
//	go run ./cmd/capture-fixtures --out internal/eventparser/testdata/captured
//	# then in another shell:
//	OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317 \
//	OTEL_EXPORTER_OTLP_PROTOCOL=grpc \
//	OTEL_LOGS_EXPORTER=otlp OTEL_METRICS_EXPORTER=otlp \
//	CLAUDE_CODE_ENABLE_TELEMETRY=1 claude
//	# drive a prompt that triggers tool calls, then Ctrl-C this process.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/kamikaze011001/claude-code-observer/internal/receiver"
)

func main() {
	var outDir string
	var addr string
	flag.StringVar(&outDir, "out", "captured", "output directory for JSON dumps")
	flag.StringVar(&addr, "listen", "127.0.0.1:4317", "OTLP/gRPC listen address")
	flag.Parse()

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fail("mkdir: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	logIng := &fileIngester{dir: outDir, kind: "logs", log: logger}
	metricIng := &fileIngester{dir: outDir, kind: "metrics", log: logger}

	srv := receiver.NewServer(receiver.Config{
		Addr:    addr,
		Logs:    logIng,
		Metrics: metricIng,
		Logger:  logger,
	})
	if err := srv.Listen(); err != nil {
		fail("listen: %v", err)
	}
	logger.Info("capture-fixtures listening", "addr", srv.Addr(), "out", outDir)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve() }()
	<-ctx.Done()
	srv.Stop()
	<-errCh
	logger.Info("capture-fixtures stopped",
		"logs_written", logIng.count.Load(),
		"metrics_written", metricIng.count.Load(),
	)
}

type fileIngester struct {
	dir   string
	kind  string
	log   *slog.Logger
	count atomic.Int64
}

func (f *fileIngester) IngestLogs(_ context.Context, req *collogspb.ExportLogsServiceRequest) error {
	return f.write(req)
}

func (f *fileIngester) IngestMetrics(_ context.Context, req *colmetricspb.ExportMetricsServiceRequest) error {
	return f.write(req)
}

// write serializes any protojson-marshalable message under f.dir.
// reason: receiver gives us two distinct generated proto types and we want one helper.
type protoMessage interface {
	ProtoReflect() protoreflect.Message
}

func (f *fileIngester) write(msg any) error {
	pm, ok := msg.(protoMessage)
	if !ok {
		return fmt.Errorf("not a proto message: %T", msg)
	}
	bs, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(pm.(interface {
		ProtoReflect() protoreflect.Message
	}))
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	n := f.count.Add(1)
	name := fmt.Sprintf("%s-%s-%03d.json", f.kind, time.Now().UTC().Format("20060102T150405"), n)
	path := filepath.Join(f.dir, name)
	if err := os.WriteFile(path, bs, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	f.log.Info("captured", "kind", f.kind, "path", path)
	return nil
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "capture-fixtures: "+format+"\n", args...)
	os.Exit(1)
}
```

- [ ] **Step 2: Fix the protoreflect import**

The snippet above uses `protoreflect` without importing. Replace the imports block with:

```go
import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/kamikaze011001/claude-code-observer/internal/receiver"
)
```

And simplify `write` — drop the awkward double type assertion:

```go
func (f *fileIngester) write(msg protoreflect.ProtoMessage) error {
	bs, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	n := f.count.Add(1)
	name := fmt.Sprintf("%s-%s-%03d.json", f.kind, time.Now().UTC().Format("20060102T150405"), n)
	path := filepath.Join(f.dir, name)
	if err := os.WriteFile(path, bs, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	f.log.Info("captured", "kind", f.kind, "path", path)
	return nil
}
```

Then delete the `protoMessage` type and update the two call sites — they already pass `req`, which satisfies `protoreflect.ProtoMessage`.

Final form of the imports + write helpers should not reference the deleted `protoMessage` interface anywhere.

- [ ] **Step 3: Build the tool**

Run: `go build -o bin/capture-fixtures ./cmd/capture-fixtures`
Expected: clean build.

- [ ] **Step 4: Smoke test (no Claude needed yet)**

Run: `./bin/capture-fixtures --out /tmp/cco-cap --listen 127.0.0.1:4319 &`
Then: `nc -z 127.0.0.1 4319 && echo open` → `open`
Then: `kill %1`

- [ ] **Step 5: Commit**

```bash
git add cmd/capture-fixtures/
git commit -m "feat(capture-fixtures): one-shot tool to dump OTLP requests as JSON"
```

## Task 8: Capture real Claude fixtures (manual)

This is a developer task, not a code change. You run Claude against `capture-fixtures` and harvest real payloads.

- [ ] **Step 1: Ensure no other process holds 4317**

Run: `lsof -nP -iTCP:4317 -sTCP:LISTEN`
Expected: no output. Stop any running `cco serve` first.

- [ ] **Step 2: Start capture-fixtures**

```bash
mkdir -p internal/eventparser/testdata/captured
./bin/capture-fixtures --out internal/eventparser/testdata/captured
```

Leave it running.

- [ ] **Step 3: Configure Claude to export OTLP**

In a fresh shell:

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
export OTEL_LOGS_EXPORTER=otlp
export OTEL_METRICS_EXPORTER=otlp
export OTEL_METRIC_EXPORT_INTERVAL=5000
export OTEL_LOGS_EXPORT_INTERVAL=2000
```

- [ ] **Step 4: Drive a session that exercises every event type**

```bash
claude
```

In Claude:
- Ask a small question that needs zero tool use → `user_prompt` + `api_request`.
- Ask: "list files in the cwd with ls" — drives `tool_decision` + `tool_result`.
- Run a slash command (e.g. `/help`) → another `user_prompt` with `command_name` set.
- Trigger a file Edit if comfortable → another `tool_result` with a different `tool_name`.
- Exit Claude (`/exit`).

- [ ] **Step 5: Stop capture-fixtures**

Ctrl-C the capture-fixtures terminal. It logs the count of writes.

- [ ] **Step 6: Curate fixtures**

You should now have a pile of `logs-*.json` and `metrics-*.json` under `internal/eventparser/testdata/captured/`.

For M1.2 we want **one LogRecord per fixture file**, named after the event. Use `jq` to extract:

```bash
cd internal/eventparser/testdata
mkdir -p fixtures

# Find a user_prompt record:
for f in captured/logs-*.json; do
  jq -e '.resourceLogs[].scopeLogs[].logRecords[] | select(.attributes[]? | select(.key=="event.name").value.stringValue == "claude_code.user_prompt")' "$f" | head -50
  echo "--- $f"
done
```

For each event name in the list below, pick one matching record and save it as a single-record file:

```bash
EVENT=claude_code.user_prompt
SRC=captured/logs-...json   # the file you found
OUT=fixtures/user_prompt.json
jq --arg e "$EVENT" '
  .resourceLogs[] as $rl
  | $rl.scopeLogs[].logRecords[]
  | select(.attributes[]? | select(.key=="event.name").value.stringValue == $e)
  | {resource: $rl.resource, scope: {name:"capture"}, record: .}' "$SRC" > "$OUT"
```

Required fixtures (one per file):

- `fixtures/user_prompt.json`
- `fixtures/api_request.json`
- `fixtures/api_error.json` (if you can trigger one — e.g. force a 429 by spamming; otherwise mark TODO and synthesize in Task 18)
- `fixtures/tool_decision.json`
- `fixtures/tool_result.json`
- `fixtures/session_start.json`
- `fixtures/session_end.json`
- `fixtures/compact.json` (may not appear; Task 23 has a synthesis fallback)
- `fixtures/subagent_dispatch.json` (may not appear; same fallback)

Each file's shape: `{"resource": {...}, "scope": {...}, "record": {...}}`.

- [ ] **Step 7: Remove the raw `captured/` dir from git tracking**

```bash
echo "internal/eventparser/testdata/captured/" >> .gitignore
```

- [ ] **Step 8: Commit fixtures**

```bash
git add .gitignore internal/eventparser/testdata/fixtures
git commit -m "test(eventparser): add captured Claude OTLP fixtures"
```

If any of the optional fixtures (`api_error`, `compact`, `subagent_dispatch`) didn't appear, note them and revisit during Tasks 18–23 — those tasks include a synthesis fallback path.

---

# Milestone M1.2 — eventparser deep module

Goal at end of M1.2: `Parse(rec *logspb.LogRecord, resource *resourcepb.Resource) (domain.Event, error)` correctly maps every documented + community event to a `domain.Event`. ≥90% coverage.

## Task 9: attrs flattening helpers

**Files:**
- Create: `internal/eventparser/attrs.go`
- Create: `internal/eventparser/attrs_test.go`

OTel `KeyValue` slices are nested unions. We collapse them to `map[string]any` for `domain.Event.Attrs`.

- [ ] **Step 1: Write the failing test**

Create `internal/eventparser/attrs_test.go`:

```go
package eventparser

import (
	"reflect"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

func TestFlattenKVs(t *testing.T) {
	in := []*commonpb.KeyValue{
		{Key: "s", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "x"}}},
		{Key: "i", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 7}}},
		{Key: "d", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: 1.5}}},
		{Key: "b", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: true}}},
	}
	got := FlattenKVs(in)
	want := map[string]any{"s": "x", "i": int64(7), "d": 1.5, "b": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestFlattenKVs_Nested(t *testing.T) {
	in := []*commonpb.KeyValue{{
		Key: "kv",
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_KvlistValue{
			KvlistValue: &commonpb.KeyValueList{Values: []*commonpb.KeyValue{
				{Key: "inner", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "y"}}},
			}},
		}},
	}}
	got := FlattenKVs(in)
	want := map[string]any{"kv": map[string]any{"inner": "y"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestFlattenKVs_Array(t *testing.T) {
	in := []*commonpb.KeyValue{{
		Key: "arr",
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{
			ArrayValue: &commonpb.ArrayValue{Values: []*commonpb.AnyValue{
				{Value: &commonpb.AnyValue_StringValue{StringValue: "a"}},
				{Value: &commonpb.AnyValue_IntValue{IntValue: 2}},
			}},
		}},
	}}
	got := FlattenKVs(in)
	want := map[string]any{"arr": []any{"a", int64(2)}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestFlattenKVs_NilSafe(t *testing.T) {
	if got := FlattenKVs(nil); got != nil {
		t.Fatalf("nil input → got %#v, want nil", got)
	}
}
```

- [ ] **Step 2: Run, expect compile failure**

Run: `go test ./internal/eventparser/ -run TestFlattenKVs -v`
Expected: FAIL — `undefined: FlattenKVs`.

- [ ] **Step 3: Implement attrs.go**

Replace `internal/eventparser/doc.go` with `internal/eventparser/attrs.go`:

```bash
rm internal/eventparser/doc.go
```

Create `internal/eventparser/attrs.go`:

```go
// Package eventparser converts OTLP LogRecord values into domain.Event values.
// It is a pure module: no I/O, no DB, no gRPC concerns. Resource attributes are
// flattened into the event's attrs map so downstream code does not need to join.
package eventparser

import (
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

// FlattenKVs collapses an OTLP KeyValue slice into map[string]any. Returns nil
// for nil input. Unknown AnyValue variants are stored as their Go zero-value
// proto representation; this is rare in practice for Claude Code emits.
func FlattenKVs(kvs []*commonpb.KeyValue) map[string]any {
	if kvs == nil {
		return nil
	}
	out := make(map[string]any, len(kvs))
	for _, kv := range kvs {
		out[kv.GetKey()] = anyValueToGo(kv.GetValue())
	}
	return out
}

func anyValueToGo(v *commonpb.AnyValue) any {
	if v == nil {
		return nil
	}
	switch x := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_IntValue:
		return x.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return x.DoubleValue
	case *commonpb.AnyValue_BoolValue:
		return x.BoolValue
	case *commonpb.AnyValue_BytesValue:
		return x.BytesValue
	case *commonpb.AnyValue_KvlistValue:
		return FlattenKVs(x.KvlistValue.GetValues())
	case *commonpb.AnyValue_ArrayValue:
		arr := x.ArrayValue.GetValues()
		out := make([]any, len(arr))
		for i, e := range arr {
			out[i] = anyValueToGo(e)
		}
		return out
	default:
		return nil
	}
}
```

- [ ] **Step 4: Run, expect pass**

Run: `go test ./internal/eventparser/ -v`
Expected: 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/eventparser/
git commit -m "feat(eventparser): add KeyValue flattening helpers"
```

## Task 10: `Parse` skeleton with required-field validation

**Files:**
- Create: `internal/eventparser/parser.go`
- Create: `internal/eventparser/parser_test.go`

`Parse` extracts: ts, session_id (required), prompt_id, event_name, then merges flat record attrs + selected resource attrs.

- [ ] **Step 1: Write the failing test**

Create `internal/eventparser/parser_test.go`:

```go
package eventparser

import (
	"errors"
	"reflect"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func kvStr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}
func kvInt(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}
func kvFloat(k string, v float64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: v}}}
}

func TestParse_MissingSessionIDReturnsErrDrop(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 100,
		Attributes:   []*commonpb.KeyValue{kvStr("event.name", "claude_code.user_prompt")},
	}
	_, err := Parse(rec, nil)
	if !errors.Is(err, ErrDrop) {
		t.Fatalf("err = %v, want ErrDrop", err)
	}
}

func TestParse_MinimalUserPrompt(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 1700000000000000000,
		Attributes: []*commonpb.KeyValue{
			kvStr("event.name", "claude_code.user_prompt"),
			kvStr("session.id", "sess-1"),
			kvStr("prompt.id", "pr-1"),
			kvInt("prompt_length", 42),
		},
	}
	res := &resourcepb.Resource{
		Attributes: []*commonpb.KeyValue{
			kvStr("project.name", "demo"),
			kvStr("app.version", "2.x"),
			kvStr("os.type", "darwin"),
		},
	}
	got, err := Parse(rec, res)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.SessionID != "sess-1" || got.PromptID != "pr-1" || got.EventName != "claude_code.user_prompt" {
		t.Fatalf("identity wrong: %+v", got)
	}
	if got.TS != 1700000000000000000 {
		t.Fatalf("ts = %d", got.TS)
	}
	wantAttrs := map[string]any{
		"prompt_length": int64(42),
		"project.name":  "demo",
		"app.version":   "2.x",
		"os.type":       "darwin",
	}
	if !reflect.DeepEqual(got.Attrs, wantAttrs) {
		t.Fatalf("attrs:\n got %#v\nwant %#v", got.Attrs, wantAttrs)
	}
}

func TestParse_UnknownEventNameStoredVerbatim(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 5,
		Attributes: []*commonpb.KeyValue{
			kvStr("event.name", "claude_code.something_new"),
			kvStr("session.id", "sess-1"),
			kvStr("custom_field", "yo"),
		},
	}
	got, err := Parse(rec, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.EventName != "claude_code.something_new" {
		t.Fatalf("event_name = %q", got.EventName)
	}
	if got.Attrs["custom_field"] != "yo" {
		t.Fatalf("attrs missing custom_field: %#v", got.Attrs)
	}
}
```

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/eventparser/ -run TestParse -v`
Expected: FAIL — `undefined: Parse, ErrDrop`.

- [ ] **Step 3: Implement parser.go**

Create `internal/eventparser/parser.go`:

```go
package eventparser

import (
	"errors"
	"fmt"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

// ErrDrop signals the caller that the record should be skipped (logged at WARN)
// without failing the surrounding batch. Returned for records missing required
// identity fields like session.id.
var ErrDrop = errors.New("eventparser: drop record")

// resourceAttrAllowlist is the subset of resource attributes that are merged
// into Event.Attrs. Keep this tight — they end up in the events.attrs JSON
// blob and we don't want to balloon row size.
var resourceAttrAllowlist = map[string]struct{}{
	"project.name":     {},
	"project.cwd":      {},
	"app.version":      {},
	"service.version":  {},
	"os.type":          {},
	"os.version":       {},
	"host.arch":        {},
	"user.id":          {},
	"user.email":       {},
	"organization.id":  {},
}

// Parse converts an OTLP LogRecord (plus its enclosing Resource) into a
// domain.Event. Returns ErrDrop when session.id is missing.
func Parse(rec *logspb.LogRecord, resource *resourcepb.Resource) (domain.Event, error) {
	if rec == nil {
		return domain.Event{}, fmt.Errorf("eventparser: nil record: %w", ErrDrop)
	}
	flat := FlattenKVs(rec.GetAttributes())
	sessionID, _ := flat["session.id"].(string)
	if sessionID == "" {
		return domain.Event{}, fmt.Errorf("eventparser: missing session.id: %w", ErrDrop)
	}
	promptID, _ := flat["prompt.id"].(string)
	eventName := eventNameOf(rec, flat)

	// Move identity fields out of attrs — they are first-class columns.
	delete(flat, "session.id")
	delete(flat, "prompt.id")
	delete(flat, "event.name")

	if resource != nil {
		for _, kv := range resource.GetAttributes() {
			if _, ok := resourceAttrAllowlist[kv.GetKey()]; !ok {
				continue
			}
			if _, exists := flat[kv.GetKey()]; exists {
				continue // record-level attr wins
			}
			flat[kv.GetKey()] = anyValueToGo(kv.GetValue())
		}
	}

	return domain.Event{
		TS:        int64(rec.GetTimeUnixNano()),
		SessionID: sessionID,
		PromptID:  promptID,
		EventName: eventName,
		Attrs:     flat,
	}, nil
}

// eventNameOf checks (in order) the LogRecord.EventName field, then the
// "event.name" attribute. If neither is set, returns the empty string.
func eventNameOf(rec *logspb.LogRecord, flat map[string]any) string {
	if n := rec.GetEventName(); n != "" {
		return n
	}
	if s, ok := flat["event.name"].(string); ok {
		return s
	}
	return ""
}
```

- [ ] **Step 4: Run, expect pass**

Run: `go test ./internal/eventparser/ -v`
Expected: all pass (FlattenKVs + Parse).

- [ ] **Step 5: Commit**

```bash
git add internal/eventparser/parser.go internal/eventparser/parser_test.go
git commit -m "feat(eventparser): add Parse with ErrDrop sentinel and resource-attr flattening"
```

## Task 11: Fixture loader test helper

**Files:**
- Create: `internal/eventparser/fixture_test.go`

The captured-fixture format is `{"resource": {...}, "scope": {...}, "record": {...}}`. Tests use a helper to load + Parse one in a single line.

- [ ] **Step 1: Write the helper**

Create `internal/eventparser/fixture_test.go`:

```go
package eventparser

import (
	"os"
	"path/filepath"
	"testing"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

type fixtureFile struct {
	Resource *resourcepb.Resource `protojson:"resource"`
	Record   *logspb.LogRecord    `protojson:"record"`
}

// loadFixture parses a single-record JSON file under testdata/fixtures.
// Schema: {"resource": <Resource>, "record": <LogRecord>}.
func loadFixture(t *testing.T, name string) (*resourcepb.Resource, *logspb.LogRecord) {
	t.Helper()
	bs, err := os.ReadFile(filepath.Join("testdata", "fixtures", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	// Manual two-step decode: protojson can't natively unmarshal a struct with
	// two distinct proto fields, so we pull each out by string indexing the JSON.
	resBytes, recBytes := splitFixtureJSON(t, bs)
	var res resourcepb.Resource
	if len(resBytes) > 0 {
		if err := protojson.Unmarshal(resBytes, &res); err != nil {
			t.Fatalf("unmarshal resource in %s: %v", name, err)
		}
	}
	var rec logspb.LogRecord
	if err := protojson.Unmarshal(recBytes, &rec); err != nil {
		t.Fatalf("unmarshal record in %s: %v", name, err)
	}
	return &res, &rec
}

// splitFixtureJSON pulls the "resource" and "record" sub-objects out of the
// top-level fixture JSON. Implemented with encoding/json since protojson does
// not support unknown wrapper structs.
func splitFixtureJSON(t *testing.T, raw []byte) (resJSON, recJSON []byte) {
	t.Helper()
	var wrap struct {
		Resource any `json:"resource"`
		Record   any `json:"record"`
	}
	if err := jsonUnmarshal(raw, &wrap); err != nil {
		t.Fatalf("split fixture: %v", err)
	}
	resJSON, _ = jsonMarshal(wrap.Resource)
	recJSON, _ = jsonMarshal(wrap.Record)
	return resJSON, recJSON
}

func parseFixture(t *testing.T, name string) domain.Event {
	t.Helper()
	res, rec := loadFixture(t, name)
	ev, err := Parse(rec, res)
	if err != nil {
		t.Fatalf("Parse(%s): %v", name, err)
	}
	return ev
}
```

And add a thin wrapper file `internal/eventparser/json_test.go`:

```go
package eventparser

import "encoding/json"

func jsonUnmarshal(b []byte, v any) error { return json.Unmarshal(b, v) }
func jsonMarshal(v any) ([]byte, error)   { return json.Marshal(v) }
```

(Wrappers exist so a future swap to encoding/json/v2 is one-file.)

- [ ] **Step 2: Build**

Run: `go build ./internal/eventparser/...`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add internal/eventparser/fixture_test.go internal/eventparser/json_test.go
git commit -m "test(eventparser): add fixture loader helper"
```

## Task 12: `user_prompt` fixture-driven test

**Files:**
- Modify: `internal/eventparser/parser_test.go`

- [ ] **Step 1: Add the test**

Append to `internal/eventparser/parser_test.go`:

```go
func TestParse_Fixture_UserPrompt(t *testing.T) {
	ev := parseFixture(t, "user_prompt.json")
	if ev.EventName != "claude_code.user_prompt" {
		t.Fatalf("event_name = %q", ev.EventName)
	}
	if ev.SessionID == "" {
		t.Fatal("missing session_id")
	}
	if ev.PromptID == "" {
		t.Fatal("missing prompt_id")
	}
	if ev.TS == 0 {
		t.Fatal("missing ts")
	}
	// prompt_length is documented "always present" — assert int64 type.
	pl, ok := ev.Attrs["prompt_length"].(int64)
	if !ok || pl <= 0 {
		t.Fatalf("prompt_length = %v (%T)", ev.Attrs["prompt_length"], ev.Attrs["prompt_length"])
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/eventparser/ -run TestParse_Fixture_UserPrompt -v`
Expected: PASS (fixture should already be in `testdata/fixtures/user_prompt.json` from Task 8).

If FAIL because of missing fixture, return to Task 8 and capture it.

- [ ] **Step 3: Commit**

```bash
git add internal/eventparser/parser_test.go
git commit -m "test(eventparser): cover user_prompt via real fixture"
```

## Task 13: `api_request` fixture test

**Files:**
- Modify: `internal/eventparser/parser_test.go`

- [ ] **Step 1: Append test**

```go
func TestParse_Fixture_APIRequest(t *testing.T) {
	ev := parseFixture(t, "api_request.json")
	if ev.EventName != "claude_code.api_request" {
		t.Fatalf("event_name = %q", ev.EventName)
	}
	for _, k := range []string{"input_tokens", "output_tokens", "cost_usd", "model"} {
		if _, ok := ev.Attrs[k]; !ok {
			t.Errorf("missing attr %q", k)
		}
	}
	// cost_usd is a float
	if _, ok := ev.Attrs["cost_usd"].(float64); !ok {
		t.Errorf("cost_usd type = %T, want float64", ev.Attrs["cost_usd"])
	}
}
```

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/eventparser/ -run TestParse_Fixture_APIRequest -v
git add internal/eventparser/parser_test.go
git commit -m "test(eventparser): cover api_request via real fixture"
```

## Task 14: `tool_decision` fixture test

**Files:**
- Modify: `internal/eventparser/parser_test.go`

- [ ] **Step 1: Append test**

```go
func TestParse_Fixture_ToolDecision(t *testing.T) {
	ev := parseFixture(t, "tool_decision.json")
	if ev.EventName != "claude_code.tool_decision" {
		t.Fatalf("event_name = %q", ev.EventName)
	}
	if d, _ := ev.Attrs["decision"].(string); d != "allow" && d != "deny" {
		t.Errorf("decision = %v", ev.Attrs["decision"])
	}
	if _, ok := ev.Attrs["tool_name"].(string); !ok {
		t.Error("tool_name missing or wrong type")
	}
}
```

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/eventparser/ -run TestParse_Fixture_ToolDecision -v
git add internal/eventparser/parser_test.go
git commit -m "test(eventparser): cover tool_decision via real fixture"
```

## Task 15: `tool_result` fixture test

**Files:**
- Modify: `internal/eventparser/parser_test.go`

- [ ] **Step 1: Append test**

```go
func TestParse_Fixture_ToolResult(t *testing.T) {
	ev := parseFixture(t, "tool_result.json")
	if ev.EventName != "claude_code.tool_result" {
		t.Fatalf("event_name = %q", ev.EventName)
	}
	if _, ok := ev.Attrs["tool_use_id"].(string); !ok {
		t.Error("tool_use_id missing or wrong type")
	}
	if _, ok := ev.Attrs["tool_name"].(string); !ok {
		t.Error("tool_name missing or wrong type")
	}
}
```

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/eventparser/ -run TestParse_Fixture_ToolResult -v
git add internal/eventparser/parser_test.go
git commit -m "test(eventparser): cover tool_result via real fixture"
```

## Task 16: `session_start` and `session_end` fixture tests

**Files:**
- Modify: `internal/eventparser/parser_test.go`

- [ ] **Step 1: Append**

```go
func TestParse_Fixture_SessionStart(t *testing.T) {
	ev := parseFixture(t, "session_start.json")
	if ev.EventName != "claude_code.session_start" {
		t.Fatalf("event_name = %q", ev.EventName)
	}
	if ev.SessionID == "" {
		t.Fatal("session_id required")
	}
}

func TestParse_Fixture_SessionEnd(t *testing.T) {
	ev := parseFixture(t, "session_end.json")
	if ev.EventName != "claude_code.session_end" {
		t.Fatalf("event_name = %q", ev.EventName)
	}
}
```

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/eventparser/ -run TestParse_Fixture_Session -v
git add internal/eventparser/parser_test.go
git commit -m "test(eventparser): cover session_start/end fixtures"
```

## Task 17: `api_error` test (synthesize if no fixture)

**Files:**
- Modify: `internal/eventparser/parser_test.go`

- [ ] **Step 1: Decide path**

If `internal/eventparser/testdata/fixtures/api_error.json` exists → use fixture path; otherwise synthesize.

- [ ] **Step 2a (fixture exists): append test**

```go
func TestParse_Fixture_APIError(t *testing.T) {
	ev := parseFixture(t, "api_error.json")
	if ev.EventName != "claude_code.api_error" {
		t.Fatalf("event_name = %q", ev.EventName)
	}
	if _, ok := ev.Attrs["error_message"].(string); !ok {
		t.Error("error_message missing")
	}
	if _, ok := ev.Attrs["http_status_code"].(int64); !ok {
		t.Error("http_status_code missing or wrong type")
	}
}
```

- [ ] **Step 2b (no fixture): inline synthesized test instead**

```go
func TestParse_APIError_Synth(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 9,
		Attributes: []*commonpb.KeyValue{
			kvStr("event.name", "claude_code.api_error"),
			kvStr("session.id", "s"),
			kvStr("prompt.id", "p"),
			kvStr("model", "claude-opus-4-7"),
			kvStr("error_message", "rate limit"),
			kvInt("http_status_code", 429),
			kvInt("attempt", 3),
		},
	}
	ev, err := Parse(rec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Attrs["error_message"] != "rate limit" {
		t.Errorf("error_message = %v", ev.Attrs["error_message"])
	}
	if ev.Attrs["http_status_code"] != int64(429) {
		t.Errorf("http_status_code = %v", ev.Attrs["http_status_code"])
	}
}
```

- [ ] **Step 3: Run + commit**

```bash
go test ./internal/eventparser/ -v
git add internal/eventparser/parser_test.go
git commit -m "test(eventparser): cover api_error"
```

## Task 18: `compact` and `subagent_dispatch` (synthesize if no fixture)

**Files:**
- Modify: `internal/eventparser/parser_test.go`

- [ ] **Step 1: Append synthesized tests (use fixture variants if available)**

```go
func TestParse_Compact_Synth(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 1,
		Attributes: []*commonpb.KeyValue{
			kvStr("event.name", "claude_code.compact"),
			kvStr("session.id", "s"),
			kvStr("prompt.id", "p"),
			kvInt("tokens_before", 100000),
			kvInt("tokens_after", 50000),
		},
	}
	ev, err := Parse(rec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ev.EventName != "claude_code.compact" {
		t.Fatalf("event_name = %q", ev.EventName)
	}
	if ev.Attrs["tokens_before"] != int64(100000) {
		t.Errorf("tokens_before = %v", ev.Attrs["tokens_before"])
	}
}

func TestParse_SubagentDispatch_Synth(t *testing.T) {
	rec := &logspb.LogRecord{
		TimeUnixNano: 1,
		Attributes: []*commonpb.KeyValue{
			kvStr("event.name", "claude_code.subagent_dispatch"),
			kvStr("session.id", "parent-1"),
			kvStr("parent_session.id", "parent-1"),
			kvStr("child_session.id", "child-1"),
		},
	}
	ev, err := Parse(rec, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Attrs["child_session.id"] != "child-1" {
		t.Errorf("child_session.id = %v", ev.Attrs["child_session.id"])
	}
}
```

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/eventparser/ -v
git add internal/eventparser/parser_test.go
git commit -m "test(eventparser): cover compact + subagent_dispatch"
```

## Task 19: Coverage gate ≥ 90% on eventparser

**Files:**
- (no source change unless coverage falls short)

- [ ] **Step 1: Measure**

Run: `go test -cover ./internal/eventparser/`
Expected: ≥ 90.0%.

- [ ] **Step 2: If under 90%, add a focused test**

If you're missing coverage, the most likely gap is a branch in `anyValueToGo` (e.g. `BytesValue` or the `default` case). Add a test:

```go
func TestFlattenKVs_BytesAndUnknown(t *testing.T) {
	in := []*commonpb.KeyValue{
		{Key: "by", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BytesValue{BytesValue: []byte{1, 2, 3}}}},
		{Key: "nil", Value: nil},
	}
	got := FlattenKVs(in)
	if !reflect.DeepEqual(got["by"], []byte{1, 2, 3}) {
		t.Errorf("bytes mismatch: %#v", got["by"])
	}
	if got["nil"] != nil {
		t.Errorf("nil value not nil: %v", got["nil"])
	}
}
```

Run cover again. Once ≥ 90%:

- [ ] **Step 3: Commit (only if you added a test)**

```bash
git add internal/eventparser/
git commit -m "test(eventparser): close coverage gap on AnyValue variants"
```

## Task 20: `cmd/parser-debug` tool

**Files:**
- Create: `cmd/parser-debug/main.go`

- [ ] **Step 1: Write the tool**

Create `cmd/parser-debug/main.go`:

```go
// parser-debug reads a fixture JSON file (the {resource, scope, record} shape
// used in internal/eventparser/testdata/fixtures) and prints the parsed
// domain.Event to stdout.
//
// Usage: parser-debug path/to/fixture.json
package main

import (
	"encoding/json"
	"fmt"
	"os"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/kamikaze011001/claude-code-observer/internal/eventparser"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: parser-debug <fixture.json>")
		os.Exit(2)
	}
	bs, err := os.ReadFile(os.Args[1])
	if err != nil {
		fail("read: %v", err)
	}
	var wrap struct {
		Resource json.RawMessage `json:"resource"`
		Record   json.RawMessage `json:"record"`
	}
	if err := json.Unmarshal(bs, &wrap); err != nil {
		fail("decode wrapper: %v", err)
	}
	var res resourcepb.Resource
	if len(wrap.Resource) > 0 && string(wrap.Resource) != "null" {
		if err := protojson.Unmarshal(wrap.Resource, &res); err != nil {
			fail("decode resource: %v", err)
		}
	}
	var rec logspb.LogRecord
	if err := protojson.Unmarshal(wrap.Record, &rec); err != nil {
		fail("decode record: %v", err)
	}
	ev, err := eventparser.Parse(&rec, &res)
	if err != nil {
		fail("parse: %v", err)
	}
	out, _ := json.MarshalIndent(ev, "", "  ")
	fmt.Println(string(out))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "parser-debug: "+format+"\n", args...)
	os.Exit(1)
}
```

- [ ] **Step 2: Build & smoke**

```bash
go build -o bin/parser-debug ./cmd/parser-debug
./bin/parser-debug internal/eventparser/testdata/fixtures/user_prompt.json
```

Expected: pretty-printed JSON of the `domain.Event` struct, with `EventName`, `SessionID`, `PromptID`, and `Attrs` populated.

- [ ] **Step 3: Commit**

```bash
git add cmd/parser-debug/
git commit -m "feat(parser-debug): CLI to print parsed Event for a fixture"
```

---

# Milestone M1.3 — End-to-end ingest

Goal at end of M1.3: real `claude` → `cco serve` → rows in `events` and `metric_snapshots`. Integration test passes.

## Task 21: Repository.InsertEvents

**Files:**
- Create: `internal/repository/events.go`
- Create: `internal/repository/events_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/repository/events_test.go`:

```go
package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

func TestInsertEvents(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()

	evs := []domain.Event{
		{TS: 100, SessionID: "s1", PromptID: "p1", EventName: "claude_code.user_prompt", Attrs: map[string]any{"prompt_length": int64(10)}},
		{TS: 200, SessionID: "s1", PromptID: "p1", EventName: "claude_code.api_request", Attrs: map[string]any{"cost_usd": 0.01, "model": "claude-opus-4-7"}},
	}
	if err := repo.InsertEvents(context.Background(), evs); err != nil {
		t.Fatalf("InsertEvents: %v", err)
	}

	rows, err := repo.DB().QueryContext(context.Background(), "SELECT ts, session_id, COALESCE(prompt_id,''), event_name, attrs FROM events ORDER BY ts")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var got []domain.Event
	for rows.Next() {
		var e domain.Event
		var attrsJSON string
		if err := rows.Scan(&e.TS, &e.SessionID, &e.PromptID, &e.EventName, &attrsJSON); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if err := json.Unmarshal([]byte(attrsJSON), &e.Attrs); err != nil {
			t.Fatalf("unmarshal attrs: %v", err)
		}
		got = append(got, e)
	}
	if len(got) != 2 {
		t.Fatalf("rows = %d, want 2", len(got))
	}
	if got[0].EventName != "claude_code.user_prompt" || got[1].EventName != "claude_code.api_request" {
		t.Fatalf("ordering wrong: %+v", got)
	}
	if got[0].Attrs["prompt_length"].(float64) != 10 {
		t.Fatalf("attr roundtrip failed: %#v", got[0].Attrs)
	}
}

func TestInsertEvents_EmptyIsNoop(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()
	if err := repo.InsertEvents(context.Background(), nil); err != nil {
		t.Fatalf("nil: %v", err)
	}
	if err := repo.InsertEvents(context.Background(), []domain.Event{}); err != nil {
		t.Fatalf("empty: %v", err)
	}
}
```

Note: `openTempRepo` is the helper already defined in `internal/repository/repository_test.go`. If absent, add this helper to the test file:

```go
func openTempRepo(t *testing.T) *Repository {
	t.Helper()
	dir := t.TempDir()
	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return repo
}
```

(Skip adding it if it already exists in the package.)

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/repository/ -run TestInsertEvents -v`
Expected: FAIL — `undefined: InsertEvents`.

- [ ] **Step 3: Implement events.go**

Create `internal/repository/events.go`:

```go
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
)

// InsertEvents inserts a batch of events in a single write transaction.
// Empty / nil input is a no-op. attrs is JSON-encoded.
func (r *Repository) InsertEvents(ctx context.Context, events []domain.Event) error {
	if len(events) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertEventsTx(ctx, tx, events); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// insertEventsTx is the work shared between InsertEvents and the combined
// IngestBatch path used by service.Service. Caller owns the transaction.
func insertEventsTx(ctx context.Context, tx *sql.Tx, events []domain.Event) error {
	const q = `INSERT INTO events (ts, session_id, prompt_id, event_name, attrs) VALUES (?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return fmt.Errorf("prepare events: %w", err)
	}
	defer stmt.Close()

	for i := range events {
		ev := events[i]
		attrs := ev.Attrs
		if attrs == nil {
			attrs = map[string]any{}
		}
		bs, err := json.Marshal(attrs)
		if err != nil {
			return fmt.Errorf("marshal attrs[%d]: %w", i, err)
		}
		var promptID any
		if ev.PromptID != "" {
			promptID = ev.PromptID
		}
		if _, err := stmt.ExecContext(ctx, ev.TS, ev.SessionID, promptID, ev.EventName, string(bs)); err != nil {
			return fmt.Errorf("insert events[%d]: %w", i, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run, expect pass**

Run: `go test ./internal/repository/ -v`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/events.go internal/repository/events_test.go
git commit -m "feat(repository): InsertEvents batches into one tx"
```

## Task 22: Repository.InsertMetricSnapshots

**Files:**
- Modify: `internal/repository/events.go`
- Modify: `internal/repository/events_test.go`

- [ ] **Step 1: Append the failing test**

```go
func TestInsertMetricSnapshots(t *testing.T) {
	repo := openTempRepo(t)
	defer repo.Close()

	snaps := []domain.MetricSnapshot{
		{TS: 1, SessionID: "s1", MetricName: "claude_code.cost.usage", Value: 0.05, Attrs: map[string]any{"model": "x"}},
		{TS: 2, SessionID: "s1", MetricName: "claude_code.token.usage", Value: 1000, Attrs: nil},
	}
	if err := repo.InsertMetricSnapshots(context.Background(), snaps); err != nil {
		t.Fatalf("InsertMetricSnapshots: %v", err)
	}
	var n int
	if err := repo.DB().QueryRow("SELECT COUNT(*) FROM metric_snapshots").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}
}
```

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/repository/ -run TestInsertMetricSnapshots -v`
Expected: FAIL — undefined.

- [ ] **Step 3: Implement**

Append to `internal/repository/events.go`:

```go
// InsertMetricSnapshots inserts a batch of metric datapoints in one tx.
func (r *Repository) InsertMetricSnapshots(ctx context.Context, snaps []domain.MetricSnapshot) error {
	if len(snaps) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := insertMetricSnapshotsTx(ctx, tx, snaps); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func insertMetricSnapshotsTx(ctx context.Context, tx *sql.Tx, snaps []domain.MetricSnapshot) error {
	const q = `INSERT INTO metric_snapshots (ts, session_id, metric_name, value, attrs) VALUES (?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return fmt.Errorf("prepare metric_snapshots: %w", err)
	}
	defer stmt.Close()

	for i := range snaps {
		s := snaps[i]
		attrs := s.Attrs
		if attrs == nil {
			attrs = map[string]any{}
		}
		bs, err := json.Marshal(attrs)
		if err != nil {
			return fmt.Errorf("marshal attrs[%d]: %w", i, err)
		}
		var sessID any
		if s.SessionID != "" {
			sessID = s.SessionID
		}
		if _, err := stmt.ExecContext(ctx, s.TS, sessID, s.MetricName, s.Value, string(bs)); err != nil {
			return fmt.Errorf("insert metric_snapshots[%d]: %w", i, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run, expect pass; commit**

```bash
go test ./internal/repository/ -v
git add internal/repository/
git commit -m "feat(repository): InsertMetricSnapshots batches into one tx"
```

## Task 23: Service.IngestLogs

**Files:**
- Create: `internal/service/service.go`
- Create: `internal/service/service_test.go`

The Service satisfies `receiver.LogIngester` and `receiver.MetricIngester`. It walks the request, parses each record, collects survivors, writes one tx.

- [ ] **Step 1: Write the failing test**

Create `internal/service/service_test.go`:

```go
package service

import (
	"context"
	"path/filepath"
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
)

func openTempRepo(t *testing.T) *repository.Repository {
	t.Helper()
	repo, err := repository.Open(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func kvStr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func TestService_IngestLogs_WritesAllRecords(t *testing.T) {
	repo := openTempRepo(t)
	svc := New(repo, nil)

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvStr("project.name", "demo")}},
			ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{
				{TimeUnixNano: 1, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.user_prompt"),
					kvStr("session.id", "s1"),
					kvStr("prompt.id", "p1"),
				}},
				{TimeUnixNano: 2, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.api_request"),
					kvStr("session.id", "s1"),
					kvStr("prompt.id", "p1"),
				}},
			}}},
		}},
	}
	if err := svc.IngestLogs(context.Background(), req); err != nil {
		t.Fatalf("IngestLogs: %v", err)
	}

	var n int
	if err := repo.DB().QueryRow("SELECT COUNT(*) FROM events").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}
}

func TestService_IngestLogs_DropsRecordsMissingSessionID(t *testing.T) {
	repo := openTempRepo(t)
	svc := New(repo, nil)

	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{
				{TimeUnixNano: 1, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.user_prompt"),
					kvStr("session.id", "good"),
				}},
				{TimeUnixNano: 2, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.user_prompt"),
					// no session.id → dropped
				}},
			}}},
		}},
	}
	if err := svc.IngestLogs(context.Background(), req); err != nil {
		t.Fatalf("IngestLogs: %v", err)
	}
	var n int
	_ = repo.DB().QueryRow("SELECT COUNT(*) FROM events").Scan(&n)
	if n != 1 {
		t.Fatalf("rows = %d, want 1 (one dropped)", n)
	}
}
```

- [ ] **Step 2: Run, expect failure**

Run: `go test ./internal/service/ -v`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Implement service.go**

Replace `internal/service/doc.go` with `internal/service/service.go`:

```bash
rm internal/service/doc.go
```

Create `internal/service/service.go`:

```go
// Package service is the orchestration layer between the OTLP receiver and
// the SQLite repository. It owns the parse-and-write transaction boundary.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/domain"
	"github.com/kamikaze011001/claude-code-observer/internal/eventparser"
	"github.com/kamikaze011001/claude-code-observer/internal/repository"
)

// Service satisfies receiver.LogIngester and receiver.MetricIngester.
type Service struct {
	repo *repository.Repository
	log  *slog.Logger
}

// New constructs a Service. A nil logger means slog.Default().
func New(repo *repository.Repository, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{repo: repo, log: log}
}

// IngestLogs implements receiver.LogIngester.
func (s *Service) IngestLogs(ctx context.Context, req *collogspb.ExportLogsServiceRequest) error {
	events := make([]domain.Event, 0, 16)
	for _, rl := range req.GetResourceLogs() {
		res := rl.GetResource()
		for _, sl := range rl.GetScopeLogs() {
			for _, rec := range sl.GetLogRecords() {
				ev, err := eventparser.Parse(rec, res)
				if err != nil {
					if errors.Is(err, eventparser.ErrDrop) {
						s.log.Warn("dropping log record", "err", err)
						continue
					}
					return fmt.Errorf("parse log record: %w", err)
				}
				events = append(events, ev)
			}
		}
	}
	if err := s.repo.InsertEvents(ctx, events); err != nil {
		return fmt.Errorf("insert events: %w", err)
	}
	return nil
}

// IngestMetrics implements receiver.MetricIngester. It walks every metric
// datapoint and persists each one verbatim.
func (s *Service) IngestMetrics(ctx context.Context, req *colmetricspb.ExportMetricsServiceRequest) error {
	snaps := make([]domain.MetricSnapshot, 0, 16)
	for _, rm := range req.GetResourceMetrics() {
		res := rm.GetResource()
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				snaps = append(snaps, datapointsToSnapshots(m, res)...)
			}
		}
	}
	if err := s.repo.InsertMetricSnapshots(ctx, snaps); err != nil {
		return fmt.Errorf("insert metric snapshots: %w", err)
	}
	return nil
}

// datapointsToSnapshots flattens every datapoint of every metric type into
// MetricSnapshot rows. Histograms are stored as their .Sum value; rollups in
// Phase 2 only consume Sum/Gauge series anyway.
func datapointsToSnapshots(m *metricspb.Metric, res *resourcepb.Resource) []domain.MetricSnapshot {
	resAttrs := flattenResourceAttrs(res)
	out := []domain.MetricSnapshot{}
	switch d := m.GetData().(type) {
	case *metricspb.Metric_Sum:
		for _, dp := range d.Sum.GetDataPoints() {
			out = append(out, snapFromNumber(m.GetName(), dp, resAttrs))
		}
	case *metricspb.Metric_Gauge:
		for _, dp := range d.Gauge.GetDataPoints() {
			out = append(out, snapFromNumber(m.GetName(), dp, resAttrs))
		}
	case *metricspb.Metric_Histogram:
		for _, dp := range d.Histogram.GetDataPoints() {
			out = append(out, domain.MetricSnapshot{
				TS:         int64(dp.GetTimeUnixNano()),
				SessionID:  sessionFromKVs(dp.GetAttributes()),
				MetricName: m.GetName(),
				Value:      dp.GetSum(),
				Attrs:      mergeAttrs(eventparser.FlattenKVs(dp.GetAttributes()), resAttrs),
			})
		}
	}
	return out
}

func snapFromNumber(name string, dp *metricspb.NumberDataPoint, resAttrs map[string]any) domain.MetricSnapshot {
	var v float64
	switch x := dp.GetValue().(type) {
	case *metricspb.NumberDataPoint_AsDouble:
		v = x.AsDouble
	case *metricspb.NumberDataPoint_AsInt:
		v = float64(x.AsInt)
	}
	return domain.MetricSnapshot{
		TS:         int64(dp.GetTimeUnixNano()),
		SessionID:  sessionFromKVs(dp.GetAttributes()),
		MetricName: name,
		Value:      v,
		Attrs:      mergeAttrs(eventparser.FlattenKVs(dp.GetAttributes()), resAttrs),
	}
}

func sessionFromKVs(kvs []*commonpb.KeyValue) string {
	for _, kv := range kvs {
		if kv.GetKey() == "session.id" {
			if s, ok := kv.GetValue().GetValue().(*commonpb.AnyValue_StringValue); ok {
				return s.StringValue
			}
		}
	}
	return ""
}

func mergeAttrs(a, b map[string]any) map[string]any {
	if a == nil && b == nil {
		return nil
	}
	out := make(map[string]any, len(a)+len(b))
	for k, v := range b {
		out[k] = v
	}
	for k, v := range a {
		out[k] = v
	}
	return out
}

func flattenResourceAttrs(res *resourcepb.Resource) map[string]any {
	if res == nil {
		return nil
	}
	return eventparser.FlattenKVs(res.GetAttributes())
}
```

- [ ] **Step 4: Run all tests**

Run: `go vet ./... && go test ./internal/service/ -v`
Expected: PASS for both Service tests.

- [ ] **Step 5: Commit**

```bash
git add internal/service/
git commit -m "feat(service): IngestLogs/IngestMetrics with parse+single-tx semantics"
```

## Task 24: Service metrics test

**Files:**
- Modify: `internal/service/service_test.go`

- [ ] **Step 1: Append**

```go
func TestService_IngestMetrics_StoresAllDatapoints(t *testing.T) {
	repo := openTempRepo(t)
	svc := New(repo, nil)

	req := &colmetricspb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{{
			ScopeMetrics: []*metricspb.ScopeMetrics{{
				Metrics: []*metricspb.Metric{{
					Name: "claude_code.cost.usage",
					Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
						DataPoints: []*metricspb.NumberDataPoint{
							{TimeUnixNano: 1, Attributes: []*commonpb.KeyValue{kvStr("session.id", "s1")}, Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 0.05}},
						},
					}},
				}, {
					Name: "claude_code.token.usage",
					Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{
						DataPoints: []*metricspb.NumberDataPoint{
							{TimeUnixNano: 2, Attributes: []*commonpb.KeyValue{kvStr("session.id", "s1")}, Value: &metricspb.NumberDataPoint_AsInt{AsInt: 1500}},
						},
					}},
				}},
			}},
		}},
	}
	if err := svc.IngestMetrics(context.Background(), req); err != nil {
		t.Fatalf("IngestMetrics: %v", err)
	}
	var n int
	_ = repo.DB().QueryRow("SELECT COUNT(*) FROM metric_snapshots").Scan(&n)
	if n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}
}
```

Add the import `colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"` and `metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"` to the test file if not already present.

- [ ] **Step 2: Run + commit**

```bash
go test ./internal/service/ -v
git add internal/service/service_test.go
git commit -m "test(service): cover IngestMetrics for Sum and Gauge data"
```

## Task 25: End-to-end integration test

**Files:**
- Create: `internal/service/integration_test.go`

Spin up the full receiver in-process on a real local port + temp SQLite + Service; send a real gRPC request; assert rows.

- [ ] **Step 1: Write the test**

Create `internal/service/integration_test.go`:

```go
package service

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"

	"github.com/kamikaze011001/claude-code-observer/internal/receiver"
)

func TestE2E_LogsLandInEvents(t *testing.T) {
	repo := openTempRepo(t)
	svc := New(repo, nil)

	srv := receiver.NewServer(receiver.Config{
		Addr:    "127.0.0.1:0",
		Logs:    svc,
		Metrics: svc,
	})
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := srv.Addr()
	go func() { _ = srv.Serve() }()
	t.Cleanup(srv.Stop)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(
		addr,
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "tcp", addr)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	cli := collogspb.NewLogsServiceClient(conn)
	req := &collogspb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{{
			Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{kvStr("project.name", "demo")}},
			ScopeLogs: []*logspb.ScopeLogs{{LogRecords: []*logspb.LogRecord{
				{TimeUnixNano: 100, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.user_prompt"), kvStr("session.id", "S"), kvStr("prompt.id", "P"),
				}},
				{TimeUnixNano: 200, Attributes: []*commonpb.KeyValue{
					kvStr("event.name", "claude_code.api_request"), kvStr("session.id", "S"), kvStr("prompt.id", "P"),
				}},
			}}},
		}},
	}
	if _, err := cli.Export(ctx, req); err != nil {
		t.Fatalf("Export: %v", err)
	}

	rows, err := repo.DB().QueryContext(ctx, "SELECT event_name, session_id, prompt_id, attrs FROM events ORDER BY ts")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var name, sess, prompt, attrs string
		if err := rows.Scan(&name, &sess, &prompt, &attrs); err != nil {
			t.Fatal(err)
		}
		if sess != "S" || prompt != "P" {
			t.Fatalf("identity wrong: %s/%s", sess, prompt)
		}
		got = append(got, name)
	}
	if len(got) != 2 || got[0] != "claude_code.user_prompt" || got[1] != "claude_code.api_request" {
		t.Fatalf("got %v", got)
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/service/ -run TestE2E -v`
Expected: PASS.

- [ ] **Step 3: Verify coverage**

Run: `go test -cover ./internal/service/ ./internal/repository/`
Expected: ≥ 80% on both.

- [ ] **Step 4: Commit**

```bash
git add internal/service/integration_test.go
git commit -m "test(service): end-to-end gRPC → events table integration"
```

## Task 26: Wire Service into `cco serve`

**Files:**
- Modify: `cmd/app/serve.go`

Replace the M1.1 stub ingesters with the real Service.

- [ ] **Step 1: Edit serve.go**

Replace the body of `cmd/app/serve.go` (the entire file) with:

```go
package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/kamikaze011001/claude-code-observer/internal/receiver"
	"github.com/kamikaze011001/claude-code-observer/internal/repository"
	"github.com/kamikaze011001/claude-code-observer/internal/service"
)

const defaultListenAddr = "127.0.0.1:4317"

func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
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

			schemaVersion, err := readSchemaVersion(ctx, repo)
			if err != nil {
				return fmt.Errorf("read schema_version: %w", err)
			}

			svc := service.New(repo, logger)
			srv := receiver.NewServer(receiver.Config{
				Addr:    addr,
				Logs:    svc,
				Metrics: svc,
				Logger:  logger,
			})
			if err := srv.Listen(); err != nil {
				return fmt.Errorf("receiver listen: %w", err)
			}
			logger.Info("daemon started",
				"home", homeDir,
				"binary_version", versionString(),
				"schema_version", schemaVersion,
				"otlp_addr", srv.Addr(),
			)

			errCh := make(chan error, 1)
			go func() { errCh <- srv.Serve() }()

			select {
			case <-ctx.Done():
				srv.Stop()
				<-errCh
			case err := <-errCh:
				if err != nil {
					return err
				}
			}
			logger.Info("daemon stopped")
			return nil
		},
	}
	cmd.Flags().StringVar(&addr, "listen", defaultListenAddr, "OTLP/gRPC listen address")
	return cmd
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

The two stub ingester types are gone — Service replaces them.

- [ ] **Step 2: Build + verify**

```bash
go vet ./...
go test ./...
go build -o bin/cco ./cmd/app
```

Expected: all green.

- [ ] **Step 3: Commit**

```bash
git add cmd/app/serve.go
git commit -m "feat(cmd): wire Service into serve, replacing M1.1 stubs"
```

## Task 27: Demo with real Claude (manual verification)

This is the M1.3 demo gate from the roadmap. No code changes; document the result.

- [ ] **Step 1: Reset DB to start clean**

```bash
rm -f ~/.claude-code-observer/db.sqlite ~/.claude-code-observer/db.sqlite-*
```

- [ ] **Step 2: Run cco**

In one terminal: `./bin/cco serve`. Confirm `otlp_addr=127.0.0.1:4317` in the log line.

- [ ] **Step 3: Drive Claude**

In a second terminal:

```bash
export CLAUDE_CODE_ENABLE_TELEMETRY=1
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317
export OTEL_LOGS_EXPORTER=otlp
export OTEL_METRICS_EXPORTER=otlp
claude
```

Drive a prompt with at least one tool call (e.g. `ls`). Exit Claude.

- [ ] **Step 4: Verify rows**

```bash
sqlite3 ~/.claude-code-observer/db.sqlite "SELECT COUNT(*) FROM events"
sqlite3 ~/.claude-code-observer/db.sqlite "SELECT event_name, COUNT(*) FROM events GROUP BY event_name"
sqlite3 ~/.claude-code-observer/db.sqlite "SELECT metric_name, COUNT(*) FROM metric_snapshots GROUP BY metric_name"
```

Expected: nonzero events count; group-by includes at minimum `claude_code.user_prompt`, `claude_code.api_request`, `claude_code.tool_result`; metrics include `claude_code.cost.usage` and `claude_code.token.usage`.

- [ ] **Step 5: Stop daemon**

Ctrl-C in the cco terminal. Should log `daemon stopped`.

- [ ] **Step 6: Update CLAUDE.md current state**

Modify the `## Current State` section in `CLAUDE.md`:

```markdown
## Current State

- **Milestone:** Phase 1 complete (M1.1 + M1.2 + M1.3). OTLP ingest live.
- **Known issues:** none.
- **Tech debt:** capture-fixtures uses receiver — slight test coupling, fine for now.
```

- [ ] **Step 7: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: mark Phase 1 complete in CLAUDE.md"
```

---

# Self-review checklist before handoff

Run through these once at the end:

- [ ] `go vet ./...` clean.
- [ ] `golangci-lint run` clean (if installed).
- [ ] `go test ./...` green.
- [ ] `go test -cover ./internal/eventparser/` ≥ 90%.
- [ ] `go test -cover ./internal/repository/` ≥ 80%.
- [ ] `go test -cover ./internal/service/` ≥ 80%.
- [ ] `go test -cover ./internal/receiver/` ≥ 70%.
- [ ] `go build -o bin/cco ./cmd/app` succeeds.
- [ ] No `TODO` / `FIXME` without a tracked follow-up.
- [ ] M1.3 demo passed end-to-end with real Claude.
