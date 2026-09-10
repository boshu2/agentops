package retry

import "time"

// Delay doubles initial for each attempt and saturates at cap. Invalid inputs
// (negative attempt or nonpositive duration) produce zero.
func Delay(attempt int, initial, cap time.Duration) time.Duration {
	if attempt < 0 || initial <= 0 || cap <= 0 {
		return 0
	}
	delay := initial
	for i := 0; i < attempt && delay < cap; i++ {
		if delay > cap/2 {
			return cap
		}
		delay *= 2
	}
	if delay > cap {
		return cap
	}
	return delay
}
