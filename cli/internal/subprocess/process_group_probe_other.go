//go:build !windows && !darwin && !linux

package subprocess

import "syscall"

func probeProcessGroup(processGroupID int) error {
	return syscall.Kill(-processGroupID, 0)
}
