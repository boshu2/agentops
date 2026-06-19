//go:build windows

package worktree

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procCreateEventW = modkernel32.NewProc("CreateEventW")
	procCloseHandle  = modkernel32.NewProc("CloseHandle")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
	procWFSO         = modkernel32.NewProc("WaitForSingleObject")
)

const (
	lockfileExclusiveLock = uintptr(0x00000002)
	errorIOPending        = syscall.Errno(997) // ERROR_IO_PENDING
)

func lockFile(f *os.File) error {
	// CreateEventW(lpEventAttributes=NULL, bManualReset=FALSE, bInitialState=FALSE, lpName=NULL)
	hEvent, _, err := procCreateEventW.Call(0, 0, 0, 0)
	if hEvent == 0 {
		return err
	}
	defer procCloseHandle.Call(hEvent) //nolint:errcheck

	var ol syscall.Overlapped
	ol.HEvent = syscall.Handle(hEvent)

	// LockFileEx(hFile, dwFlags, dwReserved=0, nBytesToLockLow=1, nBytesToLockHigh=0, lpOverlapped)
	r, _, err := procLockFileEx.Call(
		f.Fd(),
		lockfileExclusiveLock,
		0, 1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r != 0 {
		return nil
	}
	if errno, ok := err.(syscall.Errno); ok && errno == errorIOPending {
		// Lock is pending: block until the event is signaled.
		res, _, werr := procWFSO.Call(hEvent, 0xFFFFFFFF /* INFINITE */)
		if res == 0xFFFFFFFF { // WAIT_FAILED
			return werr
		}
		return nil
	}
	return err
}

func unlockFile(f *os.File) error {
	var ol syscall.Overlapped
	// UnlockFileEx(hFile, dwReserved=0, nBytesToUnlockLow=1, nBytesToUnlockHigh=0, lpOverlapped)
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
