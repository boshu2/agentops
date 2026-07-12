//go:build legacy && windows

package main

import "os/exec"

func configureCodexDispatchProcessGroup(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		return killCodexDispatchProcessGroup(cmd)
	}
}

func killCodexDispatchProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
