package retry

import (
	"testing"
	"time"
)

func TestOrdinaryDelay(t *testing.T) {
	if got := Delay(2, time.Second, 10*time.Second); got != 4*time.Second {
		t.Fatalf("Delay = %v", got)
	}
}
