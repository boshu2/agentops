//go:build !windows

package goals

import (
	"syscall"
)

// killAllChildren sends SIGKILL to all tracked child process groups.
func killAllChildren() {
	childGroups.mu.Lock()
	defer childGroups.mu.Unlock()
	for pid := range childGroups.pids {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	childGroups.pids = make(map[int]struct{})
}
