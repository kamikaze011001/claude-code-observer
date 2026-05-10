package projectinit

import (
	"context"
	"fmt"
	"time"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Prober reports whether a daemon is reachable at the given endpoint.
// A nil error means reachable. Implementations must respect ctx cancellation.
type Prober interface {
	Probe(ctx context.Context, endpoint string, timeout time.Duration) error
}

// GRPCProbe dials endpoint over plaintext gRPC and calls LogsService.Export
// with an empty request. The daemon's own server is what we expect, but any
// listening LogsService implementation counts as "reachable".
type GRPCProbe struct{}

// Probe dials endpoint with a deadline of timeout (or ctx, whichever fires
// first) and issues one Export call. Returns nil iff the call returns
// without a transport error.
func (GRPCProbe) Probe(ctx context.Context, endpoint string, timeout time.Duration) error {
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	//nolint:staticcheck // grpc.DialContext + WithBlock chosen deliberately for blocking probe semantics
	conn, err := grpc.DialContext(
		dialCtx,
		endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(), //nolint:staticcheck
	)
	if err != nil {
		return fmt.Errorf("dial %s: %w", endpoint, err)
	}
	defer conn.Close()
	client := collogspb.NewLogsServiceClient(conn)
	_, err = client.Export(dialCtx, &collogspb.ExportLogsServiceRequest{})
	if err != nil {
		return fmt.Errorf("export probe: %w", err)
	}
	return nil
}
