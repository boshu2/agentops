//go:build windows

package procrun

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// taskkillTimeout bounds each taskkill invocation so a hung taskkill cannot make
// Run (via Cancel) or the post-Wait reap block indefinitely.
const taskkillTimeout = 5 * time.Second

// configureProcessGroup starts the child in a new process group
// (CREATE_NEW_PROCESS_GROUP) and installs a Cancel that terminates the process
// tree via `taskkill /T /F`. Note this is a best-effort tree kill, NOT a
// containment guarantee (no Job Object); see the package doc. Cancel tolerates a
// not-yet-started process.
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

// killProcessTree best-effort terminates a process and its descendants via
// taskkill /T /F, under its own timeout so a hung taskkill cannot stall the
// caller. Best-effort only — taskkill after the root exits can miss re-parented
// descendants.
func killProcessTree(pid int) error {
	ctx, cancel := context.WithTimeout(context.Background(), taskkillTimeout)
	defer cancel()
	kill := exec.CommandContext(ctx, "taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if out, err := kill.CombinedOutput(); err != nil {
		return fmt.Errorf("taskkill PID %d: %w (%s)", pid, err, out)
	}
	return nil
}

// reapProcessGroup best-effort terminates any surviving descendants after Wait
// returned. Errors are ignored — this runs as post-Wait cleanup and, on Windows,
// is only a best-effort guarantee.
func reapProcessGroup(pid int) {
	_ = killProcessTree(pid)
}
