//go:build !windows

package eval

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunSuiteContextCancellationTerminatesCommandDescendants(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "grandchild.pid")
	suitePath := writeEvalSuite(t, dir, fmt.Sprintf(`{
  "schema_version": 1,
  "id": "command.cancel",
  "name": "Command cancellation",
  "domain": "cli",
  "visibility": "public_canary",
  "tier": "deterministic",
  "scoring": {
    "aggregate_threshold": 1,
    "dimensions": [
      {"name": "correctness", "weight": 1, "threshold": 1}
    ]
  },
  "baseline_policy": {"mode": "none"},
  "cases": [
    {
      "id": "cancel-tree",
      "title": "caller cancellation cleans descendants",
      "kind": "command",
      "objective": "Thread the eval caller context through the subprocess tree.",
      "runtime": "shell",
      "inputs": {
        "argv": ["/bin/sh", "-c", "sleep 30 & child=$!; printf '%%s' \"$child\" > \"$PID_FILE\"; wait \"$child\""],
        "env": {"PID_FILE": %q}
      },
      "expectations": [
        {"type": "exit_code", "value": 0}
      ]
    }
  ]
}`, pidFile))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := RunSuiteContext(ctx, RunOptions{SuitePath: suitePath, RunID: "cancel-run", Now: fixedEvalTime})
		done <- err
	}()

	pid := awaitEvalPID(t, pidFile)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("RunSuiteContext error = %v, want context.Canceled", err)
	}
	awaitEvalProcessGone(t, pid)
}

func awaitEvalPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if convErr != nil {
				t.Fatalf("parse pid %q: %v", data, convErr)
			}
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for eval pid file %s", path)
	return 0
}

func awaitEvalProcessGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("eval descendant pid %d survived caller cancellation", pid)
}
