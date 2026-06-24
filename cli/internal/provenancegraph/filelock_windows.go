//go:build windows

package provenancegraph

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
	hEvent, _, err := procCreateEventW.Call(0, 0, 0, 0)
	if hEvent == 0 {
		return err
	}
	defer func() {
		if hEvent != 0 {
			procCloseHandle.Call(hEvent) //nolint:errcheck
		}
	}()

	// ol is stack-allocated but its lifetime is safe: WaitForSingleObject
	// blocks until the async I/O completes, so ol remains valid for the
	// duration of the overlapped operation.
	var ol syscall.Overlapped
	ol.HEvent = syscall.Handle(hEvent)

	r, _, err := procLockFileEx.Call(
		f.Fd(),
		lockfileExclusiveLock,
		0, 1, 0,
		uintptr(unsafe.Pointer(&ol)),
	)
	if r != 0 {
		return nil
	}
	if err.(syscall.Errno) == errorIOPending {
		res, _, werr := procWFSO.Call(hEvent, 0xFFFFFFFF)
		if res == 0xFFFFFFFF {
			return werr
		}
		return nil
	}
	return err
}

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
