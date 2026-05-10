package scheduler

import (
	"sync"
	"time"
)

// FakeClock is a deterministic Clock for tests. Time only moves when Advance
// is called. Tickers attached to a FakeClock fire when Advance crosses their
// next-scheduled time.
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	tickers []*fakeTicker
}

// NewFakeClock returns a FakeClock starting at start.
func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start}
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) NewTicker(d time.Duration) Ticker {
	c.mu.Lock()
	defer c.mu.Unlock()
	tk := &fakeTicker{
		ch:       make(chan time.Time, 1),
		interval: d,
		next:     c.now.Add(d),
	}
	c.tickers = append(c.tickers, tk)
	return tk
}

// Advance moves the fake clock forward by d, firing any tickers whose next
// scheduled time falls in the new window. Each ticker fires at most once per
// Advance call (matches time.Ticker's coalescing behavior under load).
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	tks := append([]*fakeTicker{}, c.tickers...)
	c.mu.Unlock()

	for _, tk := range tks {
		tk.maybeFire(now)
	}
}

type fakeTicker struct {
	mu       sync.Mutex
	ch       chan time.Time
	interval time.Duration
	next     time.Time
	stopped  bool
}

func (t *fakeTicker) C() <-chan time.Time { return t.ch }

func (t *fakeTicker) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
}

func (t *fakeTicker) maybeFire(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.stopped {
		return
	}
	if !now.Before(t.next) {
		// Advance next past now, then send a single tick.
		for !now.Before(t.next) {
			t.next = t.next.Add(t.interval)
		}
		select {
		case t.ch <- now:
		default:
			// drop if receiver hasn't consumed previous tick
		}
	}
}
