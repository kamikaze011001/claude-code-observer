package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Set via -ldflags "-X main.version=... -X main.commit=..."
var (
	version = "dev"
	commit  = "none"
)

// Resolved at root PersistentPreRun and read by subcommands.
var (
	homeDir  string
	logLevel string
	logger   *slog.Logger
)

// Theme and icon set are resolved in tui.go before opening the TUI.
var (
	themeName string
	iconsName string
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "claude-code-observer",
		Short:         "Local observability for Claude Code via OTLP",
		Long:          "claude-code-observer ingests OTLP/gRPC telemetry from Claude Code into a local SQLite store and renders it in a TUI.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := resolveHomeDir(); err != nil {
				return err
			}
			logger = newLogger(logLevel)
			return nil
		},
	}
	cmd.PersistentFlags().StringVar(&homeDir, "home", "", "Data directory (default: $CCO_HOME or ~/.claude-code-observer)")
	cmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level: debug|info|warn|error")
	cmd.PersistentFlags().StringVar(&themeName, "theme", "", "Color theme: mocha|macchiato|frappe|latte|auto (default: $CCO_THEME or auto)")
	cmd.PersistentFlags().StringVar(&iconsName, "icons", "", "Icon set: unicode|nerd (default: $CCO_ICONS or unicode)")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return newTUICmd().RunE(cmd, args)
	}
	return cmd
}

func resolveHomeDir() error {
	if homeDir != "" {
		return nil
	}
	if env := os.Getenv("CCO_HOME"); env != "" {
		homeDir = env
		return nil
	}
	hd, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	homeDir = filepath.Join(hd, ".claude-code-observer")
	return nil
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}

func main() {
	root := newRootCmd()
	registerSubcommands(root)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// registerSubcommands is implemented across subcommand files.
func registerSubcommands(root *cobra.Command) {
	root.AddCommand(
		newServeCmd(),
		newTUICmd(),
		newInitCmd(),
		newRebuildCmd(),
		newVersionCmd(),
	)
}
