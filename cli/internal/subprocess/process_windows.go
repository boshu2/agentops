//go:build windows

package subprocess

import (
	"errors"
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
	return killProcessTree(cmd)
}

func killProcessTree(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
	if output, err := kill.CombinedOutput(); err != nil {
		// taskkill uses exit code 128 when the waited parent and its tree are
		// already absent. That is a completed cleanup, not a cleanup failure.
		if taskkillTargetAbsent(err) {
			return nil
		}
		return fmt.Errorf("taskkill PID %d: %w (%s)", cmd.Process.Pid, err, output)
	}
	return nil
}

func taskkillTargetAbsent(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 128
}
