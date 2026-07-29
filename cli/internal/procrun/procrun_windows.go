//go:build windows

package procrun

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
)

// configureProcessGroup starts the child in a new process group
// (CREATE_NEW_PROCESS_GROUP) and installs a Cancel that terminates the whole
// process tree via `taskkill /T /F`, the Windows analogue of kill(-pgid). Cancel
// tolerates a not-yet-started process.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return killProcessTree(cmd.Process.Pid)
	}
}

// killProcessTree terminates a process and all its descendants on Windows.
func killProcessTree(pid int) error {
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if out, err := kill.CombinedOutput(); err != nil {
		return fmt.Errorf("taskkill PID %d: %w (%s)", pid, err, out)
	}
	return nil
}
