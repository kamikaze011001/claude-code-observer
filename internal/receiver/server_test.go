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
