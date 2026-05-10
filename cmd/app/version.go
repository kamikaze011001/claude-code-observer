package main

import "github.com/spf13/cobra"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and commit SHA",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Printf("claude-code-observer %s (commit %s)\n", version, commit)
		},
	}
}
