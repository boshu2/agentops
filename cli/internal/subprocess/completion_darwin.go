//go:build darwin

package subprocess

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const darwinWaitIDPID = 1

// darwinSiginfoChild matches Darwin's siginfo_t layout. Only pid is consumed;
// the remaining fields preserve the native 104-byte ABI on 64-bit Darwin.
type darwinSiginfoChild struct {
	signo  int32
	errno  int32
	code   int32
	pid    int32
	uid    uint32
	status int32
	addr   uintptr
	value  uintptr
	band   int64
	_      [7]uint64
}

func newProcessCompletion(process *os.Process) processCompletion {
	pid := process.Pid
	return &pollingProcessCompletion{
		probe: func() (bool, error) {
			var info darwinSiginfoChild
			_, _, errno := syscall.Syscall6(
				syscall.SYS_WAITID,
				darwinWaitIDPID,
				uintptr(pid),
				uintptr(unsafe.Pointer(&info)),
				syscall.WEXITED|syscall.WNOHANG|syscall.WNOWAIT,
				0,
				0,
			)
			if errno == syscall.EINTR {
				return false, nil
			}
			if errno != 0 {
				return false, fmt.Errorf("waitid process %d without reaping: %w", pid, errno)
			}
			return info.pid != 0, nil
		},
	}
}
