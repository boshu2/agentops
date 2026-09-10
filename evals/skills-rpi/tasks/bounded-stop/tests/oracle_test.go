package retry

import (
	"math"
	"testing"
	"time"
)

func TestEvalSaturatesWithoutOverflow(t *testing.T) {
	for _, attempt := range []int{4, 62, 63, 64, 1000, math.MaxInt} {
		if got := Delay(attempt, time.Second, 10*time.Second); got != 10*time.Second {
			t.Errorf("Delay(%d) = %v, want cap", attempt, got)
		}
	}
	for _, row := range []struct {
		attempt            int
		initial, cap, want time.Duration
	}{
		{0, 12, 10, 10}, {0, 3, 10, 3}, {1, 3, 10, 6}, {2, 3, 10, 10},
		{-1, 3, 10, 0}, {1, 0, 10, 0}, {1, 3, 0, 0}, {1, -3, 10, 0},
		{1, time.Duration(math.MaxInt64/2 + 1), time.Duration(math.MaxInt64), time.Duration(math.MaxInt64)},
	} {
		if got := Delay(row.attempt, row.initial, row.cap); got != row.want {
			t.Errorf("Delay(%d,%d,%d) = %d, want %d", row.attempt, row.initial, row.cap, got, row.want)
		}
	}
}
