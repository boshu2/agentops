//go:build !windows

package subprocess

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

func TestRunCancellationTerminatesDescendants(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	ctx, cancel := context.WithCancel(context.Background())
	command := Command{
		Name: "/bin/sh",
		Args: []string{"-c", `sleep 30 & child=$!; printf '%s' "$child" > "$PID_FILE"; wait "$child"`},
		Env:  append(os.Environ(), "PID_FILE="+pidFile),
	}

	type runOutcome struct {
		result Result
		err    error
	}
	done := make(chan runOutcome, 1)
	go func() {
		result, err := Run(ctx, command)
		done <- runOutcome{result: result, err: err}
	}()
	pid := awaitPID(t, pidFile)
	cancel()
	outcome := <-done
	if !errors.Is(outcome.err, context.Canceled) {
		t.Fatalf("Run error = %v, want context.Canceled", outcome.err)
	}
	assertCleanupCompleted(t, outcome.result.Cleanup)
	awaitProcessGone(t, pid)
}

func TestRunTimeoutTerminatesDescendants(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	command := Command{
		Name: "/bin/sh",
		Args: []string{"-c", `sleep 30 & child=$!; printf '%s' "$child" > "$PID_FILE"; wait "$child"`},
		Env:  append(os.Environ(), "PID_FILE="+pidFile),
	}

	result, err := Run(ctx, command)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want context deadline", err)
	}
	assertCleanupCompleted(t, result.Cleanup)
	pid := awaitPID(t, pidFile)
	awaitProcessGone(t, pid)
}

func TestRunAbnormalParentDoesNotWaitForOrphanedPipe(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	command := Command{
		Name:           "/bin/sh",
		Args:           []string{"-c", `sleep 30 & child=$!; printf '%s' "$child" > "$PID_FILE"; echo parent-exit; exit 7`},
		Env:            append(os.Environ(), "PID_FILE="+pidFile),
		CombinedOutput: true,
		OutputLimit:    CaptureLimit{HeadBytes: 128, TailBytes: 128},
		WaitDelay:      100 * time.Millisecond,
	}

	start := time.Now()
	result, err := Run(context.Background(), command)
	if time.Since(start) > 2*time.Second {
		t.Fatalf("Run waited %s for orphaned pipe holder", time.Since(start))
	}
	if result.ExitCode != 7 || err == nil {
		t.Fatalf("result/error = %#v / %v, want abnormal exit 7", result, err)
	}
	assertCleanupCompleted(t, result.Cleanup)
	pid := awaitPID(t, pidFile)
	awaitProcessGone(t, pid)
}

func TestRunSuccessfulParentTerminatesBackgroundDescendant(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	command := Command{
		Name:           "/bin/sh",
		Args:           []string{"-c", `sleep 30 & child=$!; printf '%s' "$child" > "$PID_FILE"; echo parent-success; exit 0`},
		Env:            append(os.Environ(), "PID_FILE="+pidFile),
		CombinedOutput: true,
		OutputLimit:    CaptureLimit{HeadBytes: 128, TailBytes: 128},
		WaitDelay:      100 * time.Millisecond,
	}

	start := time.Now()
	result, err := Run(context.Background(), command)
	if err != nil {
		t.Fatalf("Run successful parent: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("Run waited %s for successful parent's background descendant", time.Since(start))
	}
	if result.ExitCode != 0 || !strings.Contains(result.Combined.String(), "parent-success") {
		t.Fatalf("result = %#v, want successful parent output", result)
	}
	assertCleanupCompleted(t, result.Cleanup)
	pid := awaitPID(t, pidFile)
	assertProcessGoneNow(t, pid)
}

func TestRunPreservesWaitDelayWhenBackgroundCleanupFails(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "grandchild.pid")
	command := Command{
		Name:           "/bin/sh",
		Args:           []string{"-c", `sleep 30 & child=$!; printf '%s' "$child" > "$PID_FILE"; echo parent-success; exit 0`},
		Env:            append(os.Environ(), "PID_FILE="+pidFile),
		CombinedOutput: true,
		OutputLimit:    CaptureLimit{HeadBytes: 128, TailBytes: 128},
		WaitDelay:      100 * time.Millisecond,
	}
	cleanupErr := errors.New("injected cleanup failure")

	result, err := runWithCleanup(context.Background(), command, func(*exec.Cmd) error {
		return cleanupErr
	})
	pid := awaitPID(t, pidFile)
	t.Cleanup(func() {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	})
	if !errors.Is(err, exec.ErrWaitDelay) || !errors.Is(err, cleanupErr) {
		t.Fatalf("Run error = %v, want wait-delay and cleanup identities", err)
	}
	if result.Cleanup.Status != CleanupFailed {
		t.Fatalf("cleanup = %#v, want failed outcome", result.Cleanup)
	}

	_ = syscall.Kill(pid, syscall.SIGKILL)
	awaitProcessGone(t, pid)
}

func awaitPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			value := strings.TrimSpace(string(data))
			if value == "" {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			pid, convErr := strconv.Atoi(value)
			if convErr != nil {
				t.Fatalf("parse pid %q: %v", data, convErr)
			}
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for pid file %s", path)
	return 0
}

func awaitProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("descendant pid %d is still alive after process-tree cleanup", pid)
}

func assertProcessGoneNow(t *testing.T, pid int) {
	t.Helper()
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("Run reported cleanup completed while descendant pid %d remained observable: %v", pid, err)
	}
}

