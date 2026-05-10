package main

import "github.com/spf13/cobra"

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Write/update .claude/settings.json in the current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("init not yet implemented")
			cmd.Println("init not yet wired (Phase 4).")
			return nil
		},
	}
}
