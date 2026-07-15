//go:build windows

package config

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	configKernel32     = syscall.NewLazyDLL("kernel32.dll")
	configCreateEventW = configKernel32.NewProc("CreateEventW")
	configCloseHandle  = configKernel32.NewProc("CloseHandle")
	configLockFileEx   = configKernel32.NewProc("LockFileEx")
	configUnlockFileEx = configKernel32.NewProc("UnlockFileEx")
	configWait         = configKernel32.NewProc("WaitForSingleObject")
)

const (
	configExclusiveLock = uintptr(0x00000002)
	configIOPending     = syscall.Errno(997)
)

func lockConfigFile(file *os.File) error {
	event, _, err := configCreateEventW.Call(0, 0, 0, 0)
	if event == 0 {
		return err
	}
	defer configCloseHandle.Call(event) //nolint:errcheck
	var overlapped syscall.Overlapped
	overlapped.HEvent = syscall.Handle(event)
	result, _, err := configLockFileEx.Call(
		file.Fd(), configExclusiveLock, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)),
	)
	if result != 0 {
		return nil
	}
	if err.(syscall.Errno) == configIOPending {
		waitResult, _, waitErr := configWait.Call(event, 0xFFFFFFFF)
		if waitResult == 0xFFFFFFFF {
			return waitErr
		}
		return nil
	}
	return err
}

func unlockConfigFile(file *os.File) error {
	var overlapped syscall.Overlapped
	result, _, err := configUnlockFileEx.Call(
		file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)),
	)
	if result != 0 {
		return nil
	}
	return err
}
