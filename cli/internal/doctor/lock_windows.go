//go:build windows

package doctor

import (
	"os"
	"syscall"
	"unsafe"
)

// Windows advisory-lock primitives. Mirrors the filelock_windows.go shape used
// by internal/storage, but uses LOCKFILE_FAIL_IMMEDIATELY for a non-blocking
// acquire so the doctor can map a held lock to ErrLockHeld (exit 5).
var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const (
	lockfileFailImmediately = uintptr(0x00000001)
	lockfileExclusiveLock   = uintptr(0x00000002)
)

// tryLockExclusive takes a non-blocking exclusive lock on f. A non-nil error
// (e.g. ERROR_LOCK_VIOLATION when another process holds it, or any other
// failure) tells the caller to map the result to ErrLockHeld.
func tryLockExclusive(f *os.File) error {
	var ol syscall.Overlapped
	r, _, err := procLockFileEx.Call(
		f.Fd(),
		lockfileExclusiveLock|lockfileFailImmediately,
		0, 1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r != 0 {
		return nil
	}
	return err
}

// unlockFile drops the advisory lock held on f.
func unlockFile(f *os.File) error {
	var ol syscall.Overlapped
	r, _, err := procUnlockFileEx.Call(
		f.Fd(),
		0, 1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r != 0 {
		return nil
	}
	return err
}
