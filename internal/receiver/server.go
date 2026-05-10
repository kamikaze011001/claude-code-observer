package receiver

import (
	"fmt"
	"log/slog"
	"net"

	"google.golang.org/grpc"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

// errServerStopped is the sentinel gRPC returns after GracefulStop/Stop.
// It is not a real failure — treat it as a clean shutdown.
var errServerStopped = grpc.ErrServerStopped

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
// grpc.ErrServerStopped is swallowed — it signals a clean shutdown, not a failure.
func (s *Server) Serve() error {
	if s.gs == nil || s.lis == nil {
		return fmt.Errorf("server: Listen not called")
	}
	if err := s.gs.Serve(s.lis); err != nil && err != errServerStopped {
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
