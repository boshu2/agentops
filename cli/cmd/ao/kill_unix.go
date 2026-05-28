//go:build !windows

// practices: [sre, resilience-patterns]
package main

import (
	"errors"
	"syscall"
)

// sendSignal sends sig to the process with the given pid.
// Returns nil if the process is already gone (ESRCH).
func sendSignal(pid int, sig syscall.Signal) error {
	err := syscall.Kill(pid, sig)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
