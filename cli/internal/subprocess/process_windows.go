//go:build windows

package subprocess

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
)

func configureProcessTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
	cmd.Cancel = func() error {
		return killProcessTree(cmd)
	}
}

func terminateProcessTree(cmd *exec.Cmd) error {
	// taskkill may report that the already-exited parent PID is absent. Cleanup
	// remains best-effort after Wait; cancellation uses the error-returning path.
	_ = killProcessTree(cmd)
	return nil
}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	if output, err := kill.CombinedOutput(); err != nil {
		return fmt.Errorf("taskkill PID %d: %w (%s)", cmd.Process.Pid, err, output)
	}
	return nil
}
