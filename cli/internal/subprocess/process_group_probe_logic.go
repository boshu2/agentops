//go:build darwin || linux

package subprocess

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

const linuxZombieProcessState = "Z"

type linuxProcessGroupProbeOps struct {
	listPIDs     func() ([]int, error)
	processGroup func(int) (int, error)
	readState    func(int) (state string, processGroupID int, err error)
}

func probeLinuxProcessGroup(processGroupID int, ops linuxProcessGroupProbeOps) error {
	pids, err := ops.listPIDs()
	if err != nil {
		return fmt.Errorf("enumerate /proc for process group %d: %w", processGroupID, err)
	}
	for _, pid := range pids {
		group, err := ops.processGroup(pid)
		if linuxProcessDisappeared(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf(
				"query process group for PID %d while observing process group %d: %w",
				pid,
				processGroupID,
				err,
			)
		}
		if group != processGroupID {
			continue
		}

		state, statGroup, err := ops.readState(pid)
		if linuxProcessDisappeared(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf(
				"read state for PID %d while observing process group %d: %w",
				pid,
				processGroupID,
				err,
			)
		}
		if statGroup == processGroupID && state != linuxZombieProcessState {
			return nil
		}
	}
	return syscall.ESRCH
}

func linuxProcessDisappeared(err error) bool {
	return errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH)
}
