package main

import "github.com/spf13/cobra"

// newTUICmd is invoked when no subcommand is given. We mount it as a
// hidden subcommand and also wire it as the root's RunE in main.go via a
// fall-through. For Phase 0 it prints a stub message.
func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:    "tui",
		Short:  "Open the interactive TUI",
		Hidden: true, // exposed via default invocation in Phase 3
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("tui not yet implemented", "home", homeDir)
			cmd.Println("TUI not yet wired (Phase 3).")
			return nil
		},
	}
}
