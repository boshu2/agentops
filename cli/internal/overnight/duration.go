package overnight

import "time"

func stageDurationSince(started time.Time) time.Duration {
	elapsed := time.Since(started)
	if elapsed <= 0 {
		return time.Nanosecond
	}
	return elapsed
}
