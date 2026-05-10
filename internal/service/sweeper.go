package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
	"github.com/kamikaze011001/claude-code-observer/internal/scheduler"
)

// Sweeper closes sessions whose last_seen_at is older than idle.
type Sweeper struct {
	repo  *repository.Repository
	clock scheduler.Clock
	idle  time.Duration
	log   *slog.Logger
}

// NewSweeper constructs a Sweeper. A nil logger means slog.Default().
func NewSweeper(repo *repository.Repository, clock scheduler.Clock, idle time.Duration, log *slog.Logger) *Sweeper {
	if log == nil {
		log = slog.Default()
	}
	return &Sweeper{repo: repo, clock: clock, idle: idle, log: log}
}

// Tick runs one sweep pass. Logs at info when sessions were closed, debug
// otherwise. Returns the underlying repository error if the UPDATE fails.
func (s *Sweeper) Tick(ctx context.Context) error {
	cutoff := s.clock.Now().Add(-s.idle).UnixNano()
	n, err := s.repo.CloseIdleSessions(ctx, cutoff)
	if err != nil {
		return err
	}
	if n > 0 {
		s.log.Info("sweeper closed idle sessions", "count", n, "idle", s.idle)
	} else {
		s.log.Debug("sweeper found no idle sessions", "idle", s.idle)
	}
	return nil
}
