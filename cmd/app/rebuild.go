package main

import "github.com/spf13/cobra"

func newRebuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild-rollups",
		Short: "Recompute sessions/prompts rollups from the events table",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("rebuild-rollups not yet implemented", "home", homeDir)
			cmd.Println("rebuild-rollups not yet wired (Phase 2).")
			return nil
		},
	}
}
