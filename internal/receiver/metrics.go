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
