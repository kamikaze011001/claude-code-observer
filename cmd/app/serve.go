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
