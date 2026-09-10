package retry

import "time"

// Delay doubles initial for each attempt and saturates at cap. Invalid inputs
// (negative attempt or nonpositive duration) produce zero.
func Delay(attempt int, initial, cap time.Duration) time.Duration {
	if attempt < 0 || initial <= 0 || cap <= 0 {
		return 0
	}
	delay := initial << attempt
	if delay > cap {
		return cap
	}
	return delay
}
