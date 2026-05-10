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
	"google.golang.org/protobuf/reflect/protoreflect"

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

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "capture-fixtures: "+format+"\n", args...)
	os.Exit(1)
}
