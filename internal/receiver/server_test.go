package receiver

import (
	"context"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
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

var errBoom = errors.New("boom")

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
