package projectinit

import (
	"context"
	"net"
	"testing"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc"
)

type stubLogsServer struct {
	collogspb.UnimplementedLogsServiceServer
}

func (s *stubLogsServer) Export(ctx context.Context, req *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	return &collogspb.ExportLogsServiceResponse{}, nil
}

func startStubServer(t *testing.T) (string, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	collogspb.RegisterLogsServiceServer(srv, &stubLogsServer{})
	go srv.Serve(lis)
	return lis.Addr().String(), func() { srv.Stop(); _ = lis.Close() }
}

func TestGRPCProbe_Reachable(t *testing.T) {
	addr, stop := startStubServer(t)
	defer stop()
	var p GRPCProbe
	err := p.Probe(context.Background(), addr, 2*time.Second)
	if err != nil {
		t.Errorf("expected reachable, got error: %v", err)
	}
}

func TestGRPCProbe_Unreachable(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()
	var p GRPCProbe
	err = p.Probe(context.Background(), addr, 500*time.Millisecond)
	if err == nil {
		t.Error("expected unreachable error, got nil")
	}
}

func TestGRPCProbe_RespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var p GRPCProbe
	err := p.Probe(ctx, "127.0.0.1:1", 30*time.Second)
	if err == nil {
		t.Error("expected error after ctx cancel")
	}
}
