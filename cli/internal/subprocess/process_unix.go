//go:build !windows

package subprocess

import (
	"errors"
	"os/exec"
	"syscall"
	"time"
)

type unixProcessTree struct{}

func configureProcessTree(cmd *exec.Cmd, _ time.Duration) (*unixProcessTree, error) {
	tree := &unixProcessTree{}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return tree.terminate(cmd)
	}
	return tree, nil
}

func (*unixProcessTree) attach(*exec.Cmd) error { return nil }

func (*unixProcessTree) terminate(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func (*unixProcessTree) close() error { return nil }
