package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
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

			logger.Info("daemon started",
				"home", homeDir,
				"binary_version", versionString(),
				"schema_version", schemaVersion,
			)
			<-ctx.Done()
			logger.Info("daemon stopped")
			return nil
		},
	}
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
