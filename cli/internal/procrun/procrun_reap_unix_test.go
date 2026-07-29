//go:build !windows

package procrun

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

// TestRun_ReapsSleeperAfterNormalChildExit is the review-round-1 finding-1
// witness: the direct child backgrounds a sleeper and exits NORMALLY (ctx is
// never cancelled, so Cancel never fires). Only the unconditional post-Wait reap
// can kill the orphan; without it the sleeper survives past WaitDelay.
func TestRun_ReapsSleeperAfterNormalChildExit(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "sleeper.pid")
	ctx := context.Background()
	// Note: no `wait` — the shell exits immediately, leaving the sleeper holding
	// the stdout pipe until WaitDelay unblocks Wait.
	cmd := shellCmd(ctx, "sleep 60 & echo $! > "+pidFile)
	res, err := Run(ctx, cmd, Options{Combined: true, WaitDelay: 500 * time.Millisecond})
	if err != nil {
		t.Fatalf("Run start error: %v", err)
	}
	_ = res
	sleeper := waitForPIDFile(t, pidFile)
	assertReaped(t, sleeper)
}

// TestCancelHook_ESRCHMapsToProcessDone is the deterministic finding-3 witness:
// after the child and its group have fully exited, kill(-pgid) returns ESRCH,
// which the Cancel hook must report as ErrProcessDone (success), never as a
// failure that Wait would surface as "exec: canceling Cmd".
func TestCancelHook_ESRCHMapsToProcessDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := exec.CommandContext(ctx, "true")
	configureProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
	// The group is now empty; kill(-pgid) yields ESRCH.
	if err := cmd.Cancel(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("Cancel after exit = %v, want nil or ErrProcessDone", err)
	}
}

// TestRun_CompletedCommandNotReportedCancelledUnderRace is the finding-3 race
// witness: a fast command whose completion races an immediate cancel must never
// surface a spurious error when it actually completed. Run under -race.
func TestRun_CompletedCommandNotReportedCancelledUnderRace(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 50; i++ {
		marker := filepath.Join(dir, "done-"+strconv.Itoa(i))
		ctx, cancel := context.WithCancel(context.Background())
		cmd := shellCmd(ctx, "echo done > "+marker)
		go cancel() // race the child's completion against cancellation
		res, _ := Run(ctx, cmd, Options{Combined: true})
		// A marker on disk does not prove the shell finished exiting — the
		// kill can land between the redirect and the shell's own exit, and
		// that outcome (killed, error) is a legitimate result of this race.
		// The finding-3 regression is a MIXED state: a wait that observed a
		// clean exit surfacing a spurious cancel error, or a killed process
		// reported as clean. Assert the two outcomes stay unmixed.
		if res.Err == nil {
			if res.ExitCode != 0 {
				t.Fatalf("iter %d: no error but ExitCode = %d, want 0", i, res.ExitCode)
			}
			data, readErr := os.ReadFile(marker)
			if readErr != nil || strings.TrimSpace(string(data)) != "done" {
				t.Fatalf("iter %d: clean run without its work completed (marker read: %v)", i, readErr)
			}
		} else if res.ExitCode == 0 {
			t.Fatalf("iter %d: clean exit 0 surfaced spurious error %v", i, res.Err)
		}
		cancel()
	}
}
