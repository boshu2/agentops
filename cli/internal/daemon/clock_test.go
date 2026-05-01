package daemon

import (
	"sync"
	"testing"
	"time"
)

// FakeClock is a deterministic Clock for tests. Production code uses RealClock.
type FakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []fakeClockWaiter
}

type fakeClockWaiter struct {
	deadline time.Time
	ch       chan time.Time
}

func NewFakeClock(start time.Time) *FakeClock { return &FakeClock{now: start} }

func (f *FakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *FakeClock) After(d time.Duration) <-chan time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	ch := make(chan time.Time, 1)
	f.waiters = append(f.waiters, fakeClockWaiter{deadline: f.now.Add(d), ch: ch})
	return ch
}

// Advance moves the clock forward and fires any waiters whose deadline has passed.
func (f *FakeClock) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
	remaining := make([]fakeClockWaiter, 0, len(f.waiters))
	for _, w := range f.waiters {
		if !w.deadline.After(f.now) {
			w.ch <- f.now
			close(w.ch)
		} else {
			remaining = append(remaining, w)
		}
	}
	f.waiters = remaining
}

func TestRealClock_NowAdvances(t *testing.T) {
	c := RealClock{}
	t1 := c.Now()
	time.Sleep(2 * time.Millisecond)
	t2 := c.Now()
	if !t2.After(t1) {
		t.Fatalf("expected t2 > t1; got t1=%v t2=%v", t1, t2)
	}
}

func TestFakeClock_AdvanceFiresAfter(t *testing.T) {
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	fc := NewFakeClock(start)
	ch := fc.After(5 * time.Second)
	select {
	case <-ch:
		t.Fatal("After channel fired before Advance")
	default:
	}
	fc.Advance(5 * time.Second)
	select {
	case got := <-ch:
		if !got.Equal(start.Add(5 * time.Second)) {
			t.Fatalf("expected %v; got %v", start.Add(5*time.Second), got)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("After channel did not fire after Advance")
	}
}
