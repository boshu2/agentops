//go:build linux

package subprocess

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

const linuxZombieProcessState = "Z"

func probeProcessGroup(processGroupID int) error {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return fmt.Errorf("enumerate /proc for process group %d: %w", processGroupID, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		statPath := "/proc/" + entry.Name() + "/stat"
		stat, err := os.ReadFile(statPath)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s while observing process group %d: %w", statPath, processGroupID, err)
		}
		state, group, err := parseLinuxProcessStat(string(stat))
		if err != nil {
			return fmt.Errorf("parse %s while observing process group %d: %w", statPath, processGroupID, err)
		}
		if group == processGroupID && state != linuxZombieProcessState {
			return nil
		}
	}
	return syscall.ESRCH
}

func parseLinuxProcessStat(stat string) (state string, processGroupID int, err error) {
	commEnd := strings.LastIndex(stat, ") ")
	if commEnd < 0 {
		return "", 0, fmt.Errorf("missing command terminator")
	}
	fields := strings.Fields(stat[commEnd+2:])
	if len(fields) < 3 {
		return "", 0, fmt.Errorf("need state, parent PID, and process group")
	}
	processGroupID, err = strconv.Atoi(fields[2])
	if err != nil {
		return "", 0, fmt.Errorf("parse process group %q: %w", fields[2], err)
	}
	return fields[0], processGroupID, nil
}
