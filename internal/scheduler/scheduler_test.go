package scheduler

import (
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
