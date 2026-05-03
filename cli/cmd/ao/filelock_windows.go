//go:build windows

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32WinLock   = syscall.NewLazyDLL("kernel32.dll")
	procCreateEventWLock = modkernel32WinLock.NewProc("CreateEventW")
	procCloseHandleLock  = modkernel32WinLock.NewProc("CloseHandle")
	procLockFileExLock   = modkernel32WinLock.NewProc("LockFileEx")
	procUnlockFileExLock = modkernel32WinLock.NewProc("UnlockFileEx")
	procWFSOLock         = modkernel32WinLock.NewProc("WaitForSingleObject")
)

const (
	lockfileExclusiveLockAO = uintptr(0x00000002)
	lockfileFailImmediately = uintptr(0x00000001)
	errorIOPendingAO        = syscall.Errno(997) // ERROR_IO_PENDING
	errorLockViolation      = syscall.Errno(33)  // ERROR_LOCK_VIOLATION
)

func flockLock(f *os.File) error {
	hEvent, _, err := procCreateEventWLock.Call(0, 0, 0, 0)
	if hEvent == 0 {
		return err
	}
	defer procCloseHandleLock.Call(hEvent) //nolint:errcheck

	var ol syscall.Overlapped
	ol.HEvent = syscall.Handle(hEvent)

	r, _, err := procLockFileExLock.Call(
		f.Fd(), lockfileExclusiveLockAO, 0, 1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r != 0 {
		return nil
	}
	if errno, ok := err.(syscall.Errno); ok && errno == errorIOPendingAO {
		res, _, werr := procWFSOLock.Call(hEvent, 0xFFFFFFFF)
		if res == 0xFFFFFFFF {
			return werr
		}
		return nil
	}
	return err
}

// flockLockNB attempts a non-blocking exclusive lock.
// Returns errLockWouldBlock if the lock is already held by another process.
func flockLockNB(f *os.File) error {
	var ol syscall.Overlapped
	r, _, err := procLockFileExLock.Call(
		f.Fd(), lockfileExclusiveLockAO|lockfileFailImmediately, 0, 1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r != 0 {
		return nil
	}
	if errno, ok := err.(syscall.Errno); ok && errno == errorLockViolation {
		return errLockWouldBlock
	}
	return err
}

func flockUnlock(f *os.File) error {
	var ol syscall.Overlapped
	r, _, err := procUnlockFileExLock.Call(
		f.Fd(), 0, 1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r != 0 {
		return nil
	}
	return err
}
