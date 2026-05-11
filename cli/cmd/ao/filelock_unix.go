//go:build !windows

// practices: [resilience-patterns, sre]
package main

import (
	"errors"
	"os"
	"syscall"
)

func flockLock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// flockLockNB attempts a non-blocking exclusive lock.
// Returns errLockWouldBlock if the lock is already held by another process.
func flockLockNB(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
		return errLockWouldBlock
	}
	return err
}

func flockUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
