// Package subprocess runs bounded child processes with caller cancellation and
// process-tree cleanup.
package subprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
	"unicode/utf8"
)

const (
	defaultWaitDelay        = 500 * time.Millisecond
	maxCleanupDiagnosticLen = 2048
	cleanupTruncationMarker = "…[truncated]"
)

// Command describes one child process and its capture policy.
type Command struct {
	Name string
	Args []string
	Dir  string
	Env  []string

	Stdin io.Reader

	CombinedOutput bool
	OutputLimit    CaptureLimit
	StdoutLimit    CaptureLimit
	StderrLimit    CaptureLimit
	WaitDelay      time.Duration

	// OnStart and OnExit support callers that maintain a signal-time child
	// registry. They are invoked exactly once around a successfully started
	// process.
	OnStart func(pid int)
	OnExit  func(pid int)
}

// CleanupStatus identifies the terminal state of process-tree cleanup.
type CleanupStatus string

const (
	CleanupNotStarted CleanupStatus = "not_started"
	CleanupCompleted  CleanupStatus = "completed"
	CleanupFailed     CleanupStatus = "failed"
)

// CleanupOutcome records whether process-tree cleanup ran and completed.
// Error is bounded so callers can safely serialize Result.
type CleanupOutcome struct {
	Status    CleanupStatus `json:"status"`
	Attempted bool          `json:"attempted"`
	Completed bool          `json:"completed"`
	Error     string        `json:"error,omitempty"`
}

// Failed reports whether cleanup was attempted but did not complete.
func (outcome CleanupOutcome) Failed() bool {
	return outcome.Status == CleanupFailed
}

// Result captures bounded subprocess output, the observed exit status, and
// process-tree cleanup state.
type Result struct {
	Stdout   Output
	Stderr   Output
	Combined Output
	ExitCode int
	Cleanup  CleanupOutcome
}

// Run executes command under ctx. Output is bounded while it is streamed,
// cancellation terminates the process tree, and WaitDelay prevents a
// descendant holding inherited pipes from blocking completion indefinitely.
func Run(ctx context.Context, command Command) (Result, error) {
	return runWithCleanup(ctx, command, terminateProcessTree)
}

type cleanupProcessTree func(*exec.Cmd) error

func runWithCleanup(ctx context.Context, command Command, cleanup cleanupProcessTree) (Result, error) {
	result := Result{
		ExitCode: -1,
		Cleanup: CleanupOutcome{
			Status: CleanupNotStarted,
		},
	}
	if command.Name == "" {
		return result, fmt.Errorf("subprocess command name is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cmd := exec.CommandContext(ctx, command.Name, command.Args...)
	cmd.Dir = command.Dir
	if command.Env != nil {
		cmd.Env = command.Env
	}
	cmd.Stdin = command.Stdin
	waitDelay := command.WaitDelay
	if waitDelay <= 0 {
		waitDelay = defaultWaitDelay
	}
	cmd.WaitDelay = waitDelay
	configureProcessTree(cmd)

	var stdoutCapture, stderrCapture, combinedCapture *capture
	if command.CombinedOutput {
		combinedCapture = newCapture(command.OutputLimit)
		cmd.Stdout = combinedCapture
		cmd.Stderr = combinedCapture
	} else {
		stdoutCapture = newCapture(command.StdoutLimit)
		stderrCapture = newCapture(command.StderrLimit)
		cmd.Stdout = stdoutCapture
		cmd.Stderr = stderrCapture
	}

	if err := cmd.Start(); err != nil {
		snapshotResult(&result, stdoutCapture, stderrCapture, combinedCapture)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return result, fmt.Errorf("start %q: %w", command.Name, ctxErr)
		}
		return result, fmt.Errorf("start %q: %w", command.Name, err)
	}
	pid := cmd.Process.Pid
	if command.OnStart != nil {
		command.OnStart(pid)
	}

	waitErr := cmd.Wait()
	cleanupErr := cleanup(cmd)
	result.Cleanup = cleanupOutcome(cleanupErr)
	if command.OnExit != nil {
		command.OnExit(pid)
	}
	snapshotResult(&result, stdoutCapture, stderrCapture, combinedCapture)
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	// A successful parent can leave a background descendant holding an
	// inherited pipe. WaitDelay closes that pipe; the unconditional tree
	// cleanup above then kills the descendant. The parent's exit status remains
	// authoritative once cleanup has completed.
	if errors.Is(waitErr, exec.ErrWaitDelay) && result.ExitCode == 0 {
		waitErr = nil
	}
	var primaryErr error
	if ctxErr := ctx.Err(); ctxErr != nil {
		primaryErr = fmt.Errorf("run %q: %w", command.Name, ctxErr)
	} else if waitErr != nil {
		var exitErr *exec.ExitError
		if result.ExitCode != 0 && !errors.As(waitErr, &exitErr) && cmd.ProcessState != nil {
			waitErr = errors.Join(&exec.ExitError{ProcessState: cmd.ProcessState}, waitErr)
		}
		primaryErr = waitErr
	}
	if cleanupErr != nil {
		cleanupErr = fmt.Errorf("clean process tree for %q: %w", command.Name, cleanupErr)
	}
	return result, errors.Join(primaryErr, cleanupErr)
}

func cleanupOutcome(err error) CleanupOutcome {
	if err == nil {
		return CleanupOutcome{
			Status:    CleanupCompleted,
			Attempted: true,
			Completed: true,
		}
	}
	return CleanupOutcome{
		Status:    CleanupFailed,
		Attempted: true,
		Completed: false,
		Error:     boundedCleanupDiagnostic(err.Error()),
	}
}

func boundedCleanupDiagnostic(message string) string {
	if len(message) <= maxCleanupDiagnosticLen {
		return message
	}
	limit := maxCleanupDiagnosticLen - len(cleanupTruncationMarker)
	for limit > 0 && !utf8.ValidString(message[:limit]) {
		limit--
	}
	return message[:limit] + cleanupTruncationMarker
}

func snapshotResult(result *Result, stdoutCapture, stderrCapture, combinedCapture *capture) {
	if stdoutCapture != nil {
		result.Stdout = stdoutCapture.snapshot()
	}
	if stderrCapture != nil {
		result.Stderr = stderrCapture.snapshot()
	}
	if combinedCapture != nil {
		result.Combined = combinedCapture.snapshot()
	}
}
