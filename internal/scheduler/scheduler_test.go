package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestRealClock_Now_IsMonotonicallyIncreasing(t *testing.T) {
	c := RealClock{}
	a := c.Now()
	time.Sleep(1 * time.Millisecond)
	b := c.Now()
	if !b.After(a) {
		t.Fatalf("Now did not advance: a=%v b=%v", a, b)
	}
}

func TestRealClock_NewTicker_FiresOnInterval(t *testing.T) {
	c := RealClock{}
	tk := c.NewTicker(5 * time.Millisecond)
	defer tk.Stop()

	select {
	case <-tk.C():
		// good
	case <-time.After(200 * time.Millisecond):
		t.Fatal("ticker did not fire")
	}
}

func TestFakeClock_AdvanceMovesNow(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewFakeClock(start)

	if got := c.Now(); !got.Equal(start) {
		t.Fatalf("initial Now = %v, want %v", got, start)
	}
	c.Advance(2 * time.Hour)
	if got := c.Now(); !got.Equal(start.Add(2 * time.Hour)) {
		t.Fatalf("after Advance Now = %v, want %v", got, start.Add(2*time.Hour))
	}
}

func TestFakeClock_TickerFiresOnAdvance(t *testing.T) {
	c := NewFakeClock(time.Unix(0, 0))
	tk := c.NewTicker(1 * time.Second)
	defer tk.Stop()

	c.Advance(1 * time.Second)
	select {
	case <-tk.C():
		// good
	case <-time.After(200 * time.Millisecond):
		t.Fatal("fake ticker did not fire after Advance")
	}
}

func TestFakeClock_TickerDoesNotFireBelowInterval(t *testing.T) {
	c := NewFakeClock(time.Unix(0, 0))
	tk := c.NewTicker(1 * time.Second)
	defer tk.Stop()

	c.Advance(500 * time.Millisecond)
	select {
	case <-tk.C():
		t.Fatal("ticker fired before interval elapsed")
	case <-time.After(50 * time.Millisecond):
		// good
	}
}

func TestRun_CallsFnEveryTick(t *testing.T) {
	clock := NewFakeClock(time.Unix(0, 0))
	var calls int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		Run(ctx, clock, 1*time.Second, "test", silentLogger(), func(context.Context) error {
			atomic.AddInt32(&calls, 1)
			return nil
		})
		close(done)
	}()

	// allow Run to register its ticker
	time.Sleep(10 * time.Millisecond)
	clock.Advance(1 * time.Second)
	time.Sleep(10 * time.Millisecond)
	clock.Advance(1 * time.Second)
	time.Sleep(10 * time.Millisecond)

	cancel()
	<-done

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
}

func TestRun_LogsAndContinuesOnFnError(t *testing.T) {
	clock := NewFakeClock(time.Unix(0, 0))
	var calls int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		Run(ctx, clock, 1*time.Second, "test", silentLogger(), func(context.Context) error {
			atomic.AddInt32(&calls, 1)
			return errors.New("boom")
		})
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	clock.Advance(1 * time.Second)
	time.Sleep(10 * time.Millisecond)
	clock.Advance(1 * time.Second)
	time.Sleep(10 * time.Millisecond)
	cancel()
	<-done

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d, want 2 (loop should not stop on error)", got)
	}
}

func TestRun_ReturnsOnContextCancel(t *testing.T) {
	clock := NewFakeClock(time.Unix(0, 0))
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		Run(ctx, clock, 1*time.Second, "test", silentLogger(), func(context.Context) error { return nil })
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// good
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return after ctx cancel")
	}
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
