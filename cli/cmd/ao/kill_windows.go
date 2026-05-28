//go:build windows

// practices: [sre, resilience-patterns]
package main

import (
	"os"
	"syscall"
)

// sendSignal terminates the process with the given pid on Windows.
// The signal parameter is accepted for API compatibility but ignored —
// Windows has no POSIX signal model; all signals map to process termination.
// Returns nil if the process is already gone.
func sendSignal(pid int, _ syscall.Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return nil // process not found — equivalent to ESRCH
	}
	if err := p.Kill(); err != nil {
		errStr := err.Error()
		// Treat "not found" / "invalid handle" as already-gone
		if errStr == "os: process already finished" {
			return nil
		}
		return err
	}
	return nil
}
