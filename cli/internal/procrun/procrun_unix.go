//go:build !windows

package procrun

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup starts the child in its own process group (Setpgid) and
// installs a Cancel that kills the ENTIRE group (kill(-pgid)), so cancelling or
// timing out a run reaps grandchildren the direct child spawned instead of
// orphaning them. Cancel tolerates a not-yet-started process.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Negative PID targets the process group whose leader is the child;
		// with Setpgid the child's PGID equals its PID.
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
