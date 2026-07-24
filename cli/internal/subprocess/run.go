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
)

const defaultWaitDelay = 500 * time.Millisecond

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

// Result captures bounded subprocess output and the observed exit status.
type Result struct {
	Stdout   Output
	Stderr   Output
	Combined Output
	ExitCode int
}

// Run executes command under ctx. Output is bounded while it is streamed,
// cancellation terminates the process tree, and WaitDelay prevents a
// descendant holding inherited pipes from blocking completion indefinitely.
func Run(ctx context.Context, command Command) (Result, error) {
	result := Result{ExitCode: -1}
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
	cleanupErr := terminateProcessTree(cmd)
	if command.OnExit != nil {
		command.OnExit(pid)
	}
	snapshotResult(&result, stdoutCapture, stderrCapture, combinedCapture)
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, fmt.Errorf("run %q: %w", command.Name, ctxErr)
	}
	// A successful parent can leave a background descendant holding an
	// inherited pipe. WaitDelay closes that pipe; the unconditional tree
	// cleanup above then kills the descendant. The parent's exit status remains
	// authoritative once cleanup has completed.
	if errors.Is(waitErr, exec.ErrWaitDelay) && result.ExitCode == 0 {
		waitErr = nil
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if result.ExitCode != 0 && !errors.As(waitErr, &exitErr) && cmd.ProcessState != nil {
			waitErr = errors.Join(&exec.ExitError{ProcessState: cmd.ProcessState}, waitErr)
		}
		return result, waitErr
	}
	if cleanupErr != nil {
		return result, fmt.Errorf("clean process tree for %q: %w", command.Name, cleanupErr)
	}
	return result, nil
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
