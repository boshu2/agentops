// Package subprocess runs bounded child processes with caller cancellation and
// process-tree cleanup.
package subprocess

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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
	return runWithCleanup(ctx, command, nil)
}

type cleanupProcessTree func(*exec.Cmd) error

type managedProcessTree interface {
	attach(*exec.Cmd) error
	terminate(*exec.Cmd) error
	observeTermination(*exec.Cmd) error
	close() error
}

type runDependencies struct {
	configureProcessTree func(*exec.Cmd, time.Duration) (managedProcessTree, error)
	newCommandIO         func(*exec.Cmd, Command) (*ownedCommandIO, error)
}

type cancellationWatcher struct {
	stop chan struct{}
	done chan error
}

var defaultRunDependencies = runDependencies{
	configureProcessTree: func(cmd *exec.Cmd, waitDelay time.Duration) (managedProcessTree, error) {
		return configureProcessTree(cmd, waitDelay)
	},
	newCommandIO: newOwnedCommandIO,
}

type startedCommand struct {
	cmd            *exec.Cmd
	tree           managedProcessTree
	ownedIO        *ownedCommandIO
	waitDelay      time.Duration
	ioLifecycleErr error
}

func startOwnedCommand(
	ctx context.Context,
	command Command,
	dependencies runDependencies,
	result *Result,
) (*startedCommand, error) {
	// AgentOps owns cancellation so an attach failure can return without
	// stranding os/exec's context watcher, which otherwise exits only through
	// Cmd.Wait.
	cmd := exec.CommandContext(context.WithoutCancel(ctx), command.Name, command.Args...)
	cmd.Dir = command.Dir
	if command.Env != nil {
		cmd.Env = command.Env
	}
	waitDelay := command.WaitDelay
	if waitDelay <= 0 {
		waitDelay = defaultWaitDelay
	}
	cmd.WaitDelay = waitDelay
	tree, err := dependencies.configureProcessTree(cmd, waitDelay)
	if err != nil {
		return nil, fmt.Errorf("configure process tree for %q: %w", command.Name, err)
	}

	ownedIO, err := dependencies.newCommandIO(cmd, command)
	if err != nil {
		closeErr := tree.close()
		if closeErr != nil {
			closeErr = fmt.Errorf("close process tree for %q after I/O setup failure: %w", command.Name, closeErr)
		}
		return nil, errors.Join(fmt.Errorf("configure process I/O for %q: %w", command.Name, err), closeErr)
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		ioCloseErr := ownedIO.closeBeforeStart()
		closeErr := tree.close()
		ownedIO.snapshot(result)
		if ioCloseErr != nil {
			ioCloseErr = fmt.Errorf("close process I/O for %q before canceled start: %w", command.Name, ioCloseErr)
		}
		if closeErr != nil {
			closeErr = fmt.Errorf("close process tree for %q before canceled start: %w", command.Name, closeErr)
		}
		return nil, errors.Join(fmt.Errorf("start %q: %w", command.Name, ctxErr), ioCloseErr, closeErr)
	}

	if err := cmd.Start(); err != nil {
		ioCloseErr := ownedIO.closeBeforeStart()
		closeErr := tree.close()
		ownedIO.snapshot(result)
		var startErr error
		if ctxErr := ctx.Err(); ctxErr != nil {
			startErr = fmt.Errorf("start %q: %w", command.Name, ctxErr)
		} else {
			startErr = fmt.Errorf("start %q: %w", command.Name, err)
		}
		if closeErr != nil {
			closeErr = fmt.Errorf("close process tree for %q after start failure: %w", command.Name, closeErr)
		}
		if ioCloseErr != nil {
			ioCloseErr = fmt.Errorf("close process I/O for %q after start failure: %w", command.Name, ioCloseErr)
		}
		return nil, errors.Join(startErr, ioCloseErr, closeErr)
	}
	return &startedCommand{
		cmd:            cmd,
		tree:           tree,
		ownedIO:        ownedIO,
		waitDelay:      waitDelay,
		ioLifecycleErr: ownedIO.afterStart(),
	}, nil
}

