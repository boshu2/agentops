package provenanceapp

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

var checkpointGetFileSecurity = syscall.NewLazyDLL("advapi32.dll").NewProc("GetFileSecurityW")

func checkpointSecurity(path string, _ os.FileInfo) ([]byte, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	// Owner, group, DACL, integrity label, resource attributes and central
	// access policy. These queries require READ_CONTROL, not audit privileges.
	// https://learn.microsoft.com/en-us/windows/win32/secauthz/security-information
	const securityInformation = 0x1 | 0x2 | 0x4 | 0x10 | 0x20 | 0x40
	var size uint32
	_, _, callErr := checkpointGetFileSecurity.Call(uintptr(unsafe.Pointer(name)), securityInformation,
		0, 0, uintptr(unsafe.Pointer(&size)))
	if callErr != syscall.ERROR_INSUFFICIENT_BUFFER {
		return nil, fmt.Errorf("query security descriptor size: %w", callErr)
	}
	if size == 0 || size > 65536 {
		return nil, fmt.Errorf("unsupported security descriptor size %d", size)
	}
	buf := make([]byte, size)
	ok, _, callErr := checkpointGetFileSecurity.Call(uintptr(unsafe.Pointer(name)), securityInformation,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&size)))
	runtime.KeepAlive(name)
	if ok == 0 {
		return nil, fmt.Errorf("read security descriptor: %w", callErr)
	}
	if size > uint32(len(buf)) {
		return nil, fmt.Errorf("security descriptor changed during inspection")
	}
	return buf[:size], nil
}
