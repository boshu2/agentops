//go:build darwin

package subprocess

import (
	"syscall"

	"golang.org/x/sys/unix"
)

const darwinZombieProcessState = 5

func probeProcessGroup(processGroupID int) error {
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", processGroupID)
	if err != nil {
		return err
	}
	for _, process := range processes {
		if process.Eproc.Pgid == int32(processGroupID) &&
			process.Proc.P_stat != darwinZombieProcessState {
			return nil
		}
	}
	return syscall.ESRCH
}
