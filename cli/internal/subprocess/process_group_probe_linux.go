//go:build linux

package subprocess

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func probeProcessGroup(processGroupID int) error {
	return probeLinuxProcessGroup(processGroupID, linuxProcessGroupProbeOps{
		listPIDs:     listLinuxPIDs,
		processGroup: syscall.Getpgid,
		readState:    readLinuxProcessState,
	})
}

func listLinuxPIDs() ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	pids := make([]int, 0, len(entries))
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func readLinuxProcessState(pid int) (state string, processGroupID int, err error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	stat, err := os.ReadFile(statPath)
	if err != nil {
		return "", 0, fmt.Errorf("read %s: %w", statPath, err)
	}
	state, processGroupID, err = parseLinuxProcessStat(string(stat))
	if err != nil {
		return "", 0, fmt.Errorf("parse %s: %w", statPath, err)
	}
	return state, processGroupID, nil
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
