package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/kamikaze011001/claude-code-observer/internal/receiver"
	"github.com/kamikaze011001/claude-code-observer/internal/repository"
	"github.com/kamikaze011001/claude-code-observer/internal/service"
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

			svc := service.New(repo, logger)
			srv := receiver.NewServer(receiver.Config{
				Addr:    addr,
				Logs:    svc,
				Metrics: svc,
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
