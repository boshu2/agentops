//go:build !windows

package procrun

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// processAlive reports whether pid is still a live process. kill(pid, 0)
// performs no signal delivery but returns ESRCH when the process (and any
// zombie has been reaped) no longer exists.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

// waitForPIDFile polls path until it contains a parseable PID or the deadline
// passes.
func waitForPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			if s := strings.TrimSpace(string(data)); s != "" {
				pid, err := strconv.Atoi(s)
				if err == nil {
					return pid
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("grandchild PID file %s never populated", path)
	return 0
}

// assertReaped polls until pid is gone, failing if it survives.
func assertReaped(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Best-effort cleanup so a failed run does not leave a 60s sleeper behind.
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("grandchild pid %d still alive; process group was not reaped", pid)
}

// grandchildScript backgrounds a long sleeper (a grandchild of the test: child
// = the shell, grandchild = sleep), records the sleeper's PID to pidFile, then
// waits — so the shell holds the pipe open until the sleeper is killed, exactly
// the orphaning hazard the runner must defeat.
func grandchildScript(pidFile string) string {
	return "sleep 60 & echo $! > " + pidFile + "; wait"
}

// TestRun_CancelReapsGrandchild is witness (2): cancelling the caller context
// terminates the child AND its spawned grandchild.
func TestRun_CancelReapsGrandchild(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := shellCmd(ctx, grandchildScript(pidFile))
	done := make(chan Result, 1)
	go func() {
		res, _ := Run(ctx, cmd, Options{Combined: true})
		done <- res
	}()

	grandchild := waitForPIDFile(t, pidFile)
	if !processAlive(grandchild) {
		t.Fatalf("grandchild pid %d not alive before cancel", grandchild)
	}

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("Run did not return within 5s of cancel (Wait hung on inherited pipe)")
	}
	assertReaped(t, grandchild)
}

// TestRun_TimeoutReapsGrandchild is witness (3): a context deadline behaves like
// an explicit cancel, reaping the whole tree.
func TestRun_TimeoutReapsGrandchild(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	cmd := shellCmd(ctx, grandchildScript(pidFile))
	done := make(chan Result, 1)
	go func() {
		res, _ := Run(ctx, cmd, Options{Combined: true})
		done <- res
	}()

	grandchild := waitForPIDFile(t, pidFile)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("Run did not return within 5s of timeout")
	}
	if ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("ctx.Err() = %v, want DeadlineExceeded", ctx.Err())
	}
	assertReaped(t, grandchild)
}
