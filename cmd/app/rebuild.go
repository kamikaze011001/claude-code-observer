package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
)

func newRebuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rebuild-rollups",
		Short: "Recompute sessions/prompts rollups from the events table",
		Long: "Truncates the sessions and prompts tables and replays every event " +
			"and metric snapshot in (ts, id) order through the rollup engine. " +
			"Stop `cco serve` first.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			repo, err := repository.Open(homeDir)
			if err != nil {
				return fmt.Errorf("open repository: %w", err)
			}
			defer repo.Close()

			start := time.Now()
			if err := repo.RebuildRollups(ctx); err != nil {
				return fmt.Errorf("rebuild rollups: %w", err)
			}
			elapsed := time.Since(start)

			var events, sessions, prompts int64
			var linesAdded, linesRemoved, commits int64
			db := repo.DB()
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&events)
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&sessions)
			_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM prompts`).Scan(&prompts)
			_ = db.QueryRowContext(ctx,
				`SELECT COALESCE(SUM(lines_added),0), COALESCE(SUM(lines_removed),0), COALESCE(SUM(commits),0) FROM sessions`).
				Scan(&linesAdded, &linesRemoved, &commits)

			cmd.Printf("rebuilt: %d events → %d sessions, %d prompts | +%d/-%d lines, %d commits (elapsed: %s)\n",
				events, sessions, prompts, linesAdded, linesRemoved, commits, elapsed)
			logger.Info("rebuild-rollups complete",
				"events", events, "sessions", sessions, "prompts", prompts,
				"lines_added", linesAdded, "lines_removed", linesRemoved, "commits", commits,
				"elapsed_ms", elapsed.Milliseconds())
			return nil
		},
	}
}