func TestTerminateUnixProcessGroupPollsUntilAbsent(t *testing.T) {
	now := time.Unix(0, 0)
	probes := 0
	ops := unixProcessGroupOps{
		signal: func(int, syscall.Signal) error { return nil },
		probe: func(int) error {
			probes++
			if probes < 3 {
				return nil
			}
			return syscall.ESRCH
		},
		clock: unixClock{
			now: func() time.Time { return now },
			sleep: func(delay time.Duration) {
				now = now.Add(delay)
			},
		},
	}

	if err := terminateUnixProcessGroup(42, 100*time.Millisecond, ops); err != nil {
		t.Fatalf("terminateUnixProcessGroup: %v", err)
	}
	if probes != 3 {
		t.Fatalf("probes = %d, want observation through ESRCH", probes)
	}
}

func TestTerminateUnixProcessGroupFailsWhenAbsenceCannotBeProven(t *testing.T) {
	now := time.Unix(0, 0)
	ops := unixProcessGroupOps{
		signal: func(int, syscall.Signal) error { return nil },
		probe:  func(int) error { return nil },
		clock: unixClock{
			now: func() time.Time { return now },
			sleep: func(delay time.Duration) {
				now = now.Add(delay)
			},
		},
	}

	if err := terminateUnixProcessGroup(42, 25*time.Millisecond, ops); err == nil {
		t.Fatal("terminateUnixProcessGroup reported completion without proving absence")
	}
}

func TestTerminateUnixProcessGroupPollsFromEPERMToESRCH(t *testing.T) {
	now := time.Unix(0, 0)
	probes := 0
	ops := unixProcessGroupOps{
		signal: func(int, syscall.Signal) error { return nil },
		probe: func(int) error {
			probes++
			if probes < 3 {
				return syscall.EPERM
			}
			return syscall.ESRCH
		},
		clock: unixClock{
			now: func() time.Time { return now },
			sleep: func(delay time.Duration) {
				now = now.Add(delay)
			},
		},
	}

	if err := terminateUnixProcessGroup(42, 50*time.Millisecond, ops); err != nil {
		t.Fatalf("terminateUnixProcessGroup: %v", err)
	}
	if probes != 3 {
		t.Fatalf("probes = %d, want EPERM polling through ESRCH", probes)
	}
}

func TestTerminateUnixProcessGroupAcceptsSignalEPERMOnlyAfterAbsenceProbe(t *testing.T) {
	probes := 0
	ops := unixProcessGroupOps{
		signal: func(int, syscall.Signal) error { return syscall.EPERM },
		probe: func(int) error {
			probes++
			return syscall.ESRCH
		},
		clock: unixClock{
			now:   time.Now,
			sleep: func(time.Duration) {},
		},
	}

	if err := terminateUnixProcessGroup(42, time.Millisecond, ops); err != nil {
		t.Fatalf("terminateUnixProcessGroup: %v", err)
	}
	if probes != 1 {
		t.Fatalf("probes = %d, want one absence proof after signal EPERM", probes)
	}
}

func TestTerminateUnixProcessGroupSignalEPERMDoesNotHideLiveGroup(t *testing.T) {
	now := time.Unix(0, 0)
	ops := unixProcessGroupOps{
		signal: func(int, syscall.Signal) error { return syscall.EPERM },
		probe:  func(int) error { return nil },
		clock: unixClock{
			now: func() time.Time { return now },
			sleep: func(delay time.Duration) {
				now = now.Add(delay)
			},
		},
	}

	err := terminateUnixProcessGroup(42, 25*time.Millisecond, ops)
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("error = %v, want signal EPERM retained for live group", err)
	}
	if !strings.Contains(err.Error(), "remained observable") {
		t.Fatalf("error = %q, want live-group timeout diagnostic", err)
	}
}

func TestTerminateUnixProcessGroupRetainsEPERMAtTimeout(t *testing.T) {
	now := time.Unix(0, 0)
	ops := unixProcessGroupOps{
		signal: func(int, syscall.Signal) error { return nil },
		probe:  func(int) error { return syscall.EPERM },
		clock: unixClock{
			now: func() time.Time { return now },
			sleep: func(delay time.Duration) {
				now = now.Add(delay)
			},
		},
	}

	err := terminateUnixProcessGroup(42, 25*time.Millisecond, ops)
	if !errors.Is(err, syscall.EPERM) {
		t.Fatalf("error = %v, want last EPERM identity at timeout", err)
	}
	if !strings.Contains(err.Error(), "remained observable") {
		t.Fatalf("error = %q, want timeout diagnostic", err)
	}
}

func TestUnixProcessGroupEPERMStateIsStableAcrossCallOrder(t *testing.T) {
	now := time.Unix(0, 0)
	timeoutOps := unixProcessGroupOps{
		signal: func(int, syscall.Signal) error { return nil },
		probe:  func(int) error { return syscall.EPERM },
		clock: unixClock{
			now: func() time.Time { return now },
			sleep: func(delay time.Duration) {
				now = now.Add(delay)
			},
		},
	}
	if err := terminateUnixProcessGroup(42, time.Millisecond, timeoutOps); !errors.Is(err, syscall.EPERM) {
		t.Fatalf("first call error = %v, want EPERM timeout", err)
	}

	successOps := unixProcessGroupOps{
		signal: func(int, syscall.Signal) error { return nil },
		probe:  func(int) error { return syscall.ESRCH },
		clock: unixClock{
			now:   time.Now,
			sleep: func(time.Duration) {},
		},
	}
	if err := terminateUnixProcessGroup(43, time.Millisecond, successOps); err != nil {
		t.Fatalf("second call inherited prior EPERM state: %v", err)
	}
}
