package goals

import (
	"testing"
)

func TestTrackAndUntrackChild(t *testing.T) {
	// Record state
	childGroups.mu.Lock()
	before := len(childGroups.pids)
	childGroups.mu.Unlock()

	trackChild(99999)
	childGroups.mu.Lock()
	_, tracked := childGroups.pids[99999]
	afterAdd := len(childGroups.pids)
	childGroups.mu.Unlock()
	if !tracked {
		t.Error("should be tracked")
	}
	if afterAdd != before+1 {
		t.Errorf("count: %d -> %d", before, afterAdd)
	}

	untrackChild(99999)
	childGroups.mu.Lock()
	_, stillTracked := childGroups.pids[99999]
	afterRemove := len(childGroups.pids)
	childGroups.mu.Unlock()
	if stillTracked {
		t.Error("should be untracked")
	}
	if afterRemove != before {
		t.Errorf("count not restored: %d != %d", afterRemove, before)
	}
}

func TestKillAllChildren_EmptiesMap(t *testing.T) {
	// Use a fake PID that won't match any running process group.
	// killAllChildren tries kill(-pid, SIGKILL) which silently errors here
	// (the PID doesn't exist); but importantly it should clear the map.
	trackChild(99998)
	killAllChildren()

	childGroups.mu.Lock()
	_, stillTracked := childGroups.pids[99998]
	mapLen := len(childGroups.pids)
	childGroups.mu.Unlock()

	if stillTracked {
		t.Error("killAllChildren should remove pids from map")
	}
	if mapLen != 0 {
		t.Errorf("map should be empty after killAllChildren, got %d entries", mapLen)
	}
}
