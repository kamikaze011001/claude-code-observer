package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kamikaze011001/claude-code-observer/internal/projectinit"
)

func newInitCmd() *cobra.Command {
	var (
		force bool
		print bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write/update .claude/settings.json in the current directory",
		Long: "Configures the current project to export OpenTelemetry to localhost:4317 " +
			"by writing the canonical OTel env block to .claude/settings.json. " +
			"Existing user keys are preserved; conflicts on cco-owned keys prompt for confirmation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getwd: %w", err)
			}
			opts := projectinit.Options{
				ProjectDir: cwd,
				Endpoint:   "127.0.0.1:4317",
				Force:      force,
				Print:      print,
				Stdout:     cmd.OutOrStdout(),
				Stderr:     cmd.ErrOrStderr(),
				Prompter:   &stdinPrompter{in: cmd.InOrStdin(), out: cmd.OutOrStdout()},
				Prober:     projectinit.GRPCProbe{},
			}
			return projectinit.Run(opts)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Skip prompts; overwrite owned-key conflicts")
	cmd.Flags().BoolVar(&print, "print", false, "Render merged settings.json to stdout, do not write")
	return cmd
}

type stdinPrompter struct {
	in  io.Reader
	out io.Writer
}

func (p *stdinPrompter) Confirm(prompt string) (bool, error) {
	fmt.Fprintf(p.out, "%s ", prompt)
	s := bufio.NewScanner(p.in)
	if !s.Scan() {
		return false, s.Err()
	}
	line := strings.ToLower(strings.TrimSpace(s.Text()))
	return line == "y" || line == "yes", nil
}
