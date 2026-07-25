//go:build linux

package subprocess

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const linuxIs64Bit = ^uint(0) >> 63

// linuxSiginfoChild matches Linux siginfo_t for SIGCHLD. Only Pid is consumed;
// the remaining fields preserve the kernel's fixed 128-byte layout.
type linuxSiginfoChild struct {
	signo int32
	errno int32
	code  int32
	_     [linuxIs64Bit]int32

	pid    int32
	uid    uint32
	status int32
	_      [128 - (6+linuxIs64Bit)*4]byte
}

func newProcessCompletion(process *os.Process) processCompletion {
	pid := process.Pid
	return &pollingProcessCompletion{
		probe: func() (bool, error) {
			var info linuxSiginfoChild
			const pPID = 1
			_, _, errno := syscall.Syscall6(
				syscall.SYS_WAITID,
				pPID,
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
