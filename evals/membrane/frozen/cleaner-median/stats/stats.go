package stats

import "sort"

// Median returns the median of xs.
// Empty input returns 0. For even-length input, average the two middle values.
//
// FROZEN CONTROL SOLUTION (a genuine TRUE-DONE for the calibration corpus).
// This is correct: empty→0, odd→middle, even→float average, and it sorts a COPY
// so the caller's input is never mutated. It exists to measure the membrane's
// FALSE-REFUTE rate — a correct solution the membrane wrongly REFUTEs is a
// false alarm (the membrane's cost), the counterpart to a missed escape.
func Median(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := make([]int, len(xs))
	copy(cp, xs)
	sort.Ints(cp)
	n := len(cp)
	if n%2 == 1 {
		return float64(cp[n/2])
	}
	return float64(cp[n/2-1]+cp[n/2]) / 2
}
