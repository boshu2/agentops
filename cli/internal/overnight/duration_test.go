package overnight

import (
	"testing"
	"time"
)

func TestStageDurationSinceFloorsNonPositiveElapsedTime(t *testing.T) {
	started := time.Now().Add(time.Hour)
	if got := stageDurationSince(started); got <= 0 {
		t.Fatalf("expected positive duration floor, got %s", got)
	}
}
