//go:build !windows

package procrun

import (
	"os"
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
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
			// ESRCH means the group already exited between the ctx cancellation
			// and this kill — that is the success case. Report ErrProcessDone so
			// Wait surfaces the real exit status instead of a spurious
			// "exec: canceling Cmd" error for a command that actually completed.
			if err == syscall.ESRCH {
				return os.ErrProcessDone
			}
			return err
		}
		return nil
	}
}

// reapProcessGroup best-effort kills any process still alive in the child's
// group after Wait returned, so a grandchild that outlived the direct child
// (holding a pipe past WaitDelay) is always reaped. ESRCH — the group is already
// gone — is the success case; all errors are ignored because this runs as
// post-Wait cleanup.
func reapProcessGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
