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
