//go:build !windows

package subprocess

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

const unixProcessGroupPollInterval = 10 * time.Millisecond

type unixClock struct {
	now   func() time.Time
	sleep func(time.Duration)
}

type unixProcessGroupOps struct {
	signal func(int, syscall.Signal) error
	probe  func(int) error
	clock  unixClock
}

type unixProcessTree struct {
	waitDelay time.Duration
	ops       unixProcessGroupOps
}

func configureProcessTree(cmd *exec.Cmd, waitDelay time.Duration) (*unixProcessTree, error) {
	if waitDelay <= 0 {
		waitDelay = defaultWaitDelay
	}
	tree := &unixProcessTree{
		waitDelay: waitDelay,
		ops: unixProcessGroupOps{
			signal: func(processGroupID int, signal syscall.Signal) error {
				return syscall.Kill(-processGroupID, signal)
			},
			probe: probeProcessGroup,
			clock: unixClock{
				now:   time.Now,
				sleep: time.Sleep,
			},
		},
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return tree.requestTermination(cmd)
	}
	return tree, nil
}

func (*unixProcessTree) attach(*exec.Cmd) error { return nil }

func (tree *unixProcessTree) terminate(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return terminateUnixProcessGroup(cmd.Process.Pid, tree.waitDelay, tree.ops)
}

func (tree *unixProcessTree) requestTermination(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	err := tree.ops.signal(cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("signal process group %d: %w", cmd.Process.Pid, err)
	}
	return nil
}

func terminateUnixProcessGroup(processGroupID int, timeout time.Duration, ops unixProcessGroupOps) error {
	if timeout <= 0 {
		timeout = defaultWaitDelay
	}
	err := ops.signal(processGroupID, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	if err != nil && !errors.Is(err, syscall.EPERM) {
		return fmt.Errorf("signal process group %d: %w", processGroupID, err)
	}

	// Setpgid makes this process group the lifecycle ownership boundary. A
	// successful SIGKILL request is not completion: only ESRCH from the
	// platform probe proves that no live member remains. Platform probes filter
	// exited zombies while the root remains unreaped, so PID/PGID identity stays
	// reserved throughout cleanup. EPERM means the group may still be live but
	// not signalable, so retain its identity and keep polling. Every other
	// observation error is opaque and fails cleanup immediately.
	deadline := ops.clock.now().Add(timeout)
	var lastPermissionErr error
	if errors.Is(err, syscall.EPERM) {
		lastPermissionErr = err
	}
	for {
		err = ops.probe(processGroupID)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if errors.Is(err, syscall.EPERM) {
			lastPermissionErr = err
		} else if err != nil {
			return fmt.Errorf("observe process group %d after termination: %w", processGroupID, err)
		}
		now := ops.clock.now()
		if !now.Before(deadline) {
			timeoutErr := fmt.Errorf("process group %d remained observable for %s after SIGKILL", processGroupID, timeout)
			return errors.Join(timeoutErr, lastPermissionErr)
		}
		delay := min(unixProcessGroupPollInterval, deadline.Sub(now))
		ops.clock.sleep(delay)
	}
}

func (*unixProcessTree) close() error { return nil }