func startCancellationWatcher(ctx context.Context, terminate func() error) *cancellationWatcher {
	if ctx.Done() == nil {
		return nil
	}
	watcher := &cancellationWatcher{
		stop: make(chan struct{}),
		done: make(chan error, 1),
	}
	go func() {
		select {
		case <-ctx.Done():
			watcher.done <- terminate()
		case <-watcher.stop:
			watcher.done <- nil
		}
	}()
	return watcher
}

func (watcher *cancellationWatcher) stopAndWait() error {
	if watcher == nil {
		return nil
	}
	close(watcher.stop)
	return <-watcher.done
}

func runWithCleanup(ctx context.Context, command Command, cleanup cleanupProcessTree) (Result, error) {
	return runWithDependencies(ctx, command, cleanup, defaultRunDependencies)
}

func runWithDependencies(
	ctx context.Context,
	command Command,
	cleanup cleanupProcessTree,
	dependencies runDependencies,
) (Result, error) {
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
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, fmt.Errorf("start %q: %w", command.Name, ctxErr)
	}

	started, err := startOwnedCommand(ctx, command, dependencies, &result)
	if err != nil {
		return result, err
	}
	cmd := started.cmd
	tree := started.tree
	ownedIO := started.ownedIO
	waitDelay := started.waitDelay
	pid := cmd.Process.Pid
	attachErr := tree.attach(cmd)
	var watcher *cancellationWatcher
	if attachErr == nil {
		watcher = startCancellationWatcher(ctx, func() error { return tree.terminate(cmd) })
		ownedIO.startStdinCopy()
	}
	if command.OnStart != nil {
		command.OnStart(pid)
	}
	var waitErr error
	ioAborted := false
	if attachErr != nil {
		attachErr = fmt.Errorf("attach %q to process tree: %w", command.Name, attachErr)
		var waited bool
		waitErr, attachErr, waited = handleAttachFailure(
			attachErr,
			cmd.Process.Kill,
			func() error { return tree.observeTermination(cmd) },
			cmd.Wait,
			func() error { return ownedIO.abortAfterUnprovenTermination(waitDelay) },
			cmd.Process.Release,
		)
		ioAborted = !waited
	} else {
		waitErr = cmd.Wait()
	}
	watcherErr := watcher.stopAndWait()

	cleanupErr := finishProcessTree(tree, cmd, cleanup, errors.Join(attachErr, started.ioLifecycleErr, watcherErr))
	if !ioAborted {
		cleanupErr = errors.Join(cleanupErr, ownedIO.finishAfterWait(waitDelay))
	}
	result.Cleanup = cleanupOutcome(cleanupErr)
	if command.OnExit != nil {
		command.OnExit(pid)
	}
	ownedIO.snapshot(&result)
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
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

func handleAttachFailure(
	attachErr error,
	kill func() error,
	observeTermination func() error,
	wait func() error,
	abortIO func() error,
	release func() error,
) (waitErr error, cleanupErr error, waited bool) {
	cleanupErr = attachErr
	killErr := kill()
	if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("kill after process-tree attach failure: %w", killErr))
	}
	observationErr := observeTermination()
	if observationErr == nil {
		return wait(), cleanupErr, true
	}
	cleanupErr = errors.Join(cleanupErr, fmt.Errorf("observe process termination after attach failure: %w", observationErr))
	if ioErr := abortIO(); ioErr != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("close process I/O after unproven termination: %w", ioErr))
	}
	if releaseErr := release(); releaseErr != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("release process after unproven termination: %w", releaseErr))
	}
	return nil, cleanupErr, false
}

func finishProcessTree(tree managedProcessTree, cmd *exec.Cmd, cleanup cleanupProcessTree, attachErr error) error {
	var cleanupErr error
	if cleanup == nil {
		cleanupErr = tree.terminate(cmd)
	} else {
		cleanupErr = cleanup(cmd)
	}
	cleanupErr = errors.Join(attachErr, cleanupErr)
	if closeErr := tree.close(); closeErr != nil {
		cleanupErr = errors.Join(cleanupErr, fmt.Errorf("close process tree: %w", closeErr))
	}
	return cleanupErr
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
