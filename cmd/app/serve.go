package main

import (
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the OTLP receiver daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			logger.Info("daemon started",
				"home", homeDir,
				"version", version,
				"commit", commit,
			)
			<-ctx.Done()
			logger.Info("daemon stopped")
			return nil
		},
	}
}
