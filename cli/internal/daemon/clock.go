package daemon

import "time"

// Clock abstracts time for testability. The recurrence supervisor uses Clock so
// tests can drive ticks deterministically with a fake clock.
type Clock interface {
	Now() time.Time
	After(d time.Duration) <-chan time.Time
}

// RealClock is the production Clock backed by the standard library.
type RealClock struct{}

func (RealClock) Now() time.Time                         { return time.Now() }
func (RealClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
