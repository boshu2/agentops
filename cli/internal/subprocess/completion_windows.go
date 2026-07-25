//go:build windows

package subprocess

import (
	"fmt"
	"os"
	"syscall"
)

func newProcessCompletion(process *os.Process) processCompletion {
	return &pollingProcessCompletion{
		probe: func() (bool, error) {
			var completed bool
			var waitErr error
			if err := process.WithHandle(func(handle uintptr) {
				result, err := syscall.WaitForSingleObject(syscall.Handle(handle), 0)
				if err != nil {
					waitErr = err
					return
				}
				switch result {
				case syscall.WAIT_OBJECT_0:
					completed = true
				case syscall.WAIT_TIMEOUT:
				case syscall.WAIT_FAILED:
					waitErr = fmt.Errorf("WaitForSingleObject returned WAIT_FAILED")
				default:
					waitErr = fmt.Errorf("WaitForSingleObject returned status %#x", result)
				}
			}); err != nil {
				return false, fmt.Errorf("access process handle for completion: %w", err)
			}
			if waitErr != nil {
				return false, fmt.Errorf("observe process handle completion: %w", waitErr)
			}
			return completed, nil
		},
	}
}
