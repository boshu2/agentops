//go:build windows

package agentworker

import "os/exec"

func configureIsolatedProcess(_ *exec.Cmd) {}

func killIsolatedProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
