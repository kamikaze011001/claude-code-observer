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
