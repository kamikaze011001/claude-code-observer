package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/kamikaze011001/claude-code-observer/internal/tui/app"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/dashboard"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/readstore"
	"github.com/kamikaze011001/claude-code-observer/internal/tui/theme"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:    "tui",
		Short:  "Open the interactive TUI",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := filepath.Join(homeDir, "db.sqlite")
			pool, err := readstore.OpenRO(dbPath)
			if err != nil {
				return fmt.Errorf("open read pool: %w", err)
			}
			defer func() { _ = pool.Close() }()

			envTheme := os.Getenv("CCO_THEME")
			envIcons := os.Getenv("CCO_ICONS")
			colorFGBG := os.Getenv("COLORFGBG")
			th, _, _ := theme.Resolve(themeName, iconsName, envTheme, envIcons, colorFGBG)
			shell := app.New(th)
			shell.Push(dashboard.New(pool, &th))

			prog := tea.NewProgram(shell, tea.WithAltScreen(), tea.WithContext(cmd.Context()))
			if _, err := prog.Run(); err != nil {
				return fmt.Errorf("tui: %w", err)
			}
			return nil
		},
	}
}
