//go:build !windows

package doctor

import (
	"os"
	"syscall"
)

// tryLockExclusive takes a non-blocking exclusive advisory lock on f. A non-nil
// error — including EWOULDBLOCK when another process holds the lock — tells the
// caller to map the result to ErrLockHeld.
func tryLockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// unlockFile drops the advisory lock held on f.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
