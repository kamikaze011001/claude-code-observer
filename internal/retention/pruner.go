// Package retention prunes raw events and metric snapshots older than a
// configured window. Rollup tables (sessions, prompts) are never touched.
package retention

import (
	"context"
	"log/slog"
	"time"

	"github.com/kamikaze011001/claude-code-observer/internal/repository"
	"github.com/kamikaze011001/claude-code-observer/internal/scheduler"
)

// Pruner deletes raw events and metric snapshots older than retention.
type Pruner struct {
	repo      *repository.Repository
	clock     scheduler.Clock
	retention time.Duration
	log       *slog.Logger
}

// New constructs a Pruner. A nil logger means slog.Default().
func New(repo *repository.Repository, clock scheduler.Clock, retention time.Duration, log *slog.Logger) *Pruner {
	if log == nil {
		log = slog.Default()
	}
	return &Pruner{repo: repo, clock: clock, retention: retention, log: log}
}

// Tick runs one prune pass. If deleting from one table fails, the other is
// still attempted. Errors are logged; the first error encountered is returned
// so scheduler.Run can log it once with the worker name.
func (p *Pruner) Tick(ctx context.Context) error {
	cutoff := p.clock.Now().Add(-p.retention).UnixNano()

	var firstErr error
	events, err := p.repo.DeleteEventsBefore(ctx, cutoff)
	if err != nil {
		p.log.Error("pruner delete events failed", "err", err)
		firstErr = err
	}
	metrics, err := p.repo.DeleteMetricSnapshotsBefore(ctx, cutoff)
	if err != nil {
		p.log.Error("pruner delete metric_snapshots failed", "err", err)
		if firstErr == nil {
			firstErr = err
		}
	}
	p.log.Info("pruner ran",
		"events_deleted", events,
		"metrics_deleted", metrics,
		"cutoff_ns", cutoff,
	)
	return firstErr
}
