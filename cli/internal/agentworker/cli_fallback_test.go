//go:build !windows

package agentworker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWorkerProcessGroupIsolation_StartsInOwnProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are POSIX-only")
	}
	worker := newShellFallbackWorker(t, `printf "%s %s" "$$" "$(ps -o pgid= -p $$)"`)
	session, err := worker.Start(context.Background(), StartRequest{
		WorkerKind: WorkerKindCodex,
		Provider:   ProviderCLIFallback,
		Prompt:     "process group proof",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	transcript, err := session.Transcript(context.Background())
	if err != nil {
		t.Fatalf("Transcript: %v", err)
	}
	fields := strings.Fields(transcript.Text)
	if len(fields) != 2 {
		t.Fatalf("transcript = %q, want pid and pgid", transcript.Text)
	}
	childPID, err := strconv.Atoi(fields[0])
	if err != nil {
		t.Fatalf("child pid %q: %v", fields[0], err)
	}
	childPGID, err := strconv.Atoi(fields[1])
	if err != nil {
		t.Fatalf("child pgid %q: %v", fields[1], err)
	}
	if childPGID != childPID {
		t.Fatalf("child pgid = %d, want its pid %d", childPGID, childPID)
	}
	if childPGID == syscall.Getpgrp() {
		t.Fatalf("child pgid = parent pgid %d, want isolated process group", childPGID)
	}
}

func TestWorkerProcessGroupIsolation_KillsHungProcessGroup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process group kill is POSIX-only")
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	script := `sleep 10 & echo $! > "$PIDFILE"; wait`
	worker, err := NewCLIFallbackWorker(CLIFallbackWorkerOptions{
		Command:          "/bin/sh",
		Args:             []string{"-c", script},
		Env:              []string{"PIDFILE=" + pidFile},
		WallClockTimeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewCLIFallbackWorker: %v", err)
	}
	started := time.Now()
	session, err := worker.Start(context.Background(), StartRequest{
		WorkerKind: WorkerKindCodex,
		Provider:   ProviderCLIFallback,
		Prompt:     "timeout proof",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if time.Since(started) > time.Second {
		t.Fatalf("worker timeout took too long")
	}
	terminal, err := session.TerminalState(context.Background())
	if err != nil {
		t.Fatalf("TerminalState: %v", err)
	}
	if terminal.Status != StatusFailed || !strings.Contains(terminal.Reason, "wall-clock") {
		t.Fatalf("terminal = %#v, want wall-clock failure", terminal)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read child pid: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse child pid: %v", err)
	}
	if processAliveEventually(childPID) {
		t.Fatalf("child process %d survived process-group cancellation", childPID)
	}
}

func TestCLIFallbackWorkerAppliesCgroupV2MemoryLimit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("cgroup v2 caps are Linux-only")
	}
	root := t.TempDir()
	worker, err := NewCLIFallbackWorker(CLIFallbackWorkerOptions{
		Command:          "/bin/sh",
		Args:             []string{"-c", "cat >/dev/null"},
		CgroupRoot:       root,
		MemoryMaxBytes:   123456,
		CgroupNamePrefix: "agentops-test",
	})
	if err != nil {
		t.Fatalf("NewCLIFallbackWorker: %v", err)
	}
	session, err := worker.Start(context.Background(), StartRequest{
		WorkerKind: WorkerKindCodex,
		Provider:   ProviderCLIFallback,
		JobID:      "job-cgroup",
		Prompt:     "cgroup proof",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	status := session.(*CLIFallbackSession).CgroupStatus()
	if !status.Applied || status.Path == "" {
		t.Fatalf("cgroup status = %#v, want applied", status)
	}
	data, err := os.ReadFile(filepath.Join(status.Path, "memory.max"))
	if err != nil {
		t.Fatalf("read memory.max: %v", err)
	}
	if strings.TrimSpace(string(data)) != "123456" {
		t.Fatalf("memory.max = %q, want 123456", strings.TrimSpace(string(data)))
	}
}

func newShellFallbackWorker(t *testing.T, script string) *CLIFallbackWorker {
	t.Helper()
	worker, err := NewCLIFallbackWorker(CLIFallbackWorkerOptions{
		Command:           "/bin/sh",
		Args:              []string{"-c", script},
		DisableCgroupCaps: true,
		WallClockTimeout:  time.Second,
		CgroupNamePrefix:  "agentops-test",
	})
	if err != nil {
		t.Fatalf("NewCLIFallbackWorker: %v", err)
	}
	return worker
}

func processAliveEventually(pid int) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
	return processAlive(pid)
}

// processAlive returns true only when pid refers to a process that has not
// yet exited. A zombie that is awaiting reaping by init counts as dead: the
// process group has been killed, the child is no longer running, and the only
// reason kill(pid, 0) still succeeds is that the kernel has not yet released
// the PID slot.
func processAlive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		// /proc may be unavailable (non-Linux POSIX); fall back to kill(0)
		// which already returned nil, so treat as alive.
		return true
	}
	// /proc/<pid>/stat is "<pid> (<comm>) <state> ...". comm can contain
	// spaces and parens, so locate the LAST ')' before reading state.
	text := string(data)
	rparen := strings.LastIndexByte(text, ')')
	if rparen < 0 || rparen+2 >= len(text) {
		return true
	}
	state := text[rparen+2]
	return state != 'Z'
}
