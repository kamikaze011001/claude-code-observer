// Package scheduler provides a minimal Clock abstraction and a Run helper
// for periodic background workers. It exists so worker tests can advance
// time deterministically without sleeping.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

// Clock is the time source used by workers. Production code uses RealClock;
// tests use FakeClock from this package.
type Clock interface {
	Now() time.Time
	NewTicker(d time.Duration) Ticker
}

// Ticker abstracts time.Ticker so a fake implementation can fire on demand.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// RealClock wraps the standard library time package.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }

func (RealClock) NewTicker(d time.Duration) Ticker {
	return realTicker{t: time.NewTicker(d)}
}

type realTicker struct{ t *time.Ticker }

func (r realTicker) C() <-chan time.Time { return r.t.C }
func (r realTicker) Stop()               { r.t.Stop() }

// Run blocks until ctx is cancelled. On every tick from clock.NewTicker(interval)
// it calls fn(ctx). Errors from fn are logged at error level (with worker=name)
// but do not stop the loop. On exit, logs "worker stopped" at info level.
func Run(ctx context.Context, clock Clock, interval time.Duration, name string, log *slog.Logger, fn func(context.Context) error) {
	tk := clock.NewTicker(interval)
	defer tk.Stop()
	log = log.With("worker", name)

	for {
		select {
		case <-ctx.Done():
			log.Info("worker stopped")
			return
		case <-tk.C():
			if err := fn(ctx); err != nil {
				log.Error("worker tick failed", "err", err)
			}
		}
	}
}
